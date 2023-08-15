package main

import (
	"context"
	"github.com/tectiv3/go-lsp"
	"github.com/tectiv3/go-lsp/jsonrpc"
	"go.bug.st/json"
	"io"
	"log"
	"os"
	"os/exec"
)

func startIntelephense(in mrChan) {
	var stdin io.WriteCloser
	var stdout, stderr io.ReadCloser

	cmd := exec.Command("/opt/homebrew/opt/node@16/bin/node", "/opt/homebrew/bin/intelephense", "--stdio")

	if cin, err := cmd.StdinPipe(); err != nil {
		panic("getting clangd stdin: " + err.Error())
	} else if cout, err := cmd.StdoutPipe(); err != nil {
		panic("getting clangd stdout: " + err.Error())
	} else if cerr, err := cmd.StderrPipe(); err != nil {
		panic("getting clangd stderr: " + err.Error())
	} else if err := cmd.Start(); err != nil {
		panic("running clangd: " + err.Error())
	} else {
		stdin = cin
		stdout = cout
		stderr = cerr
	}

	stdio := NewReadWriteCloser(stdout, stdin)
	stdio = LogReadWriteCloserAs(stdio, "intelephense.log")
	go io.Copy(openLogFileAs("intelephense-err.log"), stderr)

	handler := &cmdHandler{
		Diagnostics: make(chan *lsp.PublishDiagnosticsParams),
	}
	lsc := lsp.NewClient(stdio, stdio, handler, func(err error) {
		log.Println(errorString("Error: %v", err))
	})
	handler.client = lsc

	lsc.SetLogger(&Logger{
		IncomingPrefix: "LS <-- Intelephense", OutgoingPrefix: "LS --> Intelephense",
		HiColor: hiRedString, LoColor: redString, ErrorColor: errorString,
	})
	lsc.RegisterCustomNotification("indexingStarted", func(logger jsonrpc.FunctionLogger, params json.RawMessage) {
		//
	})
	lsc.RegisterCustomNotification("indexingEnded", func(logger jsonrpc.FunctionLogger, params json.RawMessage) {
		//
	})

	go lsc.Run()
	go processIntelephenseRequests(in, lsc)

	defer stdin.Close()
	cmd.Wait()
}

