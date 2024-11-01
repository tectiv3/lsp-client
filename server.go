package main

import (
	"log"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/tectiv3/go-lsp"
	"go.bug.st/json"
)

var server mateServer

func (s *mateServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	defer catchAndLogPanic(func() {
		w.WriteHeader(http.StatusInternalServerError)
	})

	//Log("method: %s, length: %d %s", r.Method, r.ContentLength, r.URL.Path)

	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	decoder := json.NewDecoder(r.Body)
	mr := mateRequest{}
	err := decoder.Decode(&mr)
	if err != nil {
		LogError(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	resultChan := make(kvChan)
	var result *KeyValue
	tick := time.After(10 * time.Second)

	go s.processRequest(mr, resultChan)

	// block until result or timeout
	select {
	case <-tick:
		w.WriteHeader(http.StatusGatewayTimeout)
		w.Header().Set("Content-Type", "application/json")

		s.logger.LogOutgoingResponse("", mr.Method, json.RawMessage(`{"result": "error", "message": "time out"}`), nil)
		json.NewEncoder(w).Encode(KeyValue{"result": "error", "message": "time out"})
		return
	case result = <-resultChan:
	}

	if result == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	tr, _ := json.Marshal(result)

	s.logger.LogOutgoingResponse("", mr.Method, json.RawMessage(tr), nil)
	json.NewEncoder(w).Encode(result)
}

func (s *mateServer) processRequest(mr mateRequest, cb kvChan) {
	defer s.handlePanic(mr)
	s.logger.LogIncomingRequest("", mr.Method, mr.Body)

	if mr.Method != "initialize" && !s.initialized {
		cb <- &KeyValue{"result": "error", "message": "not initialized"}
		return
	}

	switch mr.Method {
	case "hover":
		//params := lsp.TextDocumentPositionParams{}
		//if err := json.Unmarshal(mr.Body, &params); err != nil {
		//  cb <- &KeyValue{"result": "error", "message": err.Error()}
		//  return
		//}
		//s.requestAndWait("textDocument/hover", params, cb)
		params := KeyValue{}
		if err := json.Unmarshal(mr.Body, &params); err != nil {
			cb <- &KeyValue{"result": "error", "message": err.Error()}
			return
		}
		languageId := params.string("languageId", "")
		var result *KeyValue
		if languageId == "php" {
			result = s.sendLSPRequest(s.intelephense, "textDocument/hover", params)
		} else if languageId == "go" {
			result = s.sendLSPRequest(s.gopls, "textDocument/hover", params)
		} else {
			result = s.sendLSPRequest(s.volar, "textDocument/hover", params)
		}

		cb <- result
	case "completion":
		//params := lsp.CompletionParams{}
		//if err := json.Unmarshal(mr.Body, &params); err != nil {
		//  cb <- &KeyValue{"result": "error", "message": err.Error()}
		//  return
		//}
		//s.requestAndWait("textDocument/completion", params, cb)
		params := KeyValue{}
		if err := json.Unmarshal(mr.Body, &params); err != nil {
			cb <- &KeyValue{"result": "error", "message": err.Error()}
			return
		}
		languageId := params.string("languageId", "")
		var result *KeyValue
		if languageId == "php" {
			result = s.sendLSPRequest(s.intelephense, "textDocument/completion", params)
		} else if languageId == "go" {
			result = s.sendLSPRequest(s.gopls, "textDocument/completion", params)
		} else if languageId == "javascript" || languageId == "typescript" || languageId == "vue" {
			result = s.sendLSPRequest(s.volar, "textDocument/completion", params)
		}
		if config.EnableLogging {
			Log("Sending completion response")
		}
		cb <- result
	case "definition":
		//params := lsp.TextDocumentPositionParams{}
		//if err := json.Unmarshal(mr.Body, &params); err != nil {
		//  cb <- &KeyValue{"result": "error", "message": err.Error()}
		//  return
		//}
		//s.requestAndWait("textDocument/definition", params, cb)
		params := KeyValue{}
		if err := json.Unmarshal(mr.Body, &params); err != nil {
			cb <- &KeyValue{"result": "error", "message": err.Error()}
			return
		}
		languageId := params.string("languageId", "")
		var result *KeyValue
		if languageId == "php" {
			result = s.sendLSPRequest(s.intelephense, "textDocument/definition", params)
		} else if languageId == "go" {
			result = s.sendLSPRequest(s.gopls, "textDocument/definition", params)
		} else {
			result = s.sendLSPRequest(s.volar, "textDocument/definition", params)
		}
		if config.EnableLogging {
			Log("Sending definition response")
		}
		cb <- result

	case "initialize":
		s.onInitialize(mr, cb)
	case "didOpen":
		s.onDidOpen(mr, cb)
	case "didClose":
		s.onDidClose(mr, cb)
	case "getCompletions":
		params := KeyValue{}
		if err := json.Unmarshal(mr.Body, &params); err != nil {
			cb <- &KeyValue{"result": "error", "message": err.Error()}
			return
		}
		result := s.sendLSPRequest(s.copilot, "getCompletions", params)

		if config.EnableLogging {
			Log("Sending copilot completions")
		}
		cb <- result
	case "getCompletionsCycling":
		params := KeyValue{}
		result := s.sendLSPRequest(s.copilot, "getCompletionsCycling", params)

		if config.EnableLogging {
			Log("Sending copilot completions cycling")
		}
		cb <- result
	default:
		cb <- &KeyValue{"result": "error", "message": "unknown method"}
	}
	if config.EnableLogging {
		Log("method: %s %s", mr.Method, "processRequest finished")
	}
}

func (s *mateServer) onDidOpen(mr mateRequest, cb kvChan) {
	s.Lock()
	defer s.Unlock()
	if !s.initialized {
		cb <- &KeyValue{"result": "error", "message": "not initialized"}
		return
	}

	params := KeyValue{}
	if err := json.Unmarshal(mr.Body, &params); err != nil {
		cb <- &KeyValue{"result": "error", "message": err.Error()}
		return
	}

	fn := params.string("uri", "")
	languageId := params.string("languageId", "")
	if len(fn) == 0 {
		cb <- &KeyValue{"result": "error", "message": "Invalid document uri"}
		return
	}

	//if _, ok := s.openFiles[fn]; ok {
	//Log("file %s already opened", fn)
	//s.sendLSPRequest(s.intelephense, "textDocument/didClose", KeyValue{
	//  "uri": fn,
	//})
	//time.Sleep(100 * time.Millisecond)
	//}
	s.openFiles[fn] = time.Now()
	// sort slice and remove items if there are over 20 of them
	if len(s.openFiles) > 19 {
		//Log("openFiles: %v", s.openFiles)
		for k, v := range s.openFiles {
			if time.Since(v).Seconds() > 60 {
				Log("Removing %s from openFiles", k)
				delete(s.openFiles, k)
				s.sendLSPRequest(s.intelephense, "textDocument/didClose", KeyValue{
					"uri": k,
				})
				s.sendLSPRequest(s.volar, "textDocument/didClose", KeyValue{
					"uri": k,
				})
				s.sendLSPRequest(s.copilot, "textDocument/didClose", KeyValue{
					"uri": k,
				})
			}
		}
	}

	go s.sendLSPRequest(s.copilot, "textDocument/didOpen", params)

	var diagnostics *KeyValue
	var ch mrChan
	if languageId == "vue" {
		ch = s.volar
		s.sendLSPRequest(s.volar, "textDocument/didOpen", params)
		diagnostics = s.sendLSPRequest(s.volar, "textDocument/documentSymbol", KeyValue{
			"textDocument": KeyValue{"uri": fn},
		})
	} else if languageId == "php" {
		ch = s.intelephense
	} else if languageId == "go" {
		ch = s.gopls
	}
	s.sendLSPRequest(ch, "textDocument/didOpen", params)
	diagnostics = s.sendLSPRequest(ch, "textDocument/documentSymbol", KeyValue{
		"textDocument": KeyValue{"uri": fn},
	})

	if diagnostics != nil {
		if config.EnableLogging {
			Log("Sending diagnostics response")
		}
		cb <- diagnostics
		return
	}

	cb <- &KeyValue{"result": "ok"}
}

func (s *mateServer) onDidClose(mr mateRequest, cb kvChan) {
	s.Lock()
	defer s.Unlock()
	params := KeyValue{}
	if err := json.Unmarshal(mr.Body, &params); err != nil {
		cb <- &KeyValue{"result": "error", "message": err.Error()}
		return
	}
	fn := params.string("uri", "")
	if len(fn) == 0 {
		cb <- &KeyValue{"result": "error", "message": "Invalid document uri"}
		return
	}
	go s.sendLSPRequest(s.intelephense, "textDocument/didClose", KeyValue{
		"uri": fn,
	})
	go s.sendLSPRequest(s.volar, "textDocument/didClose", KeyValue{
		"uri": fn,
	})
	go s.sendLSPRequest(s.copilot, "textDocument/didClose", KeyValue{
		"uri": fn,
	})
	delete(s.openFiles, fn)

	cb <- &KeyValue{"result": "ok"}
}

func (s *mateServer) onInitialize(mr mateRequest, cb kvChan) {
	s.Lock()
	defer s.Unlock()
	// initialize copilot
	go func() {
		if s.initialized {
			return
		}
		s.sendLSPRequest(s.copilot, "initialize", KeyValue{})
		s.sendLSPRequest(s.copilot, "signIn", KeyValue{})
	}()

	params := KeyValue{}
	if err := json.Unmarshal(mr.Body, &params); err != nil {
		cb <- &KeyValue{"result": "error", "message": err.Error()}
		return
	}
	dir := params.string("dir", "")
	if len(dir) == 0 {
		cb <- &KeyValue{"result": "error", "message": "Empty dir"}
		return
	}

	name := params.string("name", "unknown")
	if s.currentWS != nil && s.currentWS.name == name {
		cb <- &KeyValue{"result": "ok", "message": "already initialized"}
		return
	}

	if !s.initialized {
		// initialize intelephense
		s.sendLSPRequest(s.intelephense, "initialize", params)
		s.sendLSPRequest(s.volar, "initialize", params)
		s.sendLSPRequest(s.gopls, "initialize", params)
		s.initialized = true
		s.openFolders[name] = lsp.NewDocumentURI(dir)
	} else if _, ok := s.openFolders[name]; !ok {
		Log("First time opening workspace %s", name)
		s.openFolders[name] = lsp.NewDocumentURI(dir)
		params["folders"] = []KeyValue{}
		for f, v := range s.openFolders {
			params["folders"] = append(params["folders"].([]KeyValue), KeyValue{"uri": v, "name": f})
		}
		s.sendLSPRequest(s.intelephense, "initialize", params)
		s.sendLSPRequest(s.volar, "initialize", params)
		s.sendLSPRequest(s.gopls, "initialize", params)
	}

	//go s.sendLSPRequest(s.intelephense, "didChangeWorkspaceFolders", KeyValue{
	//  "uri":  s.openFolders[name],
	//  "name": name,
	//})
	s.currentWS = &workSpace{name, dir}
	cb <- &KeyValue{"result": "ok"}
}

func (s *mateServer) sendLSPRequest(out mrChan, method string, params KeyValue) *KeyValue {
	cb := make(kvChan)
	body, _ := json.Marshal(params)
	out <- &mateRequest{
		Method: method,
		Body:   body,
		CB:     cb,
	}

	return <-cb
}

func (s *mateServer) handlePanic(mr mateRequest) {
	if err := recover(); err != nil {
		Log("method: %s, bt: %s, Recovered from: %s", mr.Method, string(debug.Stack()), err)
	}
}

func startServer(intelephense, copilot, volar, gopls mrChan, port string) {
	Log("Running webserver on port: %s", port)
	server = mateServer{
		intelephense: intelephense,
		volar:        volar,
		copilot:      copilot,
		gopls:        gopls,
		initialized:  false,
		logger: &Logger{
			IncomingPrefix: "HTTP <-- IDE", OutgoingPrefix: "HTTP --> IDE",
			HiColor: hiGreenString, LoColor: greenString, ErrorColor: errorString,
		},
		openFiles:   make(map[string]time.Time),
		openFolders: make(map[string]lsp.DocumentURI),
	}

	log.Fatal(http.ListenAndServe(":"+port, &server))
}
