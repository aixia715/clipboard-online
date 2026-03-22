<#
.SYNOPSIS
    build script - Compiles Go program and Python HEIF converter
#>

param (
    [string]$version = "",
    [Switch]$debug
)

Set-Location -Path $(Split-Path -Parent $MyInvocation.MyCommand.Definition)

$ROOT = $PWD
$prog = $($(Select-String module go.mod) -replace '.*?/')
$RELEASE_DIR = "${ROOT}\release"
$ENV_GOAPTH = $(go env GOPATH)
$output = "${RELEASE_DIR}/${prog}.exe"
$heif_converter_exe = "${RELEASE_DIR}\heif_converter.exe"
$heif_converter_py = "${ROOT}\heif_converter.py"
$mode = "release"

if ($version -eq "") {
    git describe --exact-match 2>$null
    if ($?) {
        $version = $(git describe --tags --abbrev=0)
    }
    else {
        $version = $(git log --pretty=format:'%h' -1)
    }
}

if ($debug) {
    $mode = "debug"
}

function check_python_env() {
    Write-Output "[1/4] Checking Python environment..."
    try {
        $python = python --version 2>&1
        Write-Output "  ✓ Python found: $python"
        return $true
    }
    catch {
        Write-Output "  ⚠ Python not found in PATH. HEIF support will be disabled."
        return $false
    }
}

function build_heif_converter() {
    if (-not (Test-Path $heif_converter_py)) {
        Write-Output "  ⚠ heif_converter.py not found, skipping HEIF converter build"
        return $false
    }

    Write-Output "  Building heif_converter.exe..."
    
    # 检查 PyInstaller
    $pyinstaller_check = python -c "import PyInstaller; print('OK')" 2>&1
    if ($pyinstaller_check -ne "OK") {
        Write-Output "    Installing PyInstaller..."
        python -m pip install pyinstaller -q
    }
    
    # 编译
    python -m PyInstaller --onefile --console --name heif_converter --noupx --distpath "${RELEASE_DIR}" --workpath "${ROOT}\.build" --specpath "${ROOT}\.build" $heif_converter_py 2>&1 | Out-Null
    
    if (Test-Path $heif_converter_exe) {
        Write-Output "  ✓ heif_converter.exe created: $(Get-Item $heif_converter_exe | Select-Object -ExpandProperty Length) bytes"
        return $true
    }
    else {
        Write-Output "  ✗ Failed to create heif_converter.exe"
        return $false
    }
}

function build_go_program() {
    Write-Output "[3/4] Building Go program..."
    & $ENV_GOAPTH/bin/rsrc.exe -manifest clipboard-online.manifest -ico app.ico -o rsrc.syso
    
    if ($mode -eq "debug") {
        build_debug
    }
    else {
        build_release
    }
}

function build_debug() {
    Write-Output "  Building in debug mode..."
    go build -ldflags="-X 'main.mode=$mode' -X 'main.version=$version'" -o $output
}

function build_release() {
    Write-Output "  Building in release mode..."
    go build -ldflags="-s -w -H windowsgui -X 'main.mode=$mode' -X 'main.version=$version'" -o $output
}

function cleanup() {
    Write-Output "[4/4] Cleaning up..."
    if (Test-Path "${ROOT}\.build") {
        Remove-Item "${ROOT}\.build" -Recurse -Force
    }
    if (Test-Path "${ROOT}\build") {
        Remove-Item "${ROOT}\build" -Recurse -Force
    }
    if (Test-Path "${ROOT}\rsrc.syso") {
        Remove-Item "${ROOT}\rsrc.syso" -Force
    }
    if (Test-Path "${ROOT}\heif_converter.exe") {
        Remove-Item "${ROOT}\heif_converter.exe" -Force
    }
    Write-Output "  ✓ Cleaned up intermediate files"
}

function verify_build() {
    Write-Output ""
    Write-Output "========================================"
    Write-Output "Build Summary"
    Write-Output "========================================"
    
    if (Test-Path $output) {
        $size = Get-Item $output | Select-Object -ExpandProperty Length
        Write-Output "✓ clipboard-online.exe: $size bytes"
    }
    else {
        Write-Output "✗ clipboard-online.exe: NOT FOUND"
    }
    
    $heif_release = "${RELEASE_DIR}\heif_converter.exe"
    if (Test-Path $heif_release) {
        $size = Get-Item $heif_release | Select-Object -ExpandProperty Length
        Write-Output "✓ heif_converter.exe: $size bytes"
    }
    else {
        Write-Output "⚠ heif_converter.exe: OPTIONAL (HEIF support disabled)"
    }
    
    Write-Output ""
    Write-Output "Release directory: $RELEASE_DIR"
    Write-Output "Ready to run: .\release\clipboard-online.exe"
    Write-Output "========================================"
}

function main() {
    Write-Output "clipboard-online build script"
    Write-Output "Version: $version, Mode: $mode"
    Write-Output ""
    
    # 确保 release 目录存在
    if (-not (Test-Path $RELEASE_DIR)) {
        New-Item -ItemType Directory -Path $RELEASE_DIR | Out-Null
    }
    
    Write-Output "[2/4] Building HEIF converter..."
    if (check_python_env) {
        build_heif_converter | Out-Null
    }
    
    build_go_program
    cleanup
    verify_build
}

# Start build
main