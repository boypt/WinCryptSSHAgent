# AGENTS.md

Windows-only SSH agent (Go) exposing Windows Certificate Store / smart-card keys over multiple SSH agent protocols. Fork of `buptczq/WinCryptSSHAgent` — module path is still `github.com/buptczq/WinCryptSSHAgent`; use that in imports.

## Build & verify

- No test suite exists. Verification = a successful cross-compile + `go vet`. Do not invent `go test` expectations.
- `go vet ./...` has pre-existing `possible misuse of unsafe.Pointer` warnings (`utils/pageant.go`, `capi/wincapi.go` - Win32 interop). Don't treat them as regressions; just don't add new ones.
- The app is GUI + Win32 (`x/sys/windows`, winio, WMI): it cannot run on Linux/macOS. Cross-compile instead:
  - `GOOS=windows GOARCH=amd64 go build` — quick check build
  - `./build.sh` — full 64-bit build: syncs `versioninfo.json` from the latest `v*` git tag (requires `jq`), runs `go generate`, injects version ldflags (`-X main.agentVersion/...`), outputs `WinCryptSSHAgent.exe`. Restores `versioninfo.json` afterward locally (CI keeps it changed).
  - `./build.sh all` → 386 + amd64; `./build.sh <arch>` → single arch.
  - `build.bat` is the Windows equivalent but does not inject version ldflags.
  - When building manually with `go build`, add `-ldflags "-w -s -H=windowsgui"` (same as in `build.sh`).
- `go generate` runs `goversioninfo` — install first: `go install github.com/josephspurrier/goversioninfo/cmd/goversioninfo@latest`. It generates `resource.syso` (gitignored, embedded icon/version info).

## Release

CI (`.github/workflows/go.yml`) triggers only on pushing a `v*` tag: builds via `build.sh` and attaches `WinCryptSSHAgent*.exe` to a draft prerelease. Full git history is fetched because the version derives from `git describe`.

## Architecture

- `main.go` — entrypoint: single-instance mutex, systray, debug-log setup, selects agent backend, launches all `app.Application`s.
- `app/` — one file per transport/protocol (WSL, Hyper-V vsock, Cygwin socket, named pipe, Pageant, XShell, pubkey view). Each implements the `Application` interface (`app/app.go`); new apps must be added to the `applications` slice in `main.go`.
- `sshagent/` — agent backends: `CAPIAgent` (Windows CryptoAPI), `KeyRingAgent` (in-memory fallback), `HVAgent` (Hyper-V guest mode), `WrappedAgent` (composes backends).
- `capi/` — CryptoAPI bindings; `utils/` — Win32 helpers (UAC, WSL2 detection, Hyper-V, notifications).
- Runtime mode switch in `main.go`: if a Hyper-V host connection is detected, the process acts as a Hyper-V guest agent (`HVAgent`) instead of serving local cert-store keys.

## Runtime flags / env (useful when debugging)

- `WCSA_DEBUG=1` → redirects stdout/stderr to `%USERPROFILE%\WCSA_DEBUG.log` (no console; binary is built `-H=windowsgui`).
- `-i` installs the Hyper-V guest communication service (needs elevation); `-disable-capi` forces in-memory keyring; `-disable-pin-cache` clears smart-card PIN cache after each op.

## Conventions

- Per README: use GitHub issues for everything; discuss non-trivial changes in an issue before a PR.
- Comments and some script messages are a mix of English and Chinese — match the file you're editing.
