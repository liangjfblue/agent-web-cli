package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/agent/web-cli/internal/ipc"
	"github.com/agent/web-cli/internal/proto"
)

func TestValidateHTTPURL(t *testing.T) {
	for _, valid := range []string{"https://example.com", "http://localhost:8080/api"} {
		if err := validateHTTPURL(valid); err != nil {
			t.Errorf("validateHTTPURL(%q): %v", valid, err)
		}
	}
	for _, invalid := range []string{"", "example.com", "file:///tmp/a"} {
		if err := validateHTTPURL(invalid); err == nil {
			t.Errorf("validateHTTPURL(%q) should fail", invalid)
		}
	}
}

type recordingSessionCaller struct {
	op   string
	args map[string]any
}

func (c *recordingSessionCaller) Call(op string, args map[string]any) (map[string]any, error) {
	c.op = op
	c.args = args
	return map[string]any{"removed": true}, nil
}

func TestClearConfiguredAuthCookie(t *testing.T) {
	caller := &recordingSessionCaller{}
	cfg := AuthConfig{LoggedInWhen: AuthCondition{Cookie: &CookieCond{
		URL:  "https://vue.ruoyi.vip",
		Name: "Admin-Token",
	}}}

	if err := clearConfiguredAuthCookie(caller, cfg); err != nil {
		t.Fatal(err)
	}
	if caller.op != "cookies.remove" {
		t.Fatalf("op = %q, want cookies.remove", caller.op)
	}
	if caller.args["url"] != "https://vue.ruoyi.vip" || caller.args["name"] != "Admin-Token" {
		t.Fatalf("unexpected args: %#v", caller.args)
	}
}

func TestValidateSessionRefresh(t *testing.T) {
	if err := validateSessionRefresh(true, true); err != nil {
		t.Fatalf("interactive refresh should be valid: %v", err)
	}
	if err := validateSessionRefresh(false, true); err == nil {
		t.Fatal("refresh without interactive should fail")
	}
}

func TestSessionAcquireJSONHostUnavailable(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	authDir := filepath.Join(home, ".awc", "auth")
	if err := os.MkdirAll(authDir, 0o700); err != nil {
		t.Fatal(err)
	}
	config := `{"loginUrl":"https://vue.ruoyi.vip/login","loggedInWhen":{"cookie":{"url":"https://vue.ruoyi.vip","name":"Admin-Token"}}}`
	if err := os.WriteFile(filepath.Join(authDir, "ruoyi.json"), []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}

	missing := filepath.Join(t.TempDir(), "missing.sock")
	if runtime.GOOS == "windows" {
		missing = fmt.Sprintf(`\\.\pipe\awc-missing-%d`, time.Now().UnixNano())
	}
	rt := &Runtime{client: &ipc.Client{Endpoint: missing}}
	root := NewRootCmd(rt)
	root.SetArgs([]string{"session:acquire", "ruoyi", "--url", "https://vue.ruoyi.vip", "--json"})

	read, write, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	originalStdout := os.Stdout
	os.Stdout = write
	execErr := root.Execute()
	write.Close()
	os.Stdout = originalStdout
	out, readErr := io.ReadAll(read)
	read.Close()
	if readErr != nil {
		t.Fatal(readErr)
	}
	if ExitCode(execErr) != ExitHostUnavailable || !IsSilent(execErr) {
		t.Fatalf("error = %v, code = %d, silent = %v", execErr, ExitCode(execErr), IsSilent(execErr))
	}
	var envelope sessionEnvelope
	if err := json.Unmarshal(out, &envelope); err != nil {
		t.Fatalf("invalid JSON %q: %v", out, err)
	}
	if envelope.OK || envelope.Data["status"] != "host_unavailable" {
		t.Fatalf("unexpected envelope: %+v", envelope)
	}
}

func TestExitCode(t *testing.T) {
	cases := []struct {
		err  error
		want int
	}{
		{NewExitError(ExitAuthRequired, errors.New("login")), ExitAuthRequired},
		{&ipc.ErrSetup{Detail: "missing"}, ExitHostUnavailable},
		{&proto.WireError{Code: "FAILED", Msg: "failed"}, ExitExtensionFailure},
		{errors.New("other"), 1},
	}
	for _, tc := range cases {
		if got := ExitCode(tc.err); got != tc.want {
			t.Errorf("ExitCode(%T) = %d, want %d", tc.err, got, tc.want)
		}
	}
}
