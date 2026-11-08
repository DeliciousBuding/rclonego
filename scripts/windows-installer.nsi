; RcloneGo's per-user NSIS installer. Build values come from windows_installer.py.
; Unlike the framework's generic installer, this never closes or kills a process.
Unicode true
ManifestDPIAware true
SetCompressor /SOLID lzma
Name "@@NAME@@"
OutFile "@@OUTPUT@@"
InstallDir "$LOCALAPPDATA\Programs\@@FOLDER@@"
InstallDirRegKey HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\@@ID@@" "InstallLocation"
RequestExecutionLevel user
BrandingText " "
Icon "@@ICON@@"
UninstallIcon "@@ICON@@"
@@SIGNING@@

!include "MUI2.nsh"
!define MUI_ICON "@@ICON@@"
!define MUI_UNICON "@@ICON@@"
!define MUI_FINISHPAGE_RUN "$INSTDIR\@@EXE@@"
!define MUI_FINISHPAGE_SHOWREADME
!define MUI_FINISHPAGE_SHOWREADME_NOTCHECKED
!define MUI_FINISHPAGE_SHOWREADME_TEXT "$(APP_DESKTOP_SHORTCUT)"
!define MUI_FINISHPAGE_SHOWREADME_FUNCTION CreateDesktopShortcut
!insertmacro MUI_PAGE_DIRECTORY
!insertmacro MUI_PAGE_INSTFILES
!insertmacro MUI_PAGE_FINISH
!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES
!insertmacro MUI_LANGUAGE "English"
!insertmacro MUI_LANGUAGE "SimpChinese"

LangString APP_DESKTOP_SHORTCUT ${LANG_ENGLISH} "Create a desktop shortcut"
LangString APP_DESKTOP_SHORTCUT ${LANG_SIMPCHINESE} "创建桌面快捷方式"
LangString APP_BUSY ${LANG_ENGLISH} "RcloneGo is still running. Choose Quit in RcloneGo, then run this installer again. Transfers and mounts will not be interrupted."
LangString APP_BUSY ${LANG_SIMPCHINESE} "RcloneGo 仍在运行。请在程序中选择“退出”，然后重新运行安装程序。安装程序不会中断传输或挂载。"
LangString APP_INSPECTION_ERROR ${LANG_ENGLISH} "The installer could not verify that this installation is idle. No application files were changed. Quit RcloneGo and try again."
LangString APP_INSPECTION_ERROR ${LANG_SIMPCHINESE} "安装程序无法确认此安装目录已空闲，尚未修改程序文件。请退出 RcloneGo 后重试。"
LangString APP_INSTALL_ERROR ${LANG_ENGLISH} "Some application files could not be installed. Check that this folder is writable, then try again."
LangString APP_INSTALL_ERROR ${LANG_SIMPCHINESE} "部分程序文件无法安装。请确认此目录可写，然后重试。"
LangString APP_UNINSTALL_ERROR ${LANG_ENGLISH} "Some application files could not be removed. Close programs using this folder, then try again."
LangString APP_UNINSTALL_ERROR ${LANG_SIMPCHINESE} "部分程序文件无法删除。请关闭正在使用此目录的程序，然后重试。"

; One read-only check runs immediately before each install or uninstall.
; Resolve the selected directory, include background/tray instances and fail
; closed when process inspection is unavailable. No timeout or force fallback.
!macro AppFunctions Prefix
Function ${Prefix}PrepareCheck
  InitPluginsDir
  FileOpen $2 "$PLUGINSDIR\installation-check.ps1" w
@@GUARD_LINES@@
  FileClose $2
  StrCpy $1 "$WINDIR\Sysnative\WindowsPowerShell\v1.0\powershell.exe"
  IfFileExists $1 +2
  StrCpy $1 "$SYSDIR\WindowsPowerShell\v1.0\powershell.exe"
  System::Call 'Kernel32::SetEnvironmentVariable(t "RCLONEGO_INSTALLER_SCRIPT", t "$PLUGINSDIR\installation-check.ps1")'
  System::Call 'Kernel32::SetEnvironmentVariable(t "RCLONEGO_INSTALLER_DIR", t "$INSTDIR")'
  System::Call 'Kernel32::SetEnvironmentVariable(t "RCLONEGO_INSTALLER_ID", t "@@ID@@")'
  System::Call 'Kernel32::SetEnvironmentVariable(t "RCLONEGO_INSTALLER_EXE", t "@@EXE@@")'
FunctionEnd