func processIntelephenseRequests(in mrChan, lsc *lsp.Client) {
	Log("Waiting for input")
	ctx := context.Background()

	for {
		request := <-in
		Log("LS <-- IDE %s %s %s", "request", request.Method, string(request.Body))

		switch request.Method {
		case "initialize":
			pid := os.Getpid()
			var params KeyValue
			if err := json.Unmarshal(request.Body, &params); err != nil {
				LogError(err)
				request.CB <- &KeyValue{"result": "error", "message": "empty dir"}
				continue
			}
			dir := params.string("dir", "")
			license := params.string("license", "")
			name := params.string("name", "phpProject")
			storage := params.string("storage", "/tmp/intelephense/")

			_, respErr, err := lsc.Initialize(ctx, &lsp.InitializeParams{
				ProcessID: &pid,
				RootURI:   lsp.NewDocumentURI(dir),
				RootPath:  dir,
				InitializationOptions: lsp.KeyValue{
					"storagePath": storage, "clearCache": true,
					"licenceKey": license, "isVscode": true,
				},
				Capabilities: lsp.KeyValue{
					"textDocument": KeyValue{
						"synchronization": KeyValue{
							"dynamicRegistration": true,
							"didSave":             true,
							"willSaveWaitUntil":   false,
							"willSave":            true,
						},
						"publishDiagnostics": true,
						"completion": KeyValue{
							"dynamicRegistration": true,
							"contextSupport":      true,
							"completionItem": KeyValue{
								"snippetSupport":          true,
								"commitCharactersSupport": true,
								"documentationFormat":     []string{"markdown", "plaintext"},
								"deprecatedSupport":       true,
								"preselectSupport":        true,
							},
							"completionItemKind": KeyValue{
								"valueSet": []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25},
							},
						},
						"hover": KeyValue{
							"dynamicRegistration": true,
							"contentFormat":       []string{"markdown", "plaintext"},
						},
						"signatureHelp": KeyValue{
							"dynamicRegistration": true,
							"signatureInformation": KeyValue{
								"documentationFormat":  []string{"markdown", "plaintext"},
								"parameterInformation": KeyValue{"labelOffsetSupport": true},
							},
						},
						"codeLens":         KeyValue{"dynamicRegistration": true},
						"formatting":       KeyValue{"dynamicRegistration": true},
						"rangeFormatting":  KeyValue{"dynamicRegistration": true},
						"onTypeFormatting": KeyValue{"dynamicRegistration": true},
						"rename": KeyValue{
							"dynamicRegistration": true,
							"prepareSupport":      true,
						},
						"documentLink": KeyValue{"dynamicRegistration": true},
						"typeDefinition": KeyValue{
							"dynamicRegistration": true,
							"linkSupport":         true,
						},
						"implementation": KeyValue{
							"dynamicRegistration": true,
							"linkSupport":         true,
						},
						"colorProvider": KeyValue{"dynamicRegistration": true},
						"foldingRange": KeyValue{
							"dynamicRegistration": true,
							"rangeLimit":          5000,
							"lineFoldingOnly":     true,
						},
						"declaration": KeyValue{
							"dynamicRegistration": true,
							"linkSupport":         true,
						},
					},

					"workspace": KeyValue{
						"applyEdit": true,
						// "didChangeConfiguration": KeyValue{"dynamicRegistration": true},
						// "configuration":    true,
						"executeCommand":   KeyValue{"dynamicRegistration": true},
						"workspaceFolders": true,
						"symbol": KeyValue{
							"dynamicRegistration": true,
							"symbolKind": KeyValue{
								"valueSet": []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25},
							},
						},
						"workspaceEdit": KeyValue{
							"documentChanges":    true,
							"resourceOperations": []string{"create", "rename", "delete"},
							"failureHandling":    "textOnlyTransactional",
						},
						"didChangeWatchedFiles": KeyValue{"dynamicRegistration": false},
					},
					"workspaceFolders": []KeyValue{
						KeyValue{
							"uri":  "file://" + dir,
							"name": name,
						},
					},
				},
				WorkspaceFolders: &[]lsp.WorkspaceFolder{
					{
						URI:  lsp.NewDocumentURI(dir),
						Name: name,
					},
				},
			})
			if respErr != nil || err != nil {
				log.Println("respErr: ", respErr)
				LogError(err)
				request.CB <- &KeyValue{"status": "error", "error": "initialize error"}
				continue
			}

			go lsc.Initialized(&lsp.InitializedParams{})
			go lsc.WorkspaceDidChangeConfiguration(&lsp.DidChangeConfigurationParams{
				Settings: []byte("{\"intelephense.files.maxSize\": 3000000}"),
			})
			request.CB <- &KeyValue{"status": "ok"}
		case "textDocument/hover":
			params := lsp.TextDocumentPositionParams{}
			if err := json.Unmarshal(request.Body, &params); err != nil {
				request.CB <- &KeyValue{"result": "error", "message": err.Error()}
				continue
			}
			response, respErr, err := lsc.TextDocumentHover(ctx, &lsp.HoverParams{TextDocumentPositionParams: params})
			//response, respErr, err := lsc.GetConnection().SendRequest(ctx, "textDocument/hover", request.Body)
			if respErr != nil || err != nil {
				log.Println("respErr: ", respErr)
				LogError(err)
				request.CB <- &KeyValue{"status": "error", "error": "hover error"}
				continue
			}
			request.CB <- &KeyValue{"status": "ok", "result": response}
		case "textDocument/documentSymbol":
			lsc.GetConnection().SendRequest(ctx, "textDocument/documentSymbol", request.Body)

			go func() {
				diagnostics := <-lsc.GetHandler().GetDiagnosticChannel()
				request.CB <- &KeyValue{"status": "ok", "result": diagnostics.Diagnostics}
			}()
		case "textDocument/didOpen":
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
