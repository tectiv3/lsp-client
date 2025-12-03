# Tech Stack

## Language & Runtime
- **Go**: Version 1.21
- **Toolchain**: go1.23.3
- **Platform**: macOS (Darwin) - but cross-platform compatible

## Dependencies
Main dependencies from `go.mod`:
- `github.com/tectiv3/go-lsp v0.0.0-20240419022041-0a0a5672827e` - LSP client library
- `go.bug.st/json v1.15.6` - JSON processing
- `github.com/arduino/go-paths-helper v1.12.1` - Path utilities (indirect)

## External Tools Required
The following external tools must be installed and configured in `config.json`:
- **Node.js**: Required for running language servers
- **Copilot Node Server**: GitHub Copilot language server
- **Intelephense**: PHP language server
- **Volar (Vue Language Server)**: Vue.js language server  
- **Gopls**: Go language server (optional, usually bundled with Go)
- **TypeScript SDK**: For Vue/TypeScript support

## Communication Protocols
- **JSON-RPC 2.0**: For LSP communication
- **HTTP**: For TextMate integration (default port: 8787)
- **GitHub OAuth**: For Copilot authentication
