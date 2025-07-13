### Copilot and Intelephense for Textmate

Requires [this bundle](https://github.com/tectiv3/PHP-LSP.tmbundle) in Textmate.

Copilot can work without Intelephense with any supported by copilot language.

## Authentication

The server now supports automatic GitHub Copilot authentication during startup with a terminal-based flow.

### Automatic Terminal Authentication

When the server starts, it automatically:
1. Checks if Copilot is already authenticated
2. If not authenticated, starts an interactive terminal login flow
3. Displays a user code and opens GitHub in your browser
4. Waits for you to complete authentication
5. Verifies the authentication was successful

The authentication happens automatically when you start the server - no additional setup required!

**Features:**
- Automatic browser opening (cross-platform)
- User-friendly terminal interface with colored output
- Retry mechanism (up to 3 attempts)
- Graceful fallback if authentication fails

### Manual API Authentication

The server also supports GitHub Copilot authentication through the following API endpoints:

### Authentication Flow

1. **Start Authentication** - `POST /` with method `signIn`
   - Initiates GitHub OAuth flow
   - Returns user code and verification URI
   - Response includes:
     - `status`: "pending" or "success" (if already signed in)
     - `userCode`: Code to enter on GitHub
     - `verificationUri`: GitHub URL to visit
     - `expiresIn`: Code expiration time in seconds
     - `interval`: Polling interval in seconds

2. **Complete Authentication** - `POST /` with method `signInConfirm`
   - Confirms authentication after user enters code on GitHub
   - Request body: `{"userCode": "YOUR_CODE"}`
   - Returns success/error status with user info

3. **Check Status** - `POST /` with method `checkStatus` or `authStatus`
   - Checks current authentication status
   - Returns user info if authenticated

### Example Authentication Flow

```bash
# 1. Start authentication
curl -X POST http://localhost:8080 \
  -H "Content-Type: application/json" \
  -d '{"Method": "signIn", "Body": {}}'

# Response: {"status": "pending", "userCode": "ABC123", "verificationUri": "https://github.com/login/device", ...}

# 2. User visits the verification URI and enters the code

# 3. Confirm authentication
curl -X POST http://localhost:8080 \
  -H "Content-Type: application/json" \
  -d '{"Method": "signInConfirm", "Body": {"userCode": "ABC123"}}'

# Response: {"status": "success", "user": "username"}

# 4. Check authentication status anytime
curl -X POST http://localhost:8080 \
  -H "Content-Type: application/json" \
  -d '{"Method": "authStatus", "Body": {}}'
```