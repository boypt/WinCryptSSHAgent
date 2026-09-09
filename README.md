# WinCrypt SSH Agent

> This repository is a maintained fork based on upstream [`buptczq/WinCryptSSHAgent`](https://github.com/buptczq/WinCryptSSHAgent) (base commit: `1e526e8`).

## Fork Enhancements

- **Richer notifications** — toasts now tag the source transport channel (e.g. `[Pageant]`, `[WSL]`) and reliably display the app icon; icons are categorized by event type (auth, key added, key removed).
- **Signing confirmation** — the tray menu offers `Manual Confirm` / `Auto Confirm` (a `•` marks the current mode). In Manual mode a blocking dialog is shown before each signing operation. The choice is persisted in the registry (`HKCU\Software\WinCryptSSHAgent`) and survives restarts; `-confirm` or `WCSA_CONFIRM=1` forces Manual mode at startup and overrides the registry.
- **Instant & graceful shutdown** — all listeners cleanly unblock on exit; single-instance mutex prevents conflicts.
- **Improved protocol support** — better XShell 5/7/8 Xagent compatibility (by zzmark); Hyper-V VSock migrated from deprecated `linuxkit/virtsock` to `go-winio`.
- **Automated build & versioning** — `make` produces multi-arch binaries with Git-tag-derived version, commit hash, and build date injected at link time; dependencies upgraded.

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
