package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const copilotTokenEndpoint = "https://api.github.com/copilot_internal/v2/token"

var errLockBusy = errors.New("copilot token lock busy")

type copilotAPIToken struct {
	Token     string  `json:"token"`
	ExpiresAt float64 `json:"expires_at"`
}

type copilotTokenRefresher struct {
	mu sync.Mutex

	oauthToken string
	appsHost   string

	configDir string
	tokenFile string
	appsFile  string
	hostsFile string

	token      *copilotAPIToken
	httpClient *http.Client
}

var copilotTokenRefresherOnce sync.Once

func startCopilotTokenMaintenance() {
	copilotTokenRefresherOnce.Do(func() {
		refresher, err := newCopilotTokenRefresher()
		if err != nil {
			LogError(fmt.Errorf("copilot token refresh disabled: %w", err))
			return
		}
		go refresher.Run(context.Background())
	})
}

func newCopilotTokenRefresher() (*copilotTokenRefresher, error) {
	configDir, err := resolveCopilotConfigDir()
	if err != nil {
		return nil, err
	}

	refresher := &copilotTokenRefresher{
		configDir:  configDir,
		tokenFile:  filepath.Join(configDir, "token.json"),
		appsFile:   filepath.Join(configDir, "apps.json"),
		hostsFile:  filepath.Join(configDir, "hosts.json"),
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}

	oauthToken, host, err := refresher.loadOAuthToken()
	if err != nil {
		return nil, err
	}
	refresher.oauthToken = oauthToken
	refresher.appsHost = host

	_ = refresher.loadTokenFromDisk()

	return refresher, nil
}

func (r *copilotTokenRefresher) Run(ctx context.Context) {
	Log("Starting Copilot token refresher")
	for {
		if err := r.refreshIfNeeded(ctx); err != nil {
			LogError(fmt.Errorf("copilot token refresh failed: %w", err))
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Minute):
			}
			continue
		}

		sleep := r.nextSleepDuration()
		select {
		case <-ctx.Done():
			return
		case <-time.After(sleep):
		}
	}
}

func (r *copilotTokenRefresher) refreshIfNeeded(ctx context.Context) error {
	if err := r.loadTokenFromDisk(); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if !r.shouldRefresh() {
		return nil
	}
	return r.refreshToken(ctx)
}

func (r *copilotTokenRefresher) shouldRefresh() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.token == nil {
		return true
	}
	expiresAt := time.Unix(int64(r.token.ExpiresAt), 0).Add(-2 * time.Minute)
	return time.Now().After(expiresAt)
}

func (r *copilotTokenRefresher) nextSleepDuration() time.Duration {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.token == nil {
		return time.Minute
	}
	expiresAt := time.Unix(int64(r.token.ExpiresAt), 0).Add(-2 * time.Minute)
	sleep := time.Until(expiresAt)
	if sleep < time.Minute {
		return time.Minute
	}
	if sleep > 30*time.Minute {
		return 30 * time.Minute
	}
	return sleep
}

func (r *copilotTokenRefresher) refreshToken(ctx context.Context) error {
	if r.oauthToken == "" {
		return errors.New("missing Copilot OAuth token")
	}

	lockPath := r.tokenFile + ".lock"
	if err := acquireFileLock(lockPath); err != nil {
		if errors.Is(err, errLockBusy) {
			_ = r.loadTokenFromDisk()
			return nil
		}
		return err
	}
	defer releaseFileLock(lockPath)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, copilotTokenEndpoint, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "token "+r.oauthToken)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Editor-Plugin-Version", "lsp-client")

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("token refresh failed: %s", resp.Status)
	}

	var token copilotAPIToken
	if err := json.NewDecoder(resp.Body).Decode(&token); err != nil {
		return err
	}
	if token.Token == "" || token.ExpiresAt == 0 {
		return errors.New("received invalid Copilot token response")
	}

	r.mu.Lock()
	r.token = &token
	r.mu.Unlock()

	if err := r.writeTokenFile(token); err != nil {
		return err
	}
	if err := r.updateAppsFile(token); err != nil {
		LogError(fmt.Errorf("failed to update apps.json with Copilot token: %w", err))
	}
	return nil
}

