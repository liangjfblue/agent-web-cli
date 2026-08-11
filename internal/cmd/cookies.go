package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// registerCookies adds the flat cookies:* commands.
func registerCookies(root *cobra.Command, rt *Runtime) {
	root.AddCommand(cookiesGet(rt))
}

func cookiesGet(rt *Runtime) *cobra.Command {
	var url, name string
	var header, redact, all bool
	c := &cobra.Command{
		Use:   "cookies:get",
		Short: "Read cookies for a URL or the active tab",
		RunE: func(cmd *cobra.Command, args []string) error {
			args_, err := cookieRequestArgs(url, name, all)
			if err != nil {
				return errExit(err)
			}
			data, err := rt.Call("cookies.getAll", args_)
			if err != nil {
				return errExit(err)
			}
			cookies := mapSlice(data, "cookies")
			if rt.JSON {
				if redact {
					redacted := make(map[string]any, len(data))
					for key, value := range data {
						redacted[key] = value
					}
					redactedCookies := make([]map[string]any, 0, len(cookies))
					for _, cookie := range cookies {
						copyCookie := make(map[string]any, len(cookie))
						for key, value := range cookie {
							copyCookie[key] = value
						}
						copyCookie["value"] = "<redacted>"
						redactedCookies = append(redactedCookies, copyCookie)
					}
					redacted["cookies"] = redactedCookies
					rt.PrintJSON(redacted)
					return nil
				}
				rt.PrintJSON(data)
				return nil
			}
			if header {
				fmt.Println(cookiesHeader(cookies, redact))
				return nil
			}
			printCookiesTable(cookies, redact)
			return nil
		},
	}
	c.Flags().StringVar(&url, "url", "", "read cookies for this URL")
	c.Flags().BoolVar(&all, "all", false, "read all cookies (cannot be combined with --url)")
	c.Flags().StringVar(&name, "name", "", "filter by cookie name")
	c.Flags().BoolVar(&header, "header", false, "output as Cookie request header")
	c.Flags().BoolVar(&redact, "redact", false, "hide cookie values")
	return c
}

func cookieRequestArgs(url, name string, all bool) (map[string]any, error) {
	if url != "" && all {
		return nil, fmt.Errorf("--url and --all cannot be used together")
	}
	args := map[string]any{}
	switch {
	case url != "":
		args["url"] = url
	case all:
		args["all"] = true
	default:
		args["activeTab"] = true
	}
	if name != "" {
		args["name"] = name
	}
	return args, nil
}

// printCookiesTable renders a compact table of cookies.
func printCookiesTable(cookies []map[string]any, redact bool) {
	if len(cookies) == 0 {
		fmt.Println("(no cookies)")
		return
	}
	fmt.Printf("%-28s %-40s %-6s %-8s\n", "NAME", "VALUE", "DOMAIN", "PATH")
	for _, c := range cookies {
		val := mapStr(c, "value")
		if redact {
			val = "<redacted>"
		}
		fmt.Printf("%-28s %-40s %-6s %-8s\n",
			truncate(mapStr(c, "name"), 28),
			truncate(val, 40),
			truncate(mapStr(c, "domain"), 24),
			truncate(mapStr(c, "path"), 16))
	}
}

// cookiesHeader joins cookies into a "Cookie: a=1; b=2" header line.
func cookiesHeader(cookies []map[string]any, redact bool) string {
	parts := make([]string, 0, len(cookies))
	for _, c := range cookies {
		val := mapStr(c, "value")
		if redact {
			val = "<redacted>"
		}
		parts = append(parts, mapStr(c, "name")+"="+val)
	}
	return strings.Join(parts, "; ")
}

// mapSlice extracts a []map[string]any from data[key].
func mapSlice(m map[string]any, key string) []map[string]any {
	if m == nil {
		return nil
	}
	v, ok := m[key]
	if !ok {
		return nil
	}
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]map[string]any, 0, len(arr))
	for _, item := range arr {
		if mm, ok := item.(map[string]any); ok {
			out = append(out, mm)
		}
	}
	return out
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}
