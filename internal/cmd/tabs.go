package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// registerTabs adds the flat tabs:* commands.
func registerTabs(root *cobra.Command, rt *Runtime) {
	root.AddCommand(
		tabsList(rt),
		tabsOpen(rt),
		tabsFocus(rt),
	)
}

func tabsList(rt *Runtime) *cobra.Command {
	return &cobra.Command{
		Use:   "tabs:list",
		Short: "List open tabs",
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := rt.Call("tabs.query", nil)
			if err != nil {
				return errExit(err)
			}
			if rt.JSON {
				rt.PrintJSON(data)
				return nil
			}
			tabs := mapSlice(data, "tabs")
			printTabsTable(tabs)
			return nil
		},
	}
}

func tabsOpen(rt *Runtime) *cobra.Command {
	var foreground bool
	c := &cobra.Command{
		Use:   "tabs:open <url>",
		Short: "Open or reuse a tab for a URL",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := rt.Call("tabs.create", map[string]any{
				"url":       args[0],
				"active":    foreground,
				"reuse":     true,
			})
			if err != nil {
				return errExit(err)
			}
			if rt.JSON {
				rt.PrintJSON(data)
				return nil
			}
			tab := mapVal(data, "tab")
			fmt.Printf("tab:  %s\n", mapStr(tab, "title"))
			fmt.Printf("url:  %s\n", mapStr(tab, "url"))
			fmt.Printf("id:   %d\n", mapInt(tab, "id"))
			return nil
		},
	}
	c.Flags().BoolVar(&foreground, "foreground", false, "activate the tab and focus its window")
	return c
}

func tabsFocus(rt *Runtime) *cobra.Command {
	return &cobra.Command{
		Use:   "tabs:focus <tab-id>",
		Short: "Activate a tab",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := rt.Call("tabs.update", map[string]any{
				"tabId":  args[0],
				"active": true,
			})
			if err != nil {
				return errExit(err)
			}
			if rt.JSON {
				rt.PrintJSON(data)
				return nil
			}
			tab := mapVal(data, "tab")
			fmt.Printf("focused: %s\n", mapStr(tab, "title"))
			return nil
		},
	}
}

func printTabsTable(tabs []map[string]any) {
	if len(tabs) == 0 {
		fmt.Println("(no tabs)")
		return
	}
	fmt.Printf("%-8s %-4s %-30s %s\n", "ID", "ACT", "TITLE", "URL")
	for _, t := range tabs {
		marker := " "
		if mapBool(t, "active") {
			marker = "*"
		}
		fmt.Printf("%-8s %-4s %-30s %s\n",
			fmt.Sprintf("%d", mapInt(t, "id")),
			marker,
			truncate(mapStr(t, "title"), 30),
			truncate(mapStr(t, "url"), 50))
	}
}
