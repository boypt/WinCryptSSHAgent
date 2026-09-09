# AGENTS.md

Windows-only SSH agent (Go) exposing Windows Certificate Store / smart-card keys over multiple SSH agent protocols. Fork of `buptczq/WinCryptSSHAgent` — module path is still `github.com/buptczq/WinCryptSSHAgent`; use that in imports.

## Development Rules
- Do not preserve backward compatibility.
- Choose the simplest implementation that fully meets the current requirements.
- Prefer established, well-maintained libraries over custom implementations.

## Forked deps (windows/arm64; remove when upstream catches up)
- `go.mod` replaces `hattya/go.notify => boypt/go.notify v0.1.1` (adds missing `windows/arm64` syscall bindings, isomorphic to amd64).
- Drop the replace once upstream merges/releases an equivalent (go.notify with arm64 files).

## Build & verify

- No test suite exists. Verification = a successful cross-compile + `go vet`. Do not invent `go test` expectations.
- `go vet ./...` has pre-existing `possible misuse of unsafe.Pointer` warnings (`utils/pageant.go`, `capi/wincapi.go` - Win32 interop). Don't treat them as regressions; just don't add new ones.
- The app is GUI + Win32 (`x/sys/windows`, winio, WMI): it cannot run on Linux/macOS. Cross-compile instead:
  - `GOOS=windows GOARCH=amd64 go build` — quick check build
  - `make` — full 64-bit build: syncs `versioninfo.json` from the latest `v*` git tag (requires `jq`), runs `go generate`, injects version ldflags (`-X main.agentVersion/...`), outputs `WinCryptSSHAgent.exe`. Restores `versioninfo.json` afterward locally (CI keeps it changed).
  - `make all` → amd64 + arm64; `make <arch>` (`amd64`/`arm64`) → single arch; `make clean` removes built exes and generated `*.syso`.
  - When building manually with `go build`, add `-ldflags "-w -s -H=windowsgui"` (same as in `Makefile`).
- `go generate` runs `goversioninfo` with `-platform-specific` — install first: `go install github.com/josephspurrier/goversioninfo/cmd/goversioninfo@latest`. It generates `resource_windows_<arch>.syso` files (all gitignored via `*.syso`).
  - Known quirk: the `Makefile` injects `-X main.agentCommit`, but `main.go` declares `agentBuildHash` — the linker silently ignores `-X` for nonexistent vars, so the commit hash never lands in the binary.

## Release

- Git remotes: `origin` = upstream `buptczq/WinCryptSSHAgent` (no push access), `pt` = fork `boypt/WinCryptSSHAgent` (the push target; `master` tracks `pt`). Release flow: `git push pt master` first, then create the lightweight tag (`git tag vX.Y.Z`) and `git push pt v*` — never `git push origin` (permission denied). CI only needs the tag (it checks out the tag ref with full history), but mainline must be pushed explicitly: pushing a tag uploads the commits without moving the remote `master` ref.
- CI (`.github/workflows/go.yml`) triggers only on pushing a `v*` tag: builds amd64 + arm64 via `make all` and uploads `WinCryptSSHAgent*.exe` to a **prerelease**. Full git history is fetched because the version derives from `git describe`.
- House style after CI: edit the release to a formal/latest one with notes, e.g. `gh release edit v1.1.14 --notes-file <md> --prerelease=false --latest`. Tags are lightweight.

## Architecture

