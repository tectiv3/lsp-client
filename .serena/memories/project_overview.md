# LSP Client Project Overview

## Purpose
This is an LSP (Language Server Protocol) client that acts as a bridge between TextMate editor and various language servers. It provides:

- GitHub Copilot integration with automatic authentication
- Intelephense language server for PHP
- Gopls language server for Go
- Volar language server for Vue.js
- HTTP server to handle requests from TextMate editor

The project requires the [PHP-LSP.tmbundle](https://github.com/tectiv3/PHP-LSP.tmbundle) for TextMate integration.

## Architecture
The application starts multiple language server clients concurrently:
1. Each language server runs in its own goroutine
2. An HTTP server (`mateServer`) handles incoming requests from TextMate
3. Requests are routed to the appropriate language server based on the method
4. Results are returned to TextMate via HTTP responses

## Key Features
- **Automatic Copilot Authentication**: Terminal-based GitHub OAuth flow on startup
- **Multi-language Support**: Works with PHP, Go, Vue, and any language supported by Copilot
- **Configurable**: All paths and settings via `config.json`
- **Logging**: Optional detailed logging with colored output
- **RPC Communication**: Uses JSON-RPC 2.0 for LSP communication
