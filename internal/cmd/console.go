package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// registerConsole adds the flat console:* commands.
func registerConsole(root *cobra.Command, rt *Runtime) {
	root.AddCommand(
		consoleWatch(rt),
		consoleClear(rt),
	)
}

func consoleWatch(rt *Runtime) *cobra.Command {
	var to TargetOptions
	var duration, level, text string
	c := &cobra.Command{
		Use:   "console:watch",
		Short: "Capture page console output for a duration",
		RunE: func(cmd *cobra.Command, args []string) error {
			a := map[string]any{}
			a["durationMs"] = parseDurationMs(duration)
			if level != "" {
				a["level"] = level
			}
			to.Merge(a)
			data, err := rt.Call("console.watch", a)
			if err != nil {
				return errExit(err)
			}
			if rt.JSON {
				rt.PrintJSON(data)
				return nil
			}
			events := mapSlice(data, "events")
			fmt.Printf("captured: %d events\n\n", len(events))
			for _, e := range events {
				fmt.Printf("[%s] %s\n", mapStr(e, "level"), mapStr(e, "text"))
			}
			return nil
		},
	}
	c.Flags().StringVar(&duration, "duration", "5s", "capture duration")
	c.Flags().StringVar(&level, "level", "all", "filter: all|error|warn|info|log")
	c.Flags().StringVar(&text, "text", "", "(reserved) filter by text")
	AddTargetFlags(c, &to)
	return c
}

func consoleClear(rt *Runtime) *cobra.Command {
	var to TargetOptions
	c := &cobra.Command{
		Use:   "console:clear",
		Short: "Clear the page console",
		RunE: func(cmd *cobra.Command, args []string) error {
			a := map[string]any{}
			to.Merge(a)
			data, err := rt.Call("console.clear", a)
			if err != nil {
				return errExit(err)
			}
			if rt.JSON {
				rt.PrintJSON(data)
				return nil
			}
			fmt.Println("console cleared")
			return nil
		},
	}
	AddTargetFlags(c, &to)
	return c
}
