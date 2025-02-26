#!/bin/bash

# 设置 Windows 目标平台
export GOOS=windows

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
    # 生成资源文件
    go generate

    # 编译所有架构
    build_arch "386" "WinCryptSSHAgent_32bit.exe"
    build_arch "amd64" "WinCryptSSHAgent.exe"
elif [ -z "$1" ]; then
    # 默认只编译 64 位版本
    go generate
    build_arch "amd64" "WinCryptSSHAgent.exe"
else
    # 编译指定架构
    go generate
    build_arch "$1" "WinCryptSSHAgent-$1.exe"
fi