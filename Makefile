# WinCryptSSHAgent build (Windows-only Go binary).
#
#   make            amd64 only -> WinCryptSSHAgent.exe (default)
#   make all        amd64 + arm64
#   make amd64      amd64 only -> WinCryptSSHAgent.exe
#   make arm64      arm64 only -> WinCryptSSHAgent-arm64.exe
#   make clean      remove built exes and generated *.syso files
#
# Requires: go, jq, goversioninfo
#   (go install github.com/josephspurrier/goversioninfo/cmd/goversioninfo@latest)

SHELL := /bin/bash
GOOS := windows
export GOOS

.DEFAULT_GOAL := amd64

BUILD_VER = $(shell git describe --tags --always --abbrev=0 --match='v[0-9]*.[0-9]*.[0-9]*' 2>/dev/null | sed 's/^.//')
COMMIT_HASH = $(shell git rev-parse --short HEAD)
BUILD_TIME = $(shell date '+%Y-%m-%d')

LDFLAGS = -w -s -H=windowsgui -X main.agentVersion=$(BUILD_VER) -X main.agentCommit=$(COMMIT_HASH) -X main.agentBuildTime=$(BUILD_TIME)

.PHONY: all amd64 arm64 clean sync-version restore-version

define build-arch
GOARCH=$(1) go build -trimpath -ldflags "$(LDFLAGS)" -o $(2) || ($(MAKE) restore-version && exit 1)
endef

all: sync-version
	go generate
	$(call build-arch,amd64,WinCryptSSHAgent.exe)
	$(call build-arch,arm64,WinCryptSSHAgent-arm64.exe)
	$(MAKE) restore-version

amd64: sync-version
	go generate
	$(call build-arch,amd64,WinCryptSSHAgent.exe)
	$(MAKE) restore-version

arm64: sync-version
	go generate
	$(call build-arch,arm64,WinCryptSSHAgent-arm64.exe)
	$(MAKE) restore-version

clean:
	rm -f WinCryptSSHAgent*.exe resource_windows_*.syso

# Syncs versioninfo.json from the latest v* git tag (requires jq).
sync-version:
	@BUILD_VER="$(BUILD_VER)"; \
	if [[ -z "$$BUILD_VER" || ! "$$BUILD_VER" =~ ^[0-9]+\.[0-9]+\.[0-9]+$$ ]]; then \
		echo "Skip versioninfo.json sync (invalid or empty BUILD_VER: '$$BUILD_VER')"; \
	else \
		IFS='.' read -r MAJ MIN PAT <<< "$$BUILD_VER"; \
		jq --argjson maj "$$MAJ" --argjson min "$$MIN" --argjson pat "$$PAT" --arg ver "$$BUILD_VER" \
			'.FixedFileInfo.FileVersion.Major = $$maj \
			| .FixedFileInfo.FileVersion.Minor = $$min \
			| .FixedFileInfo.FileVersion.Patch = $$pat \
			| .FixedFileInfo.ProductVersion.Major = $$maj \
			| .FixedFileInfo.ProductVersion.Minor = $$min \
			| .FixedFileInfo.ProductVersion.Patch = $$pat \
			| .StringFileInfo.ProductVersion = $$ver' \
			versioninfo.json > versioninfo.json.tmp && mv versioninfo.json.tmp versioninfo.json; \
		echo "versioninfo.json synced to $$BUILD_VER"; \
	fi

# Restores versioninfo.json after a local build (CI keeps it changed).
restore-version:
	@if [ -z "$$CI" ] && git rev-parse --git-dir > /dev/null 2>&1; then \
		git checkout -- versioninfo.json 2>/dev/null || true; \
	fi
