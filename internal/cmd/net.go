package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// registerNet adds the flat net:* commands.
func registerNet(root *cobra.Command, rt *Runtime) {
	root.AddCommand(
		netWatch(rt),
		netStop(rt),
		netDebug(rt),
		netBody(rt),
	)
}

// netWatch starts a webRequest capture; the CLI sleeps for the duration, then
// collects results via net.stop.
func netWatch(rt *Runtime) *cobra.Command {
	var to TargetOptions
	var duration, urlPattern string
	var includeStatic bool
	c := &cobra.Command{
		Use:   "net:watch",
		Short: "Capture network request metadata for a duration",
		RunE: func(cmd *cobra.Command, args []string) error {
			a := map[string]any{}
			a["durationMs"] = parseDurationMs(duration)
			a["ignoreStatic"] = !includeStatic
			if urlPattern != "" {
				a["urlPattern"] = urlPattern
			}
			to.Merge(a)

			// 1. Start capture.
			startData, err := rt.Call("net.watch", a)
			if err != nil {
				return errExit(err)
			}
			captureID := mapStr(startData, "captureId")
			if !rt.JSON {
				fmt.Printf("capturing: %s (%s)\n", captureID, duration)
			}
			// 2. Sleep locally.
			d := time.Duration(parseDurationMs(duration)) * time.Millisecond
			time.Sleep(d)
			// 3. Collect.
			stopData, err := rt.Call("net.stop", map[string]any{"captureId": captureID})
			if err != nil {
				return errExit(err)
			}
			if rt.JSON {
				rt.PrintJSON(stopData)
				return nil
			}
			printNetRequests(mapSlice(stopData, "requests"))
			return nil
		},
	}
	c.Flags().StringVar(&duration, "duration", "10s", "capture duration (e.g. 10s, 5000ms)")
	c.Flags().StringVar(&urlPattern, "url-pattern", "", "only capture URLs containing this substring")
	c.Flags().BoolVar(&includeStatic, "include-static", false, "include js/css/img/font resources")
	AddTargetFlags(c, &to)
	return c
}

// netStop stops an active capture (or all) and prints results.
func netStop(rt *Runtime) *cobra.Command {
	var captureID string
	var all bool
	c := &cobra.Command{
		Use:   "net:stop",
		Short: "Stop an active network capture and collect results",
		RunE: func(cmd *cobra.Command, args []string) error {
			a := map[string]any{}
			if all {
				// empty captureId stops all
			} else if captureID != "" {
				a["captureId"] = captureID
			}
			data, err := rt.Call("net.stop", a)
			if err != nil {
				return errExit(err)
			}
			if rt.JSON {
				rt.PrintJSON(data)
				return nil
			}
			reqs := mapSlice(data, "requests")
			if reqs == nil {
				// stop-all returns captures array
				for _, cap := range mapSlice(data, "captures") {
					fmt.Printf("capture: %s\n", mapStr(cap, "captureId"))
					printNetRequests(mapSlice(cap, "requests"))
				}
			} else {
				printNetRequests(reqs)
			}
			return nil
		},
	}
	c.Flags().StringVar(&captureID, "capture-id", "", "specific capture to stop")
	c.Flags().BoolVar(&all, "all", false, "stop all active captures")
	return c
}

// netDebug attaches CDP and captures requests with response bodies.
func netDebug(rt *Runtime) *cobra.Command {
	var to TargetOptions
	var duration, urlPattern string
	var maxBody int
	var noBody, includeConsole, includeStatic, json bool
	c := &cobra.Command{
		Use:   "net:debug",
		Short: "Capture requests with response bodies via CDP debugger",
		RunE: func(cmd *cobra.Command, args []string) error {
			a := map[string]any{}
			a["durationMs"] = parseDurationMs(duration)
			a["maxBodyBytes"] = maxBody
			a["noBody"] = noBody
			a["includeConsole"] = includeConsole
			a["ignoreStatic"] = !includeStatic
			if urlPattern != "" {
				a["urlPattern"] = urlPattern
			}
			to.Merge(a)
			data, err := rt.Call("net.debug", a)
			if err != nil {
				return errExit(err)
			}
			// Cache response bodies to disk for later retrieval via net:body.
			cacheBodies(data)
			_ = json
			if rt.JSON {
				rt.PrintJSON(data)
				return nil
			}
			printNetDebug(data)
			return nil
		},
	}
	c.Flags().StringVar(&duration, "duration", "10s", "capture duration")
	c.Flags().StringVar(&urlPattern, "url-pattern", "", "only capture URLs containing this substring")
	c.Flags().IntVar(&maxBody, "max-body-bytes", 500000, "max response body bytes before truncation")
	c.Flags().BoolVar(&noBody, "no-body", false, "skip response bodies")
	c.Flags().BoolVar(&includeConsole, "include-console", false, "also capture console events")
	c.Flags().BoolVar(&includeStatic, "include-static", false, "include static resources")
	_ = json
	AddTargetFlags(c, &to)
	return c
}

