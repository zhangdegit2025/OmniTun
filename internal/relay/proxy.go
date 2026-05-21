package relay

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/websocket"
	"github.com/klauspost/compress/zstd"
	"github.com/omnitun/omnitun/internal/protocol"
	"github.com/omnitun/omnitun/pkg/tracing"
)

var wsUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type ReverseProxy struct {
	dispatcher    *Dispatcher
	streamMux     *StreamMultiplexer
	trafficLogger *TrafficLogger
}

func NewReverseProxy(d *Dispatcher, sm *StreamMultiplexer) *ReverseProxy {
	return &ReverseProxy{
		dispatcher: d,
		streamMux:  sm,
	}
}

func (p *ReverseProxy) SetTrafficLogger(l *TrafficLogger) {
	p.trafficLogger = l
}

func (p *ReverseProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx, span := tracing.StartSpan(r.Context(), "relay.proxy")
	defer func() {
		if span != nil {
			span.End(ctx)
		}
	}()
	r = r.WithContext(ctx)

	start := time.Now()

	host := r.Host
	if h := r.Header.Get("X-Forwarded-Host"); h != "" {
		host = h
	}

	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}

	slog.Debug("proxy request",
		"method", r.Method,
		"host", host,
		"path", r.URL.Path,
	)

	tunnelCtx, ok := p.dispatcher.Lookup(host)
	if !ok {
		slug := extractSlugFromPath(r.URL.Path)
		if slug != "" {
			tunnelCtx, ok = p.dispatcher.LookupBySlug(slug)
		}
	}

	if !ok {
		slog.Warn("tunnel not found",
			"host", host,
			"method", r.Method,
			"path", r.URL.Path,
		)
		recordProxyError("tunnel_not_found")
		http.Error(w, "502 Bad Gateway: tunnel not found", http.StatusBadGateway)
		return
	}

	if websocket.IsWebSocketUpgrade(r) {
		p.handleWebSocket(w, r, tunnelCtx)
		return
	}

	p.handleHTTP(w, r, tunnelCtx, start)
}

func (p *ReverseProxy) handleHTTP(w http.ResponseWriter, r *http.Request, tunnelCtx *TunnelContext, start time.Time) {
	ctx, span := tracing.StartSpan(r.Context(), "relay.http.forward")
	defer func() {
		if span != nil {
			span.End(ctx)
		}
	}()
	_ = ctx

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		slog.Error("failed to read request body", "error", err)
		recordProxyError("read_body")
		p.logHTTPErrorTraffic(tunnelCtx, r, nil, http.StatusInternalServerError, "read_body", start)
		http.Error(w, "500 Internal Server Error", http.StatusInternalServerError)
		return
	}
	r.Body.Close()

	requestPayload, err := serializeHTTPRequest(r, bodyBytes)
	if err != nil {
		slog.Error("failed to serialize request", "error", err)
		recordProxyError("serialize_request")
		p.logHTTPErrorTraffic(tunnelCtx, r, bodyBytes, http.StatusInternalServerError, "serialize_request", start)
		http.Error(w, "500 Internal Server Error", http.StatusInternalServerError)
		return
	}

	compressedPayload, err := CompressPayload(requestPayload)
	if err != nil {
		slog.Error("failed to compress payload", "error", err)
		recordProxyError("compress")
		p.logHTTPErrorTraffic(tunnelCtx, r, bodyBytes, http.StatusInternalServerError, "compress", start)
		http.Error(w, "500 Internal Server Error", http.StatusInternalServerError)
		return
	}

	frame := protocol.NewDataFrame(tunnelCtx.StreamID, compressedPayload, true)

	if err := p.streamMux.ForwardFrame(tunnelCtx.StreamID, frame); err != nil {
		slog.Error("failed to forward frame to agent",
			"tunnel_id", tunnelCtx.TunnelID,
			"stream_id", tunnelCtx.StreamID,
			"error", err,
		)
		recordProxyError("forward_frame")
		p.logHTTPErrorTraffic(tunnelCtx, r, bodyBytes, http.StatusBadGateway, "agent_unreachable", start)
		http.Error(w, "502 Bad Gateway: agent unreachable", http.StatusBadGateway)
		return
	}

	recordBytesForwarded(float64(len(compressedPayload)))

	respFrame, err := p.streamMux.Receive(tunnelCtx.StreamID)
	if err != nil {
		slog.Error("failed to receive response frame from agent",
			"tunnel_id", tunnelCtx.TunnelID,
			"stream_id", tunnelCtx.StreamID,
			"error", err,
		)
		recordProxyError("receive_frame")
		p.logHTTPErrorTraffic(tunnelCtx, r, bodyBytes, http.StatusBadGateway, "agent_response_error", start)
		http.Error(w, "502 Bad Gateway: agent response error", http.StatusBadGateway)
		return
	}

	if respFrame.Type == protocol.FrameTypeError {
		slog.Error("agent returned error frame",
			"tunnel_id", tunnelCtx.TunnelID,
			"message", respFrame.ErrorMessage(),
		)
		recordProxyError("agent_error")
		p.logHTTPErrorTraffic(tunnelCtx, r, bodyBytes, http.StatusBadGateway, "agent_error", start)
		http.Error(w, "502 Bad Gateway", http.StatusBadGateway)
		return
	}

	var respPayload []byte
	if respFrame.IsCompressed() {
		respPayload, err = DecompressPayload(respFrame.Payload)
		if err != nil {
			slog.Error("failed to decompress response", "error", err)
			recordProxyError("decompress")
			p.logHTTPErrorTraffic(tunnelCtx, r, bodyBytes, http.StatusInternalServerError, "decompress", start)
			http.Error(w, "500 Internal Server Error", http.StatusInternalServerError)
			return
		}
	} else {
		respPayload = respFrame.Payload
	}

	if err := deserializeHTTPResponse(w, respPayload); err != nil {
		slog.Error("failed to write response", "error", err)
		recordProxyError("write_response")
	}

	recordBytesForwarded(float64(len(respPayload)))

	slog.Debug("proxy request completed",
		"tunnel_id", tunnelCtx.TunnelID,
		"method", r.Method,
		"host", r.Host,
	)

	p.logHTTPTraffic(tunnelCtx, r, bodyBytes, respPayload, start)
}

