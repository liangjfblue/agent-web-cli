package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// registerCdp adds the flat cdp:* commands.
func registerCdp(root *cobra.Command, rt *Runtime) {
	root.AddCommand(
		cdpSend(rt),
		cdpListen(rt),
	)
}

// cdpSend sends a single CDP method and prints the result.
func cdpSend(rt *Runtime) *cobra.Command {
	var to TargetOptions
	var params, paramsFile string
	c := &cobra.Command{
		Use:   "cdp:send <method>",
		Short: "Send one Chrome DevTools Protocol command",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			a := map[string]any{
				"method": args[0],
			}
			p, err := loadParams(params, paramsFile)
			if err != nil {
				return errExit(err)
			}
			if p != nil {
				a["params"] = p
			}
			to.Merge(a)
			data, err := rt.Call("cdp.send", a)
			if err != nil {
				return errExit(err)
			}
			if rt.JSON {
				rt.PrintJSON(data)
				return nil
			}
			fmt.Printf("method: %s\n", args[0])
			if r := mapVal(data, "result"); r != nil {
				rt.PrintJSON(r)
			}
			return nil
		},
	}
	c.Flags().StringVar(&params, "params", "", "JSON params for the method")
	c.Flags().StringVar(&paramsFile, "params-file", "", "read JSON params from this file")
	AddTargetFlags(c, &to)
	return c
}

// cdpListen attaches CDP, enables domains, and collects events for a duration.
func cdpListen(rt *Runtime) *cobra.Command {
	var to TargetOptions
	var duration string
	var events, enables []string
	c := &cobra.Command{
		Use:   "cdp:listen",
		Short: "Listen to Chrome DevTools Protocol events",
		RunE: func(cmd *cobra.Command, args []string) error {
			a := map[string]any{
				"durationMs": parseDurationMs(duration),
				"events":     events,
			}
			if len(enables) > 0 {
				a["enables"] = enables
			}
			to.Merge(a)
			data, err := rt.Call("cdp.listen", a)
			if err != nil {
				return errExit(err)
			}
			if rt.JSON {
				rt.PrintJSON(data)
				return nil
			}
			evs := mapSlice(data, "events")
			fmt.Printf("captured: %d events\n\n", len(evs))
			for _, e := range evs {
				fmt.Printf("%s\n", mapStr(e, "method"))
				if p := mapVal(e, "params"); p != nil {
					rt.PrintJSON(p)
				}
			}
			return nil
		},
	}
	c.Flags().StringVar(&duration, "duration", "5s", "listen duration")
	c.Flags().StringArrayVar(&events, "event", nil, "event pattern (e.g. Network.*, Page.loadEventFired). Repeatable")
	c.Flags().StringArrayVar(&enables, "enable", nil, "CDP enable command (e.g. Network.enable). Repeatable; auto-inferred if omitted")
	AddTargetFlags(c, &to)
	return c
}

// loadParams parses --params JSON or reads --params-file.
func loadParams(params, paramsFile string) (map[string]any, error) {
	if params != "" {
		var p map[string]any
		if err := json.Unmarshal([]byte(params), &p); err != nil {
			return nil, fmt.Errorf("parse --params: %w", err)
		}
		return p, nil
	}
	if paramsFile != "" {
		b, err := os.ReadFile(paramsFile)
		if err != nil {
			return nil, fmt.Errorf("read --params-file: %w", err)
		}
		var p map[string]any
		if err := json.Unmarshal(b, &p); err != nil {
			return nil, fmt.Errorf("parse --params-file: %w", err)
		}
		return p, nil
	}
	return nil, nil
}
