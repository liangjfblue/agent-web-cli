package install

import (
	"crypto/rand"
	"encoding/hex"
	"net"
	"os"
	"os/user"
	"regexp"
	"runtime"
	"time"

	"github.com/agent/web-cli/internal/proto"
)

// probeHostViaSocket dials the host socket, sends one op, and reports whether
// a valid response comes back. It is a minimal, dependency-free echo of
// ipc.Client so the install package stays free of import cycles.
func probeHostViaSocket(op string) bool {
	endpoint := probeEndpoint()
	conn, err := net.DialTimeout(socketNetwork(), endpoint, 1500*time.Millisecond)
	if err != nil {
		return false
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(3 * time.Second))

	tid := randTID()
	req := proto.Request{Tid: tid, Op: op}
	payload, err := proto.EncodeRequest(req)
	if err != nil {
		return false
	}
	if err := proto.WriteFrame(conn, payload); err != nil {
		return false
	}
	respPayload, err := proto.ReadFrame(conn)
	if err != nil {
		return false
	}
	resp, err := proto.DecodeResponse(respPayload)
	if err != nil {
		return false
	}
	return resp.Ok && resp.Tid == tid
}

func socketNetwork() string {
	if runtime.GOOS == "windows" {
		return "npipe"
	}
	return "unix"
}

// probeEndpoint mirrors ipc.Endpoint without importing ipc.
func probeEndpoint() string {
	seg := "default"
	if u, err := user.Current(); err == nil {
		if u.Uid != "" {
			seg = sanitize(u.Uid)
		} else if u.Username != "" {
			seg = sanitize(u.Username)
		}
	}
	if runtime.GOOS == "windows" {
		return `\\.\pipe\awc-` + seg
	}
	dir := os.Getenv("AWC_RUNTIME_DIR")
	if dir == "" {
		dir = "/tmp"
	}
	return dir + "/awc-host-" + seg + ".sock"
}

var unsafeChars = regexp.MustCompile(`[^a-zA-Z0-9_-]`)

func sanitize(s string) string {
	return unsafeChars.ReplaceAllString(s, "_")
}

func randTID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}
