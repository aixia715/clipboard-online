#!/bin/bash

cd "$( dirname "${BASH_SOURCE[0]}" )"

ROOT=$(pwd)
RELEASE_DIR="$ROOT/release"
ENV_GOAPTH=$(go env GOPATH)

prog=$(grep "^module .*$" go.mod | sed -r "s/^module .*\/(.*)$/\1/")
version=$(if [[ $(git describe --exact-match 2>/dev/null) != "" ]]; then git describe --tags --abbrev=0; else git log --pretty=format:'%h' -1; fi)
output="$RELEASE_DIR/$prog.exe"
heif_converter_exe="$RELEASE_DIR/heif_converter.exe"
heif_converter_py="$ROOT/heif_converter.py"
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

function check_python_env() {
    echo "[2/4] Checking Python environment..."
    if command -v python3 &> /dev/null; then
        python_ver=$(python3 --version 2>&1)
        echo "  ✓ Python found: $python_ver"
        return 0
    elif command -v python &> /dev/null; then
        python_ver=$(python --version 2>&1)
        echo "  ✓ Python found: $python_ver"
        return 0
    else
        echo "  ⚠ Python not found. HEIF support will be disabled."
        return 1
    fi
}

function get_python_cmd() {
    if command -v python3 &> /dev/null; then
        echo "python3"
    else
        echo "python"
    fi
}

function build_heif_converter() {
    if [[ ! -f "$heif_converter_py" ]]; then
        echo "  ⚠ heif_converter.py not found, skipping HEIF converter build"
        return 1
    fi
    
    echo "  Building heif_converter.exe..."
    python_cmd=$(get_python_cmd)
    
    # 检查 PyInstaller
    $python_cmd -c "import PyInstaller" 2>/dev/null
    if [[ $? -ne 0 ]]; then
        echo "    Installing PyInstaller..."
        $python_cmd -m pip install pyinstaller -q
    fi
    
    # 编译（Windows 特定）
    if [[ "$OSTYPE" == "msys" || "$OSTYPE" == "win32" ]]; then
        $python_cmd -m PyInstaller --onefile --console --name heif_converter --noupx --distpath "$RELEASE_DIR" --workpath "$ROOT/.build" --specpath "$ROOT/.build" "$heif_converter_py" 2>&1 >/dev/null
        
        if [[ -f "$heif_converter_exe" ]]; then
            size=$(stat -f%z "$heif_converter_exe" 2>/dev/null || stat -c%s "$heif_converter_exe" 2>/dev/null || echo "?")
            echo "  ✓ heif_converter.exe created: $size bytes"
            return 0
        else
            echo "  ✗ Failed to create heif_converter.exe"
            return 1
        fi
    else
        echo "  ⚠ Skipping HEIF converter build (Windows-only tool)"
        return 1
    fi
}

function build_go_program() {
    echo "[3/4] Building Go program..."
    $ENV_GOAPTH/bin/rsrc.exe -manifest clipboard-online.manifest -ico app.ico -o rsrc.syso 2>/dev/null || true
    
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
    go build -ldflags="-s -w -H windowsgui -X 'main.mode=$mode' -X 'main.version=$version'" -o "$output"
}

function cleanup() {
    echo "[4/4] Cleaning up..."
    rm -rf "$ROOT/.build" "$ROOT/build" "$ROOT/rsrc.syso" "$ROOT/__pycache__"
    rm -f "$ROOT/heif_converter.exe"
    echo "  ✓ Cleaned up intermediate files"
}

function verify_build() {
    echo ""
    echo "========================================"
    echo "Build Summary"
    echo "========================================"
    
    if [[ -f "$output" ]]; then
        size=$(stat -f%z "$output" 2>/dev/null || stat -c%s "$output" 2>/dev/null || echo "?")
        echo "✓ clipboard-online.exe: $size bytes"
    else
        echo "✗ clipboard-online.exe: NOT FOUND"
    fi
    
    if [[ -f "$RELEASE_DIR/heif_converter.exe" ]]; then
        size=$(stat -f%z "$RELEASE_DIR/heif_converter.exe" 2>/dev/null || stat -c%s "$RELEASE_DIR/heif_converter.exe" 2>/dev/null || echo "?")
        echo "✓ heif_converter.exe: $size bytes"
    else
        echo "⚠ heif_converter.exe: OPTIONAL (HEIF support disabled)"
    fi
    
    echo ""
    echo "Release directory: $RELEASE_DIR"
    echo "Ready to run: ./release/clipboard-online.exe"
    echo "========================================"
}

function main() {
    echo "clipboard-online build script"
    echo "Version: $version, Mode: $mode"
    echo ""
    
    # 确保 release 目录存在
    mkdir -p "$RELEASE_DIR"
    
    echo "[1/4] Preparing environment..."
    
    if check_python_env; then
        build_heif_converter
    else
        echo ""
    fi
    
    build_go_program
    cleanup
    verify_build
}

# Start build
main