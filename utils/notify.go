package utils

import (
	"time"

	"github.com/godbus/dbus/v5"
)

const (
	notifyService   = "org.freedesktop.Notifications"
	notifyPath      = "/org/freedesktop/Notifications"
	notifyInterface = "org.freedesktop.Notifications"
)

// NotifyAction 表示通知上的一个按钮
type NotifyAction struct {
	Key   string // action 标识，ActionInvoked 信号返回它
	Label string // 按钮文案
}

// Notify 发送一条普通通知，超时 -1（服务器默认）
func Notify(title, body string) error {
	_, err := notifyCall(title, body, nil, -1, nil)
	return err
}

// NotifyError 发送一条错误通知：urgency=2 hint、超时 0（常驻直至关闭）
func NotifyError(title, body string) error {
	hints := map[string]dbus.Variant{"urgency": dbus.MakeVariant(byte(2))}
	_, err := notifyCall(title, body, nil, 0, hints)
	return err
}

// NotifyWithAction 发送带按钮的通知，阻塞等待用户选择。
// 返回被点击按钮的 Key；用户关闭通知或超时（通知超时 + 5s 宽限）返回 ""。
// 非本通知 id 的信号继续等待。
func NotifyWithAction(title, body string, actions []NotifyAction, timeoutMillis int32) (string, error) {
	id, err := notifyCall(title, body, actions, timeoutMillis, nil)
	if err != nil {
		return "", err
	}

	conn, err := dbus.SessionBus()
	if err != nil {
		return "", err
	}
	if err := conn.AddMatchSignal(
		dbus.WithMatchObjectPath(dbus.ObjectPath(notifyPath)),
		dbus.WithMatchInterface(notifyInterface),
	); err != nil {
		return "", err
	}
	ch := make(chan *dbus.Signal, 16)
	conn.Signal(ch)
	defer conn.RemoveSignal(ch)

	timeout := time.After(time.Duration(timeoutMillis)*time.Millisecond + 5*time.Second)
	for {
		select {
		case sig, ok := <-ch:
			if !ok {
				return "", nil
			}
			switch sig.Name {
			case notifyInterface + ".ActionInvoked":
				if len(sig.Body) >= 2 {
					if sigID, ok := sig.Body[0].(uint32); ok && sigID == id {
						if key, ok := sig.Body[1].(string); ok {
							return key, nil
						}
						return "", nil
					}
				}
			case notifyInterface + ".NotificationClosed":
				if len(sig.Body) >= 1 {
					if sigID, ok := sig.Body[0].(uint32); ok && sigID == id {
						return "", nil
					}
				}
			}
		case <-timeout:
			return "", nil
		}
	}
}

// notifyCall 调用 org.freedesktop.Notifications.Notify，返回通知 id
func notifyCall(title, body string, actions []NotifyAction, timeoutMillis int32, extraHints map[string]dbus.Variant) (uint32, error) {
	conn, err := dbus.SessionBus()
	if err != nil {
		return 0, err
	}
	obj := conn.Object(notifyService, dbus.ObjectPath(notifyPath))

	flatActions := make([]string, 0, len(actions)*2)
	for _, a := range actions {
		flatActions = append(flatActions, a.Key, a.Label)
	}
	hints := make(map[string]dbus.Variant, len(extraHints))
	for k, v := range extraHints {
		hints[k] = v
	}

	var id uint32
	err = obj.Call(notifyInterface+".Notify", 0,
		"clipboard-online", uint32(0), "", title, body,
		flatActions, hints, timeoutMillis,
	).Store(&id)
	return id, err
}
