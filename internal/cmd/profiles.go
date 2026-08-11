package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/agent/web-cli/internal/ipc"
	"github.com/spf13/cobra"
)

// registerProfiles adds the flat profiles:* commands.
//
// Multi-profile support works in two layers:
//   - profile.identity / profile.rename — the extension reports a stable id
//     generated per Chrome user-profile (stored in chrome.storage.local).
//   - profiles:default — the CLI persists the chosen profile selector in
//     ~/.awc/config.json so subsequent commands target it automatically.
//
// The host publishes one private endpoint per connected Chrome profile; these
// commands expose discovery, naming, and default selection to the user.
func registerProfiles(root *cobra.Command, rt *Runtime) {
	root.AddCommand(
		profilesList(rt),
		profilesRename(rt),
		profilesDefault(rt),
	)
}

// profilesList queries status.get (which carries the connected profile) and
// prints it. With a single host connected it shows one profile; the structure
// accommodates future multi-host discovery.
func profilesList(rt *Runtime) *cobra.Command {
	return &cobra.Command{
		Use:   "profiles:list",
		Short: "List connected browser profiles",
		RunE: func(cmd *cobra.Command, args []string) error {
			profiles, err := ipc.ActiveProfiles(context.Background())
			if err != nil {
				return errExit(err)
			}
			defaultProfile := ""
			if cfg := readConfig(); cfg != nil {
				defaultProfile = cfg.DefaultProfile
			}
			if rt.JSON {
				rt.PrintJSON(map[string]any{"profiles": profiles, "defaultProfile": defaultProfile})
				return nil
			}
			fmt.Printf("%-16s %-20s %-12s %s\n", "PROFILE_ID", "NAME", "VERSION", "PID")
			for _, prof := range profiles {
				fmt.Printf("%-16s %-20s %-12s %d\n", prof.ProfileID, prof.ProfileName, prof.Version, prof.PID)
			}
			if defaultProfile != "" {
				fmt.Printf("\ndefault: %s\n", defaultProfile)
			}
			return nil
		},
	}
}

// profilesRename sets the display name on the connected profile.
func profilesRename(rt *Runtime) *cobra.Command {
	return &cobra.Command{
		Use:   "profiles:rename <name>",
		Short: "Set a display name for the connected browser profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := rt.Call("profile.rename", map[string]any{"name": args[0]})
			if err != nil {
				return errExit(err)
			}
			if rt.JSON {
				rt.PrintJSON(data)
				return nil
			}
			fmt.Printf("renamed: %s -> %s\n", mapStr(data, "profileId"), mapStr(data, "profileName"))
			return nil
		},
	}
}

// profilesDefault reads or writes the persisted default profile selector.
func profilesDefault(rt *Runtime) *cobra.Command {
	var clear bool
	c := &cobra.Command{
		Use:   "profiles:default [selector]",
		Short: "Show or set the default browser profile",
		RunE: func(cmd *cobra.Command, args []string) error {
			if clear {
				cfg := readConfig()
				if cfg != nil {
					cfg.DefaultProfile = ""
					writeConfig(cfg)
				}
				fmt.Println("default profile cleared")
				return nil
			}
			if len(args) == 0 {
				cfg := readConfig()
				if cfg == nil || cfg.DefaultProfile == "" {
					fmt.Println("(no default profile set)")
				} else {
					fmt.Println(cfg.DefaultProfile)
				}
				return nil
			}
			cfg := readConfigOrCreate()
			cfg.DefaultProfile = args[0]
			if err := writeConfig(cfg); err != nil {
				return errExit(err)
			}
			fmt.Printf("default profile set: %s\n", args[0])
			return nil
		},
	}
	c.Flags().BoolVar(&clear, "clear", false, "clear the default profile")
	return c
}

// --- config persistence (~/.awc/config.json) ---

type awcConfig struct {
	DefaultProfile string `json:"defaultProfile,omitempty"`
}

func configPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".awc", "config.json")
}

func readConfig() *awcConfig {
	b, err := os.ReadFile(configPath())
	if err != nil {
		return nil
	}
	var cfg awcConfig
	if err := json.Unmarshal(b, &cfg); err != nil {
		return nil
	}
	return &cfg
}

func readConfigOrCreate() *awcConfig {
	if cfg := readConfig(); cfg != nil {
		return cfg
	}
	return &awcConfig{}
}

func writeConfig(cfg *awcConfig) error {
	dir := filepath.Dir(configPath())
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(configPath(), append(b, '\n'), 0o600); err != nil {
		return err
	}
	return os.Chmod(configPath(), 0o600)
}
