package cmd

import (
	"errors"
	"fmt"
	"net/url"

	"github.com/spf13/cobra"
)

type sessionEnvelope struct {
	SchemaVersion int            `json:"schemaVersion"`
	OK            bool           `json:"ok"`
	Data          map[string]any `json:"data"`
}

type sessionCaller interface {
	Call(op string, args map[string]any) (map[string]any, error)
}

func registerSession(root *cobra.Command, rt *Runtime) {
	root.AddCommand(sessionAcquire(rt))
}

func sessionAcquire(rt *Runtime) *cobra.Command {
	var apiURL string
	var interactive bool
	var refresh bool
	c := &cobra.Command{
		Use:   "session:acquire <auth-name>",
		Short: "Acquire browser credentials for a business CLI",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return sessionFailure(rt, ExitInvalidArguments, "invalid_arguments", "", fmt.Errorf("accepts 1 arg(s), received %d", len(args)))
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateSessionRefresh(interactive, refresh); err != nil {
				return sessionFailure(rt, ExitInvalidArguments, "invalid_arguments", args[0], err)
			}
			if err := validateHTTPURL(apiURL); err != nil {
				return sessionFailure(rt, ExitInvalidArguments, "invalid_arguments", args[0], err)
			}
			cfg, err := readAuthConfig(args[0])
			if err != nil {
				return sessionFailure(rt, ExitInvalidArguments, "invalid_auth_config", args[0], err)
			}
			if refresh {
				if err := clearConfiguredAuthCookie(rt, cfg); err != nil {
					return sessionCallFailure(rt, args[0], err)
				}
				if _, err := runLoginLoop(rt, cfg, args[0]); err != nil {
					if ExitCode(err) == ExitLoginTimeout {
						return sessionFailure(rt, ExitLoginTimeout, "login_timeout", args[0], err)
					}
					return sessionCallFailure(rt, args[0], err)
				}
			} else {
				check, err := rt.Call("auth.check", map[string]any{
					"loginUrl":     cfg.LoginURL,
					"loggedInWhen": condToMap(cfg.LoggedInWhen),
				})
				if err != nil {
					return sessionCallFailure(rt, args[0], err)
				}
				if !mapBool(check, "loggedIn") {
					if !interactive {
						return sessionUnavailable(rt, args[0], "login_required")
					}
					if _, err := runLoginLoop(rt, cfg, args[0]); err != nil {
						if ExitCode(err) == ExitLoginTimeout {
							return sessionFailure(rt, ExitLoginTimeout, "login_timeout", args[0], err)
						}
						return sessionCallFailure(rt, args[0], err)
					}
				}
			}

			data, err := rt.Call("cookies.getAll", map[string]any{"url": apiURL})
			if err != nil {
				return sessionCallFailure(rt, args[0], err)
			}
			cookies := mapSlice(data, "cookies")
			if len(cookies) == 0 {
				return sessionUnavailable(rt, args[0], "credentials_unavailable")
			}
			result := sessionEnvelope{
				SchemaVersion: 1,
				OK:            true,
				Data: map[string]any{
					"status":       "credentials_available",
					"profileId":    rt.selectedProfileID(),
					"cookieHeader": cookiesHeader(cookies, false),
				},
			}
			if rt.JSON {
				rt.PrintJSON(result)
			} else {
				fmt.Println(result.Data["cookieHeader"])
			}
			return nil
		},
	}
	c.Flags().StringVar(&apiURL, "url", "", "API origin whose cookies should be acquired (required)")
	c.Flags().BoolVar(&interactive, "interactive", false, "open Chrome and wait for login when required")
	c.Flags().BoolVar(&refresh, "refresh", false, "clear the configured auth cookie before interactive login")
	return c
}

func validateSessionRefresh(interactive, refresh bool) error {
	if refresh && !interactive {
		return fmt.Errorf("--refresh requires --interactive")
	}
	return nil
}

func clearConfiguredAuthCookie(caller sessionCaller, cfg AuthConfig) error {
	if cfg.LoggedInWhen.Cookie == nil || cfg.LoggedInWhen.Cookie.URL == "" || cfg.LoggedInWhen.Cookie.Name == "" {
		return fmt.Errorf("--refresh requires a cookie-based loggedInWhen condition")
	}
	_, err := caller.Call("cookies.remove", map[string]any{
		"url":  cfg.LoggedInWhen.Cookie.URL,
		"name": cfg.LoggedInWhen.Cookie.Name,
	})
	return err
}

func sessionUnavailable(rt *Runtime, authName, status string) error {
	return sessionFailure(rt, ExitAuthRequired, status, authName, errors.New("browser login is required"))
}

func sessionCallFailure(rt *Runtime, authName string, err error) error {
	code := ExitCode(err)
	status := "session_error"
	switch code {
	case ExitHostUnavailable:
		status = "host_unavailable"
	case ExitProfileRequired:
		status = "profile_required"
	case ExitProfileNotFound:
		status = "profile_not_found"
	case ExitExtensionFailure:
		status = "extension_failure"
	}
	return sessionFailure(rt, code, status, authName, err)
}

func sessionFailure(rt *Runtime, code int, status, authName string, err error) error {
	if rt.JSON {
		rt.PrintJSON(sessionEnvelope{
			SchemaVersion: 1,
			OK:            false,
			Data: map[string]any{
				"status":    status,
				"authName":  authName,
				"profileId": rt.selectedProfileID(),
			},
		})
		return SilentExitError(code, err)
	}
	if code == ExitAuthRequired {
		err = fmt.Errorf("%w; run: awc session:acquire %s --url <api-origin> --interactive", err, authName)
	}
	return NewExitError(code, err)
}

func validateHTTPURL(raw string) error {
	u, err := url.ParseRequestURI(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return fmt.Errorf("--url must be an absolute http(s) URL")
	}
	return nil
}
