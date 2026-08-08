package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// registerWait adds the flat wait:* commands.
func registerWait(root *cobra.Command, rt *Runtime) {
	root.AddCommand(waitFor(rt))
}

// waitFor blocks until a condition is met or the timeout expires.
//
// kind   condition
// ─────────────────────────────────────
// selector  an element matching --selector exists
// text      page body contains --text
// url       tab URL contains --url-pattern
// xhr       a request matching --url-pattern (and optionally --status) completes
func waitFor(rt *Runtime) *cobra.Command {
	var to TargetOptions
	var lo LocatorOptions
	var kind, timeout, interval, urlPattern, status string
	c := &cobra.Command{
		Use:   "wait:for",
		Short: "Wait for a selector, text, url, or xhr condition",
		RunE: func(cmd *cobra.Command, args []string) error {
			if kind == "" {
				if lo.Selector != "" {
					kind = "selector"
				} else if lo.Text != "" {
					kind = "text"
				} else if urlPattern != "" {
					kind = "url"
				} else {
					return errExit(fmt.Errorf("specify --kind or one of --selector/--text/--url-pattern"))
				}
			}
			a := map[string]any{
				"kind":      kind,
				"timeoutMs": parseDurationMs(timeout),
				"intervalMs": parseDurationMs(interval),
			}
			if lo.Selector != "" {
				a["selector"] = lo.Selector
			}
			if lo.Text != "" {
				a["text"] = lo.Text
			}
			if urlPattern != "" {
				a["urlPattern"] = urlPattern
			}
			if status != "" {
				a["statusCode"] = parseInt(status)
			}
			to.Merge(a)
			data, err := rt.Call("wait.for", a)
			if err != nil {
				return errExit(err)
			}
			if rt.JSON {
				rt.PrintJSON(data)
				return nil
			}
			if mapBool(data, "ok") {
				fmt.Printf("ok: %s condition met\n", kind)
			} else {
				fmt.Printf("timeout: %s not satisfied\n", kind)
			}
			return nil
		},
	}
	c.Flags().StringVar(&kind, "kind", "", "condition kind: selector|text|url|xhr")
	c.Flags().StringVar(&timeout, "timeout", "30s", "max wait time")
	c.Flags().StringVar(&interval, "interval", "500ms", "poll interval")
	c.Flags().StringVar(&urlPattern, "url-pattern", "", "url substring (for kind=url|xhr)")
	c.Flags().StringVar(&status, "status", "", "expected status code (for kind=xhr)")
	c.Flags().StringVar(&lo.Selector, "selector", "", "CSS selector (for kind=selector)")
	c.Flags().StringVar(&lo.Text, "text", "", "text to find in body (for kind=text)")
	AddTargetFlags(c, &to)
	return c
}

func parseInt(s string) int {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0
		}
		n = n*10 + int(c-'0')
	}
	return n
}
