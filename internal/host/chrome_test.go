package host

import (
	"bytes"
	"testing"
)

// TestChromeFrameRoundTrip verifies the Chrome native-messaging framing
// (4-byte little-endian length + JSON) round-trips correctly.
func TestChromeFrameRoundTrip(t *testing.T) {
	req := ChromeRequest{
		Tid:  "abc",
		Op:   "status.get",
		Args: map[string]any{"x": float64(1)},
	}
	var buf bytes.Buffer
	if err := WriteChromeRequest(&buf, req); err != nil {
		t.Fatalf("WriteChromeRequest: %v", err)
	}

	// Decode manually to verify the length prefix is little-endian uint32.
	data := buf.Bytes()
	if len(data) < 4 {
		t.Fatalf("frame too short")
	}
	// length = low byte first
	wantLen := uint32(len(data) - 4)
	gotLen := uint32(data[0]) | uint32(data[1])<<8 | uint32(data[2])<<16 | uint32(data[3])<<24
	if gotLen != wantLen {
		t.Fatalf("length prefix = %d, want %d", gotLen, wantLen)
	}
}

// TestReadChromeReplyParsesJSON verifies a framed reply decodes into ChromeReply.
func TestReadChromeReplyParsesJSON(t *testing.T) {
	// Build a framed reply manually.
	jsonPayload := []byte(`{"tid":"t1","ok":true,"data":{"connected":true}}`)
	var buf bytes.Buffer
	// 4-byte LE length
	buf.WriteByte(byte(len(jsonPayload)))
	buf.WriteByte(byte(len(jsonPayload) >> 8))
	buf.WriteByte(byte(len(jsonPayload) >> 16))
	buf.WriteByte(byte(len(jsonPayload) >> 24))
	buf.Write(jsonPayload)

	reply, err := ReadChromeReply(&buf)
	if err != nil {
		t.Fatalf("ReadChromeReply: %v", err)
	}
	if reply.Tid != "t1" || !reply.Ok {
		t.Fatalf("got %+v", reply)
	}
	if reply.Data["connected"] != true {
		t.Fatalf("data mismatch: %v", reply.Data)
	}
}

// TestReadChromeReplyRejectsTooLarge verifies oversized frames are rejected.
func TestReadChromeReplyRejectsTooLarge(t *testing.T) {
	var buf bytes.Buffer
	// Claim a 100MB message.
	buf.WriteByte(0)
	buf.WriteByte(0)
	buf.WriteByte(0)
	buf.WriteByte(0x40) // 1GB > 64MB limit
	_, err := ReadChromeReply(&buf)
	if err == nil {
		t.Fatal("expected error for oversized frame")
	}
}
