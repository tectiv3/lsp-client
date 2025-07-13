package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/tectiv3/go-lsp"
)

func TestAuthEndpoints(t *testing.T) {
	// This test verifies that our authentication endpoints are properly wired up
	// Note: This doesn't test actual copilot authentication since that requires
	// a real copilot-language-server process

	// Create a test server with mock copilot channel
	copilotChan := make(mrChan, 1)
	testServer := &mateServer{
		copilot:     copilotChan,
		initialized: true,
		logger: &Logger{
			IncomingPrefix: "TEST <-- IDE", OutgoingPrefix: "TEST --> IDE",
			HiColor:    func(s string, args ...interface{}) string { return s },
			LoColor:    func(s string, args ...interface{}) string { return s },
			ErrorColor: func(s string, args ...interface{}) string { return s },
		},
		openFiles:   make(map[string]time.Time),
		openFolders: make(map[string]lsp.DocumentURI),
	}

	// Mock response handler for copilot channel
	go func() {
		for req := range copilotChan {
			// Mock responses for different auth methods
			switch req.Method {
			case "signIn":
				req.CB <- &KeyValue{
					"status":          "pending",
					"userCode":        "TEST123",
					"verificationUri": "https://github.com/login/device",
					"expiresIn":       900,
					"interval":        5,
				}
			case "signInConfirm":
				req.CB <- &KeyValue{
					"status": "success",
					"user":   "testuser",
				}
			case "checkStatus", "authStatus":
				req.CB <- &KeyValue{
					"status": "success",
					"user":   "testuser",
				}
			default:
				req.CB <- &KeyValue{"status": "error", "message": "unknown method"}
			}
		}
	}()

	// Test signIn endpoint
	t.Run("signIn", func(t *testing.T) {
		requestBody := map[string]interface{}{
			"Method": "signIn",
			"Body":   json.RawMessage(`{}`),
		}
		body, _ := json.Marshal(requestBody)
		req := httptest.NewRequest("POST", "/", bytes.NewReader(body))
		w := httptest.NewRecorder()

		testServer.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)

		if response["status"] != "pending" {
			t.Errorf("Expected pending status, got %v", response["status"])
		}

		if response["userCode"] != "TEST123" {
			t.Errorf("Expected userCode TEST123, got %v", response["userCode"])
		}
	})

	// Test signInConfirm endpoint
	t.Run("signInConfirm", func(t *testing.T) {
		// Create the proper request structure for signInConfirm
		params := map[string]string{"userCode": "TEST123"}
		paramsBody, _ := json.Marshal(params)
		requestBody := map[string]interface{}{
			"Method": "signInConfirm",
			"Body":   json.RawMessage(paramsBody),
		}
		body, _ := json.Marshal(requestBody)
		req := httptest.NewRequest("POST", "/", bytes.NewReader(body))
		w := httptest.NewRecorder()

		testServer.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)

		if response["status"] != "success" {
			t.Errorf("Expected success status, got %v", response["status"])
		}

		if response["user"] != "testuser" {
			t.Errorf("Expected user testuser, got %v", response["user"])
		}
	})

	// Test checkStatus endpoint
	t.Run("checkStatus", func(t *testing.T) {
		requestBody := map[string]interface{}{
			"Method": "checkStatus",
			"Body":   json.RawMessage(`{}`),
		}
		body, _ := json.Marshal(requestBody)
		req := httptest.NewRequest("POST", "/", bytes.NewReader(body))
		w := httptest.NewRecorder()

		testServer.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)

		if response["status"] != "success" {
			t.Errorf("Expected success status, got %v", response["status"])
		}
	})

	// Test authStatus endpoint (alias)
	t.Run("authStatus", func(t *testing.T) {
		requestBody := map[string]interface{}{
			"Method": "authStatus",
			"Body":   json.RawMessage(`{}`),
		}
		body, _ := json.Marshal(requestBody)
		req := httptest.NewRequest("POST", "/", bytes.NewReader(body))
		w := httptest.NewRecorder()

		testServer.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)

		if response["status"] != "success" {
			t.Errorf("Expected success status, got %v", response["status"])
		}
	})
}
