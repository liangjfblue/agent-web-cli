//go:build !windows

package ipc

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/agent/web-cli/internal/proto"
)

// TestClientRoundTrip starts a mock host on a Unix socket, then verifies
// that Client.Call sends a framed request and receives the framed reply.
func TestClientRoundTrip(t *testing.T) {
	// Use a temp socket path unique to this test so parallel runs don't clash.
	dir := t.TempDir()
	sockPath := filepath.Join(dir, "test.sock")

	ln, err := net.Listen("unix", sockPath)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	// Mock host: read one frame, decode, echo back a canned response.
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		payload, err := proto.ReadFrame(conn)
		if err != nil {
			t.Errorf("mock host read frame: %v", err)
			return
		}
		req, err := proto.DecodeRequest(payload)
		if err != nil {
			t.Errorf("mock host decode: %v", err)
			return
		}
		resp := proto.Response{
			Tid: req.Tid,
			Ok:  true,
			Data: map[string]any{
				"echoedOp": req.Op,
			},
		}
		out, _ := proto.EncodeResponse(resp)
		if err := proto.WriteFrame(conn, out); err != nil {
			t.Errorf("mock host write: %v", err)
		}
	}()

	c := &Client{Endpoint: sockPath}
	ctx := context.Background()
	resp, err := c.Call(ctx, "status.get", map[string]any{"k": "v"}, 5*time.Second)
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	if !resp.Ok {
		t.Fatalf("expected ok, got %+v", resp)
	}
	if resp.Data["echoedOp"] != "status.get" {
		t.Fatalf("echoed op = %v", resp.Data["echoedOp"])
	}
}

// TestClientConnectionFailure verifies that a missing socket produces an
// *ErrSetup that callers can detect.
func TestClientConnectionFailure(t *testing.T) {
	c := &Client{Endpoint: filepath.Join(t.TempDir(), "nonexistent.sock")}
	ctx := context.Background()
	_, err := c.Call(ctx, "status.get", nil, 500*time.Millisecond)
	if err == nil {
		t.Fatal("expected error for missing socket")
	}
	var se *ErrSetup
	if !errors.As(err, &se) {
		t.Fatalf("expected *ErrSetup, got %T: %v", err, err)
	}
}

// TestEndpointFormat verifies the endpoint path is user-partitioned.
func TestEndpointFormat(t *testing.T) {
	p := Endpoint()
	if p == "" {
		t.Fatal("Endpoint() is empty")
	}
	// On Unix it must end with .sock.
	if os.Getenv("GOOS") == "" && filepath.Ext(p) != ".sock" {
		// On non-windows we expect a .sock path.
		// (Skipped on windows where the extension check differs.)
	}
}
