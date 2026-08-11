package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// registerAuth adds the flat auth:* commands.
//
// auth:login is configuration-driven: each site has a JSON file in
// ~/.awc/auth/<name>.json describing the login flow (login URL, button
// locator, "logged in" conditions, optional SSO steps). awc reads the config
// and sends it to the extension, which executes the polling loop.
func registerAuth(root *cobra.Command, rt *Runtime) {
	root.AddCommand(
		authLogin(rt),
		authList(rt),
		authConfig(rt),
	)
}

// AuthConfig is the JSON schema for ~/.awc/auth/<name>.json.
type AuthConfig struct {
	LoginURL     string        `json:"loginUrl"`
	LoginButton  AuthLocator   `json:"loginButton"`
	LoggedInWhen AuthCondition `json:"loggedInWhen"`
	SsoSteps     []AuthSsoStep `json:"ssoSteps,omitempty"`
	Timeout      string        `json:"timeout,omitempty"`
	Interval     string        `json:"interval,omitempty"`
}

// AuthLocator describes how to find the login button. Supports text/selector/role.
type AuthLocator struct {
	Text     string `json:"text,omitempty"`
	Selector string `json:"selector,omitempty"`
	Role     string `json:"role,omitempty"`
	Name     string `json:"name,omitempty"`
}

// AuthCondition defines when the session is considered logged in.
// Pick whichever signal is easiest for your site — typically just one is
// needed. All present conditions must be satisfied.
//
// Examples:
//
//	// GitHub: a "logged_in" cookie exists
//	{ "cookie": { "url": "https://github.com", "name": "logged_in" } }
//
//	// Generic: URL no longer on the login page
//	{ "urlNotContains": "/login" }
//
//	// Generic: page no longer shows a "Sign in" button
//	{ "noButtonText": "Sign in" }
type AuthCondition struct {
	URLNotContains string      `json:"urlNotContains,omitempty" msgpack:"urlNotContains,omitempty"`
	NoButtonText   string      `json:"noButtonText,omitempty" msgpack:"noButtonText,omitempty"`
	Cookie         *CookieCond `json:"cookie,omitempty" msgpack:"cookie,omitempty"`
}

// CookieCond checks that a cookie exists (optionally with a specific value).
// This is the most reliable login signal — it does not depend on DOM or CSP.
type CookieCond struct {
	URL   string `json:"url" msgpack:"url"`
	Name  string `json:"name" msgpack:"name"`
	Value string `json:"value,omitempty" msgpack:"value,omitempty"`
}

// AuthSsoStep describes one SSO redirect step (e.g. UUAP passwordless).
type AuthSsoStep struct {
	HostContains string `json:"hostContains"`
	ClickText    string `json:"clickText"`
}

// authLogin reads a config and runs the login flow.
//
// For --check: one-shot — asks the extension to check login status and return.
// For full login: the CLI drives the loop itself, calling auth.check every
// few seconds. This gives the user live feedback ("waiting for you to log
// in...") instead of a silent 120s hang. The extension handles auto-clicking
// login buttons; the CLI handles the human-facing feedback loop.
func authLogin(rt *Runtime) *cobra.Command {
	var checkOnly bool
	c := &cobra.Command{
		Use:   "auth:login <name>",
		Short: "Run a configured login flow (e.g. awc auth:login sysop)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			cfg, err := readAuthConfig(name)
			if err != nil {
				return errExit(err)
			}

			// --check: one-shot status check.
			if checkOnly {
				data, err := rt.Call("auth.check", map[string]any{
					"loginUrl":     cfg.LoginURL,
					"loggedInWhen": condToMap(cfg.LoggedInWhen),
				})
				if err != nil {
					return errExit(err)
				}
				if rt.JSON {
					rt.PrintJSON(data)
					return nil
				}
				printAuthResult(data, true)
				return nil
			}

			// Full login: CLI-driven loop with live feedback.
			data, err := runLoginLoop(rt, cfg, name)
			if err != nil {
				if rt.JSON && data != nil {
					rt.PrintJSON(data)
				}
				return err
			}
			if rt.JSON {
				rt.PrintJSON(data)
			}
			return nil
		},
	}
	c.Flags().BoolVar(&checkOnly, "check", false, "only check login status, do not trigger login")
	return c
}

// condToMap converts AuthCondition to a plain map with the exact lowercase
// keys the extension expects. This avoids any msgpack/Go struct tag mismatch.
func condToMap(c AuthCondition) map[string]any {
	m := map[string]any{}
	if c.URLNotContains != "" {
		m["urlNotContains"] = c.URLNotContains
	}
	if c.NoButtonText != "" {
		m["noButtonText"] = c.NoButtonText
	}
	if c.Cookie != nil {
		cm := map[string]any{
			"url":  c.Cookie.URL,
			"name": c.Cookie.Name,
		}
		if c.Cookie.Value != "" {
			cm["value"] = c.Cookie.Value
		}
		m["cookie"] = cm
	}
	return m
}

