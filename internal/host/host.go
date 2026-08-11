// Package host implements the awc native-messaging host.
//
// The host is launched by Chrome (via the registered native-messaging
// manifest) when the extension calls connectNative. It then:
//
//  1. Opens the stdio native-messaging channel to the extension.
//  2. Listens on a local Unix socket / named pipe for CLI connections.
//  3. Bridges the two: CLI frames (AW binary frames) become native messages
//     (Chrome 4-byte LE framed JSON), and replies flow back the same way.
//
// The host keeps one in-flight request per CLI connection. Tid is used to
// route each reply back to the originating connection.
package host

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/agent/web-cli/internal/ipc"
	"github.com/agent/web-cli/internal/proto"
)

// Version is reported in status.get replies. It is overridden at link time
// via -ldflags "-X host.Version=..." in release builds.
var Version = "0.1.1-dev"

// Host owns the socket listener and the extension channel.
type Host struct {
	socketPath      string
	listener        net.Listener
	logf            func(format string, args ...any)
	profileID       string
	helloCh         chan ChromeReply
	extensionDone   chan struct{}
	extensionOut    io.Writer
	extensionWrites chan extensionWrite
	writerDone      chan struct{}
	statusMu        sync.RWMutex
	statusData      map[string]any

	mu      sync.Mutex
	pending map[string]chan proto.Response // tid -> waiter
}

type extensionWrite struct {
	request ChromeRequest
	result  chan error
}

// Run starts the host. It blocks until ctx is cancelled or a fatal error
// occurs. The stdio channel to Chrome is read on a goroutine.
func Run(ctx context.Context) error {
	return run(ctx, os.Stdin, os.Stdout)
}

func run(ctx context.Context, extensionIn io.Reader, extensionOut io.Writer) error {
	h := &Host{
		pending:         make(map[string]chan proto.Response),
		helloCh:         make(chan ChromeReply, 1),
		extensionDone:   make(chan struct{}),
		extensionOut:    extensionOut,
		extensionWrites: make(chan extensionWrite),
		writerDone:      make(chan struct{}),
		statusData:      map[string]any{},
		logf:            func(format string, args ...any) {}, // stderr by default
	}
	go h.writeExtensionLoop(ctx)
	// Only log to stderr: stdout is reserved for native-messaging frames.
	if os.Getenv("AWC_HOST_DEBUG") != "" {
		h.logf = func(format string, args ...any) {
			log.Printf(format, args...)
		}
		log.SetOutput(os.Stderr)
	}

	// New extensions identify their Chrome profile immediately. Older
	// extensions fall back to the legacy single-profile endpoint.
	go h.readExtensionLoop(extensionIn)
	select {
	case hello := <-h.helloCh:
		h.applyHello(hello)
	case <-time.After(2 * time.Second):
		h.logf("extension hello timed out; using legacy endpoint")
	case <-h.extensionDone:
		return nil
	case <-ctx.Done():
		return nil
	}
	h.socketPath = ipc.EndpointForProfile(h.profileID)

	ln, err := listen(ctx, h.socketPath)
	if err != nil {
		return fmt.Errorf("listen %s: %w", h.socketPath, err)
	}
	h.listener = ln
	h.logf("awc host listening on %s", h.socketPath)
	if h.profileID != "" {
		if err := h.publishProfile(); err != nil {
			h.Close()
			return fmt.Errorf("register profile: %w", err)
		}
	}

	// Accept loop: handle CLI connections.
	go func() {
		select {
		case <-ctx.Done():
		case <-h.extensionDone:
		}
		h.Close()
	}()

	for {
		conn, err := ln.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return nil
			}
			return fmt.Errorf("accept: %w", err)
		}
		go h.handleCLI(conn)
	}
}

// Close shuts down the socket listener.
func (h *Host) Close() error {
	if h.profileID != "" {
		_ = ipc.UnregisterProfile(h.profileID, h.socketPath)
	}
	if h.listener != nil {
		return h.listener.Close()
	}
	return nil
}

// handleCLI serves one CLI connection: read exactly one AW frame, register a
// waiter, forward as a native message, then wait for and write back the
// matching reply.
func (h *Host) handleCLI(conn net.Conn) {
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(2 * time.Minute))

	payload, err := proto.ReadFrame(conn)
	if err != nil {
		if !errors.Is(err, io.EOF) {
			h.logf("read cli frame: %v", err)
		}
		return
	}
	req, err := proto.DecodeRequest(payload)
	if err != nil {
		h.writeError(conn, "", "BAD_REQUEST", err.Error())
		return
	}

	// status.get is handled locally — it does not need the extension.
	if req.Op == "status.get" {
		h.handleStatus(conn, req.Tid)
		return
	}

	// Register a waiter for this tid before forwarding.
	ch := make(chan proto.Response, 1)
	h.register(req.Tid, ch)
	defer h.unregister(req.Tid)

	if err := h.writeExtension(ChromeRequest{
		Tid:  req.Tid,
		Op:   req.Op,
		Args: req.Args,
	}); err != nil {
		h.logf("write native message: %v", err)
		return
	}
	// Wait for the matching reply from the extension.
	// Use the caller-supplied WaitMs hint if present, else the default.
	wait := 30 * time.Second
	if req.WaitMs > 0 {
		wait = time.Duration(req.WaitMs) * time.Millisecond
	}
	select {
	case resp := <-ch:
		out, _ := proto.EncodeResponse(resp)
		if err := proto.WriteFrame(conn, out); err != nil {
			h.logf("write cli frame: %v", err)
		}
	case <-time.After(wait):
		h.writeError(conn, req.Tid, "TIMEOUT", "extension did not respond in time")
	}
}

