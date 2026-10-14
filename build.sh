#!/bin/bash

cd "$( dirname "${BASH_SOURCE[0]}" )"

ROOT=$(pwd)
RELEASE_DIR="$ROOT/release"

prog=$(grep "^module .*$" go.mod | sed -r "s/^module .*\/(.*)$/\1/")
version=$(if [[ $(git describe --exact-match 2>/dev/null) != "" ]]; then git describe --tags --abbrev=0; else git log --pretty=format:'%h' -1; fi)
output="$RELEASE_DIR/$prog"
mode="release"

while getopts "hdv:" arg; do
  case $arg in
    h)
      echo "build [-d] [-v version]"
      echo "  -d: debug mode"
      echo "  -v: set version string"
      ;;
    d)
      mode="debug"
      ;;
    v)
      version="$OPTARG"
      ;;
  esac
done

function build_go_program() {
    echo "[1/2] Building Go program..."

    if [[ "$mode" == "debug" ]]; then
        build_debug
    else
        build_release
    fi
}

function build_debug() {
    echo "  Building in debug mode..."
    go build -ldflags="-X 'main.mode=$mode' -X 'main.version=$version'" -o "$output"
}

function build_release() {
    echo "  Building in release mode..."
    go build -ldflags="-s -w -X 'main.mode=$mode' -X 'main.version=$version'" -o "$output"
}

function verify_build() {
    echo ""
    echo "========================================"
    echo "Build Summary"
    echo "========================================"

    if [[ -f "$output" ]]; then
        size=$(stat -c%s "$output" 2>/dev/null || stat -f%z "$output" 2>/dev/null || echo "?")
        echo "✓ clipboard-online: $size bytes"
    else
        echo "✗ clipboard-online: NOT FOUND"
    fi

    echo ""
    echo "Release directory: $RELEASE_DIR"
    echo "Ready to run: ./release/clipboard-online"
    echo "========================================"
}

function main() {
    echo "clipboard-online build script"
    echo "Version: $version, Mode: $mode"
    echo ""

    # 确保 release 目录存在
    mkdir -p "$RELEASE_DIR"

    build_go_program
    verify_build
}

# Start build
main
