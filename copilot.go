package main

import (
	"context"
	"log"
	"path/filepath"

	"github.com/tectiv3/go-lsp"
	"github.com/tectiv3/go-lsp/jsonrpc"
	"go.bug.st/json"
)

var cClient *handler
var lastCompletionItems []Completion
var lastCompletionIndex int

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
			//log.Println("After initialize")
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
			//log.Println(res.Status)
			request.CB <- &KeyValue{"status": res.Status}
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
		//h.client.GetConnection().SendNotification("notifyAccepted", lsp.EncodeMessage(KeyValue{}))
		case "notifyCompletionRejected":
		//h.client.GetConnection().SendNotification("notifyRejected", lsp.EncodeMessage(KeyValue{}))
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
	body, err := json.Marshal(request)
	if err != nil {
		LogError(err)
		return []byte{}
	}

	resp, respErr, err := conn.SendRequest(ctx, method, body)
	if err != nil || respErr != nil {
		log.Println("respErr: ", respErr)
		LogError(err)
		return []byte{}
	}

	return resp
}
