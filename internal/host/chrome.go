package host

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
)

// ChromeRequest is the message the host sends TO the extension over the
// native-messaging stdio channel (forwarding a CLI op).
type ChromeRequest struct {
	Tid  string         `json:"tid"`
	Op   string         `json:"op"`
	Args map[string]any `json:"args,omitempty"`
}

// ChromeReply is the message the extension sends BACK to the host.
type ChromeReply struct {
	Tid  string         `json:"tid"`
	Ok   bool           `json:"ok"`
	Data map[string]any `json:"data,omitempty"`
	Code string         `json:"code,omitempty"`
	Msg  string         `json:"msg,omitempty"`
}

// ReadChromeReply reads one length-prefixed native message (a reply from the
// extension) from r.
//
// The framing is defined by Chrome: a 4-byte little-endian uint32 length
// followed by that many bytes of UTF-8 JSON. stdout is reserved for these
// frames, so the host must never print plain text to stdout.
func ReadChromeReply(r io.Reader) (ChromeReply, error) {
	var lenBuf [4]byte
	if _, err := io.ReadFull(r, lenBuf[:]); err != nil {
		return ChromeReply{}, err
	}
	n := binary.LittleEndian.Uint32(lenBuf[:])
	if n == 0 {
		return ChromeReply{}, nil
	}
	if n > 64*1024*1024 {
		return ChromeReply{}, fmt.Errorf("awc host: native message too large (%d)", n)
	}
	buf := make([]byte, n)
	if _, err := io.ReadFull(r, buf); err != nil {
		return ChromeReply{}, err
	}
	var reply ChromeReply
	if err := json.Unmarshal(buf, &reply); err != nil {
		return ChromeReply{}, fmt.Errorf("awc host: decode native reply: %w", err)
	}
	return reply, nil
}

// WriteChromeRequest writes one length-prefixed native message (a request to
// the extension) to w.
func WriteChromeRequest(w io.Writer, req ChromeRequest) error {
	payload, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("awc host: encode native request: %w", err)
	}
	var lenBuf [4]byte
	binary.LittleEndian.PutUint32(lenBuf[:], uint32(len(payload)))
	if _, err := w.Write(lenBuf[:]); err != nil {
		return err
	}
	if _, err := w.Write(payload); err != nil {
		return err
	}
	return nil
}
