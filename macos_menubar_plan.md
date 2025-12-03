# macOS Menu Bar Application Plan

## Executive Summary
Add a native macOS menu bar application to the LSP client that provides real-time status information, authentication management, and quick actions for developers. The menu bar app will run concurrently with the existing LSP server and provide a modern, user-friendly interface for monitoring and controlling the application.

---

## 1. Current Architecture Analysis

### Existing Components
- **Main Application**: Starts multiple LSP clients in goroutines (Copilot, Intelephense, Gopls, Volar)
- **HTTP Server**: mateServer handles TextMate editor requests on port 8787
- **Authentication**: TerminalAuth struct manages Copilot OAuth flow
- **Global State**: cClient variable holds Copilot handler with connection info
- **Communication**: JSON-RPC over channels (mrChan type)

### Key Files
- `main.go`: Application entry point, starts all services
- `auth.go`: TerminalAuth with authentication methods
- `copilot.go`: Copilot client with re-authentication handling
- `handler.go`: LSP handler with connection management
- `models.go`: Data structures including signInResponse, checkStatusResponse
- `server.go`: HTTP server implementation

### Authentication Flow
1. Startup calls CheckAndPerformAuth
2. If not authenticated, triggers PerformTerminalAuth
3. Gets user code and verification URI from Copilot server
4. User manually enters code in browser
5. Automatic re-authentication via handleReauthentication when auth errors occur

---

## 2. Architecture Design

### Component Structure

#### A. Menu Bar App Manager (New)
Create a new file `menubar.go` with:
- MenuBarApp struct to manage the menu bar lifecycle
- Integration with existing application state
- Communication channels for status updates

#### B. Status Monitor (New)
Create a new file `status.go` with:
- StatusMonitor struct to track application state
- Methods to query Copilot authentication status
- Methods to check LSP server health
- Periodic polling of server states

#### C. Auth State Bridge (Modification)
Enhance existing auth.go to:
- Add GetCurrentAuthStatus method to TerminalAuth
- Add GetAuthCode method to retrieve current pending auth code
- Add status change notification mechanism

#### D. Main Integration (Modification)
Update main.go to:
- Initialize menu bar app on startup
- Pass communication channels to menu bar
- Maintain existing functionality unchanged

### Communication Pattern

```
┌─────────────────────────────────────────────────────────┐
│                    Main Application                      │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐              │
│  │ Copilot  │  │Inteleph. │  │  Gopls   │  ┌─────────┐ │
│  │  Client  │  │  Client  │  │  Client  │  │  HTTP   │ │
│  └────┬─────┘  └────┬─────┘  └────┬─────┘  │ Server  │ │
│       │             │             │         └─────────┘ │
│       └─────────────┴─────────────┘                     │
│                     │                                    │
│              ┌──────▼────────┐                          │
│              │ Status Monitor │◄─────────┐              │
│              └──────┬────────┘          │              │
│                     │                   │              │
│              ┌──────▼────────┐    Status Channel      │
│              │  MenuBar App  │          │              │
│              └───────────────┘◄─────────┘              │
└─────────────────────────────────────────────────────────┘
```

---

## 3. Implementation Plan

### Phase 1: Core Infrastructure

#### Step 1.1: Create Status Types and Channels
In `models.go`, add new types:
- AppStatus struct with fields for each server state
- AuthStatus struct with authentication state details
- StatusUpdate struct for channel communication
- Define statusChan channel type

#### Step 1.2: Create Status Monitor
Create `status.go`:
- Define StatusMonitor struct with references to all clients
- Implement NewStatusMonitor constructor
- Create method GetCopilotAuthStatus that queries checkStatus
- Create method GetAuthCodeIfPending to check for pending auth
- Create method GetServerStatuses to check all LSP server health
- Add StartMonitoring method with polling ticker
- Add channel for broadcasting status changes

#### Step 1.3: Bridge Authentication System
Modify `auth.go`:
- Add authStatusChan field to TerminalAuth struct
- Add GetCurrentStatus method returning AuthStatus
- Add GetPendingAuthCode method returning code if auth in progress
- Modify PerformTerminalAuth to broadcast status updates via channel
- Add IsAuthInProgress method to check if auth flow is active

### Phase 2: Menu Bar Implementation

