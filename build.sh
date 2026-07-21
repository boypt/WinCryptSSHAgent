#!/bin/bash

# 设置 Windows 目标平台
export GOOS=windows

# 根据 git tag 同步 versioninfo.json 中的版本号
sync_versioninfo() {
    local BUILD_VER
    BUILD_VER="$(git describe --tags --always --abbrev=0 --match='v[0-9]*.[0-9]*.[0-9]*' 2> /dev/null | sed 's/^.//')"
    if [[ -z "$BUILD_VER" || ! "$BUILD_VER" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
        echo "Skip versioninfo.json sync (invalid or empty BUILD_VER: '$BUILD_VER')"
        return 0
    fi
    local MAJ MIN PAT
    IFS='.' read -r MAJ MIN PAT <<< "$BUILD_VER"
    jq \
      --argjson maj "$MAJ" --argjson min "$MIN" --argjson pat "$PAT" \
      --arg ver "$BUILD_VER" '
        .FixedFileInfo.FileVersion.Major = $maj
      | .FixedFileInfo.FileVersion.Minor = $min
      | .FixedFileInfo.FileVersion.Patch = $pat
      | .FixedFileInfo.ProductVersion.Major = $maj
      | .FixedFileInfo.ProductVersion.Minor = $min
      | .FixedFileInfo.ProductVersion.Patch = $pat
      | .StringFileInfo.ProductVersion = $ver
    ' versioninfo.json > versioninfo.json.tmp && mv versioninfo.json.tmp versioninfo.json
    echo "versioninfo.json synced to $BUILD_VER"
}

# 本地构建结束后还原 versioninfo.json，避免污染工作树；CI 环境保持不动
restore_versioninfo_if_local() {
    if [ -z "$CI" ] && git rev-parse --git-dir > /dev/null 2>&1; then
        git checkout -- versioninfo.json 2>/dev/null || true
    fi
}
trap restore_versioninfo_if_local EXIT

# 编译函数
build_arch() {
    local arch=$1
    local output=$2
    local BUILD_VER="$(git describe --tags --always --abbrev=0 --match='v[0-9]*.[0-9]*.[0-9]*' 2> /dev/null | sed 's/^.//')"
    local COMMIT_HASH="$(git rev-parse --short HEAD)"
    local BUILD_TIME=$(date '+%Y-%m-%d')
    
    echo "Building $arch to $output, version: $BUILD_VER, commit: $COMMIT_HASH, build time: $BUILD_TIME"

    export GOARCH=$arch
    go build -trimpath \
        -ldflags "-w -s -H=windowsgui \
        -X main.agentVersion=$BUILD_VER \
        -X main.agentCommit=$COMMIT_HASH \
        -X main.agentBuildTime=$BUILD_TIME" \
        -o "$output"
} 

# 根据参数选择编译方式
if [ "$1" = "all" ]; then
    # 同步版本信息并生成资源文件
    sync_versioninfo
    go generate

    # 编译所有架构
    build_arch "386" "WinCryptSSHAgent_32bit.exe"
    build_arch "amd64" "WinCryptSSHAgent.exe"
elif [ -z "$1" ]; then
    # 默认只编译 64 位版本
    sync_versioninfo
    go generate
    build_arch "amd64" "WinCryptSSHAgent.exe"
else
    # 编译指定架构
    sync_versioninfo
    go generate
    build_arch "$1" "WinCryptSSHAgent-$1.exe"
fi
