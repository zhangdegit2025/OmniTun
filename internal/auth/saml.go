package auth

import (
	"bytes"
	"compress/flate"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"encoding/xml"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"
	"time"
)

type SAMLProvider struct {
	EntityID    string
	ACSURL      string
	MetadataURL string
	CertFile    string
	KeyFile     string
}

type SAMLUser struct {
	NameID     string
	Email      string
	Name       string
	Attributes map[string][]string
}

const (
	samlAssertionNS = "urn:oasis:names:tc:SAML:2.0:assertion"
	samlProtocolNS  = "urn:oasis:names:tc:SAML:2.0:protocol"
	xmlDSigNS       = "http://www.w3.org/2000/09/xmldsig#"
	samlMetadataNS  = "urn:oasis:names:tc:SAML:2.0:metadata"
)

type samlAuthnRequest struct {
	XMLName                    xml.Name `xml:"urn:oasis:names:tc:SAML:2.0:protocol AuthnRequest"`
	ID                         string   `xml:"ID,attr"`
	Version                    string   `xml:"Version,attr"`
	IssueInstant               string   `xml:"IssueInstant,attr"`
	Destination                string   `xml:"Destination,attr"`
	ProtocolBinding            string   `xml:"ProtocolBinding,attr"`
	AssertionConsumerServiceURL string  `xml:"AssertionConsumerServiceURL,attr"`
	Issuer                     samlIssuer `xml:"urn:oasis:names:tc:SAML:2.0:assertion Issuer"`
	NameIDPolicy               samlNameIDPolicy `xml:"NameIDPolicy"`
}

type samlIssuer struct {
	XMLName xml.Name `xml:"urn:oasis:names:tc:SAML:2.0:assertion Issuer"`
	Format  string   `xml:"Format,attr,omitempty"`
	Value   string   `xml:",chardata"`
}

type samlNameIDPolicy struct {
	XMLName     xml.Name `xml:"NameIDPolicy"`
	Format      string   `xml:"Format,attr,omitempty"`
	AllowCreate bool     `xml:"AllowCreate,attr"`
}

type samlResponse struct {
	XMLName      xml.Name      `xml:"urn:oasis:names:tc:SAML:2.0:protocol Response"`
	ID           string        `xml:"ID,attr"`
	Version      string        `xml:"Version,attr"`
	IssueInstant string        `xml:"IssueInstant,attr"`
	Destination  string        `xml:"Destination,attr,omitempty"`
	InResponseTo string        `xml:"InResponseTo,attr,omitempty"`
	Issuer       samlIssuer    `xml:"urn:oasis:names:tc:SAML:2.0:assertion Issuer"`
	Status       samlStatus    `xml:"Status"`
	Assertion    samlAssertion `xml:"urn:oasis:names:tc:SAML:2.0:assertion Assertion"`
	Signature    *samlSignature `xml:"http://www.w3.org/2000/09/xmldsig# Signature"`
}

type samlStatus struct {
	XMLName    xml.Name `xml:"urn:oasis:names:tc:SAML:2.0:protocol Status"`
	StatusCode samlStatusCode `xml:"StatusCode"`
}

type samlStatusCode struct {
	XMLName xml.Name `xml:"urn:oasis:names:tc:SAML:2.0:protocol StatusCode"`
	Value   string   `xml:"Value,attr"`
}

type samlAssertion struct {
	XMLName      xml.Name          `xml:"urn:oasis:names:tc:SAML:2.0:assertion Assertion"`
	ID           string            `xml:"ID,attr"`
	IssueInstant string            `xml:"IssueInstant,attr"`
	Version      string            `xml:"Version,attr"`
	Issuer       samlIssuer        `xml:"urn:oasis:names:tc:SAML:2.0:assertion Issuer"`
	Subject      samlSubject       `xml:"Subject"`
	Conditions   samlConditions    `xml:"Conditions"`
	AuthnStatement *samlAuthnStatement `xml:"AuthnStatement"`
	AttributeStatement []samlAttributeStatement `xml:"AttributeStatement"`
}