func (p *ReverseProxy) logHTTPTraffic(tunnelCtx *TunnelContext, r *http.Request, requestBody []byte, responsePayload []byte, start time.Time) {
	if p.trafficLogger == nil {
		return
	}

	statusCode := parseResponseStatusCode(responsePayload)
	clientIP := r.RemoteAddr
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		clientIP = fwd
	}

	duration := time.Since(start).Milliseconds()
	requestBytes := int64(len(requestBody))
	responseBytes := int64(len(responsePayload))

	event := &TrafficEvent{
		Timestamp:   time.Now(),
		TunnelID:    tunnelCtx.TunnelID,
		Protocol:    "http",
		Direction:   "ingress",
		Bytes:       requestBytes + responseBytes,
		Method:      r.Method,
		Path:        r.URL.RequestURI(),
		StatusCode:  statusCode,
		ClientIP:    clientIP,
		DurationMs:  duration,
	}
	go p.trafficLogger.Log(context.Background(), event)
}

func parseResponseStatusCode(payload []byte) int {
	if len(payload) < 12 {
		return 0
	}
	if len(payload) >= 5 && string(payload[:5]) != "HTTP/" {
		return 0
	}
	spaceIdx := -1
	for i := 5; i < len(payload); i++ {
		if payload[i] == ' ' {
			spaceIdx = i
			break
		}
	}
	if spaceIdx < 0 || spaceIdx+4 > len(payload) {
		return 0
	}
	if code, err := strconv.Atoi(string(payload[spaceIdx+1 : spaceIdx+4])); err == nil {
		return code
	}
	return 0
}

func (p *ReverseProxy) logHTTPErrorTraffic(tunnelCtx *TunnelContext, r *http.Request, requestBody []byte, statusCode int, errMsg string, start time.Time) {
	if p.trafficLogger == nil {
		return
	}

	clientIP := r.RemoteAddr
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		clientIP = fwd
	}

	duration := time.Since(start).Milliseconds()
	requestBytes := int64(len(requestBody))

	event := &TrafficEvent{
		Timestamp:   time.Now(),
		TunnelID:    tunnelCtx.TunnelID,
		Protocol:    "http",
		Direction:   "ingress",
		Bytes:       requestBytes,
		Method:      r.Method,
		Path:        r.URL.RequestURI(),
		StatusCode:  statusCode,
		ClientIP:    clientIP,
		DurationMs:  duration,
		Error:       errMsg,
	}
	go p.trafficLogger.Log(context.Background(), event)
}

