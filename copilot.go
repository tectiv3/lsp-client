package main

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"time"

	"github.com/tectiv3/go-lsp"
	"github.com/tectiv3/go-lsp/jsonrpc"
	"go.bug.st/json"
)

var (
	cClient             *handler
	lastCompletionItems []Completion
	lastCompletionIndex int
)

func startCopilot(in mrChan) {
	cClient = startRPCServer("copilot", config.NodePath, config.CopilotPath, "--stdio")

	cClient.Requests = make(map[string]string)
	cClient.lsc.SetLogger(&Logger{
		IncomingPrefix: "LSC <-- Copilot", OutgoingPrefix: "LSC --> Copilot",
		HiColor: hiGreenString, LoColor: greenString, ErrorColor: errorString,
	})
	cClient.lsc.RegisterCustomNotification("statusNotification", func(logger jsonrpc.FunctionLogger, params json.RawMessage) {
		if config.EnableLogging {
			logger.Logf("%s", string(params))
		}
	})

	go cClient.lsc.Run()

	// Wait a moment for the LSP client to be ready
	time.Sleep(1 * time.Second)

	// Perform authentication check and terminal login if needed
	auth := NewTerminalAuth(cClient)
	if err := auth.PerformAuthWithRetry(3); err != nil {
		LogError(fmt.Errorf("Copilot authentication failed: %w", err))
		fmt.Printf("\n" + hiRedString("Warning: Copilot authentication failed. You can try again later using the API.") + "\n\n")
	}

	go cClient.processCopilotRequests(in)
}