type samlSubject struct {
	XMLName             xml.Name              `xml:"Subject"`
	NameID              samlNameID            `xml:"NameID"`
	SubjectConfirmation samlSubjectConfirmation `xml:"SubjectConfirmation"`
}

type samlNameID struct {
	XMLName xml.Name `xml:"NameID"`
	Format  string   `xml:"Format,attr,omitempty"`
	Value   string   `xml:",chardata"`
}

type samlSubjectConfirmation struct {
	XMLName                 xml.Name `xml:"SubjectConfirmation"`
	Method                  string   `xml:"Method,attr"`
	SubjectConfirmationData samlSubjectConfirmationData `xml:"SubjectConfirmationData"`
}

type samlSubjectConfirmationData struct {
	XMLName      xml.Name `xml:"SubjectConfirmationData"`
	NotOnOrAfter string   `xml:"NotOnOrAfter,attr,omitempty"`
	Recipient    string   `xml:"Recipient,attr,omitempty"`
	InResponseTo string   `xml:"InResponseTo,attr,omitempty"`
}

type samlConditions struct {
	XMLName             xml.Name `xml:"Conditions"`
	NotBefore           string   `xml:"NotBefore,attr"`
	NotOnOrAfter        string   `xml:"NotOnOrAfter,attr"`
	AudienceRestriction []samlAudienceRestriction `xml:"AudienceRestriction"`
}

type samlAudienceRestriction struct {
	XMLName  xml.Name `xml:"AudienceRestriction"`
	Audience string   `xml:"Audience"`
}

type samlAuthnStatement struct {
	XMLName            xml.Name `xml:"AuthnStatement"`
	AuthnInstant       string   `xml:"AuthnInstant,attr"`
	SessionIndex       string   `xml:"SessionIndex,attr,omitempty"`
	SessionNotOnOrAfter string  `xml:"SessionNotOnOrAfter,attr,omitempty"`
	AuthnContext        samlAuthnContext `xml:"AuthnContext"`
}

type samlAuthnContext struct {
	XMLName              xml.Name `xml:"AuthnContext"`
	AuthnContextClassRef string   `xml:"AuthnContextClassRef"`
}

type samlAttributeStatement struct {
	XMLName    xml.Name         `xml:"urn:oasis:names:tc:SAML:2.0:assertion AttributeStatement"`
	Attributes []samlAttribute  `xml:"Attribute"`
}

type samlAttribute struct {
	XMLName      xml.Name `xml:"urn:oasis:names:tc:SAML:2.0:assertion Attribute"`
	Name         string   `xml:"Name,attr"`
	NameFormat   string   `xml:"NameFormat,attr,omitempty"`
	FriendlyName string   `xml:"FriendlyName,attr,omitempty"`
	Values       []samlAttributeValue `xml:"AttributeValue"`
}

type samlAttributeValue struct {
	XMLName xml.Name `xml:"AttributeValue"`
	Type    string   `xml:"http://www.w3.org/2001/XMLSchema-instance type,attr,omitempty"`
	Value   string   `xml:",chardata"`
}

type samlSignature struct {
	XMLName xml.Name `xml:"http://www.w3.org/2000/09/xmldsig# Signature"`
	SignedInfo     samlSignedInfo     `xml:"SignedInfo"`
	SignatureValue string            `xml:"SignatureValue"`
	KeyInfo        samlKeyInfo        `xml:"KeyInfo"`
}

type samlSignedInfo struct {
	XMLName                xml.Name `xml:"SignedInfo"`
	CanonicalizationMethod samlMethod `xml:"CanonicalizationMethod"`
	SignatureMethod        samlMethod `xml:"SignatureMethod"`
	Reference              samlReference `xml:"Reference"`
}

type samlMethod struct {
	Algorithm string `xml:"Algorithm,attr"`
}

type samlReference struct {
	URI          string       `xml:"URI,attr"`
	Transforms   []samlTransform `xml:"Transforms>Transform"`
	DigestMethod  samlMethod    `xml:"DigestMethod"`
	DigestValue   string        `xml:"DigestValue"`
}

type samlTransform struct {
	Algorithm string `xml:"Algorithm,attr"`
}

