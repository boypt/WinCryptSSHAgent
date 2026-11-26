# WinCrypt SSH Agent

> This repository is a maintained fork based on upstream [`buptczq/WinCryptSSHAgent`](https://github.com/buptczq/WinCryptSSHAgent) (base commit: `1e526e8`).

## Fork Enhancements

- **Self-service key import** — add local private-key files or Windows certificate-store keys from the tray menu.
- **Signing confirmation** — Manual/Auto mode per signing request, persisted across restarts.
- **Source-tagged notifications** — toasts show the requesting transport with categorized icons.
- **Windows ARM64 builds** — amd64 + arm64 binaries from `make`, versioned from Git tags.
- **Broader protocol support** — XShell Xagent compatibility (up to XShell 8 build 0119+, incl. agent forwarding), Hyper-V vsock, graceful shutdown.

## Introduction

Windows applications use several incompatible SSH agent interfaces. Native OpenSSH clients use a Windows named pipe, PuTTY-family applications use the Pageant protocol, Git for Windows, MSYS2 and Cygwin use a Cygwin-compatible socket, XShell uses its own Xagent protocol — and WSL clients arrive over Unix sockets and Hyper-V vsock.

WinCryptSSHAgent connects all of these client interfaces to your keys in one place, so a single agent serves every client. Add Windows certificate-store keys, including smart cards such as YubiKey PIV, with **Add Windows Certificate Key…**; add local private-key files with **Import Key…**. Local key contents are held in the in-memory keyring; the registry stores only the chosen file paths and certificate fingerprints so those selections can be restored at startup. It runs as a notification-area application.

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

### Hyper-V / WSL2 vsock

The agent listens on a Hyper-V vsock service (ID `0x22223333`) so that guests can reach it without any port forwarding or socket files:

- **WSL2 / Linux on Hyper-V** — from inside the guest, bridge a local Unix socket to the host over AF_VSOCK (protocol 40). The tray menu *Show WSL2 / Linux On Hyper-V Settings* copies a ready-to-paste `socat` snippet:

  ```bash
  export SSH_AUTH_SOCK=/tmp/wincrypt-hv.sock
  ss -lnx | grep -q $SSH_AUTH_SOCK || {
    rm -f $SSH_AUTH_SOCK
    (setsid nohup socat UNIX-LISTEN:$SSH_AUTH_SOCK,fork \
      SOCKET-CONNECT:40:0:x0000x33332222x02000000x00000000 >/dev/null 2>&1) & disown
  }
  ```

  (`x0000x33332222x02000000x00000000` is the host VM's ID; socat ≥ 1.7.4 also accepts `VSOCK-CONNECT:2:0x22223333`.)

- **Windows guest VM under Hyper-V** — running the same exe inside the guest detects the host via `ConnectHyperV` (dial to `HvsockGUIDParent`) and switches to *HVAgent* mode: all signing requests are forwarded over vsock to the host's agent, which holds the actual certificates / smart-card keys. On the physical host the dial fails (not supported), so the process stays in local CAPI mode. The `-i` flag registers the guest communication service in the guest's registry (requires elevation).

Both paths use the same vsock channel; the difference is whether the guest-side endpoint is a Go program (HVAgent) or a pure byte-forwarder (socat).

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

### Touch Notifications

Touch notifications are optional and disabled by default. Enable them with `-touch-notify` or `WCSA_TOUCH_NOTIFY=1`. During a CAPI signing operation, the agent waits 1.5 seconds before showing a **Touch Security Key** notification. If signing completes sooner, the notification is cancelled; this avoids notifications for quick operations, including touch responses satisfied from a cached touch. Windows does not report whether a CAPI provider is specifically waiting for physical touch, so the notification is a reminder to touch the key if it is waiting, rather than a direct indication from the provider.

```powershell
$env:WCSA_TOUCH_NOTIFY = "1"
.\WinCryptSSHAgent.exe
```

The equivalent command-line option is `-touch-notify`. Explicit command-line values override the environment-variable defaults.

### Key Import

By default, the agent loads `~/.ssh/id_*` files and Windows certificates with private keys from Current User `Personal`. Pass `-disable-autoload` or set `WCSA_DISABLE_AUTOLOAD=1` to stop those broad scans; the agent still restores keys you explicitly selected before and the tray menu lets you add a local key or use **Add Windows Certificate Key…**. The tray menu's **Clear Loaded Keys and Saved Selections…** action removes keys from memory and deletes their saved file paths and certificate fingerprints from the registry; it does not delete source files or certificates. The registry values `SavedKeyFiles` and `SavedCAPIKeyFingerprints` under `HKCU\Software\WinCryptSSHAgent` store local file paths and SHA-256 certificate fingerprints, not private key material. Encrypted local keys may prompt for their passphrase again after a restart.

- Encrypted local keys use a system password dialog; an entered passphrase is reused in memory for later imports during the same run.
- Failures never block: decrypt failure shows a warning dialog, cancelled prompts and unreadable files are skipped with a toast.
- PuTTY `.ppk` files are not supported — convert them with PuTTYgen to OpenSSH format first.

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
| `WCSA_CHECKSVR=1` | Before CAPI signing, warn if the Smart Card service is stopped and offer to start it. |
| `WCSA_DISABLE_AUTOLOAD=1` | Disable automatic scans of `~/.ssh` and the Windows certificate store. Previously saved manual selections still restore. |
| `WCSA_TOUCH_NOTIFY=1` | Enable the 1.5-second delayed security-key touch notification during CAPI signing. |
| `WCSA_DISABLE_AUTH_NOTIFY=1` | Suppress notifications after successful signing. |
| `SSH_AUTH_SOCK` | Standard client-side variable pointing at the agent endpoint (named pipe, `wincrypt-cygwin.sock`, …); each tray menu shows the value to export. |

For `WCSA_DISABLE_AUTOLOAD`, `WCSA_TOUCH_NOTIFY`, and `WCSA_DISABLE_AUTH_NOTIFY`, an explicit command-line flag overrides the environment-variable default. These three boolean variables accept `1`/`0`, `true`/`false`, `yes`/`no`, or `on`/`off` (case-insensitive).

### Command-line flags

Run `WinCryptSSHAgent.exe -h` for the full list.

| Flag | Effect |
|---|---|
| `-i` | Install the Hyper-V guest communication service (requires elevation). |
| `-disable-autoload` | Skip automatic scans of `~/.ssh` and the Windows certificate store. Explicitly saved selections still restore. Also enabled by `WCSA_DISABLE_AUTOLOAD=1`. |
| `-touch-notify` | Show a **Touch Security Key** notification if CAPI signing takes longer than 1.5 seconds; fast operations cancel it. Off by default. Also enabled by `WCSA_TOUCH_NOTIFY=1`. |
| `-disable-auth-notify` | Suppress the “Authenticated” notification after a successful signing operation. Also enabled by `WCSA_DISABLE_AUTH_NOTIFY=1`. |
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

Remove the following:

- The exe and the startup-folder shortcut if you made one.
- Leftover socket files: `%USERPROFILE%\wincrypt-cygwin.sock` and `wincrypt-wsl.sock`.
- `%USERPROFILE%\WCSA_DEBUG.log` (only exists with `WCSA_DEBUG=1`).
- The settings key `HKCU\Software\WinCryptSSHAgent`.
- The `WinCryptSSHAgent` service subkey under the Hyper-V `GuestCommunicationServices` key (only if you ever ran `-i`; needs elevation).

Or run it all at once in PowerShell:

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
