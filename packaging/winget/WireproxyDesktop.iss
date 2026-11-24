#ifndef AppVersion
  #error Pass AppVersion to ISCC
#endif
#ifndef AppArch
  #error Pass AppArch to ISCC
#endif
#if AppArch == "amd64"
  #define InstallArch "x64compatible"
#elif AppArch == "arm64"
  #define InstallArch "arm64"
#else
  #error Unsupported architecture
#endif

[Setup]
AppId=WeslyG.WireproxyDesktop
AppName=Wireproxy Desktop
AppVersion={#AppVersion}
AppPublisher=WeslyG
AppPublisherURL=https://github.com/WeslyG/wireproxy-desktop
AppSupportURL=https://github.com/WeslyG/wireproxy-desktop/issues
DefaultDirName={localappdata}\Programs\Wireproxy Desktop
DisableProgramGroupPage=yes
PrivilegesRequired=lowest
MinVersion=10.0.19041
ArchitecturesAllowed={#InstallArch}
ArchitecturesInstallIn64BitMode={#InstallArch}
UninstallDisplayIcon={app}\wireproxy-desktop.exe
SetupIconFile=..\..\assets\favicon.ico
CloseApplications=yes
RestartApplications=no

[Files]
Source: "{#AppBinary}"; DestDir: "{app}"; DestName: "wireproxy-desktop.exe"; Flags: ignoreversion
Source: "winget-managed"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\..\LICENSE.wireproxy-awg"; DestDir: "{app}"; Flags: ignoreversion

[Icons]
Name: "{userprograms}\Wireproxy Desktop"; Filename: "{app}\wireproxy-desktop.exe"; WorkingDir: "{app}"
