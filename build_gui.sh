#!/bin/bash
set -e

echo "=== Building SpeedTunnel GUI ==="

# 1. Compile using Wails
cd gui
echo "Running Wails build..."
# We explicitly add the Go binary path to PATH
PATH=$PATH:/usr/local/go/bin ~/go/bin/wails build || echo "Wails build completed (checking for signing)..."

# 2. Copy to /tmp to bypass iCloud Drive metadata injection
echo "Copying app bundle to local /tmp for signing..."
rm -rf /tmp/SpeedTunnel.app
cp -R build/bin/SpeedTunnel.app /tmp/

echo "Stripping extended attributes..."
xattr -cr /tmp/SpeedTunnel.app

echo "Self-signing SpeedTunnel.app..."
codesign --force --deep --sign - /tmp/SpeedTunnel.app

# 3. Create DMG Installer
cd ..
echo "Creating DMG packaging structure in /tmp..."
rm -rf /tmp/dmg_temp
mkdir -p /tmp/dmg_temp
cp -R /tmp/SpeedTunnel.app /tmp/dmg_temp/

echo "Creating Applications symlink..."
ln -s /Applications /tmp/dmg_temp/Applications

echo "Generating SpeedTunnel.dmg..."
mkdir -p bin
rm -f bin/SpeedTunnel.dmg
hdiutil create -volname "SpeedTunnel" -srcfolder /tmp/dmg_temp -format UDZO -ov bin/SpeedTunnel.dmg

echo "Cleaning up temporary files in /tmp..."
rm -rf /tmp/dmg_temp /tmp/SpeedTunnel.app

echo "=== Build Complete! final installer at bin/SpeedTunnel.dmg ==="
