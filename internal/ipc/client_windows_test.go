//go:build windows

package ipc

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/Microsoft/go-winio"
	"github.com/agent/web-cli/internal/proto"
)

func TestClientRoundTrip(t *testing.T) {
	pipePath := fmt.Sprintf(`\\.\pipe\awc-test-%d-%d`, os.Getpid(), time.Now().UnixNano())
	ln, err := winio.ListenPipe(pipePath, nil)
	if err != nil {
		t.Fatalf("listen pipe: %v", err)
	}
	defer ln.Close()

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
		out, _ := proto.EncodeResponse(proto.Response{
			Tid: req.Tid,
			Ok:  true,
			Data: map[string]any{
				"echoedOp": req.Op,
			},
		})
		if err := proto.WriteFrame(conn, out); err != nil {
			t.Errorf("mock host write: %v", err)
		}
	}()

	c := &Client{Endpoint: pipePath}
	resp, err := c.Call(context.Background(), "status.get", nil, 5*time.Second)
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	if resp.Data["echoedOp"] != "status.get" {
		t.Fatalf("echoed op = %v", resp.Data["echoedOp"])
	}
}

func TestClientConnectionFailure(t *testing.T) {
	c := &Client{Endpoint: fmt.Sprintf(`\\.\pipe\awc-missing-%d`, time.Now().UnixNano())}
	_, err := c.Call(context.Background(), "status.get", nil, 200*time.Millisecond)
	if err == nil {
		t.Fatal("expected error for missing pipe")
	}
	var se *ErrSetup
	if !errors.As(err, &se) {
		t.Fatalf("expected *ErrSetup, got %T: %v", err, err)
	}
}

func TestEndpointFormat(t *testing.T) {
	if p := Endpoint(); len(p) < 9 || p[:9] != `\\.\pipe\` {
		t.Fatalf("unexpected Windows endpoint: %q", p)
	}
}