func (c *handler) processCopilotRequests(in mrChan) {
	defer catchAndLogPanic(func() {
		c.processCopilotRequests(in)
	})
	Log("Copilot is waiting for input")
	ctx := context.Background()
	lsc := c.lsc
	conn := lsc.GetConnection()
	for {
		request := <-in
		if config.EnableLogging {
			Log("LSC <-- IDE %s %s %db", "request", request.Method, len(string(request.Body)))
		}

		switch request.Method {
		case "initialize":
			sendRequest("initialize", KeyValue{
				"capabilities": KeyValue{"workspace": KeyValue{"workspaceFolders": true}},
			}, conn, ctx)
			// log.Println("After initialize")
			lsc.Initialized(&lsp.InitializedParams{})
			sendRequest("setEditorInfo", KeyValue{
				"editorInfo":       KeyValue{"name": "Textmate", "version": "2.0.23"},
				"editorPluginInfo": KeyValue{"name": "lsp-client", "version": "0.1.0"},
			}, conn, ctx)
			request.CB <- &KeyValue{"status": "ok"}
		case "signIn":
			resp := sendRequest("signInInitiate", KeyValue{}, conn, ctx)
			var res signInResponse
			json.Unmarshal(resp, &res)
			//        eval_in_emacs("browse-url", result['verificationUri'])
			//        message_emacs(f'Please enter user-code {result["userCode"]}')
			// log.Println(res.Status)
			// If already signed in, return success
			if res.Status == "AlreadySignedIn" {
				request.CB <- &KeyValue{"status": "success", "user": res.User, "message": "Already signed in"}
				return
			}

			// Return the authentication details for the client to handle
			request.CB <- &KeyValue{
				"status":          "pending",
				"userCode":        res.UserCode,
				"verificationUri": res.VerificationUri,
				"expiresIn":       res.ExpiresIn,
				"interval":        res.Interval,
			}
		case "signInConfirm":
			textDocument := &KeyValue{}
			if err := json.Unmarshal(request.Body, textDocument); err != nil {
				request.CB <- &KeyValue{"status": "error", "message": err.Error()}
				return
			}
			userCode := textDocument.string("userCode", "")
			if userCode == "" {
				request.CB <- &KeyValue{"status": "error", "message": "userCode is required"}
				return
			}

			resp := sendRequest("signInConfirm", KeyValue{"userCode": userCode}, conn, ctx)
			var res signInConfirmResponse
			json.Unmarshal(resp, &res)

			if res.Status == "NotAuthorized" {
				request.CB <- &KeyValue{"status": "error", "message": "Not authorized"}
				return
			}

			request.CB <- &KeyValue{"status": "success", "user": res.User}
		case "checkStatus":
			resp := sendRequest("checkStatus", KeyValue{}, conn, ctx)
			var res checkStatusResponse
			json.Unmarshal(resp, &res)

			if res.Status == "NotAuthorized" {
				request.CB <- &KeyValue{"status": "error", "message": "Not authorized"}
				return
			}

			request.CB <- &KeyValue{"status": "success", "user": res.User}
		case "authStatus":
			// Alias for checkStatus
			resp := sendRequest("checkStatus", KeyValue{}, conn, ctx)
			var res checkStatusResponse
			json.Unmarshal(resp, &res)

			if res.Status == "NotAuthorized" {
				request.CB <- &KeyValue{"status": "error", "message": "Not authorized"}
				return
			}

			request.CB <- &KeyValue{"status": "success", "user": res.User}
		case "getCompletions":
			go func() {
				lastCompletionItems = []Completion{}
				textDocument := &KeyValue{}
				if err := json.Unmarshal(request.Body, textDocument); err != nil {
					request.CB <- &KeyValue{"result": "error", "message": err.Error()}
					return
				}
				path := textDocument.string("uri", "")
				position := textDocument.keyValue("position", KeyValue{})
				resp := sendRequest("getCompletions", KeyValue{
					"doc": KeyValue{
						"source":       textDocument.string("text", ""),
						"tabSize":      textDocument.int("tabSize", 4),
						"indentSize":   4,
						"insertSpaces": true,
						"version":      0,
						"path":         path,
						"uri":          "file://" + path,
						"relativePath": filepath.Base(path),
						"languageId":   textDocument.string("languageId", ""),
						"position":     position,
					},
				}, conn, ctx)
				if string(resp) == "null" {
					Log("Empty response")
					request.CB <- &KeyValue{"status": "ok", "result": "No completions"}
					return
				}
				result := CompletionsResponse{}
				if err := json.Unmarshal(resp, &result); err != nil {
					request.CB <- &KeyValue{"result": "error", "message": err.Error()}
					return
				}
				// check that slice of completions is not empty
				if len(result.Completions) == 0 {
					request.CB <- &KeyValue{"status": "ok", "result": "No completions"}
					return
				}
				if len(result.Completions) > 1 {
					Log("Last completion items: %d", len(result.Completions))
					lastCompletionItems = result.Completions
				}
				lastCompletionIndex = 0
				completion := result.Completions[lastCompletionIndex]
				conn.SendNotification("notifyShown", lsp.EncodeMessage(KeyValue{
					"uuids": []string{completion.UUID},
				}))
				request.CB <- &KeyValue{
					"status": "ok", "result": lsp.EncodeMessage(completion.DisplayText),
				}
			}()
		case "getCompletionsCycling":
			go func() {
				if len(lastCompletionItems) == 0 {
					request.CB <- &KeyValue{"status": "ok", "result": "No completions"}
					return
				}
				if lastCompletionIndex+1 >= len(lastCompletionItems) {
					lastCompletionIndex = 0
				} else {
					lastCompletionIndex++
				}
				if lastCompletionIndex < 0 || lastCompletionIndex >= len(lastCompletionItems) {
					request.CB <- &KeyValue{"status": "ok", "result": "No completions"}
					return
				}
				completion := lastCompletionItems[lastCompletionIndex]
				conn.SendNotification("notifyShown", lsp.EncodeMessage(KeyValue{
					"uuids": []string{completion.UUID},
				}))
				request.CB <- &KeyValue{
					"status": "ok", "result": lsp.EncodeMessage(completion.DisplayText),
				}
			}()
		case "notifyCompletionAccepted":
			// Send notification to copilot server
			conn.SendNotification("notifyAccepted", lsp.EncodeMessage(KeyValue{}))
			request.CB <- &KeyValue{"status": "ok"}
		case "notifyCompletionRejected":
			// Send notification to copilot server
			conn.SendNotification("notifyRejected", lsp.EncodeMessage(KeyValue{}))
			request.CB <- &KeyValue{"status": "ok"}
		case "textDocument/didOpen":
			lastCompletionItems = []Completion{}
			textDocument := &KeyValue{}
			if err := json.Unmarshal(request.Body, textDocument); err != nil {
				request.CB <- &KeyValue{"result": "error", "message": err.Error()}
				return
			}
			uri, _ := lsp.NewDocumentURIFromURL(textDocument.string("uri", ""))
			go lsc.TextDocumentDidOpen(&lsp.DidOpenTextDocumentParams{TextDocument: lsp.TextDocumentItem{
				URI:        uri,
				LanguageID: textDocument.string("languageId", ""),
				Version:    textDocument.int("version", 0),
				Text:       textDocument.string("text", ""),
			}})
			request.CB <- &KeyValue{"status": "ok"}
		case "textDocument/didClose":
			lastCompletionItems = []Completion{}
			textDocument := lsp.TextDocumentIdentifier{}
			if err := json.Unmarshal(request.Body, &textDocument); err != nil {
				request.CB <- &KeyValue{"result": "error", "message": err.Error()}
				return
			}
			go lsc.TextDocumentDidClose(&lsp.DidCloseTextDocumentParams{TextDocument: textDocument})
			request.CB <- &KeyValue{"status": "ok"}
		}
	}
}