func (p *ReverseProxy) handleWebSocket(w http.ResponseWriter, r *http.Request, tunnelCtx *TunnelContext) {
	slog.Debug("websocket upgrade request",
		"tunnel_id", tunnelCtx.TunnelID,
		"host", r.Host,
	)

	downConn, err := wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("websocket upgrade failed", "error", err)
		recordProxyError("ws_upgrade")
		return
	}
	defer downConn.Close()

	upReader, upWriter := io.Pipe()

	upConn := &pipeConn{
		reader: upReader,
		writer: upWriter,
		closer: upWriter,
	}

	upStream := p.streamMux.NewStream(tunnelCtx.TunnelID+"-ws", upConn)
	defer p.streamMux.CloseStream(upStream.StreamID)

	errCh := make(chan error, 2)

	go func() {
		for {
			msgType, msg, err := downConn.ReadMessage()
			if err != nil {
				errCh <- fmt.Errorf("downstream read: %w", err)
				return
			}

			compressed, err := CompressPayload(msg)
			if err != nil {
				errCh <- fmt.Errorf("compress: %w", err)
				return
			}

			frame := protocol.NewDataFrame(upStream.StreamID, compressed, true)
			if err := p.streamMux.ForwardFrame(upStream.StreamID, frame); err != nil {
				errCh <- fmt.Errorf("forward: %w", err)
				return
			}

			recordBytesForwarded(float64(len(compressed)))
			_ = msgType
		}
	}()

	go func() {
		for {
			frame, err := p.streamMux.Receive(upStream.StreamID)
			if err != nil {
				errCh <- fmt.Errorf("upstream read: %w", err)
				return
			}

			var payload []byte
			if frame.IsCompressed() {
				payload, err = DecompressPayload(frame.Payload)
				if err != nil {
					errCh <- fmt.Errorf("decompress: %w", err)
					return
				}
			} else {
				payload = frame.Payload
			}

			if err := downConn.WriteMessage(websocket.BinaryMessage, payload); err != nil {
				errCh <- fmt.Errorf("downstream write: %w", err)
				return
			}

			recordBytesForwarded(float64(len(payload)))
		}
	}()

	err = <-errCh
	slog.Debug("websocket relay ended",
		"tunnel_id", tunnelCtx.TunnelID,
		"error", err,
	)
}

type pipeConn struct {
	reader *io.PipeReader
	writer *io.PipeWriter
	closer io.Closer
}

func (p *pipeConn) Read(b []byte) (int, error)  { return p.reader.Read(b) }
func (p *pipeConn) Write(b []byte) (int, error) { return p.writer.Write(b) }
func (p *pipeConn) Close() error                { return p.closer.Close() }

type httpRequest struct {
	Method  string            `json:"method"`
	URL     string            `json:"url"`
	Proto   string            `json:"proto"`
	Headers map[string]string `json:"headers"`
	Body    []byte            `json:"body,omitempty"`
}

type httpResponse struct {
	StatusCode int               `json:"status_code"`
	Headers    map[string]string `json:"headers"`
	Body       []byte            `json:"body,omitempty"`
}

