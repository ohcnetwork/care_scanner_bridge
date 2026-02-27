#define MyAppName "Care Scanner Bridge"
#define MyAppVersion "1.0.0"
#define MyAppPublisher "OHC Network"
#define MyAppURL "https://github.com/ohcnetwork/care_scanner_bridge"
#define MyAppExeName "care-scanner-bridge-windows-amd64.exe"

[Setup]
AppId={{8A2B3C4D-5E6F-7890-ABCD-EF1234567890}
AppName={#MyAppName}
AppVersion={#MyAppVersion}
AppPublisher={#MyAppPublisher}
AppPublisherURL={#MyAppURL}
AppSupportURL={#MyAppURL}
AppUpdatesURL={#MyAppURL}
DefaultDirName={autopf}\{#MyAppName}
DefaultGroupName={#MyAppName}
AllowNoIcons=yes
LicenseFile=..\..\LICENSE
OutputDir=..\..\Output
OutputBaseFilename=care-scanner-bridge-setup
Compression=lzma
SolidCompression=yes
WizardStyle=modern

[Languages]
Name: "english"; MessagesFile: "compiler:Default.isl"

[Tasks]
Name: "desktopicon"; Description: "{cm:CreateDesktopIcon}"; GroupDescription: "{cm:AdditionalIcons}"; Flags: unchecked
Name: "startupicon"; Description: "Start automatically with Windows"; GroupDescription: "Startup:"; Flags: unchecked

[Files]
Source: "..\..\dist\{#MyAppExeName}"; DestDir: "{app}"; DestName: "care-scanner-bridge.exe"; Flags: ignoreversion

[Icons]
Name: "{group}\{#MyAppName}"; Filename: "{app}\care-scanner-bridge.exe"
Name: "{autodesktop}\{#MyAppName}"; Filename: "{app}\care-scanner-bridge.exe"; Tasks: desktopicon

[Registry]
Root: HKCU; Subkey: "Software\Microsoft\Windows\CurrentVersion\Run"; ValueType: string; ValueName: "CareScannerBridge"; ValueData: """{app}\care-scanner-bridge.exe"""; Flags: uninsdeletevalue; Tasks: startupicon

[Run]
Filename: "{app}\care-scanner-bridge.exe"; Description: "{cm:LaunchProgram,{#StringChange(MyAppName, '&', '&&')}}"; Flags: nowait postinstall skipifsilent