#### Step 2.1: Set Up macOS Menu Bar Framework
Create `menubar.go` using cgo and Objective-C bridge:
- Import necessary Cocoa frameworks via cgo
- Create MenuBarApp struct
- Define app lifecycle methods (Initialize, Run, Terminate)
- Set up NSStatusBar item with icon

#### Step 2.2: Implement Menu Construction
In `menubar.go`:
- Create buildMenu method that constructs NSMenu
- Add menu items with dynamic content
- Set up menu item callbacks using cgo function pointers
- Implement menu refresh logic

#### Step 2.3: Create Menu Item Handlers
In `menubar.go`:
- Handler for showing authentication status (clickable to open browser)
- Handler for copying auth code to clipboard (if pending)
- Handler for manual re-authentication trigger
- Handler for showing server statuses
- Handler for opening logs directory
- Handler for quit action

### Phase 3: Integration

#### Step 3.1: Modify Main Application
In `main.go`:
- Create status channel
- Initialize StatusMonitor with references to all clients
- Start StatusMonitor goroutine
- Initialize MenuBarApp with status channel
- Start MenuBarApp in separate goroutine
- Ensure graceful shutdown includes menu bar cleanup

#### Step 3.2: Connect Copilot Client
In `copilot.go`:
- Pass status channel to startCopilot
- Broadcast auth status changes
- Send notifications when re-authentication occurs
- Update handleReauthentication to notify menu bar

#### Step 3.3: Add Health Checks
Create methods in each client file:
- Add IsHealthy method to check connection state
- Check if LSP connection is active
- Verify last successful request time

### Phase 4: Advanced Features

#### Step 4.1: Implement Nice-to-Have Features
- Server status indicators (running, idle, error states)
- Click on Copilot status to manually trigger re-auth
- Quick access to view recent logs
- Show number of open files/connections
- Display current workspace if available
- Restart individual language servers
- Show server performance metrics (request count, response time)

#### Step 4.2: Add Notification Support
- Use macOS UserNotifications framework
- Notify user when authentication required
- Notify when authentication succeeds
- Notify on server errors or crashes
- Configurable notification preferences

#### Step 4.3: Implement Preferences
- Add submenu for preferences
- Toggle notification settings
- Toggle auto-update of menu (polling interval)
- Choose which servers to monitor
- Set log verbosity from menu

---

## 4. Technical Implementation Details

### 4.1 macOS Frameworks Needed

Use one of these approaches:

**Option A: Pure Go with cgo (Recommended)**
- Use cgo to bridge to Objective-C/Cocoa APIs
- Directly call NSStatusBar, NSMenu, NSMenuItem APIs
- Provides full control and native performance
- Examples: macdriver, cocoa bindings

**Option B: Third-Party Go Library**
- Use github.com/caseymrm/menuet (simple, pure Go)
- Use github.com/getlantern/systray (cross-platform)
- Pros: Simpler implementation, less code
- Cons: Less flexible, may have limitations

**Recommendation**: Start with Option B (systray library) for rapid prototyping, then consider Option A if more customization is needed.

### 4.2 Menu Structure Design

```
┌─────────────────────────────────────┐
│        📊 LSP Client                │  (Title with icon)
├─────────────────────────────────────┤
│ ✓ Copilot: Authenticated (user)    │  (Green checkmark or red X)
│   └─ Re-authenticate                │  (Submenu, only if needed)
├─────────────────────────────────────┤
│ Server Status:                      │  (Disabled header)
│   ● Copilot: Running                │  (Color-coded dots)
│   ● Intelephense: Running           │
│   ● Gopls: Running                  │
│   ● Volar: Running                  │
│   ● HTTP Server: :8787              │
├─────────────────────────────────────┤
│ 📁 Open Logs Directory              │
│ 🔄 Restart All Servers              │  (Future enhancement)
│ ⚙️  Preferences                     │
├─────────────────────────────────────┤
│ ❌ Quit                             │
└─────────────────────────────────────┘
```

### 4.3 Auth Code Display (When Re-auth Needed)

```
┌─────────────────────────────────────┐
│        📊 LSP Client                │
├─────────────────────────────────────┤
│ ⚠️  Authentication Required         │
│                                     │
│ Your code: A1B2-C3D4               │  (Large, highlighted)
│ (Click to copy)                     │
│                                     │
│ Open GitHub to authorize ↗          │
├─────────────────────────────────────┤
│ [Rest of menu...]                   │
└─────────────────────────────────────┘
```

