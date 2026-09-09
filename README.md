# WinCrypt SSH Agent

> This repository is a maintained fork based on upstream [`buptczq/WinCryptSSHAgent`](https://github.com/buptczq/WinCryptSSHAgent) (base commit: `1e526e8`).

## Fork Enhancements

- **Self-service key import** — auto-load at startup plus tray import, no `ssh-add` needed.
- **Signing confirmation** — Manual/Auto mode per signing request, persisted across restarts.
- **Source-tagged notifications** — toasts show the requesting transport with categorized icons.
- **Windows ARM64 builds** — amd64 + arm64 binaries from `make`, versioned from Git tags.
- **Broader protocol support** — XShell Xagent compatibility, Hyper-V vsock, graceful shutdown.

## Introduction

Windows applications use several incompatible SSH agent interfaces. Native OpenSSH clients use a Windows named pipe, PuTTY-family applications use the Pageant protocol, Git for Windows, MSYS2 and Cygwin use a Cygwin-compatible socket, XShell uses its own Xagent protocol — and WSL clients arrive over Unix sockets and Hyper-V vsock.

WinCryptSSHAgent connects all of these client interfaces to your keys in one place, so a single agent serves every client. Keys come from the Windows Certificate Store — user certificates and smart cards such as Yubikey PIV work natively without installing any driver — or from an in-memory keyring (auto-loaded `~/.ssh` keys and tray imports, never written to disk). It runs as a notification-area application.

## Overview
![Overview](overview.svg)

## Feature

* One agent for fragmented Windows clients: named pipe, Pageant, Cygwin socket, XShell Xagent, WSL and Hyper-V vsock
* Work with smart cards natively without installing any driver in Windows (PIV only)
* Support for OpenSSH certificates (so you can use your smart card with an additional OpenSSH certificate)
* In-memory keyring backend: imported keys live only in process memory, never persisted
* Good compatibility

## Compatibility

There are many different, mutually incompatible SSH agent interfaces on Windows. This project implements the popular ones side by side:

* Windows OpenSSH named pipe
* Pageant SSH agent protocol
* Cygwin / MSYS2 socket
* WSL (Unix socket and Hyper-V vsock)
* XShell Xagent protocol

With all of these served by one running agent, this project is compatible with most SSH clients in Windows. For example:

* Git for Windows
* Windows Subsystem for Linux
* Windows OpenSSH
* PuTTY
* JetBrains
* SecureCRT
* XShell
* Cygwin
* MINGW
* ...

## Installing

### Manually Install

Stable versions can be obtained from the release page. 

Additionally, you may make a shortcut of this application to the startup folder.

## Usage

### Basic Usage

1. Start WinCryptSSHAgent
2. Right-click the icon on your taskbar
3. You can get necessary information by selecting your interesting item in the menu

Note: Some SSH clients using Pageant Protocol, e.g., Putty, XShell and Jetbrains, needn't any setting in system wide, thus you can't see Pageant in the menu.

Check [Yubikey with WSL tutorial](doc/wsl_tutorial.md) to start using Yubikey with SSH on WSL.

### Work with Xshell

1. Install and run WinCryptSSHAgent
2. Open the Properties dialog box of your session.
3. From Category, select 'SSH', Select 'Use Xagent (SSH agent)' for passphrase handling.
4. From Category, select 'Authentication' and select 'Public Key' as the authentication method.

### OpenSSH Certificates

OpenSSH supports authentication using SSH certificates. Certificates contain a public key, identity information and are signed with a standard SSH key.

Unlike TLS using X.509, OpenSSH uses a special certificate format, thus we can't convert your X.509 certificate into OpenSSH format.

To deal with OpenSSH Certificates, this project introduces a public key override mechanism.

If you want to work with OpenSSH certificates, you should put your OpenSSH Certificates in your `user profile` folder, rename them to `<Your Certificate Common Name>-cert.pub` or `<Your Certificate Serial Number>-cert.pub`.

### Signing Confirmation

By default the agent runs in **Auto Confirm** mode: signing requests from SSH clients are authorized immediately.

Switch to **Manual Confirm** from the tray menu (`•` marks the current mode) to review every signing request in a Yes/No dialog before it is authorized.

- The selected mode is persisted in the registry at `HKCU\Software\WinCryptSSHAgent` (`ConfirmRequired`), so it survives restarts.
- Start with `-confirm` or set `WCSA_CONFIRM=1` to force Manual Confirm; this overrides the registry and updates it.

### Key Auto-Load & Import

 At startup the agent auto-imports `~/.ssh/id_*` (excluding `*.pub` / `*.cert`) plus extra paths from `WCSA_KEYS` (separated by `;` on Windows — spaces are fine, only `;` splits); the tray menu `Import Key…` imports a chosen file with the same logic. Encrypted keys are unlocked with `WCSA_KEY_PASSPHRASE` (one passphrase tried for all keys, kept only in memory, never logged); a key whose passphrase decrypts nothing shows a warning dialog, while cancelled prompts and unreadable files are skipped with a toast — neither blocks startup. If the env passphrase is missing or wrong, a system password dialog asks for the key passphrase (cancel skips with a toast); a successfully entered passphrase is reused in memory for the remaining keys in this run. If `SSH_ASKPASS` points to a helper program it is tried before the built-in dialog; if the helper exits without a password the key is skipped with a toast and the built-in dialog is not shown. PuTTY `.ppk` files are not supported — convert them with PuTTYgen to OpenSSH format first.

### Debug log

1. Run `setx WCSA_DEBUG 1`
2. Reboot to take effect
3. Reproduce your problem
4. The debug log is located in `%USERPROFILE%\WCSA_DEBUG.log`

### Contribute

**Please use issues for everything**

- For a small change, just send a PR.
- For bigger changes open an issue for discussion before sending a PR.
- You can also contribute by:
  - Reporting issues
  - Suggesting new features or enhancements
  - Improve/fix documentation

## Advanced User Manual

### Environment variables

| Variable | Effect |
|---|---|
| `WCSA_DEBUG=1` | Append stdout/stderr to `%USERPROFILE%\WCSA_DEBUG.log` (the binary has no console window). |
| `WCSA_CONFIRM=1` | Force Manual Confirm; overwrites the registry value. |
| `WCSA_KEYS` | Extra private-key files to auto-load, separated by `;` on Windows. |
| `WCSA_KEY_PASSPHRASE` | Single passphrase tried for all encrypted keys (memory only, never logged). |
| `WCSA_CHECKSVR=1` | Before CAPI signing, warn if the Smart Card service is stopped and offer to start it. |
| `SSH_ASKPASS` | Helper program run to obtain a key passphrase when needed. |
| `SSH_AUTH_SOCK` | Standard client-side variable pointing at the agent endpoint (named pipe, `wincrypt-cygwin.sock`, …); each tray menu shows the value to export. |

### Command-line flags

Run `WinCryptSSHAgent.exe -h` for the full list.

| Flag | Effect |
|---|---|
| `-i` | Install the Hyper-V guest communication service (requires elevation). |
| `-confirm` | Force Manual Confirm (same as `WCSA_CONFIRM=1`). |
| `-disable-capi` | Serve only the in-memory keyring, skip the Windows Certificate Store. |
| `-disable-pin-cache` | Clear the smart-card PIN cache after each operation. |

### Registry

| Key | Purpose |
|---|---|
| `HKCU\Software\WinCryptSSHAgent` → DWORD `ConfirmRequired` | Persisted confirm mode (`0` = Auto, `1` = Manual; absent = Auto). Written on every toggle, `-confirm`, or `WCSA_CONFIRM=1`. |
| `HKLM\SOFTWARE\Microsoft\Windows NT\CurrentVersion\Virtualization\GuestCommunicationServices\<service-GUID>` | Hyper-V guest service registration written by `-i` (requires admin). |

### Files

- The binary lives wherever you put it (no installer). Socket files (`%USERPROFILE%\wincrypt-cygwin.sock`, `wincrypt-wsl.sock`) are created at startup and removed on exit; `%USERPROFILE%\WCSA_DEBUG.log` exists only with `WCSA_DEBUG=1`. Imported keys and passphrases live only in process memory and are never written to disk.

### Complete uninstall

Delete the exe (and the startup-folder shortcut if you made one), then run the following in PowerShell (an elevated prompt is only needed for the Hyper-V service key, i.e. if you ever ran `-i`):

```powershell
# Stop a running agent (or Quit it from the tray menu first).
Stop-Process -Name WinCryptSSHAgent,WinCryptSSHAgent-arm64 -ErrorAction SilentlyContinue

# Leftover socket files, debug log and settings.
Remove-Item "$env:USERPROFILE\wincrypt-cygwin.sock", "$env:USERPROFILE\wincrypt-wsl.sock" -Force -ErrorAction SilentlyContinue
Remove-Item "$env:USERPROFILE\WCSA_DEBUG.log" -ErrorAction SilentlyContinue
Remove-Item HKCU:\Software\WinCryptSSHAgent -Recurse -ErrorAction SilentlyContinue

# Hyper-V guest service registration (only present if you ever ran -i).
$svcRoot = 'HKLM:\SOFTWARE\Microsoft\Windows NT\CurrentVersion\Virtualization\GuestCommunicationServices'
Get-ChildItem $svcRoot -ErrorAction SilentlyContinue |
  Where-Object { (Get-ItemProperty $_.PSPath -ErrorAction SilentlyContinue).ElementName -eq 'WinCryptSSHAgent' } |
  Remove-Item -Recurse
```