// runLoginLoop drives the login flow from the CLI side, giving the user
// live feedback. Each iteration: (1) check status, (2) if not logged in,
// ask the extension to attempt login actions (click buttons/SSO), (3) wait
// and repeat until logged in or timeout.
func runLoginLoop(rt *Runtime, cfg AuthConfig, name string) (map[string]any, error) {
	totalMs := parseDurationMs(orDefault(cfg.Timeout, "120s"))
	intervalMs := parseDurationMs(orDefault(cfg.Interval, "3s"))
	deadline := time.Now().Add(time.Duration(totalMs) * time.Millisecond)
	interval := time.Duration(intervalMs) * time.Millisecond
	progress := os.Stdout
	if rt.JSON {
		progress = os.Stderr
	}

	fmt.Fprintf(progress, "auth: %s - %s\n", name, cfg.LoginURL)

	// Step 1: open the login page and trigger auto-login (click buttons/SSO).
	fmt.Fprint(progress, "  opening login page... ")
	_, err := rt.Call("auth.open", map[string]any{
		"loginUrl":     cfg.LoginURL,
		"loginButton":  cfg.LoginButton,
		"loggedInWhen": condToMap(cfg.LoggedInWhen),
		"ssoSteps":     cfg.SsoSteps,
	})
	if err != nil {
		fmt.Fprintln(progress, "failed")
		return nil, errExit(err)
	}
	fmt.Fprintln(progress, "ok")

	// Step 2: first check — maybe auto-login already worked (SSO, cached session).
	check := func() (map[string]any, error) {
		return rt.Call("auth.check", map[string]any{
			"loginUrl":     cfg.LoginURL,
			"loggedInWhen": cfg.LoggedInWhen,
		})
	}

	data, err := check()
	if err != nil {
		return nil, errExit(err)
	}
	if mapBool(data, "loggedIn") {
		fmt.Fprintln(progress, "\nlogged in")
		return data, nil
	}

	// Step 3: need manual intervention — tell the user.
	fmt.Fprintln(progress, "  not logged in - please complete login in Chrome")
	fmt.Fprintf(progress, "  waiting (timeout %s, checking every %s)...\n",
		formatDuration(time.Duration(totalMs)*time.Millisecond),
		formatDuration(interval))

	for time.Now().Before(deadline) {
		fmt.Fprintf(progress, "\r  waiting for login... (%s remaining) ", formatDuration(time.Until(deadline)))

		time.Sleep(interval)
		data, err := check()
		if err != nil {
			continue // transient errors don't abort the loop
		}
		if mapBool(data, "loggedIn") {
			fmt.Fprintln(progress, "\r\033[Klogged in                                        ")
			return data, nil
		}
	}

	fmt.Fprintln(progress, "\r\033[Ktimed out                                        ")
	return map[string]any{"loggedIn": false, "reason": "timeout"}, NewExitError(ExitLoginTimeout, fmt.Errorf("login timed out"))
}

func formatDuration(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	secs := int(d.Seconds())
	if secs < 60 {
		return fmt.Sprintf("%ds", secs)
	}
	return fmt.Sprintf("%dm%02ds", secs/60, secs%60)
}

// authList lists configured auth profiles.
func authList(rt *Runtime) *cobra.Command {
	return &cobra.Command{
		Use:   "auth:list",
		Short: "List configured login profiles",
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := authConfigDir()
			entries, err := os.ReadDir(dir)
			if err != nil {
				if os.IsNotExist(err) {
					fmt.Printf("(no auth configs in %s)\n", dir)
					return nil
				}
				return errExit(err)
			}
			type profile struct {
				Name     string `json:"name"`
				LoginURL string `json:"loginUrl"`
			}
			var profiles []profile
			for _, e := range entries {
				if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
					continue
				}
				name := strings.TrimSuffix(e.Name(), ".json")
				cfg, _ := readAuthConfig(name)
				profiles = append(profiles, profile{Name: name, LoginURL: cfg.LoginURL})
			}
			if rt.JSON {
				rt.PrintJSON(map[string]any{"profiles": profiles})
				return nil
			}
			if len(profiles) == 0 {
				fmt.Printf("(no auth configs in %s)\n", dir)
				return nil
			}
			fmt.Printf("%-16s %s\n", "NAME", "LOGIN_URL")
			for _, p := range profiles {
				fmt.Printf("%-16s %s\n", p.Name, p.LoginURL)
			}
			return nil
		},
	}
}