type samlKeyInfo struct {
	XMLName  xml.Name `xml:"KeyInfo"`
	KeyValue samlKeyValue `xml:"KeyValue"`
}

type samlKeyValue struct {
	XMLName     xml.Name `xml:"KeyValue"`
	RSAKeyValue samlRSAKeyValue `xml:"RSAKeyValue"`
}

type samlRSAKeyValue struct {
	XMLName  xml.Name `xml:"RSAKeyValue"`
	Modulus  string   `xml:"Modulus"`
	Exponent string   `xml:"Exponent"`
}

type samlSPMetadata struct {
	XMLName              xml.Name                   `xml:"urn:oasis:names:tc:SAML:2.0:metadata EntityDescriptor"`
	EntityID             string                     `xml:"entityID,attr"`
	SPSSODescriptor      samlSPSSODescriptor        `xml:"urn:oasis:names:tc:SAML:2.0:metadata SPSSODescriptor"`
	Organization         *samlOrganization          `xml:"Organization,omitempty"`
}

type samlSPSSODescriptor struct {
	XMLName                    xml.Name                   `xml:"urn:oasis:names:tc:SAML:2.0:metadata SPSSODescriptor"`
	ProtocolSupportEnumeration string                     `xml:"protocolSupportEnumeration,attr"`
	AuthnRequestsSigned        bool                       `xml:"AuthnRequestsSigned,attr"`
	WantAssertionsSigned       bool                       `xml:"WantAssertionsSigned,attr"`
	KeyDescriptors             []samlKeyDescriptor        `xml:"KeyDescriptor"`
	NameIDFormats              []samlNameIDFormat         `xml:"NameIDFormat"`
	AssertionConsumerServices  []samlACSEndpoint          `xml:"AssertionConsumerService"`
}

type samlKeyDescriptor struct {
	XMLName  xml.Name `xml:"KeyDescriptor"`
	Use      string   `xml:"use,attr,omitempty"`
	KeyInfo  samlKeyInfo `xml:"http://www.w3.org/2000/09/xmldsig# KeyInfo"`
}

type samlNameIDFormat struct {
	XMLName xml.Name `xml:"NameIDFormat"`
	Value   string   `xml:",chardata"`
}

type samlACSEndpoint struct {
	XMLName  xml.Name `xml:"AssertionConsumerService"`
	Binding  string   `xml:"Binding,attr"`
	Location string   `xml:"Location,attr"`
	Index    int      `xml:"index,attr"`
	IsDefault bool    `xml:"isDefault,attr,omitempty"`
}

type samlOrganization struct {
	XMLName          xml.Name `xml:"Organization"`
	OrganizationName string   `xml:"OrganizationName"`
	OrganizationDisplayName string `xml:"OrganizationDisplayName"`
	OrganizationURL  string   `xml:"OrganizationURL"`
}

func (p *SAMLProvider) GenerateSPMetadata() ([]byte, error) {
	certPEM := ""
	if p.CertFile != "" {
		data, err := os.ReadFile(p.CertFile)
		if err == nil {
			certPEM = strings.TrimSpace(string(data))
		}
	}

	keyDescriptors := []samlKeyDescriptor{}
	if certPEM != "" {
		pubKey, err := parseX509FromPEM(certPEM)
		if err == nil {
			keyDescriptors = append(keyDescriptors, samlKeyDescriptor{
				Use: "signing",
				KeyInfo: samlKeyInfo{
					KeyValue: samlKeyValue{
						RSAKeyValue: samlRSAKeyValue{
							Modulus:  base64.StdEncoding.EncodeToString(pubKey.N.Bytes()),
							Exponent: base64.StdEncoding.EncodeToString(bigEndianBytes(pubKey.E)),
						},
					},
				},
			})
		}
	}

	metadata := samlSPMetadata{
		EntityID: p.EntityID,
		SPSSODescriptor: samlSPSSODescriptor{
			ProtocolSupportEnumeration: "urn:oasis:names:tc:SAML:2.0:protocol",
			AuthnRequestsSigned:        false,
			WantAssertionsSigned:       true,
			KeyDescriptors:             keyDescriptors,
			NameIDFormats: []samlNameIDFormat{
				{Value: "urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress"},
				{Value: "urn:oasis:names:tc:SAML:2.0:nameid-format:persistent"},
			},
			AssertionConsumerServices: []samlACSEndpoint{
				{
					Binding:   "urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST",
					Location:  p.ACSURL,
					Index:     0,
					IsDefault: true,
				},
			},
		},
	}

	output, err := xml.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal SP metadata: %w", err)
	}
	return append([]byte(xml.Header), output...), nil
}

