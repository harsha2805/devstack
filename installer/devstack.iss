; devstack installer: per user, no admin rights. Built by build-release.ps1:
;   ISCC /DAppVersion=0.1.0 /DBin=..\dist\release\bin installer\devstack.iss
; It installs devstack-app.exe (tray + window) and devstack.exe (CLI) into
; %LOCALAPPDATA%\Programs\devstack and adds that folder to the user PATH.
; The stack itself (sites, servers, databases) lives in the folder picked on
; first run and is never touched by Setup or the uninstaller.

#ifndef AppVersion
  #error Pass the version: ISCC /DAppVersion=x.y.z installer\devstack.iss
#endif
; Folder holding the exes to install, relative to this script.
#ifndef Bin
  #define Bin "..\dist"
#endif
; The app's settings folder (appconf.Dir). Only test builds change it, to match
; DEVSTACK_APPDATA, so uninstalling a test copy leaves the real one alone.
#ifndef AppData
  #define AppData "{localappdata}\devstack"
#endif

[Setup]
; AppId must never change: it's how Setup finds an existing install to upgrade
; in place, and the app reads "<AppId>_is1" to know it is an installed copy.
AppId=devstack
AppName=devstack
AppVersion={#AppVersion}
AppVerName=devstack {#AppVersion}
AppPublisher=HoloStack
AppPublisherURL=https://devstack.holostack.in
AppSupportURL=https://github.com/harsha2805/devstack
AppUpdatesURL=https://devstack.holostack.in
VersionInfoVersion={#AppVersion}
; Publisher details Windows shows in the file's Properties and next to its
; signature, so the installer isn't an anonymous download.
VersionInfoCompany=HoloStack
VersionInfoDescription=devstack Setup
VersionInfoProductName=devstack
; Exactly the version build.ps1 writes into the exes: SignPath checks that
; every file it signs carries the same product name and version.
VersionInfoTextVersion={#AppVersion}
VersionInfoProductTextVersion={#AppVersion}
VersionInfoCopyright=Copyright (c) 2026 Harsha (HoloStack). MIT license.
AppCopyright=Copyright (c) 2026 Harsha (HoloStack)
; With PrivilegesRequired=lowest, {autopf} is %LOCALAPPDATA%\Programs.
DefaultDirName={autopf}\devstack
DefaultGroupName=devstack
DisableProgramGroupPage=yes
UninstallDisplayIcon={app}\devstack-app.exe
UninstallDisplayName=devstack
OutputDir=..\dist\release
OutputBaseFilename=devstack_setup_{#AppVersion}
SetupIconFile=devstack.ico
; What devstack sends over the network, shown before installing (silent
; updates skip it). The same text is on devstack.holostack.in/privacy.
InfoBeforeFile=privacy.txt
Compression=lzma2/ultra64
SolidCompression=yes
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
PrivilegesRequired=lowest
WizardStyle=modern
MinVersion=10.0
; The user PATH changes; tell open programs (new terminals pick it up).
ChangesEnvironment=yes
; [Code] PrepareToInstall closes devstack first. Restart Manager is the backstop.
CloseApplications=yes
RestartApplications=no

[Files]
Source: "{#Bin}\devstack-app.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "{#Bin}\devstack.exe"; DestDir: "{app}"; Flags: ignoreversion

[Icons]
Name: "{group}\devstack"; Filename: "{app}\devstack-app.exe"
Name: "{autodesktop}\devstack"; Filename: "{app}\devstack-app.exe"; Tasks: desktopicon

[Tasks]
Name: "desktopicon"; Description: "Create a desktop shortcut"; GroupDescription: "Shortcuts:"; Flags: unchecked
; Same HKCU Run value as the app's "Open devstack when you sign in" setting
; (ui.AutostartName), so the two always agree. [Code] sets the checkbox from it.
Name: "startup"; Description: "Open devstack when you sign in (in the tray)"; GroupDescription: "Startup:"; Flags: unchecked

[Registry]
Root: HKCU; Subkey: "Software\Microsoft\Windows\CurrentVersion\Run"; ValueType: string; ValueName: "devstack"; ValueData: """{app}\devstack-app.exe"" --tray"; Check: StartupWanted
Root: HKCU; Subkey: "Software\Microsoft\Windows\CurrentVersion\Run"; ValueType: none; ValueName: "devstack"; Flags: deletevalue; Check: StartupUnwanted

[Run]
; Not skipifsilent: the in-app updater runs Setup with /SILENT and relies on
; this to reopen devstack once the upgrade finishes.
Filename: "{app}\devstack-app.exe"; Description: "Open devstack"; Flags: nowait postinstall

[UninstallDelete]
; App settings, logs and the window's browser cache. Not the stack folder.
Type: filesandordirs; Name: "{#AppData}"

[Code]
const
  RunKey = 'Software\Microsoft\Windows\CurrentVersion\Run';
  RunValue = 'devstack';
  EnvKey = 'Environment';

var
  StartupWasOn: Boolean;
  StartupTaskPrepared: Boolean;
  StackRoot: String;

function InitializeSetup: Boolean;
begin
  StartupWasOn := RegValueExists(HKCU, RunKey, RunValue);
  Result := True;
end;

{ The checkbox shows what Windows will actually do, so it matches the app's Settings. }
procedure CurPageChanged(CurPageID: Integer);
begin
  if (CurPageID = wpSelectTasks) and not StartupTaskPrepared then
  begin
    StartupTaskPrepared := True;
    if StartupWasOn then
      WizardSelectTasks('startup')
    else
      WizardSelectTasks('!startup');
  end;
end;

{ Silent installs are the in-app updater: keep whatever the user chose. }
function StartupWanted: Boolean;
begin
  if WizardSilent then
    Result := StartupWasOn
  else
    Result := WizardIsTaskSelected('startup');
end;

function StartupUnwanted: Boolean;
begin
  Result := not StartupWanted;
end;

{ Closes a running devstack and stops its servers: the background service runs
  from devstack-app.exe or devstack.exe, which are about to be replaced or removed. }
procedure QuitDevstack(const Dir: String);
var
  Code: Integer;
begin
  if FileExists(Dir + '\devstack-app.exe') then
    Exec(Dir + '\devstack-app.exe', '--quit', Dir, SW_HIDE, ewWaitUntilTerminated, Code);
end;

function PrepareToInstall(var NeedsRestart: Boolean): String;
begin
  QuitDevstack(ExpandConstant('{app}'));
  Result := '';
end;

{ ---- user PATH ---- }

function PathHas(const Path, Dir: String): Boolean;
begin
  Result := Pos(';' + Uppercase(Dir) + ';', ';' + Uppercase(Path) + ';') > 0;
end;

procedure AddToPath(const Dir: String);
var
  Path: String;
begin
  if not RegQueryStringValue(HKCU, EnvKey, 'Path', Path) then
    Path := '';
  if PathHas(Path, Dir) then
    Exit;
  if (Path <> '') and (Path[Length(Path)] <> ';') then
    Path := Path + ';';
  RegWriteExpandStringValue(HKCU, EnvKey, 'Path', Path + Dir);
end;

procedure RemoveFromPath(const Dir: String);
var
  Path: String;
  P: Integer;
begin
  if not RegQueryStringValue(HKCU, EnvKey, 'Path', Path) then
    Exit;
  Path := ';' + Path + ';';
  P := Pos(';' + Uppercase(Dir) + ';', Uppercase(Path));
  if P = 0 then
    Exit;
  Delete(Path, P, Length(Dir) + 1);
  Path := Copy(Path, 2, Length(Path) - 2);
  RegWriteExpandStringValue(HKCU, EnvKey, 'Path', Path);
end;

procedure CurStepChanged(CurStep: TSetupStep);
begin
  if CurStep = ssPostInstall then
    AddToPath(ExpandConstant('{app}'));
end;

{ ---- uninstall ---- }

{ The stack folder from the app's app.json ("root": "C:\\devstack"),
  read before that folder is removed, so the goodbye message can point at it. }
function ReadStackRoot: String;
var
  Json: AnsiString;
  P: Integer;
  S: String;
begin
  Result := '';
  if not LoadStringFromFile(ExpandConstant('{#AppData}\app.json'), Json) then
    Exit;
  S := String(Json);
  P := Pos('"root": "', S);
  if P = 0 then
    Exit;
  S := Copy(S, P + 9, Length(S));
  P := Pos('"', S);
  if P = 0 then
    Exit;
  Result := Copy(S, 1, P - 1);
  StringChangeEx(Result, '\\', '\', True);
end;

procedure CurUninstallStepChanged(CurUninstallStep: TUninstallStep);
var
  Command: String;
  Code: Integer;
begin
  if CurUninstallStep = usUninstall then
  begin
    StackRoot := ReadStackRoot;
    QuitDevstack(ExpandConstant('{app}'));
    { Don't leave devstack's certificate authority trusted once the app is gone.
      Windows asks to confirm; the CA files go with the settings folder below. }
    if DirExists(ExpandConstant('{#AppData}\ca')) and FileExists(ExpandConstant('{app}\devstack.exe')) then
      Exec(ExpandConstant('{app}\devstack.exe'), 'https untrust', ExpandConstant('{app}'), SW_HIDE, ewWaitUntilTerminated, Code);
    { Remove sign-in start only when it points at this installation. }
    if RegQueryStringValue(HKCU, RunKey, RunValue, Command) and
       (CompareText(Command, '"' + ExpandConstant('{app}\devstack-app.exe') + '" --tray') = 0) then
      RegDeleteValue(HKCU, RunKey, RunValue);
    RemoveFromPath(ExpandConstant('{app}'));
  end;
  if (CurUninstallStep = usPostUninstall) and (StackRoot <> '') and DirExists(StackRoot) and not UninstallSilent then
    MsgBox('devstack is removed. Your stack folder was kept, with your sites and databases:' + #13#10#13#10 +
      StackRoot + #13#10#13#10 + 'Delete it yourself if you no longer need it.', mbInformation, MB_OK);
end;
