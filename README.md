### Copilot and Intelephense for Textmate

Requires [this bundle](https://github.com/tectiv3/PHP-LSP.tmbundle) in Textmate.

Copilot can work without Intelephense with any supported by copilot language.

## Authentication

The server supports GitHub Copilot authentication through the following endpoints:

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