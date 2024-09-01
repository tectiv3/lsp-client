package main

import (
	"context"
	"github.com/tectiv3/go-lsp"
	"github.com/tectiv3/go-lsp/jsonrpc"
	"go.bug.st/json"
	"log"
	"os"
	"time"
)

var gClient *handler

func startGopls(in mrChan) {
	if len(config.GoplsPath) == 0 {
		Log("Gopls path not set")
		go func() {
			for {
				request := <-in
				request.CB <- &KeyValue{"status": "ok"}
			}
		}()
		return
	}
	gClient = startRPCServer("gopls", config.GoplsPath, "serve")
	gClient.SetConfig(KeyValue{
		"format": KeyValue{
			"enable": false,
		},
		"environment": KeyValue{
			"documentRoot": "",
			"includePaths": []string{},
		},
		"runtime":   "",
		"maxMemory": 0,
		"trace": KeyValue{
			"server": "verbose",
		},
	})
	gClient.lsc.SetLogger(&Logger{
		IncomingPrefix: "LSI <-- Gopls", OutgoingPrefix: "LSI --> Gopls",
		HiColor: hiRedString, LoColor: redString, ErrorColor: errorString,
	})
	gClient.lsc.RegisterCustomNotification("indexingStarted", func(jsonrpc.FunctionLogger, json.RawMessage) {})
	gClient.lsc.RegisterCustomNotification("indexingEnded", func(jsonrpc.FunctionLogger, json.RawMessage) {})

	go gClient.lsc.Run()
	go gClient.processGoplsRequests(in)
}

func (c *handler) processGoplsRequests(in mrChan) {
	defer catchAndLogPanic(func() {
		c.processGoplsRequests(in)
	})

	Log("Gopls is waiting for input")
	ctx := context.Background()
	lsc := c.lsc

	for {
		request := <-in
		Log("LSI <-- IDE %s %s %db", "request", request.Method, len(string(request.Body)))

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
			name := params.string("name", "goProject")

			var folders []lsp.WorkspaceFolder
			paramFolders := params.array("folders", []interface{}{})
			if len(paramFolders) > 0 {
				Log("folders: %d, %v", len(paramFolders), paramFolders)
				for _, f := range paramFolders {
					if m, ok := f.(map[string]interface{}); ok {
						folder := KeyValue(m)
						uri, _ := lsp.NewDocumentURIFromURL(folder.string("uri", ""))
						folders = append(folders, lsp.WorkspaceFolder{
							URI:  uri,
							Name: folder.string("name", ""),
						})
					} else {
						panic("whaaaat")
					}

				}
			} else {
				folders = append(folders, lsp.WorkspaceFolder{
					URI:  lsp.NewDocumentURI(dir),
					Name: name,
				})

			}

			ctxC, cancel := context.WithTimeout(ctx, time.Second)
			_, respErr, err := lsc.Initialize(ctxC, &lsp.InitializeParams{
				ProcessID: &pid,
				//RootURI:   lsp.NewDocumentURI(dir),
				//RootPath:  dir,
				InitializationOptions: lsp.KeyValue{},
				Capabilities: lsp.KeyValue{
					"workspace":        KeyValue{"workspaceFolders": true, "configuration": true},
					"workspaceFolders": folders,
				},
				WorkspaceFolders: &folders,
			})
			if respErr != nil || err != nil {
				log.Println("respErr: ", respErr)
				LogError(err)
				request.CB <- &KeyValue{"status": "error", "error": "initialize error"}
				continue
			}
			cancel()
			go lsc.Initialized(&lsp.InitializedParams{})
			go lsc.WorkspaceDidChangeConfiguration(&lsp.DidChangeConfigurationParams{
				Settings: lsp.KeyValue{
					// "intelephense": KeyValue{"files": KeyValue{"maxSize": 3000000}},
				},
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
		case "didChangeWorkspaceFolders":
			folder := lsp.WorkspaceFolder{}
			if err := json.Unmarshal(request.Body, &folder); err != nil {
				request.CB <- &KeyValue{"result": "error", "message": err.Error()}
				continue
			}
			lsc.WorkspaceDidChangeWorkspaceFolders(&lsp.DidChangeWorkspaceFoldersParams{
				Event: lsp.WorkspaceFoldersChangeEvent{
					Added:   []lsp.WorkspaceFolder{folder},
					Removed: []lsp.WorkspaceFolder{},
				},
			})
			//lsc.GetConnection().SendNotification("workspace/didChangeWorkspaceFolders", lsp.EncodeMessage(KeyValue{
			//	"event": KeyValue{
			//		"added": []KeyValue{
			//			KeyValue{
			//				"uri":  folder.URI,
			//				"name": folder.Name,
			//			},
			//		},
			//		"removed": []KeyValue{},
			//	},
			//}))
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
				Log("Waiting for diagnostics")
				c.Lock()
				c.waitingForDiagnostics = true
				c.Unlock()

				diagnostics := <-lsc.GetHandler().GetDiagnosticChannel()
				c.Lock()
				c.waitingForDiagnostics = false
				c.Unlock()
				request.CB <- &KeyValue{"status": "ok", "result": diagnostics.Diagnostics}
			}()
		case "textDocument/didOpen":
			textDocument := &KeyValue{}
			if err := json.Unmarshal(request.Body, textDocument); err != nil {
				request.CB <- &KeyValue{"result": "error", "message": err.Error()}
				continue
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
				continue
			}
			go lsc.TextDocumentDidClose(&lsp.DidCloseTextDocumentParams{TextDocument: textDocument})
			request.CB <- &KeyValue{"status": "ok"}
		}
	}
}
