package ipc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ProfileRegistration describes one currently connected Chrome profile.
// It contains routing metadata only; credentials are never persisted here.
type ProfileRegistration struct {
	ProfileID   string `json:"profileId"`
	ProfileName string `json:"profileName,omitempty"`
	Endpoint    string `json:"endpoint"`
	PID         int    `json:"pid"`
	Version     string `json:"version"`
}

func ensureRuntimeDir() error {
	if err := os.MkdirAll(RuntimeDir(), 0o700); err != nil {
		return err
	}
	return os.Chmod(RuntimeDir(), 0o700)
}

func registrationPath(profileID string) string {
	return filepath.Join(RuntimeDir(), "profile-"+sanitize(profileID)+".json")
}

// RegisterProfile publishes a profile endpoint using an atomic rename.
func RegisterProfile(p ProfileRegistration) error {
	if p.ProfileID == "" || p.Endpoint == "" {
		return errors.New("profile id and endpoint are required")
	}
	if err := ensureRuntimeDir(); err != nil {
		return err
	}
	b, err := json.Marshal(p)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(RuntimeDir(), ".profile-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(append(b, '\n')); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	target := registrationPath(p.ProfileID)
	_ = os.Remove(target)
	return os.Rename(tmpName, target)
}

// UnregisterProfile removes only the registration owned by the closing host.
func UnregisterProfile(profileID, endpoint string) error {
	path := registrationPath(profileID)
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var p ProfileRegistration
	if json.Unmarshal(b, &p) == nil && p.Endpoint != endpoint {
		return nil
	}
	err = os.Remove(path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// ActiveProfiles returns reachable registered hosts and removes stale files.
func ActiveProfiles(ctx context.Context) ([]ProfileRegistration, error) {
	entries, err := os.ReadDir(RuntimeDir())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var profiles []ProfileRegistration
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), "profile-") || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		path := filepath.Join(RuntimeDir(), entry.Name())
		b, readErr := os.ReadFile(path)
		var p ProfileRegistration
		if readErr != nil || json.Unmarshal(b, &p) != nil || p.ProfileID == "" || p.Endpoint == "" {
			_ = os.Remove(path)
			continue
		}
		probeCtx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
		conn, dialErr := dial(probeCtx, p.Endpoint)
		cancel()
		if dialErr != nil {
			_ = os.Remove(path)
			continue
		}
		_ = conn.Close()
		profiles = append(profiles, p)
	}
	sort.Slice(profiles, func(i, j int) bool { return profiles[i].ProfileID < profiles[j].ProfileID })
	return profiles, nil
}

// SelectProfile resolves either profile id or display name.
func SelectProfile(profiles []ProfileRegistration, selector string) (ProfileRegistration, error) {
	var matches []ProfileRegistration
	for _, p := range profiles {
		if p.ProfileID == selector || (p.ProfileName != "" && p.ProfileName == selector) {
			matches = append(matches, p)
		}
	}
	if len(matches) == 0 {
		return ProfileRegistration{}, fmt.Errorf("browser profile %q is not connected", selector)
	}
	if len(matches) > 1 {
		return ProfileRegistration{}, fmt.Errorf("browser profile name %q is ambiguous; use a profile id", selector)
	}
	return matches[0], nil
}
