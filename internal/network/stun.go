package network

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"log/slog"
	"net"
	"strconv"
	"strings"
	"time"
)

const StunMagicCookie uint32 = 0x2112A442

const (
	StunAttrMappedAddress    uint16 = 0x0001
	StunAttrXorMappedAddress uint16 = 0x0020
	StunAttrResponseOrigin   uint16 = 0x802B
	StunAttrOtherAddress     uint16 = 0x802C
)

const (
	StunBindingRequest uint16 = 0x0001
	StunBindingSuccess uint16 = 0x0101
	StunBindingError   uint16 = 0x0111
)

type NATType int

const (
	NATUnknown NATType = iota
	NATOpen
	NATFullCone
	NATRestricted
	NATPortRestricted
	NATSymmetric
)

const (
	NATNone           = NATOpen
	NATRestrictedCone = NATRestricted
)

func (n NATType) String() string {
	switch n {
	case NATOpen:
		return "Open Internet"
	case NATFullCone:
		return "Full Cone NAT"
	case NATRestricted:
		return "Restricted Cone NAT"
	case NATPortRestricted:
		return "Port Restricted NAT"
	case NATSymmetric:
		return "Symmetric NAT"
	default:
		return "Unknown"
	}
}

type STUNClient struct {
	serverAddr string
	timeout    time.Duration
}

func NewSTUNClient(serverAddr string) *STUNClient {
	return &STUNClient{
		serverAddr: serverAddr,
		timeout:    3 * time.Second,
	}
}

func (c *STUNClient) DetectNATType() (NATType, string, error) {
	host, portStr, err := net.SplitHostPort(c.serverAddr)
	if err != nil {
		return NATUnknown, "", fmt.Errorf("parse server address: %w", err)
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		return NATUnknown, "", fmt.Errorf("parse server port: %w", err)
	}

	primaryAddr, err := c.GetMappedAddress()
	if err != nil {
		slog.Warn("primary STUN binding failed, assuming symmetric NAT",
			"error", err,
		)
		return NATSymmetric, "", fmt.Errorf("STUN server not available - using default NAT type: %w", err)
	}

	altAddr := net.JoinHostPort(host, strconv.Itoa(port+1))
	c.serverAddr = altAddr
	secondaryAddr, err := c.GetMappedAddress()
	c.serverAddr = net.JoinHostPort(host, portStr)

	if err != nil {
		slog.Debug("secondary STUN binding failed, determining from single response",
			"error", err,
		)
		return c.detectFromSingleResponse(primaryAddr)
	}

	if primaryAddr == secondaryAddr {
		return c.detectOpenOrFullCone(primaryAddr)
	}

	return NATSymmetric, primaryAddr, nil
}

func (c *STUNClient) GetMappedAddress() (string, error) {
	conn, err := net.DialTimeout("udp", c.serverAddr, c.timeout)
	if err != nil {
		return "", fmt.Errorf("connect to STUN server %s: %w", c.serverAddr, err)
	}
	defer conn.Close()

	msg := buildBindingRequest()
	if _, err := conn.Write(msg); err != nil {
		return "", fmt.Errorf("send binding request: %w", err)
	}

	if err := conn.SetReadDeadline(time.Now().Add(c.timeout)); err != nil {
		return "", fmt.Errorf("set read deadline: %w", err)
	}

	resp := make([]byte, 2048)
	n, err := conn.Read(resp)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	return parseXorMappedAddress(resp[:n])
}

func (c *STUNClient) detectFromSingleResponse(mappedAddr string) (NATType, string, error) {
	host, _, err := net.SplitHostPort(mappedAddr)
	if err != nil {
		return NATUnknown, mappedAddr, err
	}

	if isLocalAddress(host) {
		return NATOpen, mappedAddr, nil
	}

	return NATFullCone, mappedAddr, nil
}

func (c *STUNClient) detectOpenOrFullCone(mappedAddr string) (NATType, string, error) {
	host, _, err := net.SplitHostPort(mappedAddr)
	if err != nil {
		return NATUnknown, mappedAddr, err
	}

	if isLocalAddress(host) {
		return NATOpen, mappedAddr, nil
	}

	return NATFullCone, mappedAddr, nil
}

func buildBindingRequest() []byte {
	msg := make([]byte, 20)
	binary.BigEndian.PutUint16(msg[0:2], StunBindingRequest)
	binary.BigEndian.PutUint16(msg[2:4], 0)
	binary.BigEndian.PutUint32(msg[4:8], StunMagicCookie)

	txID := make([]byte, 12)
	if _, err := rand.Read(txID); err != nil {
		now := time.Now().UnixNano()
		binary.BigEndian.PutUint64(txID[0:8], uint64(now))
		binary.BigEndian.PutUint32(txID[8:12], uint32(now>>32))
	}
	copy(msg[8:20], txID)

	return msg
}

