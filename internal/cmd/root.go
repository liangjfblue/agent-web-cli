package cmd

import (
	"strings"

	"github.com/spf13/cobra"
)

// Version is overridden at link time via -ldflags in release builds.
var Version = "0.1.1-dev"

// NewRootCmd builds the full command tree.
//
// Commands use a flat colon-separated namespace: "sys:status", "cookies:get",
// "tabs:open", etc. cobra matches these as ordinary command names.
func NewRootCmd(rt *Runtime) *cobra.Command {
	root := &cobra.Command{
		Use:           "awc",
		Short:         "Agent Web CLI — drive Chrome from the command line",
		Version:       Version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	rt.rootFlags(root)

	// Register every command as a flat top-level entry.
	for _, add := range registry {
		add(root, rt)
	}
	return root
}

// cmdAdder registers one flat command onto the root.
type cmdAdder func(root *cobra.Command, rt *Runtime)

// registry is the single place where commands are declared. To add a new
// group, append an adder here.
var registry []cmdAdder

func init() {
	registry = append(registry,
		registerSys,
		registerCookies,
		registerTabs,
		registerShot,
		registerDom,
		registerNet,
		registerConsole,
		registerCdp,
		registerWait,
		registerRec,
		registerProfiles,
		registerAuth,
		registerSession,
	)
}

// splitColon returns the action part of a "group:action" command name.
// Used by commands that want to validate their group prefix in help text.
func splitColon(use string) (group, action string) {
	parts := strings.SplitN(use, ":", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return use, ""
}
