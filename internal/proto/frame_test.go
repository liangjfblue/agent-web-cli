package proto

import (
	"bytes"
	"testing"
)

// TestFrameRoundTrip verifies that WriteFrame followed by ReadFrame returns
// the original payload intact.
func TestFrameRoundTrip(t *testing.T) {
	cases := [][]byte{
		[]byte("hello"),
		[]byte(""),
		bytes.Repeat([]byte{0xAB}, 1024),
	}
	for i, payload := range cases {
		var buf bytes.Buffer
		if err := WriteFrame(&buf, payload); err != nil {
			t.Fatalf("case %d: WriteFrame: %v", i, err)
		}
		got, err := ReadFrame(&buf)
		if err != nil {
			t.Fatalf("case %d: ReadFrame: %v", i, err)
		}
		if !bytes.Equal(got, payload) {
			t.Fatalf("case %d: payload mismatch: got %d bytes want %d", i, len(got), len(payload))
		}
	}
}

// TestFrameMagicReject ensures a wrong magic is detected.
func TestFrameMagicReject(t *testing.T) {
	var buf bytes.Buffer
	_ = WriteFrame(&buf, []byte("x"))
	// Corrupt the magic bytes.
	data := buf.Bytes()
	data[0] = 0x00
	if _, err := ReadFrame(bytes.NewReader(data)); err != ErrBadMagic {
		t.Fatalf("expected ErrBadMagic, got %v", err)
	}
}

// TestFrameCRCReject ensures a corrupted payload is detected by the CRC.
func TestFrameCRCReject(t *testing.T) {
	var buf bytes.Buffer
	_ = WriteFrame(&buf, []byte("hello world"))
	data := buf.Bytes()
	// Flip a bit in the payload section (after the 8-byte header).
	data[8] ^= 0xFF
	if _, err := ReadFrame(bytes.NewReader(data)); err != ErrCRC {
		t.Fatalf("expected ErrCRC, got %v", err)
	}
}

// TestRequestRoundTrip verifies msgpack encode/decode of Request.
func TestRequestRoundTrip(t *testing.T) {
	req := Request{
		Tid: "abc123",
		Op:  "status.get",
		Args: map[string]any{
			"url": "https://example.com/",
			"n":   float64(42),
		},
	}
	b, err := EncodeRequest(req)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	got, err := DecodeRequest(b)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Tid != req.Tid || got.Op != req.Op {
		t.Fatalf("mismatch: got %+v want %+v", got, req)
	}
	if got.Args["url"] != req.Args["url"] {
		t.Fatalf("args mismatch: got %v want %v", got.Args["url"], req.Args["url"])
	}
}

// TestResponseRoundTrip verifies msgpack encode/decode of Response.
func TestResponseRoundTrip(t *testing.T) {
	resp := Response{
		Tid: "xyz",
		Ok:  true,
		Data: map[string]any{
			"connected": true,
		},
	}
	b, err := EncodeResponse(resp)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	got, err := DecodeResponse(b)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Tid != resp.Tid || got.Ok != resp.Ok {
		t.Fatalf("mismatch: got %+v want %+v", got, resp)
	}
}

// TestWireErrorImplementsError ensures *WireError satisfies the error
// interface, which ipc.Client.Call relies on.
func TestWireErrorImplementsError(t *testing.T) {
	var e error = &WireError{Code: "E_TEST", Msg: "boom"}
	if e.Error() != "E_TEST: boom" {
		t.Fatalf("unexpected Error(): %q", e.Error())
	}
}
