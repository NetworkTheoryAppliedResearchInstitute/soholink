#!/bin/bash
set -e

# Build Windows MSI installer using WiX Toolset

VERSION=${VERSION:-"0.0.0"}
VERSION_NUMERIC=$(echo "$VERSION" | sed 's/-.*//') # Remove pre-release suffix for MSI version

echo "Building Windows MSI for version $VERSION..."

# Create staging directory
STAGING="staging"
mkdir -p "$STAGING"
mkdir -p "$STAGING/bin"
mkdir -p "$STAGING/config"

# Copy files
cp dist/fedaaa-windows-amd64.exe "$STAGING/bin/fedaaa.exe"
cp packaging/windows/config.yaml.example "$STAGING/config/config.yaml"

# Generate WiX source with proper GUIDs
cat > "Product.wxs" << EOF
<?xml version="1.0" encoding="UTF-8"?>
<Wix xmlns="http://schemas.microsoft.com/wix/2006/wi">
  <Product Id="*"
           Name="FedAAA"
           Language="1033"
           Version="$VERSION_NUMERIC"
           Manufacturer="Network Theory Applied Research Institute"
           UpgradeCode="12345678-1234-1234-1234-123456789012">

    <Package InstallerVersion="200"
             Compressed="yes"
             InstallScope="perMachine"
             Description="FedAAA - Federated Resource Sharing Platform"
             Comments="Decentralized platform for resource sharing and workload orchestration"/>

    <MajorUpgrade DowngradeErrorMessage="A newer version of [ProductName] is already installed." />
    <MediaTemplate EmbedCab="yes" />

    <Feature Id="ProductFeature" Title="FedAAA" Level="1">
      <ComponentGroupRef Id="ProductComponents" />
      <ComponentRef Id="ApplicationShortcut" />
      <ComponentRef Id="EnvironmentVariables" />
    </Feature>

    <Directory Id="TARGETDIR" Name="SourceDir">
      <Directory Id="ProgramFiles64Folder">
        <Directory Id="INSTALLFOLDER" Name="FedAAA">
          <Directory Id="BinFolder" Name="bin"/>
          <Directory Id="ConfigFolder" Name="config"/>
          <Directory Id="DataFolder" Name="data"/>
          <Directory Id="LogsFolder" Name="logs"/>
        </Directory>
      </Directory>

      <Directory Id="ProgramMenuFolder">
        <Directory Id="ApplicationProgramsFolder" Name="FedAAA"/>
      </Directory>
    </Directory>

    <DirectoryRef Id="ApplicationProgramsFolder">
      <Component Id="ApplicationShortcut" Guid="ABCDEF01-1234-1234-1234-123456789012">
        <Shortcut Id="ApplicationStartMenuShortcut"
                  Name="FedAAA"
                  Description="FedAAA Resource Sharing Platform"
                  Target="[BinFolder]fedaaa.exe"
                  WorkingDirectory="INSTALLFOLDER"/>
        <RemoveFolder Id="CleanUpShortCut" Directory="ApplicationProgramsFolder" On="uninstall"/>
        <RegistryValue Root="HKCU" Key="Software\FedAAA" Name="installed" Type="integer" Value="1" KeyPath="yes"/>
      </Component>
    </DirectoryRef>

    <DirectoryRef Id="TARGETDIR">
      <Component Id="EnvironmentVariables" Guid="FEDAAA01-1234-1234-1234-123456789012">
        <Environment Id="PATH" Name="PATH" Value="[BinFolder]" Permanent="no" Part="last" Action="set" System="yes" />
        <CreateFolder Directory="DataFolder"/>
        <CreateFolder Directory="LogsFolder"/>
      </Component>
    </DirectoryRef>

    <ComponentGroup Id="ProductComponents" Directory="BinFolder">
      <Component Id="fedaaa.exe" Guid="11111111-1234-1234-1234-123456789012">
        <File Id="fedaaa.exe" Source="$STAGING/bin/fedaaa.exe" KeyPath="yes">
          <Shortcut Id="DesktopShortcut"
                    Directory="DesktopFolder"
                    Name="FedAAA"
                    WorkingDirectory="INSTALLFOLDER"
                    Icon="FedAAAIcon.exe"
                    IconIndex="0"
                    Advertise="yes" />
        </File>
      </Component>
    </ComponentGroup>

    <ComponentGroup Id="ConfigComponents" Directory="ConfigFolder">
      <Component Id="config.yaml" Guid="22222222-1234-1234-1234-123456789012">
        <File Id="config.yaml" Source="$STAGING/config/config.yaml" KeyPath="yes"/>
      </Component>
    </ComponentGroup>

    <Icon Id="FedAAAIcon.exe" SourceFile="$STAGING/bin/fedaaa.exe"/>

    <Property Id="WIXUI_INSTALLDIR" Value="INSTALLFOLDER" />

    <UI>
      <UIRef Id="WixUI_InstallDir"/>
      <UIRef Id="WixUI_ErrorProgressText"/>
    </UI>

    <WixVariable Id="WixUILicenseRtf" Value="packaging/windows/license.rtf" />
    <WixVariable Id="WixUIDialogBmp" Value="packaging/windows/dialog.bmp" />
    <WixVariable Id="WixUIBannerBmp" Value="packaging/windows/banner.bmp" />

  </Product>
</Wix>
EOF

# Create placeholder license RTF
cat > "license.rtf" << 'EOF'
{\rtf1\ansi\deff0
{\fonttbl{\f0 Times New Roman;}}
\f0\fs24
FedAAA License Agreement\par
\par
License to be determined (see GAP 1: License Decision)\par
\par
Copyright (c) 2024 Network Theory Applied Research Institute\par
}
EOF

# Build MSI using WiX
wix build -arch x64 -out "dist/FedAAA-${VERSION}.msi" "Product.wxs"

# Clean up
rm -rf "$STAGING" "Product.wxs" "license.rtf"

echo "Windows MSI created: dist/FedAAA-${VERSION}.msi"
