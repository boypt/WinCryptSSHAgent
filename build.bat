@echo off
setlocal

@rem Build all by default
if [%1]==[] (
	go generate
	call :build amd64 WinCryptSSHAgent.exe
	call :build arm64 WinCryptSSHAgent-arm64.exe
) else (
	go generate
	call :build %1 WinCryptSSHAgent-%1.exe
)
goto :eof


:build
set arch=%1
set output=%2
echo Build %arch% to %output%

set GOARCH=%arch%
go build -ldflags "-w -s -H=windowsgui" -trimpath -o %output%

goto :eof
