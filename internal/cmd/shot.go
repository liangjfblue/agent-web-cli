package cmd

import (
	"encoding/base64"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

// registerShot adds the flat shot:* commands.
func registerShot(root *cobra.Command, rt *Runtime) {
	root.AddCommand(shotPage(rt))
}

func shotPage(rt *Runtime) *cobra.Command {
	var output string
	var tabID string
	c := &cobra.Command{
		Use:   "shot:page",
		Short: "Capture the visible area of a tab",
		RunE: func(cmd *cobra.Command, args []string) error {
			args_ := map[string]any{}
			if tabID != "" {
				args_["tabId"] = tabID
			}
			data, err := rt.Call("screenshot.capture", args_)
			if err != nil {
				return errExit(err)
			}
			if rt.JSON {
				rt.PrintJSON(data)
				return nil
			}
			b64 := mapStr(data, "dataUrl")
			if output == "" {
				// Print as a data URL if no file is requested.
				fmt.Println(b64)
				return nil
			}
			return writeDataURL(output, b64)
		},
	}
	c.Flags().StringVarP(&output, "output", "o", "", "write PNG to this path")
	c.Flags().StringVar(&tabID, "tab-id", "", "target tab id (default: active tab)")
	return c
}

// writeDataURL decodes a "data:image/png;base64,...." URL into a binary file.
func writeDataURL(path, dataURL string) error {
	const prefix = "data:image/png;base64,"
	payload := dataURL
	if idx := strings.Index(dataURL, ","); idx >= 0 {
		payload = dataURL[idx+1:]
	}
	raw, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		return fmt.Errorf("decode screenshot data url: %w", err)
	}
	return os.WriteFile(path, raw, 0o644)
}
