package s3

import (
	"crypto/rand"
	"encoding/xml"
	"fmt"
	"net/http"
)

type s3Error struct {
	XMLName   xml.Name `xml:"Error"`
	Code      string   `xml:"Code"`
	Message   string   `xml:"Message"`
	RequestID string   `xml:"RequestId"`
}

// writeS3Error serializes an S3-style XML error envelope with a fresh
// per-request id.
func writeS3Error(w http.ResponseWriter, status int, code, message string) {
	writeXML(w, status, s3Error{
		Code:      code,
		Message:   message,
		RequestID: newRequestID(),
	})
}

// writeXML serializes v as an S3-style XML response body with the given
// HTTP status.
func writeXML(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(status)

	body, err := xml.MarshalIndent(v, "", "  ")
	if err != nil {
		return
	}
	_, _ = w.Write([]byte(xml.Header))
	_, _ = w.Write(body)
}

// newRequestID returns a random UUIDv4-formatted request id.
func newRequestID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "00000000-0000-4000-8000-000000000000"
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
