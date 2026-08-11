package ipc

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/agent/web-cli/internal/proto"
)

// DefaultTimeout is used when a caller does not specify one.
const DefaultTimeout = 30 * time.Second

// Client is a one-shot connection to the native host over the local socket.
// Each Call opens a fresh connection, sends one request, reads one response,
// and closes. This mirrors the typical CLI usage pattern (one command per
// process) and keeps the host free of per-connection state.
type Client struct {
	// Endpoint overrides the computed socket path; empty means use Endpoint().
	Endpoint string
}

// Call sends op with args and returns the decoded response data, or an error.
// The error may be a *proto.WireError if the host/extension reported a
// structured failure.
func (c *Client) Call(ctx context.Context, op string, args map[string]any, timeout time.Duration) (proto.Response, error) {
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	// Bind the socket I/O to ctx so callers can cancel long waits.
	dialCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	endpoint := c.Endpoint
	if endpoint == "" {
		endpoint = Endpoint()
	}

	conn, err := dial(dialCtx, endpoint)
	if err != nil {
		return proto.Response{}, setupError(err)
	}
	defer conn.Close()

	tid, err := newTid()
	if err != nil {
		return proto.Response{}, fmt.Errorf("awc: gen tid: %w", err)
	}
	// Pass the timeout as a WaitMs hint so the host waits for long ops
	// (auth.login can take 120s) instead of timing out at its 30s default.
	// The host's own wait is bounded by this value; the socket deadline below
	// adds a small buffer on top.
	req := proto.Request{
		Tid:    tid,
		Op:     op,
		Args:   args,
		WaitMs: timeout.Milliseconds(),
	}
	payload, err := proto.EncodeRequest(req)
	if err != nil {
		return proto.Response{}, fmt.Errorf("awc: encode request: %w", err)
	}
	if err := proto.WriteFrame(conn, payload); err != nil {
		return proto.Response{}, setupError(fmt.Errorf("write frame: %w", err))
	}

	respPayload, err := proto.ReadFrame(conn)
	if err != nil {
		if errors.Is(err, io.EOF) {
			return proto.Response{}, setupError(errors.New("host closed the connection before responding"))
		}
		return proto.Response{}, fmt.Errorf("awc: read frame: %w", err)
	}
	resp, err := proto.DecodeResponse(respPayload)
	if err != nil {
		return proto.Response{}, fmt.Errorf("awc: decode response: %w", err)
	}
	if resp.Tid != tid {
		return resp, fmt.Errorf("awc: tid mismatch (want %s got %s)", tid, resp.Tid)
	}
	if !resp.Ok {
		return resp, resp.Err
	}
	return resp, nil
}

// ErrSetup wraps connection failures and is rendered with install hints.
type ErrSetup struct {
	Detail string
}

func (e *ErrSetup) Error() string {
	return fmt.Sprintf("awc host not reachable: %s", e.Detail)
}

func setupError(err error) *ErrSetup {
	return &ErrSetup{Detail: err.Error()}
}

// newTid returns a 16-byte hex transaction id.
func newTid() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
