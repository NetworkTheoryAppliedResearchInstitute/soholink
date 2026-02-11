#!/bin/bash
set -e

# Build AppImage for Linux distribution
# This creates a self-contained executable that runs on any Linux distro

VERSION=${VERSION:-"0.0.0-dev"}
ARCH="x86_64"
APPDIR="FedAAA.AppDir"

echo "Building AppImage for version $VERSION..."

# Create AppDir structure
mkdir -p "$APPDIR/usr/bin"
mkdir -p "$APPDIR/usr/share/applications"
mkdir -p "$APPDIR/usr/share/icons/hicolor/256x256/apps"
mkdir -p "$APPDIR/usr/share/metainfo"

# Copy binary
cp dist/fedaaa-linux-amd64 "$APPDIR/usr/bin/fedaaa"
chmod +x "$APPDIR/usr/bin/fedaaa"

# Create desktop file
cat > "$APPDIR/usr/share/applications/fedaaa.desktop" << EOF
[Desktop Entry]
Type=Application
Name=FedAAA
Comment=Federated Resource Sharing Platform
Exec=fedaaa
Icon=fedaaa
Categories=Network;System;
Terminal=false
EOF

# Create AppRun script
cat > "$APPDIR/AppRun" << 'EOF'
#!/bin/bash
SELF=$(readlink -f "$0")
HERE=${SELF%/*}
export PATH="${HERE}/usr/bin:${PATH}"
export LD_LIBRARY_PATH="${HERE}/usr/lib:${LD_LIBRARY_PATH}"
exec "${HERE}/usr/bin/fedaaa" "$@"
EOF
chmod +x "$APPDIR/AppRun"

# Create icon (placeholder - should be replaced with actual icon)
cat > "$APPDIR/fedaaa.png" << EOF
placeholder_icon
EOF

# Create AppStream metadata
cat > "$APPDIR/usr/share/metainfo/fedaaa.appdata.xml" << EOF
<?xml version="1.0" encoding="UTF-8"?>
<component type="desktop-application">
  <id>com.soholink.fedaaa</id>
  <metadata_license>CC0-1.0</metadata_license>
  <name>FedAAA</name>
  <summary>Federated Authentication, Authorization, and Accounting</summary>
  <description>
    <p>
      FedAAA is a decentralized platform for resource sharing and workload orchestration.
      It enables federated authentication, authorization, and accounting across distributed systems.
    </p>
  </description>
  <categories>
    <category>Network</category>
    <category>System</category>
  </categories>
  <url type="homepage">https://github.com/NetworkTheoryAppliedResearchInstitute/soholink</url>
  <releases>
    <release version="$VERSION" date="$(date -u +%Y-%m-%d)"/>
  </releases>
</component>
EOF

# Download appimagetool if not present
if [ ! -f "appimagetool-${ARCH}.AppImage" ]; then
    echo "Downloading appimagetool..."
    wget -q "https://github.com/AppImage/AppImageKit/releases/download/continuous/appimagetool-${ARCH}.AppImage"
    chmod +x "appimagetool-${ARCH}.AppImage"
fi

# Build AppImage
ARCH=$ARCH "./appimagetool-${ARCH}.AppImage" "$APPDIR" "dist/FedAAA-${VERSION}-${ARCH}.AppImage"

echo "AppImage created: dist/FedAAA-${VERSION}-${ARCH}.AppImage"
