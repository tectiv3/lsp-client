# Task Completion Checklist

When completing a coding task in this project, follow these guidelines:

## Standard Tasks

### After Making Code Changes
1. **Format the code**: Run `go fmt ./...` to ensure consistent formatting
2. **Check for issues**: Run `go vet ./...` to catch potential problems
3. **Build the project**: Run `go build` to ensure it compiles
4. **Test manually** (if applicable): Run `./lsp-client` to verify functionality

### Testing Policy
⚠️ **IMPORTANT**: Per `dear_llm.md`:
- **Do NOT write tests** unless explicitly asked
- **Do NOT update README** unless explicitly asked
- Focus on the requested changes only

### When Tests Are Requested
If the user explicitly asks for tests:
1. Write tests following existing patterns
2. Run tests: `go test -v`
3. Ensure all tests pass
4. Check coverage if needed: `go test -cover`

### For Bug Fixes
1. Identify the root cause
2. Make the minimal necessary change
3. Format and vet the code
4. Build and test the fix
5. Verify the bug is resolved

### For New Features
1. Implement the feature in the appropriate file
2. Follow existing code patterns and structure
3. Update configuration if needed
4. Format and vet the code
5. Build and test the feature
6. Do NOT add documentation unless asked

### For Refactoring
1. Make the changes incrementally
2. Ensure backward compatibility
3. Format and vet after each step
4. Build frequently to catch issues early
5. Test manually to ensure behavior is unchanged

## What NOT to Do
- ❌ Don't write tests unless explicitly requested
- ❌ Don't update README unless explicitly requested
- ❌ Don't add excessive comments or documentation
- ❌ Don't over-engineer solutions
- ❌ Don't make changes beyond what was requested

## Quality Checks (Always Do)
- ✅ Code is formatted: `go fmt ./...`
- ✅ No vet warnings: `go vet ./...`
- ✅ Project builds: `go build`
- ✅ Changes are minimal and focused
- ✅ Follows existing code style and patterns

## Dependencies
If you add or update dependencies:
1. Run `go mod tidy` to clean up
2. Verify with `go mod verify`
3. Commit both `go.mod` and `go.sum`

## When in Doubt
- Keep changes minimal
- Follow existing patterns
- Ask the user if clarification is needed
- Don't assume additional work is wanted
