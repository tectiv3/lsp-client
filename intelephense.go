package main

import (
	"context"
	"github.com/tectiv3/go-lsp"
	"github.com/tectiv3/go-lsp/jsonrpc"
	"go.bug.st/json"
	"log"
	"os"
)

func startIntelephense(in mrChan) {
	lsc := startRPCServer("intelephense", "/opt/homebrew/opt/node@16/bin/node", "/opt/homebrew/bin/intelephense", "--stdio")

	lsc.SetLogger(&Logger{
		IncomingPrefix: "LS <-- Intelephense", OutgoingPrefix: "LS --> Intelephense",
		HiColor: hiRedString, LoColor: redString, ErrorColor: errorString,
	})
	lsc.RegisterCustomNotification("indexingStarted", func(jsonrpc.FunctionLogger, json.RawMessage) {})
	lsc.RegisterCustomNotification("indexingEnded", func(jsonrpc.FunctionLogger, json.RawMessage) {})

	go lsc.Run()
	go processIntelephenseRequests(in, lsc)
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
					"workspaceFolders": []KeyValue{
						KeyValue{
							"uri":  "file://" + dir,
							"name": name,
						},
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
		case "textDocument/definition":
			fallthrough
		case "textDocument/completion":
			response, respErr, err := lsc.GetConnection().SendRequest(ctx, request.Method, request.Body)
			if respErr != nil || err != nil {
				log.Println("respErr: ", respErr)
				LogError(err)
				request.CB <- &KeyValue{"status": "error", "error": "hover error"}
				continue
			}
			request.CB <- &KeyValue{"status": "ok", "result": response}
		case "textDocument/documentSymbol":
			lsc.GetConnection().SendRequest(ctx, request.Method, request.Body)

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