// authConfig interactively guides the user through creating a login config.
//
// It is the CLI equivalent of the awc-auth-config skill: instead of requiring
// the user to know cookie names, it inspects the site live and suggests the
// login-state cookie automatically. The flow:
//
//  1. open the login URL
//  2. dump cookies — flag the ones likely to indicate login state
//  3. prompt the user to pick (or accept the suggestion)
//  4. write ~/.awc/auth/<name>.json
//  5. validate with auth:login <name> --check
func authConfig(rt *Runtime) *cobra.Command {
	var loginURL string
	c := &cobra.Command{
		Use:   "auth:config <name>",
		Short: "Interactively configure login for a site (auto-detects the login cookie)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			if loginURL == "" {
				return errExit(fmt.Errorf("pass --url <loginPageUrl> (e.g. --url https://example.com/login)"))
			}
			return runAuthConfig(rt, name, loginURL)
		},
	}
	c.Flags().StringVar(&loginURL, "url", "", "login page URL (required)")
	_ = c.MarkFlagRequired("url")
	return c
}

// runAuthConfig interactively guides the user through creating a login config.
//
// The approach is a cookie DIFF — far more reliable than guessing cookie names:
//
//  1. Read cookies BEFORE login (anonymous baseline).
//  2. Ask the user to log in in Chrome.
//  3. Read cookies AFTER login.
//  4. The NEW or CHANGED cookies are the login-state signal.
//  5. Write config + validate.
//
// This way the user never needs to know cookie names — they just log in as
// they normally would, and awc detects what changed.
func runAuthConfig(rt *Runtime, name, loginURL string) error {
	siteURL := originOf(loginURL)
	fmt.Printf("auth:config — %s\n", name)
	fmt.Printf("  site: %s\n\n", siteURL)

	// ── Step 1: open the site and read baseline cookies ──
	fmt.Print("  step 1/4: opening site... ")
	_, err := rt.Call("tabs.create", map[string]any{"url": loginURL, "active": true})
	if err != nil {
		fmt.Println("failed")
		return errExit(err)
	}
	fmt.Println("ok")
	time.Sleep(2 * time.Second)

	fmt.Print("  step 2/4: reading cookies (before login)... ")
	beforeData, err := rt.Call("cookies.getAll", map[string]any{"url": siteURL})
	if err != nil {
		return errExit(err)
	}
	before := mapSlice(beforeData, "cookies")
	fmt.Printf("%d cookies\n\n", len(before))

	// ── Step 2: wait for the user to log in ──
	fmt.Println("  step 3/4: now log in to the site in Chrome.")
	fmt.Printf("            (use the tab that just opened: %s)\n", loginURL)
	fmt.Println("            press Enter here when you're done.")
	fmt.Print("\n  > ")
	reader := bufio.NewReader(os.Stdin)
	if _, err := reader.ReadString('\n'); err != nil {
		return errExit(err)
	}

	// ── Step 3: read cookies again, compute diff ──
	fmt.Print("\n  reading cookies (after login)... ")
	afterData, err := rt.Call("cookies.getAll", map[string]any{"url": siteURL})
	if err != nil {
		return errExit(err)
	}
	after := mapSlice(afterData, "cookies")
	fmt.Printf("%d cookies\n\n", len(after))

	// Find new cookies and existing cookies whose values changed during login.
	diffs := diffCookies(before, after)

	if len(diffs) == 0 {
		fmt.Println("  ⚠ no new or changed cookies detected after login.")
		fmt.Println("    you may already have been logged in, or the login failed.")
		fmt.Println("    try logging out first, then re-run this command.")
		fmt.Println()
		return nil
	}

	// ── Step 4: pick the login cookie and write config ──
	fmt.Printf("  step 4/4: %d cookie candidate(s) detected after login:\n\n", len(diffs))
	for i, d := range diffs {
		fmt.Printf("    [%d] %s  (%s, domain: %s)\n", i+1, d.Name, d.Status, d.Domain)
	}

	// Auto-pick: prefer session/token cookies, fall back to first.
	picked := diffs[0]
	for _, d := range diffs {
		lower := strings.ToLower(d.Name)
		if strings.Contains(lower, "sess") || strings.Contains(lower, "token") || strings.Contains(lower, "auth") {
			picked = d
			break
		}
	}

	// The cookie URL must carry the same origin (scheme + host + port) used to
	// read the cookies above, so chrome.cookies.get hits the same store entry.
	// Reuse siteURL (= originOf(loginURL)) rather than reconstructing from the
	// cookie's domain: cookie domains carry no port and we'd otherwise hardcode
	// https, which breaks http + non-default-port sites (e.g. localhost:3001).
	cookieURL := siteURL
	fmt.Printf("  ✓ detected login cookie: %s\n\n", picked.Name)

	// Write config.
	cfgPath := filepath.Join(authConfigDir(), name+".json")
	cfg := map[string]any{
		"loginUrl": loginURL,
		"loggedInWhen": map[string]any{
			"cookie": map[string]any{
				"url":  cookieURL,
				"name": picked.Name,
			},
		},
	}
	if err := os.MkdirAll(authConfigDir(), 0o700); err != nil {
		return errExit(err)
	}
	if err := os.Chmod(authConfigDir(), 0o700); err != nil {
		return errExit(err)
	}
	b, _ := json.MarshalIndent(cfg, "", "  ")
	if err := os.WriteFile(cfgPath, append(b, '\n'), 0o600); err != nil {
		return errExit(err)
	}
	if err := os.Chmod(cfgPath, 0o600); err != nil {
		return errExit(err)
	}
	fmt.Printf("  ✓ wrote %s\n\n", cfgPath)

	// Validate.
	fmt.Print("  validating... ")
	checkData, err := rt.Call("auth.check", map[string]any{
		"loginUrl":     loginURL,
		"loggedInWhen": cfg["loggedInWhen"],
	})
	if err != nil {
		fmt.Println("error (verify later: awc auth:login " + name + " --check)")
		return nil
	}
	if mapBool(checkData, "loggedIn") {
		fmt.Println("logged in ✓ — config is correct!")
	} else {
		fmt.Println("not logged in ✗ — config saved, verify: awc auth:login " + name + " --check")
	}
	return nil
}