func (p *SAMLProvider) BuildAuthnRequest(relayState string) (string, string, error) {
	requestID := generateSAMLID()
	issueInstant := time.Now().UTC().Format(time.RFC3339Nano)

	authnReq := samlAuthnRequest{
		ID:                          requestID,
		Version:                     "2.0",
		IssueInstant:                issueInstant,
		Destination:                 p.MetadataURL,
		ProtocolBinding:             "urn:oasis:names:tc:SAML:2.0:bindings:HTTP-Redirect",
		AssertionConsumerServiceURL: p.ACSURL,
		Issuer: samlIssuer{
			Format: "urn:oasis:names:tc:SAML:2.0:nameid-format:entity",
			Value:  p.EntityID,
		},
		NameIDPolicy: samlNameIDPolicy{
			Format:      "urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress",
			AllowCreate: true,
		},
	}

	requestXML, err := xml.Marshal(authnReq)
	if err != nil {
		return "", "", fmt.Errorf("failed to marshal authn request: %w", err)
	}

	var buf bytes.Buffer
	flateWriter, err := flate.NewWriter(&buf, flate.DefaultCompression)
	if err != nil {
		return "", "", fmt.Errorf("failed to create deflate writer: %w", err)
	}
	if _, err := flateWriter.Write(requestXML); err != nil {
		flateWriter.Close()
		return "", "", fmt.Errorf("failed to deflate authn request: %w", err)
	}
	flateWriter.Close()

	encodedRequest := base64.StdEncoding.EncodeToString(buf.Bytes())
	encodedRequest = url.QueryEscape(encodedRequest)

	sssoURL := p.MetadataURL
	if relayState != "" {
		encodedRelayState := url.QueryEscape(relayState)
		if strings.Contains(sssoURL, "?") {
			sssoURL += "&SAMLRequest=" + encodedRequest + "&RelayState=" + encodedRelayState
		} else {
			sssoURL += "?SAMLRequest=" + encodedRequest + "&RelayState=" + encodedRelayState
		}
	} else {
		if strings.Contains(sssoURL, "?") {
			sssoURL += "&SAMLRequest=" + encodedRequest
		} else {
			sssoURL += "?SAMLRequest=" + encodedRequest
		}
	}

	return sssoURL, requestID, nil
}

func (p *SAMLProvider) ParseSAMLResponse(samlResp string) (*SAMLUser, error) {
	decoded, err := base64.StdEncoding.DecodeString(samlResp)
	if err != nil {
		return nil, fmt.Errorf("failed to base64 decode SAML response: %w", err)
	}

	var resp samlResponse
	if err := xml.Unmarshal(decoded, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse SAML response XML: %w", err)
	}

	if resp.Status.StatusCode.Value != "urn:oasis:names:tc:SAML:2.0:status:Success" {
		return nil, fmt.Errorf("SAML authentication failed: status %s", resp.Status.StatusCode.Value)
	}

	user := &SAMLUser{
		NameID:     resp.Assertion.Subject.NameID.Value,
		Attributes: make(map[string][]string),
	}

	for _, attrStmt := range resp.Assertion.AttributeStatement {
		for _, attr := range attrStmt.Attributes {
			name := attr.Name
			if attr.FriendlyName != "" {
				name = attr.FriendlyName
			}
			for _, v := range attr.Values {
				user.Attributes[name] = append(user.Attributes[name], v.Value)
			}
		}
	}

	if emailAttrs, ok := user.Attributes["email"]; ok && len(emailAttrs) > 0 {
		user.Email = emailAttrs[0]
	}
	if emailAttrs, ok := user.Attributes["mail"]; ok && len(emailAttrs) > 0 && user.Email == "" {
		user.Email = emailAttrs[0]
	}
	if nameAttrs, ok := user.Attributes["displayName"]; ok && len(nameAttrs) > 0 {
		user.Name = nameAttrs[0]
	}
	if nameAttrs, ok := user.Attributes["givenName"]; ok && len(nameAttrs) > 0 && user.Name == "" {
		user.Name = nameAttrs[0]
	}

	if user.Email == "" {
		if strings.Contains(user.NameID, "@") {
			user.Email = user.NameID
		}
	}
	if user.Name == "" {
		user.Name = user.Email
	}

	return user, nil
}

