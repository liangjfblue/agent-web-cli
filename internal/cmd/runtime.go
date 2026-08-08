// Package cmd contains the cobra command tree for the awc CLI.
//
// Commands use a colon-separated namespace (e.g. "sys:status", "cookies:get").
// The root command accepts one positional token like "group:action" and
// dispatches to the registered handler, or to a sub-command that owns the
// full group when present. Global flags (--json, --timeout) live here.
package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/agent/web-cli/internal/ipc"
	"github.com/spf13/cobra"
)

// Runtime carries shared CLI state and helpers used by every command.
type Runtime struct {
	JSON    bool
	Timeout time.Duration
	client  *ipc.Client
}

// Call sends op to the host and returns the data map. It renders a friendly
// setup error when the socket is not reachable.
func (r *Runtime) Call(op string, args map[string]any) (map[string]any, error) {
	if r.client == nil {
		r.client = &ipc.Client{}
	}
	ctx := context.Background()
	resp, err := r.client.Call(ctx, op, args, r.timeout())
	if err != nil {
		var se *ipc.ErrSetup
		if errors.As(err, &se) {
			return nil, fmt.Errorf("%w\n\n%s", se, setupHints())
		}
		return nil, err
	}
	return resp.Data, nil
}

func (r *Runtime) timeout() time.Duration {
	if r.Timeout > 0 {
		return r.Timeout
	}
	return ipc.DefaultTimeout
}

// PrintJSON pretty-prints v as JSON. Used by --json mode.
func (r *Runtime) PrintJSON(v any) {
	b, _ := json.MarshalIndent(v, "", "  ")
	fmt.Println(string(b))
}

// setupHints returns the install/remediation text shown on connection errors.
func setupHints() string {
	return `Ensure the agent-web-cli host and extension are set up:
  1. awc sys:install
  2. Load the extension/ folder at chrome://extensions (developer mode)
  3. Reload the extension, then run: awc sys:status`
}

// rootFlags binds the global flags onto a cobra command.
func (r *Runtime) rootFlags(c *cobra.Command) {
	c.PersistentFlags().BoolVar(&r.JSON, "json", false, "output raw JSON")
	c.PersistentFlags().DurationVar(&r.Timeout, "timeout", 0, "per-call timeout (e.g. 10s)")
}

// LocatorOptions groups all element-location flags shared by dom commands.
// Any subset may be set; the extension picks anchor > semantic > selector.
type LocatorOptions struct {
	Anchor   string
	Selector string
	Role     string
	Name     string
	Text     string
	Label    string
	TestID   string
	Nth      int
	Strict   bool
}

// AddLocatorFlags binds the shared locator flags onto c.
func AddLocatorFlags(c *cobra.Command, lo *LocatorOptions) {
	c.Flags().StringVar(&lo.Anchor, "anchor", "", "element anchor from dom:snapshot (<hash>:<n>)")
	c.Flags().StringVar(&lo.Selector, "selector", "", "CSS selector")
	c.Flags().StringVar(&lo.Role, "role", "", "ARIA role or tag name")
	c.Flags().StringVar(&lo.Name, "name", "", "aria-label / title contains")
	c.Flags().StringVar(&lo.Text, "text", "", "element text contains")
	c.Flags().StringVar(&lo.Label, "label", "", "associated label text contains")
	c.Flags().StringVar(&lo.TestID, "testid", "", "data-testid exact match")
	c.Flags().IntVar(&lo.Nth, "nth", 0, "use the Nth match (1-based)")
	c.Flags().BoolVar(&lo.Strict, "strict", false, "reject if more than one match")
}

// ToArgs merges locator + target flags into an args map for the host.
func (lo LocatorOptions) ToArgs() map[string]any {
	a := map[string]any{}
	if lo.Anchor != "" {
		a["anchor"] = lo.Anchor
	}
	if lo.Selector != "" {
		a["selector"] = lo.Selector
	}
	if lo.Role != "" {
		a["role"] = lo.Role
	}
	if lo.Name != "" {
		a["name"] = lo.Name
	}
	if lo.Text != "" {
		a["text"] = lo.Text
	}
	if lo.Label != "" {
		a["label"] = lo.Label
	}
	if lo.TestID != "" {
		a["testid"] = lo.TestID
	}
	if lo.Nth > 0 {
		a["nth"] = lo.Nth
	}
	if lo.Strict {
		a["strict"] = true
	}
	return a
}

// TargetOptions groups tab-targeting flags.
type TargetOptions struct {
	TabID string
	URL   string
}

// AddTargetFlags binds tab-targeting flags onto c.
func AddTargetFlags(c *cobra.Command, to *TargetOptions) {
	c.Flags().StringVar(&to.TabID, "tab-id", "", "target tab id")
	c.Flags().StringVar(&to.URL, "url", "", "target a tab whose URL matches")
}

// Merge adds target fields into an existing args map.
func (to TargetOptions) Merge(a map[string]any) {
	if to.TabID != "" {
		a["tabId"] = to.TabID
	}
	if to.URL != "" {
		a["url"] = to.URL
	}
}

// errExit prints an error and returns exit code 1 from a RunE.
func errExit(err error) error {
	return err // cobra prints it; keep exit code non-zero via SilenceErrors=false
}
