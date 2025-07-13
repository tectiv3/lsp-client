package main

import (
	"testing"

	"go.bug.st/json"
)

func TestTerminalAuth_Creation(t *testing.T) {
	// Test that we can create a TerminalAuth instance
	mockHandler := &handler{}
	auth := NewTerminalAuth(mockHandler)

	if auth == nil {
		t.Error("NewTerminalAuth() returned nil")
	}

	if auth.client != mockHandler {
		t.Error("NewTerminalAuth() did not set client correctly")
	}
}

func TestAuthResponseParsing(t *testing.T) {
	// Test parsing of authentication responses
	tests := []struct {
		name     string
		jsonResp string
		expected string
	}{
		{
			name:     "sign_in_response",
			jsonResp: `{"status": "pending", "userCode": "ABC123", "verificationUri": "https://github.com/login/device", "expiresIn": 900, "interval": 5}`,
			expected: "pending",
		},
		{
			name:     "already_signed_in",
			jsonResp: `{"status": "AlreadySignedIn", "user": "testuser"}`,
			expected: "AlreadySignedIn",
		},
		{
			name:     "not_authorized",
			jsonResp: `{"status": "NotAuthorized"}`,
			expected: "NotAuthorized",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var resp signInResponse
			err := json.Unmarshal([]byte(tt.jsonResp), &resp)
			if err != nil {
				t.Errorf("Failed to unmarshal response: %v", err)
				return
			}

			if resp.Status != tt.expected {
				t.Errorf("Expected status %s, got %s", tt.expected, resp.Status)
			}
		})
	}
}

func TestCheckStatusResponseParsing(t *testing.T) {
	// Test parsing of checkStatus responses
	tests := []struct {
		name     string
		jsonResp string
		expected string
		user     string
	}{
		{
			name:     "authorized",
			jsonResp: `{"status": "Authorized", "user": "testuser"}`,
			expected: "Authorized",
			user:     "testuser",
		},
		{
			name:     "not_authorized",
			jsonResp: `{"status": "NotAuthorized"}`,
			expected: "NotAuthorized",
			user:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var resp checkStatusResponse
			err := json.Unmarshal([]byte(tt.jsonResp), &resp)
			if err != nil {
				t.Errorf("Failed to unmarshal response: %v", err)
				return
			}

			if resp.Status != tt.expected {
				t.Errorf("Expected status %s, got %s", tt.expected, resp.Status)
			}

			if resp.User != tt.user {
				t.Errorf("Expected user %s, got %s", tt.user, resp.User)
			}
		})
	}
}

func TestTerminalAuth_openBrowser(t *testing.T) {
	// Create a TerminalAuth instance
	mockHandler := &handler{}
	auth := NewTerminalAuth(mockHandler)

	// Test with a valid URL - should not panic
	// Note: This won't actually open a browser in test environment
	err := auth.openBrowser("https://github.com/login/device")

	// The error might vary by platform, but it shouldn't panic
	// On some systems it might work, on others it might fail
	// The important thing is that it doesn't crash
	t.Logf("openBrowser result: %v", err)
}