- `main.go` — entrypoint: single-instance mutex, systray, debug-log setup, confirm-mode resolution, selects agent backend, launches all `app.Application`s.
- `app/` — one file per transport/protocol (WSL, Hyper-V vsock, Cygwin socket, named pipe, Pageant, XShell, pubkey view). Each implements the `Application` interface (`app/app.go`); new apps must be added to the `applications` slice in `main.go`.
- `sshagent/` — agent backends: `CAPIAgent` (Windows CryptoAPI), `KeyRingAgent` (in-memory fallback), `HVAgent` (Hyper-V guest mode), `WrappedAgent` (composes backends). `SourceAgent` tags requests with the transport's full name so backends can report `Source:`.
- `capi/` — CryptoAPI bindings; `utils/` — Win32 helpers (UAC, WSL2 detection, Hyper-V, notifications, confirm dialog + registry).
- Runtime mode switch in `main.go`: if a Hyper-V host connection is detected, the process acts as a Hyper-V guest agent (`HVAgent`) instead of serving local cert-store keys.
- Systray menu (`github.com/hattya/go.notify`): the native popup is rebuilt from the Go-side item list on every right-click, and `sysTray.CreateMenu()` replaces that list. `buildMenu()` in `main.go` re-registers all app items plus the Manual/Auto Confirm items — call it from the event-loop goroutine whenever menu state changes.

## Runtime flags / env (useful when debugging)

- `WCSA_DEBUG=1` → redirects stdout/stderr to `%USERPROFILE%\WCSA_DEBUG.log` (no console; binary is built `-H=windowsgui`).
- `-i` installs the Hyper-V guest communication service (needs elevation); `-disable-capi` forces in-memory keyring; `-disable-pin-cache` clears smart-card PIN cache after each op.
- `WCSA_CHECKSVR=1` → before each CAPI signing, warns if the Smart Card service is stopped and offers to restart it.
- Confirm mode: `-confirm` flag or `WCSA_CONFIRM=1` forces Manual Confirm and writes the registry; otherwise state loads from `HKCU\Software\WinCryptSSHAgent` DWORD `ConfirmRequired` (absent → Auto). Manual = blocking Yes/No dialog per signing and the "Authenticated" toast is suppressed; Auto = silent signing + toast.

## Dialogs (confirm UI)

- Confirm dialog = `MessageBox` (`MessageBoxIndirect` with `MB_USERICON` + plain-`MessageBox` fallback in `utils/confirm.go`). `TaskDialogIndirect` was tried and reverted — its popup is noticeably slower. Keep the manifest anyway: Common-Controls v6 gives `MessageBox` modern visuals for free.
- Manifest wiring: `versioninfo.json` `ManifestPath` = `app.manifest` (Common-Controls 6.0), embedded by `goversioninfo` during `go generate`. It must be committed — the `Makefile` reverts uncommitted `versioninfo.json` locally after a successful build (`restore-version`; on a failed build restore it manually with `git checkout -- versioninfo.json`), silently dropping the manifest on the next generate.
- goversioninfo id quirk: the manifest occupies resource id 1, shifting the icon group from id 1 to id 2. All icon loads handle both layouts: `initSystray` tries `LoadIcon(2)` then `LoadIcon(1)`; `messageBoxConfirm` tries iconResId 2, then 1, then plain `MessageBox`.
- Caption icon: `MessageBox` resolves the title-bar icon as exe icon id 1 and falls back to the generic `IDI_APPLICATION` icon after the shift. The foreground watcher (`utils/foreground.go`) sets `WM_SETICON` (`ICON_SMALL` + `ICON_BIG`) with the app icon (ids {2,1}, `LR_SHARED`) once the dialog appears.
- Focus: a background tray process cannot rely on `MB_SETFOREGROUND` (foreground-lock no-op). `raiseDialogWhenShown` pins the OS thread, finds the `#32770` dialog via `EnumThreadWindows`, and forces foreground via temporary `AttachThreadInput`. Applies to any thread-owned modal dialog.
- Debugging dialogs from Linux (cannot run): inspect PE resources (e.g. throwaway `debug/pe` lister) for `RT_GROUP_ICON` / `RT_MANIFEST` ids; `WCSA_DEBUG=1` log captures Win32 `HRESULT`s.

## Conventions

- Per README: use GitHub issues for everything; discuss non-trivial changes in an issue before a PR.
- Comments and some script messages are a mix of English and Chinese — match the file you're editing.
- User-facing prompts (dialogs, toasts, menus) are English-only.
- User-facing label for the transport origin is `Source:` (not `Channel:`) in both dialogs and notifications.
