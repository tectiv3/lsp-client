.PHONY: build app clean install run icon

BINARY_NAME=lsp-client
APP_NAME=LSP Client.app
APP_DIR=$(APP_NAME)/Contents
MACOS_DIR=$(APP_DIR)/MacOS
RESOURCES_DIR=$(APP_DIR)/Resources

build:
	@echo "Building $(BINARY_NAME)..."
	@go build -o $(BINARY_NAME) .

app: build
	@echo "Creating macOS app bundle..."
	@rm -rf "$(APP_NAME)"
	@mkdir -p "$(MACOS_DIR)"
	@mkdir -p "$(RESOURCES_DIR)"
	@cp $(BINARY_NAME) "$(MACOS_DIR)/"
	@cp config.json "$(MACOS_DIR)/"
	@if [ -f AppIcon.icns ]; then cp AppIcon.icns "$(RESOURCES_DIR)/"; fi
	@printf '<?xml version="1.0" encoding="UTF-8"?>\n' > "$(APP_DIR)/Info.plist"
	@printf '<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">\n' >> "$(APP_DIR)/Info.plist"
	@printf '<plist version="1.0">\n' >> "$(APP_DIR)/Info.plist"
	@printf '<dict>\n' >> "$(APP_DIR)/Info.plist"
	@printf '    <key>CFBundleName</key>\n' >> "$(APP_DIR)/Info.plist"
	@printf '    <string>LSP Client</string>\n' >> "$(APP_DIR)/Info.plist"
	@printf '    <key>CFBundleDisplayName</key>\n' >> "$(APP_DIR)/Info.plist"
	@printf '    <string>LSP Client</string>\n' >> "$(APP_DIR)/Info.plist"
	@printf '    <key>CFBundleIdentifier</key>\n' >> "$(APP_DIR)/Info.plist"
	@printf '    <string>com.tectiv3.lsp-client</string>\n' >> "$(APP_DIR)/Info.plist"
	@printf '    <key>CFBundleVersion</key>\n' >> "$(APP_DIR)/Info.plist"
	@printf '    <string>1.0</string>\n' >> "$(APP_DIR)/Info.plist"
	@printf '    <key>CFBundlePackageType</key>\n' >> "$(APP_DIR)/Info.plist"
	@printf '    <string>APPL</string>\n' >> "$(APP_DIR)/Info.plist"
	@printf '    <key>CFBundleSignature</key>\n' >> "$(APP_DIR)/Info.plist"
	@printf '    <string>????</string>\n' >> "$(APP_DIR)/Info.plist"
	@printf '    <key>CFBundleExecutable</key>\n' >> "$(APP_DIR)/Info.plist"
	@printf '    <string>$(BINARY_NAME)</string>\n' >> "$(APP_DIR)/Info.plist"
	@printf '    <key>CFBundleIconFile</key>\n' >> "$(APP_DIR)/Info.plist"
	@printf '    <string>AppIcon</string>\n' >> "$(APP_DIR)/Info.plist"
	@printf '    <key>LSUIElement</key>\n' >> "$(APP_DIR)/Info.plist"
	@printf '    <true/>\n' >> "$(APP_DIR)/Info.plist"
	@printf '    <key>NSHighResolutionCapable</key>\n' >> "$(APP_DIR)/Info.plist"
	@printf '    <true/>\n' >> "$(APP_DIR)/Info.plist"
	@printf '</dict>\n' >> "$(APP_DIR)/Info.plist"
	@printf '</plist>\n' >> "$(APP_DIR)/Info.plist"
	@echo "App bundle created: $(APP_NAME)"

install: app
	@echo "Installing to /Applications..."
	@rm -rf "/Applications/$(APP_NAME)"
	@cp -R "$(APP_NAME)" /Applications/
	@echo "Installed to /Applications/$(APP_NAME)"

run: build
	@./$(BINARY_NAME)

icon:
	@echo "Generating app icon..."
	@./create-icon.sh

clean:
	@echo "Cleaning build artifacts..."
	@rm -f $(BINARY_NAME)
	@rm -rf "$(APP_NAME)"
	@rm -f AppIcon.icns
	@rm -rf icon.iconset
	@echo "Clean complete"

help:
	@echo "Available targets:"
	@echo "  make build    - Build the binary"
	@echo "  make app      - Create macOS app bundle"
	@echo "  make icon     - Generate app icon"
	@echo "  make install  - Install app to /Applications"
	@echo "  make run      - Build and run from terminal"
	@echo "  make clean    - Remove build artifacts"
	@echo "  make help     - Show this help message"
