package action

import (
	"fyne.io/systray"
)

// NewExitMenuItem 创建“退出”托盘菜单项
func NewExitMenuItem() *systray.MenuItem {
	item := systray.AddMenuItem("退出", "退出 clipboard-online")
	go func() {
		<-item.ClickedCh
		systray.Quit()
	}()
	return item
}
