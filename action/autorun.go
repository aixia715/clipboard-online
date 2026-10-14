package action

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"fyne.io/systray"
)

const autoRunDesktopFile = "clipboard-online.desktop"

// autostartPath 返回 XDG 自启动 desktop 文件路径。
// os.UserConfigDir 遵守 XDG_CONFIG_HOME，KDE 原生读取该目录。
func autostartPath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, "autostart", autoRunDesktopFile), nil
}

// execPath 返回当前二进制的绝对路径
func execPath() (string, error) {
	return filepath.Abs(os.Args[0])
}

// NewAutoRunMenuItem 创建“开机启动”托盘菜单项。
// 点击勾选项后按勾选状态启用/禁用自启动，失败时回滚勾选状态。
func NewAutoRunMenuItem() (*systray.MenuItem, error) {
	checked, err := queryAutoRun()
	if err != nil {
		return nil, err
	}

	item := systray.AddMenuItemCheckbox("开机启动", "登录后自动启动 clipboard-online", checked)

	// systray 的 DBusMenu 实现点击时不自动翻转 checked 状态，
	// 因此 handler 读取当前状态、执行相反操作，成功后显式更新勾选状态。
	go func() {
		for range item.ClickedCh {
			if item.Checked() {
				if err := disableAutoRun(); err != nil {
					log.Println(err)
				} else {
					item.Uncheck()
				}
			} else {
				if err := enableAutoRun(); err != nil {
					log.Println(err)
				} else {
					item.Check()
				}
			}
		}
	}()

	return item, nil
}

// queryAutoRun 查询自启动是否开启：
// desktop 文件存在且内容含一行 Exec=<execPath> → true；文件不存在 → (false, nil)。
func queryAutoRun() (bool, error) {
	path, err := autostartPath()
	if err != nil {
		return false, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}

	execAbs, err := execPath()
	if err != nil {
		return false, err
	}

	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == "Exec="+execAbs {
			return true, nil
		}
	}
	return false, nil
}

// enableAutoRun 写入 XDG autostart desktop 文件
func enableAutoRun() error {
	path, err := autostartPath()
	if err != nil {
		return err
	}

	execAbs, err := execPath()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	content := fmt.Sprintf(`[Desktop Entry]
Type=Application
Name=clipboard-online
Comment=Share clipboard between devices
Exec=%s
Terminal=false
`, execAbs)
	return os.WriteFile(path, []byte(content), 0o644)
}

// disableAutoRun 删除 XDG autostart desktop 文件（不存在时忽略）
func disableAutoRun() error {
	path, err := autostartPath()
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
