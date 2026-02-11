; SoHoLINK Windows Installer
; NSIS Script for creating installer executable

!include "MUI2.nsh"

; General Configuration
Name "SoHoLINK"
OutFile "..\..\dist\SoHoLINK-Setup.exe"
InstallDir "$PROGRAMFILES\SoHoLINK"
InstallDirRegKey HKLM "Software\SoHoLINK" "Install_Dir"
RequestExecutionLevel admin

; Version Information
VIProductVersion "1.0.0.0"
VIAddVersionKey "ProductName" "SoHoLINK"
VIAddVersionKey "CompanyName" "Network Theory Applied Research Institute"
VIAddVersionKey "FileDescription" "SoHoLINK Installer"
VIAddVersionKey "FileVersion" "1.0.0.0"
VIAddVersionKey "ProductVersion" "1.0.0.0"
VIAddVersionKey "LegalCopyright" "© 2023 NTARI"

; Interface Settings
!define MUI_ABORTWARNING
!define MUI_ICON "logo.ico"
!define MUI_UNICON "logo.ico"
!define MUI_WELCOMEFINISHPAGE_BITMAP "wizard-banner.bmp"
!define MUI_HEADERIMAGE
!define MUI_HEADERIMAGE_BITMAP "header.bmp"
!define MUI_HEADERIMAGE_RIGHT

; Welcome page with logo
!define MUI_WELCOMEPAGE_TITLE "Welcome to SoHoLINK Setup"
!define MUI_WELCOMEPAGE_TEXT "Transform your spare compute into income by joining the federated cloud marketplace.$\r$\n$\r$\nThis wizard will guide you through the installation of SoHoLINK.$\r$\n$\r$\nClick Next to continue."

; Pages
!insertmacro MUI_PAGE_WELCOME
!insertmacro MUI_PAGE_LICENSE "..\..\LICENSE"
!insertmacro MUI_PAGE_DIRECTORY
!insertmacro MUI_PAGE_INSTFILES

; Custom finish page to launch wizard
!define MUI_FINISHPAGE_TITLE "Installation Complete"
!define MUI_FINISHPAGE_TEXT "SoHoLINK has been installed successfully.$\r$\n$\r$\nClick Finish to launch the configuration wizard."
!define MUI_FINISHPAGE_RUN "$INSTDIR\soholink-wizard.exe"
!define MUI_FINISHPAGE_RUN_TEXT "Launch Configuration Wizard"
!define MUI_FINISHPAGE_RUN_CHECKED
!insertmacro MUI_PAGE_FINISH

; Uninstaller pages
!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES

; Languages
!insertmacro MUI_LANGUAGE "English"

; Installer Sections
Section "SoHoLINK Core" SecCore
    SectionIn RO

    ; Set output path
    SetOutPath $INSTDIR

    ; Install main executable
    File "..\..\build\soholink.exe"
    File "..\..\build\soholink-wizard.exe"

    ; Install documentation
    SetOutPath $INSTDIR\docs
    File /r "..\..\docs\*.*"

    ; Create start menu shortcuts
    CreateDirectory "$SMPROGRAMS\SoHoLINK"
    CreateShortcut "$SMPROGRAMS\SoHoLINK\SoHoLINK Wizard.lnk" "$INSTDIR\soholink-wizard.exe" "" "$INSTDIR\soholink-wizard.exe" 0
    CreateShortcut "$SMPROGRAMS\SoHoLINK\SoHoLINK.lnk" "$INSTDIR\soholink.exe" "" "$INSTDIR\soholink.exe" 0
    CreateShortcut "$SMPROGRAMS\SoHoLINK\Uninstall.lnk" "$INSTDIR\uninstall.exe"

    ; Create desktop shortcut with logo
    CreateShortcut "$DESKTOP\SoHoLINK Setup.lnk" "$INSTDIR\soholink-wizard.exe" "" "$INSTDIR\soholink-wizard.exe" 0

    ; Write registry keys
    WriteRegStr HKLM "Software\SoHoLINK" "Install_Dir" "$INSTDIR"
    WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\SoHoLINK" "DisplayName" "SoHoLINK"
    WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\SoHoLINK" "UninstallString" '"$INSTDIR\uninstall.exe"'
    WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\SoHoLINK" "DisplayIcon" "$INSTDIR\soholink.exe"
    WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\SoHoLINK" "Publisher" "Network Theory Applied Research Institute"
    WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\SoHoLINK" "DisplayVersion" "1.0.0"
    WriteRegDWORD HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\SoHoLINK" "NoModify" 1
    WriteRegDWORD HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\SoHoLINK" "NoRepair" 1

    ; Create uninstaller
    WriteUninstaller "$INSTDIR\uninstall.exe"
SectionEnd

; Uninstaller Section
Section "Uninstall"
    ; Remove files
    Delete "$INSTDIR\soholink.exe"
    Delete "$INSTDIR\soholink-wizard.exe"
    Delete "$INSTDIR\uninstall.exe"
    RMDir /r "$INSTDIR\docs"

    ; Remove shortcuts
    Delete "$SMPROGRAMS\SoHoLINK\*.*"
    RMDir "$SMPROGRAMS\SoHoLINK"
    Delete "$DESKTOP\SoHoLINK Setup.lnk"

    ; Remove registry keys
    DeleteRegKey HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\SoHoLINK"
    DeleteRegKey HKLM "Software\SoHoLINK"

    ; Remove installation directory
    RMDir "$INSTDIR"
SectionEnd
