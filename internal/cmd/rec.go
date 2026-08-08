package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// registerRec adds the flat rec:* commands.
func registerRec(root *cobra.Command, rt *Runtime) {
	root.AddCommand(
		recStart(rt),
		recStatus(rt),
		recStop(rt),
	)
}

// recStart injects a recorder into a tab and returns a recordId.
func recStart(rt *Runtime) *cobra.Command {
	var to TargetOptions
	var output string
	var mouse, scroll bool
	c := &cobra.Command{
		Use:   "rec:start",
		Short: "Start recording user actions on a tab",
		RunE: func(cmd *cobra.Command, args []string) error {
			a := map[string]any{
				"mouse":  mouse,
				"scroll": scroll,
			}
			to.Merge(a)
			data, err := rt.Call("rec.start", a)
			if err != nil {
				return errExit(err)
			}
			if rt.JSON {
				rt.PrintJSON(data)
				return nil
			}
			fmt.Printf("recording: %s\n", mapStr(data, "recordId"))
			fmt.Printf("tab:       %s\n", fmt.Sprintf("%v", data["tabId"]))
			if output != "" {
				fmt.Println("(rec.stop will write to " + output + ")")
			}
			return nil
		},
	}
	c.Flags().StringVarP(&output, "output", "o", "", "write events to this JSON file on rec:stop")
	c.Flags().BoolVar(&mouse, "mouse", false, "record mouse-down events")
	c.Flags().BoolVar(&scroll, "scroll", false, "record scroll events")
	AddTargetFlags(c, &to)
	return c
}

// recStatus shows active and completed recordings.
func recStatus(rt *Runtime) *cobra.Command {
	return &cobra.Command{
		Use:   "rec:status",
		Short: "Show active and completed recordings",
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := rt.Call("rec.status", nil)
			if err != nil {
				return errExit(err)
			}
			if rt.JSON {
				rt.PrintJSON(data)
				return nil
			}
			active := mapSlice(data, "active")
			fmt.Printf("active:    %d\n", len(active))
			for _, r := range active {
				fmt.Printf("  %s  (tab %s, %vms)\n",
					mapStr(r, "recordId"),
					fmt.Sprintf("%v", r["tabId"]),
					mapInt(r, "duration"))
			}
			// completed is a []string of recordIds; extract from raw data.
			if comp, ok := data["completed"].([]any); ok {
				fmt.Printf("completed: %d\n", len(comp))
				for _, id := range comp {
					if s, ok := id.(string); ok {
						fmt.Printf("  %s\n", s)
					}
				}
			}
			return nil
		},
	}
}

// recStop stops a recording and prints or saves the events.
func recStop(rt *Runtime) *cobra.Command {
	var to TargetOptions
	var recordID, output string
	c := &cobra.Command{
		Use:   "rec:stop",
		Short: "Stop recording and output captured events",
		RunE: func(cmd *cobra.Command, args []string) error {
			a := map[string]any{}
			if recordID != "" {
				a["recordId"] = recordID
			}
			to.Merge(a)
			data, err := rt.Call("rec.stop", a)
			if err != nil {
				return errExit(err)
			}
			// Write to file if --output given (here or at start).
			if output != "" {
				return writeRecording(output, data)
			}
			if rt.JSON {
				rt.PrintJSON(data)
				return nil
			}
			printRecording(data)
			return nil
		},
	}
	c.Flags().StringVar(&recordID, "record-id", "", "recording id from rec:start")
	c.Flags().StringVarP(&output, "output", "o", "", "write events JSON to this file")
	AddTargetFlags(c, &to)
	return c
}

func printRecording(data map[string]any) {
	events := mapSlice(data, "events")
	fmt.Printf("recordId: %s\n", mapStr(data, "recordId"))
	fmt.Printf("events:   %d\n", len(events))
	fmt.Printf("duration: %vms\n\n", mapInt(data, "duration"))
	for i, e := range events {
		fmt.Printf("[%d] %-10s", i+1, mapStr(e, "type"))
		if d := mapVal(e, "detail"); d != nil {
			if tag := mapStr(d, "tag"); tag != "" {
				fmt.Printf("  <%s>", tag)
			}
			if text := mapStr(d, "text"); text != "" {
				fmt.Printf("  %q", text)
			}
			if val := mapStr(d, "value"); val != "" {
				fmt.Printf("  = %q", val)
			}
		}
		fmt.Println()
	}
}

func writeRecording(path string, data map[string]any) error {
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		return err
	}
	fmt.Printf("wrote %d events to %s\n", mapInt(data, "eventCount"), path)
	return nil
}
