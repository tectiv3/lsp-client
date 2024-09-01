package main

import (
	"database/sql/driver"
	"fmt"
	"github.com/tectiv3/go-lsp"
	"github.com/tectiv3/go-lsp/jsonrpc"
	"go.bug.st/json"
	"io"
	"os"
	"sync"
	"time"
)

type Config struct {
	NodePath            string `json:"node_path"`
	CopilotPath         string `json:"copilot_path"`
	VolarPath           string `json:"volar_path"`
	GoplsPath           string `json:"gopls_path"`
	IntelephensePath    string `json:"intelephense_path"`
	IntelephenseLicense string `json:"intelephense_license"`
	IntelephenseStorage string `json:"intelephense_storage"`
	TsdkPath            string `json:"tsdk_path"`
	Port                string `json:"port"`
	EnableLogging       bool   `json:"enable_logging"`
}

type signInResponse struct {
	Status          string `json:"status"`
	User            string `json:"user"`
	VerificationUri string `json:"verificationUri,omitempty"`
	UserCode        string `json:"userCode,omitempty"`
}

type Completion struct {
	UUID  string `json:"uuid"`
	Text  string `json:"text"`
	Range struct {
		Start struct {
			Line      int `json:"line"`
			Character int `json:"character"`
		} `json:"start"`
		End struct {
			Line      int `json:"line"`
			Character int `json:"character"`
		} `json:"end"`
	} `json:"range"`
	DisplayText string `json:"displayText"`
	Position    struct {
		Line      int `json:"line"`
		Character int `json:"character"`
	} `json:"position"`
	DocVersion int `json:"docVersion"`
}

type CompletionsResponse struct {
	Completions []Completion `json:"completions"`
}

// KeyValue is basic key:value struct
type KeyValue map[string]interface{}

// bool returns the value of the given name, assuming the value is a boolean.
// If the value isn't found or is not of the type, the defaultValue is returned.
func (kv KeyValue) bool(name string, defaultValue bool) bool {
	if v, found := kv[name]; found {
		if castValue, is := v.(bool); is {
			return castValue
		}
	}
	return defaultValue
}

// int returns the value of the given name, assuming the value is an int.
// If the value isn't found or is not of the type, the defaultValue is returned.
func (kv KeyValue) int(name string, defaultValue int) int {
	if v, found := kv[name]; found {
		if castValue, is := v.(int); is {
			return castValue
		}
	}
	return defaultValue
}

// string returns the value of the given name, assuming the value is a string.
// If the value isn't found or is not of the type, the defaultValue is returned.
func (kv KeyValue) string(name string, defaultValue string) string {
	if v, found := kv[name]; found {
		if castValue, is := v.(string); is {
			return castValue
		}
	}
	return defaultValue
}

// float64 returns the value of the given name, assuming the value is a float64.
// If the value isn't found or is not of the type, the defaultValue is returned.
func (kv KeyValue) float64(name string, defaultValue float64) float64 {
	if v, found := kv[name]; found {
		if castValue, is := v.(float64); is {
			return castValue
		}
	}
	return defaultValue
}

func (kv KeyValue) keyValue(name string, defaultValue KeyValue) KeyValue {
	if v, found := kv[name]; found {
		if castValue, is := v.(KeyValue); is {
			return castValue
		}
		if castValue, is := v.(map[string]interface{}); is {
			return castValue
		}
	}
	return defaultValue
}

func (kv KeyValue) array(name string, defaultValue []interface{}) []interface{} {
	if v, found := kv[name]; found {
		if castValue, is := v.([]interface{}); is {
			return castValue
		}
	}
	return defaultValue
}

// Value get value of KeyValue
func (kv KeyValue) Value() (driver.Value, error) {
	return json.Marshal(kv)
}

// Scan scan value into KeyValue
func (kv *KeyValue) Scan(value interface{}) error {
	bytes, ok := value.([]byte)

	if !ok {
		return fmt.Errorf("failed to unmarshal JSON value: %v", value)
	}

	return json.Unmarshal(bytes, kv)
}

type mateRequest struct {
	Method string
	Body   json.RawMessage
	CB     kvChan
}

type mateServer struct {
	copilot      mrChan
	intelephense mrChan
	volar        mrChan
	gopls        mrChan
	initialized  bool
	logger       jsonrpc.Logger
	openFiles    map[string]time.Time
	currentWS    *workSpace
	openFolders  map[string]lsp.DocumentURI
	sync.Mutex
}

type workSpace struct {
	name string
	uri  string
}

type kvChan chan *KeyValue
type mrChan chan *mateRequest

type dumper struct {
	upstream io.ReadWriteCloser
	logfile  *os.File
	reading  bool
	writing  bool
}

func (d *dumper) Read(buff []byte) (int, error) {
	n, err := d.upstream.Read(buff)
	if err != nil {
		d.logfile.Write([]byte(fmt.Sprintf("<<< Read Error: %s\n", err)))
	} else {
		if !d.reading {
			d.reading = true
			d.writing = false
			d.logfile.Write([]byte("\n<<<\n"))
		}
		d.logfile.Write(buff[:n])
	}
	return n, err
}

func (d *dumper) Write(buff []byte) (int, error) {
	n, err := d.upstream.Write(buff)
	if err != nil {
		_, _ = d.logfile.Write([]byte(fmt.Sprintf(">>> Write Error: %s\n", err)))
	} else {
		if !d.writing {
			d.writing = true
			d.reading = false
			d.logfile.Write([]byte("\n>>>\n"))
		}
		_, _ = d.logfile.Write(buff[:n])
	}
	return n, err
}

func (d *dumper) Close() error {
	err := d.upstream.Close()
	_, _ = d.logfile.Write([]byte(fmt.Sprintf("--- Stream closed, err=%s\n", err)))
	_ = d.logfile.Close()
	return err
}

type combinedReadWriteCloser struct {
	reader io.ReadCloser
	writer io.WriteCloser
}

func (sd *combinedReadWriteCloser) Read(p []byte) (int, error) {
	return sd.reader.Read(p)
}

func (sd *combinedReadWriteCloser) Write(p []byte) (int, error) {
	return sd.writer.Write(p)
}

func (sd *combinedReadWriteCloser) Close() error {
	ierr := sd.reader.Close()
	oerr := sd.writer.Close()
	if ierr != nil {
		return ierr
	}
	if oerr != nil {
		return oerr
	}
	return nil
}
