package main

import (
	"go.bug.st/json"
	"log"
	"net/http"
	"runtime/debug"
	"time"
)

func (s *mateServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	defer func() {
		if err := Panicf(recover(), "%v", r.Method); err != nil {
			LogError(err)
			w.WriteHeader(http.StatusInternalServerError)
		}
	}()

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
		Log("method: %s Time out", mr.Method)
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
		//	cb <- &KeyValue{"result": "error", "message": err.Error()}
		//	return
		//}
		//s.requestAndWait("textDocument/hover", params, cb)
		params := KeyValue{}
		if err := json.Unmarshal(mr.Body, &params); err != nil {
			cb <- &KeyValue{"result": "error", "message": err.Error()}
			return
		}
		resp := s.sendLSPRequest(s.intelephense, "textDocument/hover", params)

		cb <- resp
	case "completion":
		//params := lsp.CompletionParams{}
		//if err := json.Unmarshal(mr.Body, &params); err != nil {
		//	cb <- &KeyValue{"result": "error", "message": err.Error()}
		//	return
		//}
		//s.requestAndWait("textDocument/completion", params, cb)
		params := KeyValue{}
		if err := json.Unmarshal(mr.Body, &params); err != nil {
			cb <- &KeyValue{"result": "error", "message": err.Error()}
			return
		}
		result := s.sendLSPRequest(s.intelephense, "textDocument/completion", params)
		Log("Sending completion response")
		cb <- result
	case "definition":
		//params := lsp.TextDocumentPositionParams{}
		//if err := json.Unmarshal(mr.Body, &params); err != nil {
		//	cb <- &KeyValue{"result": "error", "message": err.Error()}
		//	return
		//}
		//s.requestAndWait("textDocument/definition", params, cb)
		params := KeyValue{}
		if err := json.Unmarshal(mr.Body, &params); err != nil {
			cb <- &KeyValue{"result": "error", "message": err.Error()}
			return
		}
		result := s.sendLSPRequest(s.intelephense, "textDocument/definition", params)
		Log("Sending definition response")
		cb <- result

	case "initialize":
		s.onInitialize(mr, cb)
	case "didOpen":
		s.onDidOpen(mr, cb)
	case "didClose":
		s.onDidClose(mr, cb)
	default:
		cb <- &KeyValue{"result": "error", "message": "unknown method"}
	}
	Log("method: %s %s", mr.Method, "processRequest finished")
}

func (s *mateServer) onDidOpen(mr mateRequest, cb kvChan) {
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

	if _, ok := s.openFiles[fn]; ok {
		//Log("file %s already opened", fn)
		s.sendLSPRequest(s.intelephense, "textDocument/didClose", KeyValue{
			"uri": fn,
		})
		time.Sleep(100 * time.Millisecond)
	}
	s.openFiles[fn] = time.Now()

	Log("getting diagnostics for %s", fn)
	s.sendLSPRequest(s.intelephense, "textDocument/didOpen", params)

	diagnostics := s.sendLSPRequest(s.intelephense, "textDocument/documentSymbol", KeyValue{"textDocument": KeyValue{"uri": fn}})
	Log("Sending diagnostics response")
	cb <- diagnostics
}

func (s *mateServer) onDidClose(mr mateRequest, cb kvChan) {
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
	s.sendLSPRequest(s.intelephense, "textDocument/didClose", KeyValue{
		"uri": fn,
	})
	delete(s.openFiles, fn)

	cb <- &KeyValue{"result": "ok"}
}

func (s *mateServer) onInitialize(mr mateRequest, cb kvChan) {
	params := KeyValue{}
	if err := json.Unmarshal(mr.Body, &params); err != nil {
		cb <- &KeyValue{"result": "error", "message": err.Error()}
		return
	}

	if s.initialized {
		cb <- &KeyValue{"result": "ok", "message": "already initialized"}
		return
	}

	s.sendLSPRequest(s.copilot, "initialize", KeyValue{})
	s.sendLSPRequest(s.copilot, "signIn", KeyValue{})

	dir := params.string("dir", "")
	if len(dir) == 0 {
		cb <- &KeyValue{"result": "error", "message": "Empty dir"}
		return

	}
	s.sendLSPRequest(s.intelephense, "initialize", params)

	s.initialized = true
	s.currentWS = &workSpace{"event", ""}
	cb <- &KeyValue{"result": "ok"}
}

func (s *mateServer) sendLSPRequest(out mrChan, method string, params KeyValue) *KeyValue {
	s.Lock()
	defer s.Unlock()

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

func startServer(intelephense, copilot mrChan, port string) {
	Log("Running webserver on port: %s", port)
	server := mateServer{
		intelephense: intelephense,
		copilot:      copilot, initialized: false,
		logger: &Logger{
			IncomingPrefix: "HTTP <-- IDE", OutgoingPrefix: "HTTP --> IDE",
			HiColor: hiGreenString, LoColor: greenString, ErrorColor: errorString,
		},
		openFiles: make(map[string]time.Time),
	}

	log.Fatal(http.ListenAndServe(":"+port, &server))
}
