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

var vClient *handler

func startVolar(in mrChan) {
	if len(config.VolarPath) == 0 {
		Log("Volar path not set")
		go func() {
			for {
				request := <-in
				request.CB <- &KeyValue{"status": "ok"}
			}
		}()
		return
	}
	vClient = startRPCServer("volar", config.NodePath, config.VolarPath, "--stdio")

	vClient.lsc.SetLogger(&Logger{
		IncomingPrefix: "LSV <-- Volar", OutgoingPrefix: "LSV --> Volar",
		HiColor: hiBlueString, LoColor: blueString, ErrorColor: errorString,
	})
	vClient.lsc.RegisterCustomNotification("indexingStarted", func(jsonrpc.FunctionLogger, json.RawMessage) {})
	vClient.lsc.RegisterCustomNotification("indexingEnded", func(jsonrpc.FunctionLogger, json.RawMessage) {})

	go vClient.lsc.Run()
	go vClient.processVolarRequests(in)
}

func (c *handler) processVolarRequests(in mrChan) {
	defer catchAndLogPanic(func() {
		c.processVolarRequests(in)
	})

	Log("Volar is waiting for input")
	ctx := context.Background()
	lsc := c.lsc

	for {
		request := <-in
		Log("LSV <-- IDE %s %s %db", "request", request.Method, len(string(request.Body)))

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
			name := params.string("name", "vueProject")
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
				InitializationOptions: lsp.KeyValue{
					"clearCache": true, "isVscode": true,
					"syntaxes": []string{"vue"},
					"typescript": lsp.KeyValue{
						"tsdk": config.TsdkPath,
					},
				},
				Capabilities: lsp.KeyValue{
					"workspace":        KeyValue{"workspaceFolders": true},
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
