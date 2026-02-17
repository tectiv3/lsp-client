# Codebase Structure

## Main Files

### Entry Point
- **`main.go`**: Application entry point
  - Reads configuration
  - Starts all language server clients concurrently
  - Starts HTTP server
  - Handles graceful shutdown on Ctrl-C

### Core Components
- **`server.go`**: HTTP server (`mateServer`) handling TextMate requests
  - `ServeHTTP`: Main request handler
  - `processRequest`: Routes requests to appropriate language server
  - `onDidOpen`, `onDidClose`: Document lifecycle handlers
  - `onInitialize`: Workspace initialization
  - `sendLSPRequest`: Sends requests to language servers

- **`handler.go`**: LSP protocol handler
  - Implements LSP notification callbacks
  - `startRPCServer`: Creates and configures RPC server connections
  - Handles client/server capability registration
  - Window, workspace, and telemetry event handlers

- **`models.go`**: Data structures and types
  - `Config`: Application configuration
  - `mateRequest`, `mateServer`: HTTP request/server types
  - `workSpace`: Workspace configuration
  - Authentication response types
  - Channel types: `kvChan`, `mrChan`
  - `KeyValue`: Flexible value type with type conversion methods
  - `dumper`, `combinedReadWriteCloser`: I/O utilities

### Language Server Clients
- **`copilot.go`**: GitHub Copilot integration
  - `startCopilot`: Initializes Copilot LSP client
  - `processCopilotRequests`: Handles Copilot-specific requests
  - `sendRequest`, `sendRequestWithAuth`: Request handling with auth
  - Re-authentication logic for expired tokens

- **`intelephense.go`**: PHP language server
  - `startIntelephense`: Initializes Intelephense
  - `processIntelephenseRequests`: Handles PHP LSP requests

- **`gopls.go`**: Go language server
  - `startGopls`: Initializes Gopls
  - `processGoplsRequests`: Handles Go LSP requests

- **`vue.go`**: Vue language server (@vue/language-server)
  - `startVue`: Initializes Vue LS with --tsdk CLI arg and hybridMode option
  - `processVueRequests`: Handles Vue LSP requests

### Supporting Files
- **`auth.go`**: Authentication logic and terminal flow
- **`logger.go`**: Custom logging with colored output
- **`utils.go`**: Utility functions
- **`config.json.default`**: Configuration template
- **`config.json`**: User configuration (gitignored)

## Test Files
- **`auth_test.go`**: Authentication tests
- **`terminal_auth_test.go`**: Terminal authentication flow tests

## Directory Structure
```
.
├── main.go              # Entry point
├── server.go            # HTTP server
├── handler.go           # LSP handler
├── models.go            # Data structures
├── copilot.go           # Copilot client
├── intelephense.go      # PHP client
├── gopls.go             # Go client
├── vue.go               # Vue client
├── setup.go             # First-run config wizard (bubbletea TUI)
├── auth.go              # Authentication
├── logger.go            # Logging
├── utils.go             # Utilities
├── config.json.default  # Config template
├── go.mod              # Dependencies
├── go.sum              # Dependency checksums
├── README.md           # Documentation
├── dear_llm.md         # LLM instructions
└── logs/               # Log directory
```
