import ctypes
from ctypes import wintypes
from dataclasses import dataclass
from typing import List, Optional, Tuple


# Clipboard formats used by this project.
CF_UNICODETEXT = 13
CF_HDROP = 15
CF_DIB = 8
CF_DIBV5 = 17

GMEM_MOVEABLE = 0x0002

user32 = ctypes.WinDLL("user32", use_last_error=True)
kernel32 = ctypes.WinDLL("kernel32", use_last_error=True)
shell32 = ctypes.WinDLL("shell32", use_last_error=True)

# Define WinAPI signatures explicitly to avoid 64-bit pointer truncation.
user32.OpenClipboard.argtypes = [wintypes.HWND]
user32.OpenClipboard.restype = wintypes.BOOL
user32.CloseClipboard.argtypes = []
user32.CloseClipboard.restype = wintypes.BOOL
user32.IsClipboardFormatAvailable.argtypes = [wintypes.UINT]
user32.IsClipboardFormatAvailable.restype = wintypes.BOOL
user32.GetClipboardData.argtypes = [wintypes.UINT]
user32.GetClipboardData.restype = wintypes.HANDLE

kernel32.GlobalLock.argtypes = [wintypes.HGLOBAL]
kernel32.GlobalLock.restype = ctypes.c_void_p
kernel32.GlobalUnlock.argtypes = [wintypes.HGLOBAL]
kernel32.GlobalUnlock.restype = wintypes.BOOL

shell32.DragQueryFileW.argtypes = [wintypes.HANDLE, wintypes.UINT, wintypes.LPWSTR, wintypes.UINT]
shell32.DragQueryFileW.restype = wintypes.UINT


class BITMAPINFOHEADER(ctypes.Structure):
    _fields_ = [
        ("biSize", wintypes.DWORD),
        ("biWidth", ctypes.c_long),
        ("biHeight", ctypes.c_long),
        ("biPlanes", wintypes.WORD),
        ("biBitCount", wintypes.WORD),
        ("biCompression", wintypes.DWORD),
        ("biSizeImage", wintypes.DWORD),
        ("biXPelsPerMeter", ctypes.c_long),
        ("biYPelsPerMeter", ctypes.c_long),
        ("biClrUsed", wintypes.DWORD),
        ("biClrImportant", wintypes.DWORD),
    ]


@dataclass
class ClipboardResult:
    kind: str
    summary: str
    detail_lines: List[str]


def _raise_last_error(prefix: str) -> None:
    err = ctypes.get_last_error()
    raise OSError(err, f"{prefix} failed with WinError={err}")


def _open_clipboard() -> None:
    if not user32.OpenClipboard(None):
        _raise_last_error("OpenClipboard")


def _close_clipboard() -> None:
    user32.CloseClipboard()


def _is_format_available(fmt: int) -> bool:
    return bool(user32.IsClipboardFormatAvailable(fmt))


def _read_unicode_text() -> Optional[str]:
    if not _is_format_available(CF_UNICODETEXT):
        return None

    handle = user32.GetClipboardData(CF_UNICODETEXT)
    if not handle:
        _raise_last_error("GetClipboardData(CF_UNICODETEXT)")

    ptr = kernel32.GlobalLock(handle)
    if not ptr:
        _raise_last_error("GlobalLock(text)")

    try:
        return ctypes.wstring_at(ptr)
    finally:
        kernel32.GlobalUnlock(handle)


def _read_files() -> Optional[List[str]]:
    if not _is_format_available(CF_HDROP):
        return None

    handle = user32.GetClipboardData(CF_HDROP)
    if not handle:
        _raise_last_error("GetClipboardData(CF_HDROP)")

    count = shell32.DragQueryFileW(handle, 0xFFFFFFFF, None, 0)
    files: List[str] = []
    for i in range(count):
        length = shell32.DragQueryFileW(handle, i, None, 0)
        buf = ctypes.create_unicode_buffer(length + 1)
        shell32.DragQueryFileW(handle, i, buf, length + 1)
        files.append(buf.value)
    return files


def _read_bitmap_info() -> Optional[Tuple[int, int, int, int]]:
    # Prefer DIBV5, fallback to DIB.
    fmt = CF_DIBV5 if _is_format_available(CF_DIBV5) else (CF_DIB if _is_format_available(CF_DIB) else None)
    if fmt is None:
        return None

    handle = user32.GetClipboardData(fmt)
    if not handle:
        _raise_last_error("GetClipboardData(CF_DIB/CF_DIBV5)")

    ptr = kernel32.GlobalLock(handle)
    if not ptr:
        _raise_last_error("GlobalLock(bitmap)")

    try:
        header = BITMAPINFOHEADER.from_address(ptr)
        width = int(header.biWidth)
        height = abs(int(header.biHeight))
        bit_count = int(header.biBitCount)

        # Prefer biSizeImage when available, else estimate from width/height/bit depth.
        if header.biSizeImage:
            image_bytes = int(header.biSizeImage)
        else:
            row_bytes = ((width * bit_count + 31) // 32) * 4
            image_bytes = row_bytes * height

        return width, height, bit_count, image_bytes
    finally:
        kernel32.GlobalUnlock(handle)


def analyze_clipboard() -> ClipboardResult:
    _open_clipboard()
    try:
        # Match project priority: file > bitmap > text.
        files = _read_files()
        if files:
            preview = files[:3]
            lines = [f"- {p}" for p in preview]
            if len(files) > 3:
                lines.append(f"- ... and {len(files) - 3} more")
            return ClipboardResult(
                kind="file",
                summary=f"{len(files)} file(s)",
                detail_lines=lines,
            )

        bmp_info = _read_bitmap_info()
        if bmp_info is not None:
            width, height, bit_count, image_bytes = bmp_info
            return ClipboardResult(
                kind="bitmap",
                summary=f"{width}x{height}, {bit_count}-bit",
                detail_lines=[f"- estimated bytes: {image_bytes}"],
            )

        text = _read_unicode_text()
        if text is not None:
            preview = text.replace("\r", " ").replace("\n", " ").strip()
            if len(preview) > 80:
                preview = preview[:80] + "..."
            return ClipboardResult(
                kind="text",
                summary=f"{len(text)} character(s)",
                detail_lines=[f"- preview: {preview}"],
            )

        return ClipboardResult(
            kind="unknown",
            summary="No supported clipboard format detected",
            detail_lines=["- supported: file(CF_HDROP), bitmap(CF_DIB/CF_DIBV5), text(CF_UNICODETEXT)"],
        )
    finally:
        _close_clipboard()


def main() -> None:
    result = analyze_clipboard()
    print(f"kind: {result.kind}")
    print(f"summary: {result.summary}")
    print("details:")
    for line in result.detail_lines:
        print(line)


if __name__ == "__main__":
    main()