type cookieDiff struct {
	Name   string
	Domain string
	Status string // "new" or "changed"
}

// diffCookies compares cookie values in memory. Values are never returned,
// logged, or persisted; only the cookie identity and change status escape.
func diffCookies(before, after []map[string]any) []cookieDiff {
	values := make(map[string]string, len(before))
	for _, c := range before {
		values[cookieIdentity(c)] = mapStr(c, "value")
	}
	var diffs []cookieDiff
	for _, c := range after {
		old, existed := values[cookieIdentity(c)]
		status := ""
		if !existed {
			status = "new"
		} else if old != mapStr(c, "value") {
			status = "changed"
		}
		if status != "" {
			diffs = append(diffs, cookieDiff{
				Name:   mapStr(c, "name"),
				Domain: mapStr(c, "domain"),
				Status: status,
			})
		}
	}
	return diffs
}

func cookieIdentity(c map[string]any) string {
	return strings.Join([]string{
		mapStr(c, "domain"),
		mapStr(c, "path"),
		mapStr(c, "name"),
		mapStr(c, "storeId"),
	}, "\x00")
}

// originOf extracts the origin (scheme + host) from a URL.
func originOf(rawURL string) string {
	if idx := strings.Index(rawURL, "://"); idx > 0 {
		rest := rawURL[idx+3:]
		if slash := strings.Index(rest, "/"); slash > 0 {
			return rawURL[:idx+3+slash]
		}
		return rawURL
	}
	return rawURL
}

// maskValue truncates a cookie value for display.
func maskValue(v string) string {
	if len(v) <= 20 {
		return v
	}
	return v[:10] + "…" + fmt.Sprintf(" (%d chars)", len(v))
}

func printAuthResult(data map[string]any, checkOnly bool) {
	loggedIn := mapBool(data, "loggedIn")
	status := mapStr(data, "status")
	if checkOnly {
		if loggedIn {
			fmt.Println("logged in ✓")
		} else {
			fmt.Println("not logged in ✗")
		}
		return
	}
	if loggedIn {
		fmt.Println("login: ok ✓")
	} else {
		reason := mapStr(data, "reason")
		if reason == "" {
			reason = "unknown"
		}
		fmt.Printf("login: failed (%s)\n", reason)
	}
	if status != "" {
		fmt.Printf("status: %s\n", status)
	}
	if t := mapStr(data, "tab"); t != "" {
		fmt.Printf("tab:    %s\n", t)
	}
}

// --- config file helpers ---

// authConfigDir returns the auth config directory, honoring AWC_AUTH_DIR.
func authConfigDir() string {
	if d := os.Getenv("AWC_AUTH_DIR"); d != "" {
		return d
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".awc", "auth")
}

// readAuthConfig reads ~/.awc/auth/<name>.json.
func readAuthConfig(name string) (AuthConfig, error) {
	var cfg AuthConfig
	path := filepath.Join(authConfigDir(), name+".json")
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, fmt.Errorf("auth config not found: %s\n  create it at: %s\n  or see: awc auth:list", name, path)
		}
		return cfg, err
	}
	if err := json.Unmarshal(b, &cfg); err != nil {
		return cfg, fmt.Errorf("parse %s: %w", path, err)
	}
	if cfg.LoginURL == "" {
		return cfg, fmt.Errorf("auth config %s: loginUrl is required", name)
	}
	return cfg, nil
}

func orDefault(s, def string) string {
	if strings.TrimSpace(s) == "" {
		return def
	}
	return s
}