### 4.4 Status Polling Strategy

Implement smart polling to minimize overhead:
- Poll Copilot auth status every 30 seconds
- Poll server health every 10 seconds
- Increase frequency to 5 seconds during auth flow
- Use exponential backoff if server is down
- Subscribe to event notifications when available

### 4.5 Clipboard Integration

For copying auth code:
- Use darwin's NSPasteboard via cgo, or
- Use third-party library like atotto/clipboard
- Provide visual feedback after copy (checkmark icon)

---

## 5. File Changes Summary

### New Files
1. `menubar.go` - Menu bar app implementation (300-400 lines)
2. `status.go` - Status monitoring logic (200-300 lines)
3. `menubar_darwin.go` - Platform-specific code (if using cgo)
4. `icons/icon.png` - Menu bar icon asset

### Modified Files
1. `main.go` - Add menu bar initialization (15-20 lines)
2. `auth.go` - Add status getters and notifications (30-50 lines)
3. `copilot.go` - Add status broadcasting (20-30 lines)
4. `models.go` - Add status types (40-60 lines)
5. `go.mod` - Add new dependencies

### Configuration Changes
- Add menubar settings to config.json
- Add option to disable menu bar if desired
- Add notification preferences

---

## 6. Dependencies to Add

```go
// Option 1: Using systray (simpler)
require (
    github.com/getlantern/systray v1.2.2
    github.com/atotto/clipboard v0.1.4
)

// Option 2: Using cgo/cocoa (more control)
require (
    github.com/atotto/clipboard v0.1.4
    // Native cgo - no additional dependencies
)
```

---

## 7. Implementation Steps (Sequential)

### Day 1: Foundation
1. Add status types to models.go
2. Create status.go with monitoring logic
3. Test status monitoring independently
4. Add status getters to auth.go

### Day 2: Menu Bar Basic
1. Choose library (systray vs cgo)
2. Create menubar.go with basic structure
3. Implement menu item creation
4. Test menu bar appears and responds to clicks

### Day 3: Integration
1. Connect status monitor to menu bar
2. Update main.go to initialize everything
3. Implement status display in menu
4. Test end-to-end status updates

### Day 4: Authentication Display
1. Implement auth code display logic
2. Add clipboard copy functionality
3. Test auth flow with menu bar active
4. Add click-to-open-browser functionality

### Day 5: Polish & Features
1. Add all server status indicators
2. Implement logs directory opener
3. Add preferences submenu
4. Test notification system
5. Icon design and integration
6. Final testing and bug fixes

---

## 8. Testing Strategy

### Unit Tests
- Test StatusMonitor methods independently
- Test auth status getters
- Mock LSP clients for testing
- Test menu item creation logic

### Integration Tests
- Test status updates flow from client to menu
- Test auth code retrieval during auth flow
- Test clipboard copy functionality
- Test graceful shutdown

### Manual Testing Checklist
- Menu bar icon appears on startup
- Status updates reflect reality
- Auth code appears during authentication
- Copy to clipboard works
- Click to open browser works
- Quit properly shuts down all components
- Works after authentication expires
- Handles server crashes gracefully
- Memory usage stays reasonable

---

## 9. Nice-to-Have Features (Prioritized)

### High Priority
1. **Server Status Indicators**: Show if each LSP server is running, idle, or errored
2. **Auth Code Inline Display**: Show code directly in menu with one-click copy
3. **Open Logs**: Quick access to view recent logs
4. **Manual Re-auth**: Trigger Copilot re-authentication from menu

### Medium Priority
5. **Workspace Info**: Show current workspace path if available
6. **Connection Count**: Display number of active file connections
7. **Click to Retry**: Click on errored server to attempt restart
8. **Notifications**: macOS notifications for auth required, errors
9. **Request Statistics**: Show request count and avg response time per server

### Low Priority (Future Enhancements)
10. **Server Restart**: Restart individual servers from menu
11. **Config Editor**: Open and edit config.json from menu
12. **Performance Graph**: Mini graph showing server load over time
13. **Dark Mode Support**: Automatic icon switching for dark/light mode
14. **Keyboard Shortcuts**: Global hotkeys for common actions
15. **Recent Errors Log**: View recent error messages in submenu
16. **Auto-Update Check**: Check for new versions of LSP client

