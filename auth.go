package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"go.bug.st/json"
)

// TerminalAuth handles terminal-based authentication flow
type TerminalAuth struct {
	client *handler
}

// NewTerminalAuth creates a new terminal authentication handler
func NewTerminalAuth(client *handler) *TerminalAuth {
	return &TerminalAuth{client: client}
}

// CheckAndPerformAuth checks authentication status and performs login if needed
func (ta *TerminalAuth) CheckAndPerformAuth() error {
	Log("Checking Copilot authentication status...")

	// First check if already authenticated
	if ta.isAuthenticated() {
		Log("Already authenticated with GitHub Copilot")
		return nil
	}

	Log("Not authenticated. Starting authentication flow...")
	return ta.performTerminalAuth()
}

// isAuthenticated checks if the user is currently authenticated
func (ta *TerminalAuth) isAuthenticated() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn := ta.client.lsc.GetConnection()
	resp := sendRequest("checkStatus", KeyValue{}, conn, ctx)

	var res checkStatusResponse
	if err := json.Unmarshal(resp, &res); err != nil {
		LogError(err)
		return false
	}

	return res.Status != "NotAuthorized" && res.Status != ""
}

// performTerminalAuth performs the complete authentication flow in terminal
func (ta *TerminalAuth) performTerminalAuth() error {
	ctx := context.Background()
	conn := ta.client.lsc.GetConnection()

	// Step 1: Initiate sign-in
	resp := sendRequest("signInInitiate", KeyValue{}, conn, ctx)
	var signInResp signInResponse
	if err := json.Unmarshal(resp, &signInResp); err != nil {
		return fmt.Errorf("failed to parse sign-in response: %w", err)
	}

	if signInResp.Status == "AlreadySignedIn" {
		Log("Already signed in as %s", signInResp.User)
		return nil
	}

	if signInResp.UserCode == "" || signInResp.VerificationUri == "" {
		return fmt.Errorf("invalid sign-in response: missing user code or verification URI")
	}

	// Step 2: Display instructions to user
	fmt.Printf("\n" + hiGreenString("=== GitHub Copilot Authentication ===") + "\n\n")
	fmt.Printf("Your one-time code: %s\n\n", hiYellowString(signInResp.UserCode))
	fmt.Printf("Instructions:\n")
	fmt.Printf("1. Copy the code above: %s\n", hiYellowString(signInResp.UserCode))
	fmt.Printf("2. Open this URL in your browser: %s\n", hiBlueString(signInResp.VerificationUri))
	fmt.Printf("3. Paste the code and authorize the application\n\n")

	// Step 3: Try to open browser automatically
	if err := ta.openBrowser(signInResp.VerificationUri); err != nil {
		Log("Could not open browser automatically: %v", err)
	} else {
		fmt.Printf("Opening browser automatically...\n\n")
	}

	// Step 4: Wait for user confirmation
	fmt.Printf("Press ENTER after you have completed the authorization in your browser...")
	reader := bufio.NewReader(os.Stdin)
	reader.ReadLine()

	// Step 5: Confirm authentication
	fmt.Printf("Verifying authentication...\n")
	confirmResp := sendRequest("signInConfirm", KeyValue{"userCode": signInResp.UserCode}, conn, ctx)
	var confirmResult signInConfirmResponse
	if err := json.Unmarshal(confirmResp, &confirmResult); err != nil {
		return fmt.Errorf("failed to parse confirmation response: %w", err)
	}

	if confirmResult.Status == "NotAuthorized" {
		return fmt.Errorf("authentication failed: not authorized")
	}

	// Step 6: Final status check
	checkResp := sendRequest("checkStatus", KeyValue{}, conn, ctx)
	var checkResult checkStatusResponse
	if err := json.Unmarshal(checkResp, &checkResult); err != nil {
		return fmt.Errorf("failed to parse status check response: %w", err)
	}

	if checkResult.Status == "NotAuthorized" {
		return fmt.Errorf("authentication verification failed")
	}

	fmt.Printf("\n"+hiGreenString("✓ Successfully authenticated as %s")+"\n\n", checkResult.User)
	return nil
}

// openBrowser attempts to open the given URL in the user's default browser
func (ta *TerminalAuth) openBrowser(url string) error {
	var cmd string
	var args []string

	switch runtime.GOOS {
	case "windows":
		cmd = "cmd"
		args = []string{"/c", "start"}
	case "darwin":
		cmd = "open"
	case "linux":
		cmd = "xdg-open"
	default:
		return fmt.Errorf("unsupported platform")
	}

	args = append(args, url)
	return exec.Command(cmd, args...).Start()
}

// PerformAuthWithRetry performs authentication with retry logic
func (ta *TerminalAuth) PerformAuthWithRetry(maxRetries int) error {
	for i := 0; i < maxRetries; i++ {
		if err := ta.CheckAndPerformAuth(); err != nil {
			LogError(fmt.Errorf("authentication attempt %d failed: %w", i+1, err))
			if i < maxRetries-1 {
				fmt.Printf("\nRetrying authentication in 3 seconds...\n")
				time.Sleep(3 * time.Second)
				continue
			}
			return err
		}
		return nil
	}
	return fmt.Errorf("authentication failed after %d attempts", maxRetries)
}

// promptYesNo prompts the user for a yes/no response
func promptYesNo(message string) bool {
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Printf("%s (y/n): ", message)
		response, _ := reader.ReadString('\n')
		response = strings.ToLower(strings.TrimSpace(response))
		if response == "y" || response == "yes" {
			return true
		}
		if response == "n" || response == "no" {
			return false
		}
		fmt.Println("Please enter 'y' or 'n'")
	}
}
