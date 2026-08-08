package cmd

import (
	"fmt"
	"strings"

	"github.com/agent/web-cli/internal/install"
	"github.com/spf13/cobra"
)

// registerSys adds the flat sys:* commands.
func registerSys(root *cobra.Command, rt *Runtime) {
	root.AddCommand(
		sysSetup(rt),
		sysStatus(rt),
		sysDoctor(rt),
		sysInstall(rt),
		sysUninstall(rt),
	)
}

func sysSetup(rt *Runtime) *cobra.Command {
	return &cobra.Command{
		Use:   "sys:setup",
		Short: "Run the first-time setup wizard (build, host, PATH, extension)",
		RunE: func(cmd *cobra.Command, args []string) error {
			res, err := install.Setup()
			if err != nil {
				return errExit(err)
			}
			if rt.JSON {
				rt.PrintJSON(res)
				return nil
			}
			printSetupResult(res)
			return nil
		},
	}
}

func printSetupResult(res *install.SetupResult) {
	const (
		green = "\033[32m"
		yellow = "\033[33m"
		red    = "\033[31m"
		bold   = "\033[1m"
		reset  = "\033[0m"
	)
	fmt.Println(bold + "awc setup" + reset)
	fmt.Println(strings.Repeat("─", 40))
	for _, s := range res.Steps {
		icon := green + "✓" + reset
		switch s.Status {
		case "skipped":
			icon = yellow + "○" + reset
		case "failed":
			icon = red + "✗" + reset
		case "manual":
			icon = yellow + "!" + reset
		}
		fmt.Printf("%s %-10s %s\n", icon, s.Name, s.Detail)
	}
	fmt.Println(strings.Repeat("─", 40))
	if res.AllAuto {
		fmt.Println(green + "all steps complete. run: awc sys:status" + reset)
		return
	}
	fmt.Println(yellow + "finish the manual step(s) above, then run: awc sys:status" + reset)
}

func sysStatus(rt *Runtime) *cobra.Command {
	return &cobra.Command{
		Use:   "sys:status",
		Short: "Show whether CLI, host and extension are connected",
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := rt.Call("status.get", nil)
			if err != nil {
				return errExit(err)
			}
			if rt.JSON {
				rt.PrintJSON(data)
				return nil
			}
			host := mapVal(data, "host")
			ext := mapVal(data, "extension")
			fmt.Printf("host:      %s @ %s\n", mapStr(host, "version"), mapStr(host, "endpoint"))
			if mapStr(ext, "version") != "" {
				fmt.Printf("extension: %s\n", mapStr(ext, "version"))
				fmt.Printf("connected: %v\n", mapBool(ext, "connected"))
			}
			return nil
		},
	}
}

func sysDoctor(rt *Runtime) *cobra.Command {
	return &cobra.Command{
		Use:   "sys:doctor",
		Short: "Run full connectivity diagnostics",
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := rt.Call("status.get", nil)
			if err != nil {
				if rt.JSON {
					rt.PrintJSON(map[string]any{"ok": false, "error": err.Error()})
				} else {
					fmt.Printf("host:    UNREACHABLE (%v)\n", err)
				}
				return nil
			}
			if rt.JSON {
				rt.PrintJSON(map[string]any{"ok": true, "status": data})
				return nil
			}
			host := mapVal(data, "host")
			fmt.Printf("host:     ok (%s @ %s)\n", mapStr(host, "version"), mapStr(host, "endpoint"))
			fmt.Println("socket:   reachable")
			fmt.Println("doctor:   all checks passed")
			return nil
		},
	}
}

func sysInstall(rt *Runtime) *cobra.Command {
	return &cobra.Command{
		Use:   "sys:install",
		Short: "Register the native-messaging host with Chrome",
		RunE: func(cmd *cobra.Command, args []string) error {
			res, err := install.Install()
			if err != nil {
				return errExit(err)
			}
			if rt.JSON {
				rt.PrintJSON(res)
				return nil
			}
			fmt.Println("Native messaging host registered.")
			fmt.Printf("Host name:  %s\n", res.HostName)
			fmt.Printf("Manifest:   %s\n", res.ManifestPath)
			fmt.Printf("Launcher:   %s\n", res.LauncherPath)
			fmt.Printf("Extension:  %s\n", res.ExtensionID)
			return nil
		},
	}
}

func sysUninstall(rt *Runtime) *cobra.Command {
	return &cobra.Command{
		Use:   "sys:uninstall",
		Short: "Remove the native-messaging host registration",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := install.Uninstall(); err != nil {
				return errExit(err)
			}
			fmt.Println("Native messaging host removed.")
			return nil
		},
	}
}

// --- map helpers (shared across command files) ---

func mapVal(m map[string]any, key string) map[string]any {
	if m == nil {
		return nil
	}
	v, ok := m[key]
	if !ok {
		return nil
	}
	if mm, ok := v.(map[string]any); ok {
		return mm
	}
	return nil
}

func mapStr(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	v, ok := m[key]
	if !ok {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	case fmt.Stringer:
		return t.String()
	default:
		return fmt.Sprintf("%v", v)
	}
}

func mapBool(m map[string]any, key string) bool {
	if m == nil {
		return false
	}
	v, ok := m[key]
	if !ok {
		return false
	}
	b, _ := v.(bool)
	return b
}

func mapInt(m map[string]any, key string) int64 {
	if m == nil {
		return 0
	}
	v, ok := m[key]
	if !ok {
		return 0
	}
	switch t := v.(type) {
	case int:
		return int64(t)
	case int64:
		return t
	case float64:
		return int64(t)
	default:
		return 0
	}
}
