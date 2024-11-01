package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/tectiv3/go-lsp"
	"github.com/tectiv3/go-lsp/jsonrpc"
	"go.bug.st/json"
)

var iClient *handler

func startIntelephense(in mrChan) {
	if len(config.IntelephensePath) == 0 {
		Log("Intelephense path not set")
		go func() {
			for {
				request := <-in
				request.CB <- &KeyValue{"status": "ok"}
			}
		}()
		return
	}
	iClient = startRPCServer("intelephense", config.NodePath, config.IntelephensePath, "--stdio")

	iClient.SetConfig(KeyValue{
		"files": KeyValue{
			"maxSize":      300000,
			"associations": []string{"*.php", "*.phtml"},
			"exclude": []string{
				"**/.git/**",
				"**/.svn/**",
				"**/.hg/**",
				"**/CVS/**",
				"**/.DS_Store/**",
				"**/node_modules/**",
				"**/bower_components/**",
				"**/vendor/**/{Test,test,Tests,tests}/**",
				"**/.git",
				"**/.svn",
				"**/.hg",
				"**/CVS",
				"**/.DS_Store",
				"**/nova/tests/**",
				"**/faker/**",
				"**/*.log",
				"**/*.log*",
				"**/*.min.*",
				"**/dist",
				"**/coverage",
				"**/build/*",
				"**/nova/public/*",
				"**/public/*",
			},
		},
		"stubs": []string{
			"apache",
			"bcmath",
			"bz2",
			"calendar",
			"com_dotnet",
			"Core",
			"ctype",
			"curl",
			"date",
			"dba",
			"dom",
			"enchant",
			"exif",
			"fileinfo",
			"filter",
			"fpm",
			"ftp",
			"gd",
			"hash",
			"iconv",
			"imap",
			"interbase",
			"intl",
			"json",
			"ldap",
			"libxml",
			"mbstring",
			"mcrypt",
			"meta",
			"mssql",
			"mysqli",
			"oci8",
			"odbc",
			"openssl",
			"pcntl",
			"pcre",
			"PDO",
			"pdo_ibm",
			"pdo_mysql",
			"pdo_pgsql",
			"pdo_sqlite",
			"pgsql",
			"Phar",
			"posix",
			"pspell",
			"readline",
			"recode",
			"Reflection",
			"regex",
			"session",
			"shmop",
			"SimpleXML",
			"snmp",
			"soap",
			"sockets",
			"sodium",
			"SPL",
			"sqlite3",
			"standard",
			"superglobals",
			"sybase",
			"sysvmsg",
			"sysvsem",
			"sysvshm",
			"tidy",
			"tokenizer",
			"wddx",
			"xml",
			"xmlreader",
			"xmlrpc",
			"xmlwriter",
			"Zend OPcache",
			"zip",
			"zlib",
		},
		"completion": KeyValue{
			"insertUseDeclaration":                    true,
			"fullyQualifyGlobalConstantsAndFunctions": false,
			"triggerParameterHints":                   true,
			"maxItems":                                100,
		},
		"format": KeyValue{
			"enable": false,
		},
		"environment": KeyValue{
			"documentRoot": "",
			"includePaths": []string{},
		},
		"runtime":   "",
		"maxMemory": 0,
		"telemetry": KeyValue{"enabled": false},
		"trace": KeyValue{
			"server": "verbose",
		},
	})

	iClient.lsc.SetLogger(&Logger{
		IncomingPrefix: "LSI <-- Intelephense", OutgoingPrefix: "LSI --> Intelephense",
		HiColor: hiRedString, LoColor: redString, ErrorColor: errorString,
	})
	iClient.lsc.RegisterCustomNotification("indexingStarted", func(jsonrpc.FunctionLogger, json.RawMessage) {})
	iClient.lsc.RegisterCustomNotification("indexingEnded", func(jsonrpc.FunctionLogger, json.RawMessage) {})

	go iClient.lsc.Run()
	go iClient.processIntelephenseRequests(in)
}

func (c *handler) processIntelephenseRequests(in mrChan) {
	defer catchAndLogPanic(func() {
		c.processIntelephenseRequests(in)
	})

	Log("Intelephense is waiting for input")
	ctx := context.Background()
	lsc := c.lsc

	for {
		request := <-in
		if config.EnableLogging {
			Log("LSI <-- IDE %s %s %db", "request", request.Method, len(string(request.Body)))
		}

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
			license := params.string("license", config.IntelephenseLicense)
			name := params.string("name", "phpProject")
			storage := params.string("storage", config.IntelephenseStorage)
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
					"storagePath": storage, "clearCache": true,
					"licenceKey": license, "isVscode": true,
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
			go lsc.WorkspaceDidChangeConfiguration(&lsp.DidChangeConfigurationParams{
				Settings: lsp.KeyValue{
					"intelephense": KeyValue{"files": KeyValue{"maxSize": 3000000}},
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
				if config.EnableLogging {
					Log("Waiting for diagnostics")
				}
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
