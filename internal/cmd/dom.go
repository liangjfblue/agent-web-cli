package cmd

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
)

// registerDom adds the flat dom:* commands.
func registerDom(root *cobra.Command, rt *Runtime) {
	root.AddCommand(
		domSnapshot(rt),
		domClick(rt),
		domType(rt),
		domQuery(rt),
		domText(rt),
	)
}

// domSnapshot walks actionable elements and prints each with its anchor.
func domSnapshot(rt *Runtime) *cobra.Command {
	var lo LocatorOptions
	var to TargetOptions
	var includeHidden bool
	c := &cobra.Command{
		Use:   "dom:snapshot",
		Short: "List actionable elements with anchors",
		RunE: func(cmd *cobra.Command, args []string) error {
			a := map[string]any{}
			if includeHidden {
				a["includeHidden"] = true
			}
			to.Merge(a)
			data, err := rt.Call("dom.snapshot", a)
			if err != nil {
				return errExit(err)
			}
			if rt.JSON {
				rt.PrintJSON(data)
				return nil
			}
			printSnapshot(data)
			return nil
		},
	}
	c.Flags().BoolVar(&includeHidden, "include-hidden", false, "include hidden elements")
	AddTargetFlags(c, &to)
	_ = lo
	return c
}

// domClick clicks an element located by anchor/selector/semantic matchers.
func domClick(rt *Runtime) *cobra.Command {
	var lo LocatorOptions
	var to TargetOptions
	c := &cobra.Command{
		Use:   "dom:click",
		Short: "Click an element",
		RunE: func(cmd *cobra.Command, args []string) error {
			a := lo.ToArgs()
			to.Merge(a)
			data, err := rt.Call("dom.click", a)
			if err != nil {
				return errExit(err)
			}
			if rt.JSON {
				rt.PrintJSON(data)
				return nil
			}
			fmt.Printf("clicked: %s\n", mapStr(data, "tag"))
			if t := mapStr(data, "text"); t != "" {
				fmt.Printf("  text:  %s\n", t)
			}
			return nil
		},
	}
	AddLocatorFlags(c, &lo)
	AddTargetFlags(c, &to)
	return c
}

// domType focuses an element and sets its value.
func domType(rt *Runtime) *cobra.Command {
	var lo LocatorOptions
	var to TargetOptions
	var value string
	c := &cobra.Command{
		Use:   "dom:type",
		Short: "Type a value into an input element",
		RunE: func(cmd *cobra.Command, args []string) error {
			a := lo.ToArgs()
			a["value"] = value
			to.Merge(a)
			data, err := rt.Call("dom.type", a)
			if err != nil {
				return errExit(err)
			}
			if rt.JSON {
				rt.PrintJSON(data)
				return nil
			}
			fmt.Printf("typed into: %s\n", mapStr(data, "tag"))
			return nil
		},
	}
	c.Flags().StringVar(&value, "value", "", "value to type (required)")
	_ = c.MarkFlagRequired("value")
	AddLocatorFlags(c, &lo)
	AddTargetFlags(c, &to)
	return c
}

// domQuery returns matching element summaries without acting on them.
func domQuery(rt *Runtime) *cobra.Command {
	var lo LocatorOptions
	var to TargetOptions
	var limit int
	c := &cobra.Command{
		Use:   "dom:query",
		Short: "Find elements without acting on them",
		RunE: func(cmd *cobra.Command, args []string) error {
			a := lo.ToArgs()
			if limit > 0 {
				a["limit"] = limit
			}
			to.Merge(a)
			data, err := rt.Call("dom.query", a)
			if err != nil {
				return errExit(err)
			}
			if rt.JSON {
				rt.PrintJSON(data)
				return nil
			}
			printQueryResult(data)
			return nil
		},
	}
	c.Flags().IntVar(&limit, "limit", 50, "max items to return")
	AddLocatorFlags(c, &lo)
	AddTargetFlags(c, &to)
	return c
}

// domText reads the text content of an element or the whole body.
func domText(rt *Runtime) *cobra.Command {
	var lo LocatorOptions
	var to TargetOptions
	c := &cobra.Command{
		Use:   "dom:text",
		Short: "Read text from an element or the page body",
		RunE: func(cmd *cobra.Command, args []string) error {
			a := map[string]any{}
			// dom:text uses --selector only (no anchor needed for text).
			if lo.Selector != "" {
				a["selector"] = lo.Selector
			}
			to.Merge(a)
			data, err := rt.Call("dom.text", a)
			if err != nil {
				return errExit(err)
			}
			if rt.JSON {
				rt.PrintJSON(data)
				return nil
			}
			fmt.Println(mapStr(data, "text"))
			return nil
		},
	}
	c.Flags().StringVar(&lo.Selector, "selector", "", "read text of this element (default: body)")
	AddTargetFlags(c, &to)
	return c
}

// --- formatters ---

func printSnapshot(data map[string]any) {
	elements := mapSlice(data, "elements")
	if len(elements) == 0 {
		fmt.Println("(no actionable elements)")
		return
	}
	fmt.Printf("snapshot: %s  (%d elements)\n", mapStr(data, "snapshotHash"), mapInt(data, "count"))
	fmt.Printf("%-12s %-14s %-10s %-24s %s\n", "ANCHOR", "TAG", "ROLE", "TEXT", "SELECTOR")
	for _, e := range elements {
		fmt.Printf("%-12s %-14s %-10s %-24s %s\n",
			mapStr(e, "anchor"),
			truncate(mapStr(e, "tag"), 14),
			truncate(mapStr(e, "role"), 10),
			truncate(mapStr(e, "text"), 24),
			truncate(mapStr(e, "selector"), 40))
	}
}

func printQueryResult(data map[string]any) {
	items := mapSlice(data, "items")
	count := mapInt(data, "count")
	fmt.Printf("found: %d (showing %d)\n\n", count, len(items))
	fmt.Printf("%-12s %-10s %-20s %s\n", "TAG", "ROLE", "TEXT", "SELECTOR")
	for _, it := range items {
		fmt.Printf("%-12s %-10s %-20s %s\n",
			truncate(mapStr(it, "tag"), 12),
			truncate(mapStr(it, "role"), 10),
			truncate(mapStr(it, "text"), 20),
			truncate(mapStr(it, "selector"), 30))
	}
}

// keep strconv referenced for future numeric locator work
var _ = strconv.Itoa