// netBody reads a cached response body by key from the local body cache.
func netBody(rt *Runtime) *cobra.Command {
	var output string
	var raw bool
	c := &cobra.Command{
		Use:   "net:body <bodyKey>",
		Short: "Read a cached response body",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			key := args[0]
			cacheDir := netBodyCacheDir()
			path := filepath.Join(cacheDir, key)
			content, err := os.ReadFile(path)
			if err != nil {
				return errExit(fmt.Errorf("body key not found: %s", key))
			}
			if output != "" {
				return os.WriteFile(output, content, 0o644)
			}
			if raw {
				os.Stdout.Write(content)
				return nil
			}
			fmt.Println(string(content))
			return nil
		},
	}
	c.Flags().StringVarP(&output, "output", "o", "", "write body to this path")
	c.Flags().BoolVar(&raw, "raw", false, "write raw bytes to stdout")
	return c
}

// netBodyCacheDir returns ~/.awc/net-bodies (created on demand).
func netBodyCacheDir() string {
	home, _ := os.UserHomeDir()
	dir := filepath.Join(home, ".awc", "net-bodies")
	os.MkdirAll(dir, 0o755)
	return dir
}

// printNetRequests renders a compact table of captured requests.
func printNetRequests(reqs []map[string]any) {
	if len(reqs) == 0 {
		fmt.Println("(no requests captured)")
		return
	}
	fmt.Printf("%-6s %-6s %-50s %s\n", "STATUS", "METHOD", "URL", "TYPE")
	for _, r := range reqs {
		fmt.Printf("%-6s %-6s %-50s %s\n",
			strconv.Itoa(int(mapInt(r, "statusCode"))),
			truncate(mapStr(r, "method"), 6),
			truncate(mapStr(r, "url"), 50),
			truncate(mapStr(r, "type"), 10))
	}
}

// cacheBodies writes response bodies from net:debug to ~/.awc/net-bodies/
// so they can be retrieved later via `awc net:body <bodyKey>`. It also strips
// the bodyData field from the response so it doesn't clutter --json output.
func cacheBodies(data map[string]any) {
	reqs := mapSlice(data, "requests")
	for _, r := range reqs {
		bodyKey := mapStr(r, "bodyKey")
		bodyData := mapStr(r, "bodyData")
		if bodyKey == "" || bodyData == "" {
			continue
		}
		path := filepath.Join(netBodyCacheDir(), bodyKey)
		os.WriteFile(path, []byte(bodyData), 0o644)
		// Remove bodyData from the response so it doesn't bloat JSON output.
		delete(r, "bodyData")
	}
}

// printNetDebug renders net.debug results with body previews.
func printNetDebug(data map[string]any) {
	reqs := mapSlice(data, "requests")
	fmt.Printf("captured: %d requests\n\n", len(reqs))
	for i, r := range reqs {
		fmt.Printf("[%d] %s %s\n", i+1, mapStr(r, "status"), mapStr(r, "url"))
		if bp := mapStr(r, "bodyPreview"); bp != "" {
			fmt.Printf("    body: %s", truncate(bp, 200))
			if mapBool(r, "bodyTruncated") {
				fmt.Printf(" ... (truncated, full=%d)", mapInt(r, "bodyFullLen"))
			}
			if bk := mapStr(r, "bodyKey"); bk != "" {
				fmt.Printf(" [key: %s]", bk)
			}
			fmt.Println()
		}
	}
	if events := mapSlice(data, "consoleEvents"); len(events) > 0 {
		fmt.Printf("\nconsole events: %d\n", len(events))
		for _, e := range events {
			fmt.Printf("  [%s] %s\n", mapStr(e, "level"), truncate(mapStr(e, "text"), 100))
		}
	}
}

// parseDurationMs turns "10s" / "5000ms" / "5000" into milliseconds.
func parseDurationMs(s string) int64 {
	s = strings.TrimSpace(s)
	if d, err := time.ParseDuration(s); err == nil {
		return d.Milliseconds()
	}
	if n, err := strconv.ParseInt(s, 10, 64); err == nil {
		return n
	}
	return 10000
}