func sendRequest(method string, request KeyValue, conn *jsonrpc.Connection, ctx context.Context) json.RawMessage {
	return sendRequestWithAuth(method, request, conn, ctx, true)
}

func sendRequestWithAuth(method string, request KeyValue, conn *jsonrpc.Connection, ctx context.Context, allowReauth bool) json.RawMessage {
	body, err := json.Marshal(request)
	if err != nil {
		LogError(err)
		return []byte{}
	}

	resp, respErr, err := conn.SendRequest(ctx, method, body)
	if err != nil || respErr != nil {
		log.Println("respErr: ", respErr)
		LogError(err)

		// Check if this is an authentication error and we're allowed to re-authenticate
		if allowReauth && respErr != nil && isAuthenticationError(respErr) {
			Log("Detected authentication error, attempting re-authentication...")
			if err := handleReauthentication(conn, ctx); err != nil {
				LogError(fmt.Errorf("re-authentication failed: %w", err))
				return []byte{}
			}

			// Retry the original request without allowing further re-authentication
			Log("Re-authentication successful, retrying original request...")
			return sendRequestWithAuth(method, request, conn, ctx, false)
		}

		return []byte{}
	}

	return resp
}

// isAuthenticationError checks if the error is related to authentication
func isAuthenticationError(respErr *jsonrpc.ResponseError) bool {
	if respErr == nil {
		return false
	}

	// Check for authentication error codes and messages
	return respErr.Code == 1000 ||
		respErr.Message == "Not authenticated: NotSignedIn" ||
		respErr.Message == "Not authenticated" ||
		respErr.Message == "NotSignedIn" ||
		respErr.Message == "NotAuthorized"
}

// handleReauthentication performs automatic re-authentication
func handleReauthentication(conn *jsonrpc.Connection, ctx context.Context) error {
	Log("Starting automatic re-authentication...")

	// Check if we can create a TerminalAuth instance
	if cClient == nil {
		return fmt.Errorf("client not available for re-authentication")
	}

	// Create a new terminal auth instance
	auth := NewTerminalAuth(cClient)

	// First try to check if we're already authenticated after a brief delay
	// This handles cases where the auth token was temporarily invalid
	time.Sleep(1 * time.Second)
	if auth.IsAuthenticated() {
		Log("Authentication recovered automatically")
		return nil
	}

	// If still not authenticated, try the full auth flow
	Log("Performing full re-authentication flow...")
	if err := auth.PerformTerminalAuth(); err != nil {
		return fmt.Errorf("terminal authentication failed: %w", err)
	}

	Log("Re-authentication completed successfully")
	return nil
}

// For testing purposes
func IsAuthenticationError(respErr *jsonrpc.ResponseError) bool {
	return isAuthenticationError(respErr)
}
