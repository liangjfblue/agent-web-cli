package proto

import (
	"github.com/vmihailenco/msgpack/v5"
)

// Request is a CLI -> host -> extension call.
// Field names are intentionally short and distinct from other projects'
// wire formats.
type Request struct {
	// Tid is a client-generated transaction id, echoed in the response.
	Tid string `msgpack:"tid"`
	// Op names the operation, e.g. "status.get" or "cookies.getAll".
	Op string `msgpack:"op"`
	// Args carries op-specific parameters.
	Args map[string]any `msgpack:"args,omitempty"`
	// WaitMs is an optional hint for how long the host should wait for the
	// extension to respond, in milliseconds. Zero means use the host default.
	// This lets long ops (auth.login, net.debug, cdp.listen) override the
	// host's short default without timing out.
	WaitMs int64 `msgpack:"waitMs,omitempty"`
}

// Response is the reply flowing back.
type Response struct {
	Tid  string         `msgpack:"tid"`
	Ok   bool           `msgpack:"ok"`
	Data map[string]any `msgpack:"data,omitempty"`
	Err  *WireError     `msgpack:"err,omitempty"`
}

// WireError is the structured failure payload. It implements error so callers
// can return it directly.
type WireError struct {
	Code string `msgpack:"code,omitempty"`
	Msg  string `msgpack:"msg,omitempty"`
}

// Error renders the error as a single human-readable line.
func (e *WireError) Error() string {
	if e == nil {
		return ""
	}
	if e.Code != "" && e.Msg != "" {
		return e.Code + ": " + e.Msg
	}
	if e.Code != "" {
		return e.Code
	}
	return e.Msg
}

// EncodeRequest msgpack-encodes a Request into a frame payload.
func EncodeRequest(r Request) ([]byte, error) {
	return msgpack.Marshal(r)
}

// DecodeRequest msgpack-decodes a frame payload into a Request.
func DecodeRequest(b []byte) (Request, error) {
	var r Request
	err := msgpack.Unmarshal(b, &r)
	return r, err
}

// EncodeResponse msgpack-encodes a Response into a frame payload.
func EncodeResponse(r Response) ([]byte, error) {
	return msgpack.Marshal(r)
}

// DecodeResponse msgpack-decodes a frame payload into a Response.
func DecodeResponse(b []byte) (Response, error) {
	var r Response
	err := msgpack.Unmarshal(b, &r)
	return r, err
}