func (r *copilotTokenRefresher) writeTokenFile(token copilotAPIToken) error {
	if err := os.MkdirAll(filepath.Dir(r.tokenFile), 0o700); err != nil {
		return err
	}
	data, err := json.Marshal(token)
	if err != nil {
		return err
	}
	tmp := r.tokenFile + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, r.tokenFile)
}

func (r *copilotTokenRefresher) updateAppsFile(token copilotAPIToken) error {
	data, err := os.ReadFile(r.appsFile)
	if err != nil {
		return err
	}

	entries := map[string]json.RawMessage{}
	if err := json.Unmarshal(data, &entries); err != nil {
		return err
	}

	host := r.appsHost
	if host == "" {
		host = "github.com"
	}

	rawEntry, ok := entries[host]
	if !ok {
		return fmt.Errorf("host %s not found in apps.json", host)
	}

	entry := map[string]interface{}{}
	if err := json.Unmarshal(rawEntry, &entry); err != nil {
		return err
	}

	entry["copilot_token"] = token.Token
	entry["copilot_token_expires_at"] = token.ExpiresAt
	if _, exists := entry["token"]; exists {
		entry["token"] = token.Token
	}
	if _, exists := entry["token_expires_at"]; exists {
		entry["token_expires_at"] = token.ExpiresAt
	}

	updated, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	entries[host] = updated

	output, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return err
	}

	tmp := r.appsFile + ".tmp"
	if err := os.WriteFile(tmp, output, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, r.appsFile)
}

func (r *copilotTokenRefresher) loadOAuthToken() (string, string, error) {
	candidates := []string{r.appsFile, r.hostsFile}
	var fallbackToken string
	var fallbackHost string
	for _, path := range candidates {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var hosts map[string]struct {
			OAuthToken string `json:"oauth_token"`
		}
		if err := json.Unmarshal(data, &hosts); err != nil {
			continue
		}
		for host, entry := range hosts {
			if entry.OAuthToken == "" {
				continue
			}
			if strings.Contains(host, "github.com") {
				return entry.OAuthToken, host, nil
			}
			if fallbackToken == "" {
				fallbackToken = entry.OAuthToken
				fallbackHost = host
			}
		}
	}
	if fallbackToken != "" {
		return fallbackToken, fallbackHost, nil
	}
	return "", "", errors.New("GitHub Copilot OAuth token not found; please authenticate from TextMate first")
}

func (r *copilotTokenRefresher) loadTokenFromDisk() error {
	data, err := os.ReadFile(r.tokenFile)
	if err != nil {
		return err
	}
	var token copilotAPIToken
	if err := json.Unmarshal(data, &token); err != nil {
		return err
	}
	if token.Token == "" || token.ExpiresAt == 0 {
		return errors.New("invalid token.json content")
	}
	r.mu.Lock()
	r.token = &token
	r.mu.Unlock()
	return nil
}

func resolveCopilotConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(home, ".config", "github-copilot"), nil
}

func acquireFileLock(path string) error {
	for i := 0; i < 5; i++ {
		f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL, 0o600)
		if err == nil {
			return f.Close()
		}
		if !errors.Is(err, os.ErrExist) {
			return err
		}
		if isLockFileStale(path, 2*time.Minute) {
			_ = os.Remove(path)
			continue
		}
		time.Sleep(500 * time.Millisecond)
	}
	return errLockBusy
}

func releaseFileLock(path string) {
	_ = os.Remove(path)
}

func isLockFileStale(path string, maxAge time.Duration) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return time.Since(info.ModTime()) > maxAge
}