func parseXorMappedAddress(data []byte) (string, error) {
	if len(data) < 20 {
		return "", fmt.Errorf("response too short: %d bytes", len(data))
	}

	msgType := binary.BigEndian.Uint16(data[0:2])
	if msgType != StunBindingSuccess {
		return "", fmt.Errorf("unexpected message type: 0x%04x", msgType)
	}

	magic := binary.BigEndian.Uint32(data[4:8])
	if magic != StunMagicCookie {
		return "", fmt.Errorf("invalid magic cookie: 0x%08x", magic)
	}

	msgLen := int(binary.BigEndian.Uint16(data[2:4]))
	if len(data) < 20+msgLen {
		return "", fmt.Errorf("truncated message: have %d, need %d", len(data), 20+msgLen)
	}

	return getMappedFromResponse(data)
}

func getMappedFromResponse(resp []byte) (string, error) {
	msgLen := int(binary.BigEndian.Uint16(resp[2:4]))
	if len(resp) < 20+msgLen {
		return "", fmt.Errorf("truncated message: have %d, need %d", len(resp), 20+msgLen)
	}

	attrs := resp[20 : 20+msgLen]
	pos := 0

	for pos+4 <= len(attrs) {
		attrType := binary.BigEndian.Uint16(attrs[pos : pos+2])
		attrLen := int(binary.BigEndian.Uint16(attrs[pos+2 : pos+4]))
		pos += 4

		if pos+attrLen > len(attrs) {
			break
		}

		if attrType == StunAttrXorMappedAddress || attrType == StunAttrMappedAddress {
			addrData := attrs[pos : pos+attrLen]
			addr, err := decodeAddressAttribute(addrData, attrType, resp[4:8])
			if err != nil {
				return "", err
			}
			return addr, nil
		}

		padded := attrLen
		if padded%4 != 0 {
			padded += 4 - (padded % 4)
		}
		pos += padded
	}

	return "", fmt.Errorf("no MAPPED-ADDRESS or XOR-MAPPED-ADDRESS in response")
}

func decodeAddressAttribute(attrData []byte, attrType uint16, magicCookie []byte) (string, error) {
	if len(attrData) < 5 {
		return "", fmt.Errorf("address attribute too short: %d bytes", len(attrData))
	}

	family := attrData[1]
	xport := binary.BigEndian.Uint16(attrData[2:4])

	var xportRaw uint16
	if attrType == StunAttrXorMappedAddress {
		xportRaw = xport ^ binary.BigEndian.Uint16(magicCookie[0:2])
	} else {
		xportRaw = xport
	}

	if family == 0x01 && len(attrData) >= 8 {
		var ip net.IP
		if attrType == StunAttrXorMappedAddress {
			ip = make(net.IP, 4)
			for i := 0; i < 4; i++ {
				ip[i] = attrData[4+i] ^ magicCookie[i]
			}
		} else {
			ip = net.IP(attrData[4:8])
		}
		return net.JoinHostPort(ip.String(), strconv.Itoa(int(xportRaw))), nil
	}

	if family == 0x02 && len(attrData) >= 20 {
		var ip net.IP
		if attrType == StunAttrXorMappedAddress {
			xorKey := make([]byte, 16)
			copy(xorKey[0:4], magicCookie)
			copy(xorKey[4:16], make([]byte, 12))
			ip = make(net.IP, 16)
			for i := 0; i < 16; i++ {
				ip[i] = attrData[4+i] ^ xorKey[i]
			}
		} else {
			ip = net.IP(attrData[4:20])
		}
		return net.JoinHostPort(ip.String(), strconv.Itoa(int(xportRaw))), nil
	}

	return "", fmt.Errorf("unsupported address family: 0x%02x", family)
}

func isLocalAddress(host string) bool {
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}

	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() {
		return true
	}

	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return false
	}

	for _, addr := range addrs {
		if ipNet, ok := addr.(*net.IPNet); ok {
			if ipNet.IP.Equal(ip) {
				return true
			}
		}
	}

	return false
}

func parseAddrPort(s string) (string, int, error) {
	parts := strings.Split(s, ":")
	if len(parts) != 2 {
		return "", 0, fmt.Errorf("invalid address: %s", s)
	}
	port, err := strconv.Atoi(parts[1])
	if err != nil {
		return "", 0, fmt.Errorf("invalid port: %s", parts[1])
	}
	return parts[0], port, nil
}
