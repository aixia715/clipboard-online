#!/usr/bin/env python3
"""
Windows Toast 通知工具
显示带按钮的 Toast 通知，根据用户选择返回不同的退出码
"""

import sys
import threading


def main():
    if len(sys.argv) < 3:
        print("用法: toast_notifier.exe <title> <message>", file=sys.stderr)
        sys.exit(1)

    title = sys.argv[1]
    message = sys.argv[2]

    try:
        from windows_toasts import InteractableWindowsToaster, Toast, ToastActivatedEventArgs, ToastButton
    except ImportError:
        print("错误: 缺少 windows-toasts 库。请运行: pip install windows-toasts", file=sys.stderr)
        sys.exit(1)

    exit_code = 1  # 默认：保留原文件
    event = threading.Event()

    def on_activated(args: ToastActivatedEventArgs):
        nonlocal exit_code
        if args.arguments == "convert":
            exit_code = 0
        else:
            exit_code = 1
        event.set()

    def on_dismissed(_):
        event.set()

    def on_failed(_):
        event.set()

    toaster = InteractableWindowsToaster("clipboard-online")

    toast = Toast([title, message])
    toast.AddAction(ToastButton("是", "convert"))
    toast.AddAction(ToastButton("否", "keep"))
    toast.on_activated = on_activated
    toast.on_dismissed = on_dismissed
    toast.on_failed = on_failed

    toaster.show_toast(toast)

    # 等待用户响应，超时 30 秒
    event.wait(timeout=30)

    if not event.is_set():
        # 超时，清除通知
        toaster.clear_toasts()

    sys.exit(exit_code)


if __name__ == "__main__":
    main()
