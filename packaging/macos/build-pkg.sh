#!/bin/bash
set -e

# Build macOS PKG installer

VERSION=${VERSION:-"0.0.0"}
PKG_NAME="FedAAA-${VERSION}.pkg"
IDENTIFIER="com.soholink.fedaaa"
INSTALL_LOCATION="/usr/local"

echo "Building macOS PKG for version $VERSION..."

# Create package root structure
PKG_ROOT="pkg-root"
mkdir -p "$PKG_ROOT/bin"
mkdir -p "$PKG_ROOT/etc/fedaaa"
mkdir -p "$PKG_ROOT/var/lib/fedaaa"
mkdir -p "$PKG_ROOT/var/log/fedaaa"

# Copy universal binary
cp dist/fedaaa "$PKG_ROOT/bin/fedaaa"
chmod +x "$PKG_ROOT/bin/fedaaa"

# Copy config example
cp packaging/macos/config.yaml.example "$PKG_ROOT/etc/fedaaa/config.yaml"

# Create LaunchDaemon plist
mkdir -p "$PKG_ROOT/Library/LaunchDaemons"
cat > "$PKG_ROOT/Library/LaunchDaemons/$IDENTIFIER.plist" << EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>$IDENTIFIER</string>
    <key>ProgramArguments</key>
    <array>
        <string>$INSTALL_LOCATION/bin/fedaaa</string>
        <string>server</string>
        <string>--config</string>
        <string>$INSTALL_LOCATION/etc/fedaaa/config.yaml</string>
    </array>
    <key>RunAtLoad</key>
    <false/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>$INSTALL_LOCATION/var/log/fedaaa/stdout.log</string>
    <key>StandardErrorPath</key>
    <string>$INSTALL_LOCATION/var/log/fedaaa/stderr.log</string>
    <key>WorkingDirectory</key>
    <string>$INSTALL_LOCATION/var/lib/fedaaa</string>
</dict>
</plist>
EOF

# Create scripts directory
SCRIPTS_DIR="pkg-scripts"
mkdir -p "$SCRIPTS_DIR"

# Postinstall script
cat > "$SCRIPTS_DIR/postinstall" << 'EOF'
#!/bin/bash
set -e

# Set permissions
chown -R root:wheel /usr/local/bin/fedaaa
chmod 755 /usr/local/bin/fedaaa
chmod 644 /Library/LaunchDaemons/com.soholink.fedaaa.plist

# Create log directory if it doesn't exist
mkdir -p /usr/local/var/log/fedaaa
chmod 755 /usr/local/var/log/fedaaa

echo ""
echo "FedAAA has been installed successfully!"
echo ""
echo "Next steps:"
echo "1. Edit the configuration: /usr/local/etc/fedaaa/config.yaml"
echo "2. Load the service: sudo launchctl load /Library/LaunchDaemons/com.soholink.fedaaa.plist"
echo "3. Start the service: sudo launchctl start com.soholink.fedaaa"
echo "4. Check logs: tail -f /usr/local/var/log/fedaaa/*.log"
echo ""

exit 0
EOF
chmod +x "$SCRIPTS_DIR/postinstall"

# Preinstall script
cat > "$SCRIPTS_DIR/preinstall" << 'EOF'
#!/bin/bash
set -e

# Stop service if running
if launchctl list | grep -q "com.soholink.fedaaa"; then
    launchctl stop com.soholink.fedaaa || true
    launchctl unload /Library/LaunchDaemons/com.soholink.fedaaa.plist || true
fi

exit 0
EOF
chmod +x "$SCRIPTS_DIR/preinstall"

# Build component package
pkgbuild --root "$PKG_ROOT" \
         --identifier "$IDENTIFIER" \
         --version "$VERSION" \
         --install-location "$INSTALL_LOCATION" \
         --scripts "$SCRIPTS_DIR" \
         "FedAAA-component.pkg"

# Create distribution XML
cat > "Distribution.xml" << EOF
<?xml version="1.0" encoding="utf-8"?>
<installer-gui-script minSpecVersion="1">
    <title>FedAAA</title>
    <organization>com.soholink</organization>
    <domains enable_localSystem="true"/>
    <options customize="never" require-scripts="true" rootVolumeOnly="true"/>

    <welcome file="welcome.html"/>
    <license file="license.txt"/>
    <readme file="readme.html"/>

    <pkg-ref id="$IDENTIFIER">
        <bundle-version>
            <bundle id="$IDENTIFIER" CFBundleShortVersionString="$VERSION" path="$INSTALL_LOCATION/bin/fedaaa"/>
        </bundle-version>
    </pkg-ref>

    <choices-outline>
        <line choice="default">
            <line choice="$IDENTIFIER"/>
        </line>
    </choices-outline>

    <choice id="default"/>
    <choice id="$IDENTIFIER" visible="false">
        <pkg-ref id="$IDENTIFIER"/>
    </choice>

    <pkg-ref id="$IDENTIFIER" version="$VERSION" onConclusion="none">FedAAA-component.pkg</pkg-ref>
</installer-gui-script>
EOF

# Create welcome HTML
cat > "welcome.html" << EOF
<!DOCTYPE html>
<html>
<head>
    <meta charset="utf-8">
    <title>Welcome to FedAAA</title>
</head>
<body>
    <h1>Welcome to FedAAA $VERSION</h1>
    <p>This installer will install FedAAA on your system.</p>
    <p>FedAAA is a decentralized platform for resource sharing and workload orchestration.</p>
</body>
</html>
EOF

# Create readme HTML
cat > "readme.html" << EOF
<!DOCTYPE html>
<html>
<head>
    <meta charset="utf-8">
    <title>FedAAA README</title>
</head>
<body>
    <h1>FedAAA Installation</h1>
    <h2>Requirements</h2>
    <ul>
        <li>macOS 10.15 or later</li>
        <li>Docker Desktop (optional, for container support)</li>
    </ul>

    <h2>Installation Location</h2>
    <p>FedAAA will be installed to:</p>
    <ul>
        <li>Binary: /usr/local/bin/fedaaa</li>
        <li>Config: /usr/local/etc/fedaaa/</li>
        <li>Data: /usr/local/var/lib/fedaaa/</li>
        <li>Logs: /usr/local/var/log/fedaaa/</li>
    </ul>

    <h2>After Installation</h2>
    <ol>
        <li>Edit /usr/local/etc/fedaaa/config.yaml</li>
        <li>Load: sudo launchctl load /Library/LaunchDaemons/com.soholink.fedaaa.plist</li>
        <li>Start: sudo launchctl start com.soholink.fedaaa</li>
    </ol>
</body>
</html>
EOF

# Create placeholder license
cat > "license.txt" << EOF
License information for FedAAA
(To be determined - see GAP 1)
EOF

# Build product package
productbuild --distribution "Distribution.xml" \
             --resources "." \
             --package-path "." \
             "dist/$PKG_NAME"

# Clean up
rm -rf "$PKG_ROOT" "$SCRIPTS_DIR" "FedAAA-component.pkg" "Distribution.xml" "welcome.html" "readme.html" "license.txt"

echo "macOS PKG created: dist/$PKG_NAME"