func serializeHTTPRequest(r *http.Request, body []byte) ([]byte, error) {
	headers := make(map[string]string)
	for k, v := range r.Header {
		if len(v) > 0 {
			headers[k] = v[0]
		}
	}

	if r.Host != "" {
		headers["Host"] = r.Host
	} else if _, ok := headers["Host"]; !ok {
		headers["Host"] = "localhost"
	}

	if len(body) > 0 {
		if _, ok := headers["Content-Length"]; !ok {
			headers["Content-Length"] = strconv.Itoa(len(body))
		}
	}

	url := r.URL.RequestURI()
	if url == "" {
		url = "/"
	}

	req := httpRequest{
		Method:  r.Method,
		URL:     url,
		Proto:   r.Proto,
		Headers: headers,
		Body:    body,
	}

	var buf bytes.Buffer
	if err := encodeHTTPRequest(&buf, &req); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func encodeHTTPRequest(w io.Writer, req *httpRequest) error {
	bw := bufio.NewWriter(w)

	fmt.Fprintf(bw, "%s %s %s\r\n", req.Method, req.URL, req.Proto)
	for k, v := range req.Headers {
		fmt.Fprintf(bw, "%s: %s\r\n", k, v)
	}
	fmt.Fprintf(bw, "\r\n")

	if len(req.Body) > 0 {
		if _, err := bw.Write(req.Body); err != nil {
			return err
		}
	}

	return bw.Flush()
}

func deserializeHTTPResponse(w http.ResponseWriter, payload []byte) error {
	reader := bufio.NewReader(bytes.NewReader(payload))

	statusLine, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("read status line: %w", err)
	}

	statusCode := 200
	if len(statusLine) > 12 && statusLine[:5] == "HTTP/" {
		if code, err2 := strconv.Atoi(statusLine[9:12]); err2 == nil {
			statusCode = code
		}
	}

	headers := make(http.Header)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("read header: %w", err)
		}
		line = line[:len(line)-1]
		if line == "\r" || line == "" {
			break
		}
		if len(line) > 1 && line[len(line)-1] == '\r' {
			line = line[:len(line)-1]
		}

		colonIdx := 0
		for i, c := range line {
			if c == ':' {
				colonIdx = i
				break
			}
		}
		if colonIdx > 0 {
			key := line[:colonIdx]
			val := line[colonIdx+1:]
			if len(val) > 0 && val[0] == ' ' {
				val = val[1:]
			}
			headers.Set(key, val)
		}
	}

	body, err := io.ReadAll(reader)
	if err != nil {
		return fmt.Errorf("read body: %w", err)
	}

	for k, vals := range headers {
		for _, v := range vals {
			w.Header().Add(k, v)
		}
	}

	w.WriteHeader(statusCode)
	if len(body) > 0 {
		if _, err := w.Write(body); err != nil {
			return fmt.Errorf("write body: %w", err)
		}
	}

	return nil
}

var zstdEncoder *zstd.Encoder
var zstdDecoder *zstd.Decoder

func init() {
	var err error
	zstdEncoder, err = zstd.NewWriter(nil,
		zstd.WithEncoderLevel(zstd.SpeedDefault),
	)
	if err != nil {
		panic(fmt.Sprintf("failed to create zstd encoder: %v", err))
	}
	zstdDecoder, err = zstd.NewReader(nil)
	if err != nil {
		panic(fmt.Sprintf("failed to create zstd decoder: %v", err))
	}
}

func CompressPayload(payload []byte) ([]byte, error) {
	if len(payload) == 0 {
		return nil, nil
	}

	var buf bytes.Buffer
	enc, err := zstd.NewWriter(&buf, zstd.WithEncoderLevel(zstd.SpeedDefault))
	if err != nil {
		return nil, fmt.Errorf("create encoder: %w", err)
	}
	if _, err := enc.Write(payload); err != nil {
		enc.Close()
		return nil, fmt.Errorf("compress: %w", err)
	}
	if err := enc.Close(); err != nil {
		return nil, fmt.Errorf("close encoder: %w", err)
	}
	return buf.Bytes(), nil
}

func DecompressPayload(payload []byte) ([]byte, error) {
	if len(payload) == 0 {
		return nil, nil
	}

	dec, err := zstd.NewReader(bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("create decoder: %w", err)
	}
	defer dec.Close()

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, dec); err != nil {
		return nil, fmt.Errorf("decompress: %w", err)
	}
	return buf.Bytes(), nil
}

type connWithContext interface {
	net.Conn
	Context() context.Context
}

func extractSlugFromPath(path string) string {
	if len(path) < 2 || path[0] != '/' {
		return ""
	}
	start := 1
	end := 1
	for end < len(path) && path[end] != '/' {
		end++
	}
	return path[start:end]
}

func SerializeHTTPResponse(statusCode int, headers http.Header, body []byte) ([]byte, error) {
	var buf bytes.Buffer
	bw := bufio.NewWriter(&buf)

	fmt.Fprintf(bw, "HTTP/1.1 %d %s\r\n", statusCode, http.StatusText(statusCode))
	for k, vals := range headers {
		for _, v := range vals {
			fmt.Fprintf(bw, "%s: %s\r\n", k, v)
		}
	}
	if _, err := bw.WriteString("\r\n"); err != nil {
		return nil, err
	}
	if len(body) > 0 {
		if _, err := bw.Write(body); err != nil {
			return nil, err
		}
	}
	if err := bw.Flush(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