// writeExtensionLoop is the only goroutine that writes to Chrome's native
// messaging stdout. A complete length-prefixed frame is therefore emitted
// before the next request starts writing.
func (h *Host) writeExtensionLoop(ctx context.Context) {
	defer close(h.writerDone)
	for {
		select {
		case <-ctx.Done():
			return
		case <-h.extensionDone:
			return
		case write := <-h.extensionWrites:
			write.result <- WriteChromeRequest(h.extensionOut, write.request)
		}
	}
}

func (h *Host) writeExtension(request ChromeRequest) error {
	write := extensionWrite{request: request, result: make(chan error, 1)}
	select {
	case h.extensionWrites <- write:
	case <-h.extensionDone:
		return io.ErrClosedPipe
	case <-h.writerDone:
		return io.ErrClosedPipe
	}
	select {
	case err := <-write.result:
		return err
	case <-h.extensionDone:
		return io.ErrClosedPipe
	case <-h.writerDone:
		return io.ErrClosedPipe
	}
}

// readExtensionLoop continuously reads native replies and routes them by tid
// to the waiting CLI connection.
func (h *Host) readExtensionLoop(r io.Reader) {
	defer close(h.extensionDone)
	for {
		reply, err := ReadChromeReply(r)
		if err != nil {
			if errors.Is(err, io.EOF) {
				h.logf("native messaging stdin closed")
				return
			}
			h.logf("read native reply: %v", err)
			return
		}
		if reply.Tid == "__awc_hello__" && reply.Ok {
			h.applyHello(reply)
			select {
			case h.helloCh <- reply:
			default:
			}
			continue
		}
		// Convert the extension's reply to our proto.Response and wake the waiter.
		resp := proto.Response{
			Tid:  reply.Tid,
			Ok:   reply.Ok,
			Data: reply.Data,
		}
		if !reply.Ok && reply.Code != "" {
			resp.Err = &proto.WireError{Code: reply.Code, Msg: reply.Msg}
		}
		h.deliver(reply.Tid, resp)
	}
}

// handleStatus answers status.get locally using metadata from the extension
// hello, so it does not need a second extension round trip.
func (h *Host) handleStatus(conn net.Conn, tid string) {
	h.statusMu.RLock()
	extension := cloneMap(mapValue(h.statusData, "extension"))
	profile := cloneMap(mapValue(h.statusData, "profile"))
	h.statusMu.RUnlock()
	if extension == nil {
		extension = map[string]any{"connected": true}
	}
	if profile == nil {
		profile = map[string]any{"profileId": "", "profileName": ""}
	}
	resp := proto.Response{
		Tid: tid,
		Ok:  true,
		Data: map[string]any{
			"host": map[string]any{
				"version":      Version,
				"endpoint":     h.socketPath,
				"endpointType": endpointType(),
			},
			"extension": extension,
			"profile":   profile,
		},
	}
	out, _ := proto.EncodeResponse(resp)
	proto.WriteFrame(conn, out)
}

func (h *Host) applyHello(reply ChromeReply) {
	profile := cloneMap(mapValue(reply.Data, "profile"))
	extension := cloneMap(mapValue(reply.Data, "extension"))
	if profile == nil {
		return
	}
	profileID, _ := profile["profileId"].(string)
	if profileID == "" {
		return
	}
	h.statusMu.Lock()
	h.profileID = profileID
	h.statusData = map[string]any{"profile": profile, "extension": extension}
	h.statusMu.Unlock()
	if h.listener != nil {
		_ = h.publishProfile()
	}
}

func (h *Host) publishProfile() error {
	h.statusMu.RLock()
	profile := mapValue(h.statusData, "profile")
	name, _ := profile["profileName"].(string)
	profileID := h.profileID
	h.statusMu.RUnlock()
	return ipc.RegisterProfile(ipc.ProfileRegistration{
		ProfileID:   profileID,
		ProfileName: name,
		Endpoint:    h.socketPath,
		PID:         os.Getpid(),
		Version:     Version,
	})
}

func mapValue(m map[string]any, key string) map[string]any {
	if m == nil {
		return nil
	}
	v, _ := m[key].(map[string]any)
	return v
}

func cloneMap(m map[string]any) map[string]any {
	if m == nil {
		return nil
	}
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// writeError sends a structured error frame to a CLI connection.
func (h *Host) writeError(conn net.Conn, tid, code, msg string) {
	resp := proto.Response{
		Tid: tid,
		Ok:  false,
		Err: &proto.WireError{Code: code, Msg: msg},
	}
	out, _ := proto.EncodeResponse(resp)
	proto.WriteFrame(conn, out)
}

func (h *Host) register(tid string, ch chan proto.Response) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.pending[tid] = ch
}

func (h *Host) unregister(tid string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.pending, tid)
}

func (h *Host) deliver(tid string, resp proto.Response) {
	h.mu.Lock()
	ch, ok := h.pending[tid]
	h.mu.Unlock()
	if ok {
		ch <- resp
	}
}

func endpointType() string {
	if runtime.GOOS == "windows" {
		return "pipe"
	}
	return "unix"
}