Function ${Prefix}CheckResult
  StrCmp $2 "0" done
  IfSilent failed
  StrCmp $2 "2" busy
  MessageBox MB_OK|MB_ICONSTOP "$(APP_INSPECTION_ERROR)"
  Goto failed
  busy:
  MessageBox MB_OK|MB_ICONEXCLAMATION "$(APP_BUSY)"
  failed:
  SetErrorLevel 2
  Quit
  done:
FunctionEnd

Function ${Prefix}RequireIdle
  Call ${Prefix}PrepareCheck
  IfFileExists $1 0 inspection_failed
  System::Call 'Kernel32::SetEnvironmentVariable(t "RCLONEGO_INSTALLER_ACTION", t "check")'
  nsExec::Exec '"$1" -NoProfile -NonInteractive -ExecutionPolicy Bypass -Command "& ([scriptblock]::Create([IO.File]::ReadAllText($$env:RCLONEGO_INSTALLER_SCRIPT)))"'
  Pop $2
  Call ${Prefix}CheckResult
  Return
  inspection_failed:
  StrCpy $2 "3"
  Call ${Prefix}CheckResult
FunctionEnd
!macroend
!insertmacro AppFunctions ""
!insertmacro AppFunctions "un."

Section
  Call RequireIdle
  ClearErrors
  SetOutPath "$INSTDIR"
@@INSTALL_FILES@@
  IfErrors install_failed
  SetOutPath "$INSTDIR"
  WriteUninstaller "$INSTDIR\Uninstall.exe"
  IfErrors install_failed
  CreateShortCut "$SMPROGRAMS\@@NAME@@.lnk" "$INSTDIR\@@EXE@@"
  WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\@@ID@@" "DisplayName" "@@NAME@@"
  WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\@@ID@@" "DisplayVersion" "@@VERSION@@"
  WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\@@ID@@" "Publisher" "@@PUBLISHER@@"
  WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\@@ID@@" "DisplayIcon" "$INSTDIR\@@EXE@@"
  WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\@@ID@@" "InstallLocation" "$INSTDIR"
  WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\@@ID@@" "UninstallString" '"$INSTDIR\Uninstall.exe"'
  WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\@@ID@@" "QuietUninstallString" '"$INSTDIR\Uninstall.exe" /S'
  WriteRegDWORD HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\@@ID@@" "NoModify" 1
  WriteRegDWORD HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\@@ID@@" "NoRepair" 1
  Goto install_done
  install_failed:
  IfSilent +2
  MessageBox MB_OK|MB_ICONSTOP "$(APP_INSTALL_ERROR)"
  SetErrorLevel 2
  Quit
  install_done:
SectionEnd

Function CreateDesktopShortcut
  SetOutPath "$INSTDIR"
  CreateShortCut "$DESKTOP\@@NAME@@.lnk" "$INSTDIR\@@EXE@@"
FunctionEnd

; Explorer may briefly hold a newly created shortcut open.
Function un.DeleteShortcut
  Exch $0
  Push $1
  StrCpy $1 50
  retry:
  ClearErrors
  Delete "$0"
  IfErrors 0 done
  IntOp $1 $1 - 1
  IntCmp $1 0 done
  Sleep 100
  Goto retry
  done:
  Pop $1
  Pop $0
FunctionEnd

Section "Uninstall"
  Call un.RequireIdle
  ; The same script removes only startup values owned by this directory.
  System::Call 'Kernel32::SetEnvironmentVariable(t "RCLONEGO_INSTALLER_ACTION", t "remove-startup")'
  nsExec::Exec '"$1" -NoProfile -NonInteractive -ExecutionPolicy Bypass -Command "& ([scriptblock]::Create([IO.File]::ReadAllText($$env:RCLONEGO_INSTALLER_SCRIPT)))"'
  Pop $2
  Call un.CheckResult
  ClearErrors
@@UNINSTALL_FILES@@
  IfErrors uninstall_failed
  ReadRegStr $0 HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\@@ID@@" "InstallLocation"
  StrCmp $0 "$INSTDIR" 0 remove_directories
  Push "$SMPROGRAMS\@@NAME@@.lnk"
  Call un.DeleteShortcut
  Push "$DESKTOP\@@NAME@@.lnk"
  Call un.DeleteShortcut
  DeleteRegKey HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\@@ID@@"
  remove_directories:
  Delete "$INSTDIR\Uninstall.exe"
@@UNINSTALL_DIRECTORIES@@
  ; Nonrecursive: profile data and files not installed by RcloneGo stay.
  RMDir "$INSTDIR"
  Goto uninstall_done
  uninstall_failed:
  IfSilent +2
  MessageBox MB_OK|MB_ICONSTOP "$(APP_UNINSTALL_ERROR)"
  SetErrorLevel 2
  Quit
  uninstall_done:
SectionEnd