---

## 10. Potential Challenges & Solutions

### Challenge 1: CGo Complexity
**Problem**: Using cgo for Cocoa APIs adds build complexity and isn't cross-platform
**Solution**: Use systray library initially; provides 80% of needed functionality with pure Go

### Challenge 2: Status Polling Overhead
**Problem**: Constantly querying server status could impact performance
**Solution**: Implement smart polling with configurable intervals; use event-driven updates where possible

### Challenge 3: Auth Flow Race Conditions
**Problem**: Auth state might change between checks; menu could show stale data
**Solution**: Use channels for state updates; implement proper locking in StatusMonitor

### Challenge 4: Menu Updates Not Appearing
**Problem**: macOS menu bar updates might not appear immediately
**Solution**: Ensure updates happen on main thread; use proper notification mechanisms

### Challenge 5: Graceful Shutdown
**Problem**: Menu bar app might not quit cleanly with rest of application
**Solution**: Implement proper shutdown signals; ensure menu bar cleanup in defer statements

---

## 11. Alternative Approaches Considered

### Approach 1: Separate Menu Bar Binary
**Description**: Build menu bar as separate executable that communicates with LSP client via IPC
**Pros**: Better separation of concerns; can restart LSP without losing menu
**Cons**: More complex; requires IPC protocol; two binaries to maintain
**Decision**: Not recommended for v1

### Approach 2: Web-Based Status Dashboard
**Description**: Add web endpoint to HTTP server showing status dashboard
**Pros**: Cross-platform; no native code needed; rich UI possible
**Cons**: Not always visible; requires browser; less native feeling
**Decision**: Could complement menu bar, not replace it

### Approach 3: CLI Command for Status
**Description**: Add subcommand to binary for checking status
**Pros**: Simple to implement; scriptable
**Cons**: Not real-time; requires manual checking
**Decision**: Good addition but insufficient alone

---

## 12. Success Criteria

The implementation will be considered successful when:

1. Menu bar icon appears on application startup
2. Copilot authentication status displays accurately (authenticated/not authenticated)
3. When auth required, code appears in menu and can be copied with one click
4. All four LSP server statuses show correctly (running/stopped/error)
5. Quit option properly terminates entire application
6. No memory leaks or performance degradation over 24 hour run
7. Works correctly through full auth expiration and re-auth cycle
8. Gracefully handles server crashes and restarts
9. User can access logs directory with one menu click
10. Code follows existing project style and conventions

---

## 13. Future Expansion Possibilities

Once basic menu bar is working, consider:

1. **Multi-Workspace Support**: Show status for multiple open projects
2. **Plugin System**: Allow custom menu items via configuration
3. **Remote Monitoring**: Monitor LSP client running on remote machine
4. **Statistics Export**: Export usage statistics to file
5. **Integration with Other Tools**: Webhook notifications to Slack, Discord, etc.
6. **Custom Theming**: User-customizable menu appearance
7. **Context-Aware Actions**: Different menu items based on current file type
8. **Quick Switch**: Switch between different LSP server configurations

---

## 14. Documentation Requirements

Create documentation for:

1. **User Guide**: How to use menu bar features
2. **Build Instructions**: How to build with menu bar enabled
3. **Configuration Guide**: Menu bar settings in config.json
4. **Architecture Docs**: Update project architecture with menu bar component
5. **API Documentation**: Document status monitoring API for future extensions

---

## 15. Estimated Effort

- **Core Infrastructure**: 8-12 hours
- **Menu Bar Implementation**: 6-10 hours
- **Integration & Testing**: 4-6 hours
- **Polish & Documentation**: 3-4 hours
- **Total Estimated Time**: 21-32 hours (3-4 days of focused work)

---

## Conclusion

This plan provides a comprehensive roadmap for adding a native macOS menu bar to the LSP client. The implementation will significantly improve the user experience by providing real-time visibility into application state, streamlining the authentication process, and offering quick access to common developer actions.

The phased approach allows for incremental development and testing, with clear success criteria at each stage. The use of existing Go libraries minimizes complexity while the architecture allows for future enhancements without major refactoring.

Key benefits:
- Always-visible status information
- Simplified authentication with inline code display
- Quick access to logs and diagnostics
- Native macOS integration
- Foundation for future developer productivity features
