#!/bin/bash

# Create a simple app icon using ImageMagick or sips
# This creates a 512x512 icon with "{}" text

ICON_DIR="icon.iconset"
OUTPUT="AppIcon.icns"

# Create iconset directory
mkdir -p "$ICON_DIR"

# Create a simple 512x512 PNG with {} text on transparent background
cat > temp_icon.svg <<'EOF'
<?xml version="1.0" encoding="UTF-8"?>
<svg width="512" height="512" xmlns="http://www.w3.org/2000/svg">
  <text x="256" y="380" font-family="SF Pro Display, Helvetica, Arial" font-size="320" font-weight="300" fill="#000000" text-anchor="middle">{}</text>
</svg>
EOF

# Check if we have necessary tools
if command -v qlmanage &> /dev/null; then
    # Convert SVG to PNG using qlmanage (built into macOS)
    qlmanage -t -s 512 -o . temp_icon.svg 2>/dev/null
    mv temp_icon.svg.png "${ICON_DIR}/icon_512x512.png" 2>/dev/null
fi

# If qlmanage didn't work, try sips with a basic approach
if [ ! -f "${ICON_DIR}/icon_512x512.png" ]; then
    echo "Creating basic icon using sips..."
    # Create a solid color base
    sips -z 512 512 -s format png /System/Library/CoreServices/CoreTypes.bundle/Contents/Resources/GenericApplicationIcon.icns --out "${ICON_DIR}/icon_512x512.png" 2>/dev/null
fi

# Generate all required icon sizes
sips -z 16 16     "${ICON_DIR}/icon_512x512.png" --out "${ICON_DIR}/icon_16x16.png" >/dev/null 2>&1
sips -z 32 32     "${ICON_DIR}/icon_512x512.png" --out "${ICON_DIR}/icon_16x16@2x.png" >/dev/null 2>&1
sips -z 32 32     "${ICON_DIR}/icon_512x512.png" --out "${ICON_DIR}/icon_32x32.png" >/dev/null 2>&1
sips -z 64 64     "${ICON_DIR}/icon_512x512.png" --out "${ICON_DIR}/icon_32x32@2x.png" >/dev/null 2>&1
sips -z 128 128   "${ICON_DIR}/icon_512x512.png" --out "${ICON_DIR}/icon_128x128.png" >/dev/null 2>&1
sips -z 256 256   "${ICON_DIR}/icon_512x512.png" --out "${ICON_DIR}/icon_128x128@2x.png" >/dev/null 2>&1
sips -z 256 256   "${ICON_DIR}/icon_512x512.png" --out "${ICON_DIR}/icon_256x256.png" >/dev/null 2>&1
sips -z 512 512   "${ICON_DIR}/icon_512x512.png" --out "${ICON_DIR}/icon_256x256@2x.png" >/dev/null 2>&1
sips -z 512 512   "${ICON_DIR}/icon_512x512.png" --out "${ICON_DIR}/icon_512x512@2x.png" >/dev/null 2>&1

# Convert to icns
iconutil -c icns "$ICON_DIR" -o "$OUTPUT"

# Cleanup
rm -rf "$ICON_DIR" temp_icon.svg

if [ -f "$OUTPUT" ]; then
    echo "Icon created: $OUTPUT"
else
    echo "Failed to create icon"
    exit 1
fi
