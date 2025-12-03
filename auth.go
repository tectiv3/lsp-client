package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
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
	if ta.IsAuthenticated() {
		Log("Already authenticated with GitHub Copilot")
		return nil
	}

	Log("Not authenticated. Starting authentication flow...")
	return ta.PerformTerminalAuth()
}

// IsAuthenticated checks if the user is currently authenticated
func (ta *TerminalAuth) IsAuthenticated() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn := ta.client.lsc.GetConnection()

	sendRequest("initialize", KeyValue{
		"capabilities": KeyValue{"workspace": KeyValue{"workspaceFolders": true}},
	}, conn, ctx)

	resp := sendRequest("checkStatus", KeyValue{}, conn, ctx)

	var res checkStatusResponse
	if err := json.Unmarshal(resp, &res); err != nil {
		LogError(err)
		return false
	}

	return res.Status != "NotAuthorized" && res.Status != "NotSignedIn" && res.Status != ""
}

// IsTerminal checks if the current process is running in an interactive terminal
func IsTerminal() bool {
	fileInfo, _ := os.Stdin.Stat()
	return (fileInfo.Mode() & os.ModeCharDevice) != 0
}

// InitiateAuth starts the authentication process and returns the user code and verification URI
func (ta *TerminalAuth) InitiateAuth() (*signInResponse, error) {
	ctx := context.Background()
	conn := ta.client.lsc.GetConnection()

	// Step 1: Initiate sign-in
	resp := sendRequest("signInInitiate", KeyValue{}, conn, ctx)
	var signInResp signInResponse
	if err := json.Unmarshal(resp, &signInResp); err != nil {
		return nil, fmt.Errorf("failed to parse sign-in response: %w", err)
	}
	return &signInResp, nil
}

// ConfirmAuth confirms the authentication after the user has authorized in the browser
func (ta *TerminalAuth) ConfirmAuth(userCode string) error {
	ctx := context.Background()
	conn := ta.client.lsc.GetConnection()

	// Step 5: Confirm authentication
	fmt.Printf("Verifying authentication...\n")
	confirmResp := sendRequest("signInConfirm", KeyValue{"userCode": userCode}, conn, ctx)
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

	fmt.Println("\n" + hiGreenString("✓ Successfully authenticated as %s", checkResult.User))
	return nil
}

// PerformTerminalAuth performs the complete authentication flow in terminal
func (ta *TerminalAuth) PerformTerminalAuth() error {
	signInResp, err := ta.InitiateAuth()
	if err != nil {
		return err
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

	if !IsTerminal() {
		Log("Authentication required. Please use the following code: %s", signInResp.UserCode)
		Log("Open this URL: %s", signInResp.VerificationUri)
		Log("Not running in interactive terminal. Please authenticate using the menu bar or by running the application in a terminal.")
		return fmt.Errorf("interactive terminal required for authentication")
	}

	// Step 4: Wait for user confirmation
	fmt.Printf("Press ENTER after you have completed the authorization in your browser...")
	reader := bufio.NewReader(os.Stdin)
	reader.ReadLine()

	return ta.ConfirmAuth(signInResp.UserCode)
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