func (p *SAMLProvider) GetHTTPSSOLoginURL(idpSSOURL string) string {
	return p.BuildAuthnRequestHTTPRedirect(idpSSOURL)
}

func (p *SAMLProvider) BuildAuthnRequestHTTPRedirect(idpSSOURL string) string {
	requestID := generateSAMLID()
	issueInstant := time.Now().UTC().Format(time.RFC3339Nano)

	authnReq := `<?xml version="1.0" encoding="UTF-8"?>` + "\n" +
		`<samlp:AuthnRequest xmlns:samlp="urn:oasis:names:tc:SAML:2.0:protocol" ` +
		`ID="` + requestID + `" ` +
		`Version="2.0" ` +
		`IssueInstant="` + issueInstant + `" ` +
		`Destination="` + xmlEscape(idpSSOURL) + `" ` +
		`ProtocolBinding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST" ` +
		`AssertionConsumerServiceURL="` + xmlEscape(p.ACSURL) + `">` +
		`<saml:Issuer xmlns:saml="urn:oasis:names:tc:SAML:2.0:assertion">` + xmlEscape(p.EntityID) + `</saml:Issuer>` +
		`<samlp:NameIDPolicy Format="urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress" AllowCreate="true"/>` +
		`</samlp:AuthnRequest>`

	var buf bytes.Buffer
	flateWriter, err := flate.NewWriter(&buf, flate.DefaultCompression)
	if err != nil {
		return ""
	}
	flateWriter.Write([]byte(authnReq))
	flateWriter.Close()

	encodedRequest := base64.StdEncoding.EncodeToString(buf.Bytes())

	return encodedRequest
}

func xmlEscape(s string) string {
	buf := new(strings.Builder)
	xml.EscapeText(buf, []byte(s))
	return buf.String()
}

func generateSAMLID() string {
	b := make([]byte, 20)
	rand.Read(b)
	h := sha256.Sum256(b)
	id := fmt.Sprintf("_%x", h[:16])
	return id
}

func parseX509FromPEM(certPEM string) (*rsa.PublicKey, error) {
	block, _ := pem.Decode([]byte(certPEM))
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse certificate: %w", err)
	}
	rsaPub, ok := cert.PublicKey.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("certificate public key is not RSA")
	}
	return rsaPub, nil
}

func bigEndianBytes(e int) []byte {
	if e == 65537 {
		return []byte{1, 0, 1}
	}
	b := make([]byte, 4)
	b[3] = byte(e & 0xff)
	e >>= 8
	b[2] = byte(e & 0xff)
	e >>= 8
	b[1] = byte(e & 0xff)
	e >>= 8
	b[0] = byte(e & 0xff)
	for i := 0; i < 4; i++ {
		if b[i] != 0 {
			return b[i:]
		}
	}
	return []byte{0}
}

func DeflateAndEncode(data []byte) string {
	var buf bytes.Buffer
	w, _ := flate.NewWriter(&buf, flate.DefaultCompression)
	w.Write(data)
	w.Close()
	return base64.StdEncoding.EncodeToString(buf.Bytes())
}

func InflateDecode(encoded string) ([]byte, error) {
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, err
	}
	r := flate.NewReader(bytes.NewReader(decoded))
	defer r.Close()
	return io.ReadAll(r)
}
