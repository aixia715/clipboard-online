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
$toast_notifier_exe = "${RELEASE_DIR}\toast_notifier.exe"
$toast_notifier_py = "${ROOT}\toast_notifier.py"
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
    Write-Output "[1/5] Checking Python environment..."
    try {
        $python = python --version 2>&1
        Write-Output "  ✓ Python found: $python"
        return $true
    }
    catch {
        Write-Output "  ⚠ Python not found in PATH. HEIF and Toast support will be disabled."
        return $false
    }
}

function build_heif_converter() {
    if (-not (Test-Path $heif_converter_py)) {
        Write-Output "  ⚠ heif_converter.py not found, skipping HEIF converter build"
        return $false
    }

    Write-Output "  Building heif_converter.exe..."
    
    # 安装依赖
    Write-Output "    Checking dependencies..."
    python -m pip install pillow pillow-heif pyinstaller -q 2>&1 | Out-Null
    
    # 编译
    python -m PyInstaller --onefile --console --name heif_converter --noupx --hidden-import=pillow_heif --hidden-import=PIL --distpath "${RELEASE_DIR}" --workpath "${ROOT}\.build" --specpath "${ROOT}\.build" $heif_converter_py 2>&1 | Out-Null
    
    if (Test-Path $heif_converter_exe) {
        Write-Output "  ✓ heif_converter.exe created: $(Get-Item $heif_converter_exe | Select-Object -ExpandProperty Length) bytes"
        return $true
    }
    else {
        Write-Output "  ✗ Failed to create heif_converter.exe"
        return $false
    }
}

function build_toast_notifier() {
    if (-not (Test-Path $toast_notifier_py)) {
        Write-Output "  ⚠ toast_notifier.py not found, skipping Toast notifier build"
        return $false
    }

    Write-Output "  Building toast_notifier.exe..."
    
    # 安装依赖
    Write-Output "    Checking dependencies..."
    python -m pip install windows-toasts pyinstaller -q 2>&1 | Out-Null
    
    # 编译
    python -m PyInstaller --onefile --console --name toast_notifier --noupx --hidden-import=windows_toasts --hidden-import=winsdk --distpath "${RELEASE_DIR}" --workpath "${ROOT}\.build" --specpath "${ROOT}\.build" $toast_notifier_py 2>&1 | Out-Null
    
    if (Test-Path $toast_notifier_exe) {
        Write-Output "  ✓ toast_notifier.exe created: $(Get-Item $toast_notifier_exe | Select-Object -ExpandProperty Length) bytes"
        return $true
    }
    else {
        Write-Output "  ✗ Failed to create toast_notifier.exe"
        return $false
    }
}

function build_go_program() {
    Write-Output "[4/5] Building Go program..."
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
    Write-Output "[5/5] Cleaning up..."
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
    if (Test-Path "${ROOT}\toast_notifier.exe") {
        Remove-Item "${ROOT}\toast_notifier.exe" -Force
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
    
    $toast_release = "${RELEASE_DIR}\toast_notifier.exe"
    if (Test-Path $toast_release) {
        $size = Get-Item $toast_release | Select-Object -ExpandProperty Length
        Write-Output "✓ toast_notifier.exe: $size bytes"
    }
    else {
        Write-Output "⚠ toast_notifier.exe: OPTIONAL (Toast notification disabled)"
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
    
    Write-Output "[2/5] Building HEIF converter..."
    if (check_python_env) {
        build_heif_converter | Out-Null
        
        Write-Output "[3/5] Building Toast notifier..."
        build_toast_notifier | Out-Null
    }
    
    build_go_program
    cleanup
    verify_build
}

# Start build
main