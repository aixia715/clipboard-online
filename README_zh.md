<div align="center">
  <img src="https://raw.githubusercontent.com/YanxinTang/clipboard-online/master/images/clipboard-icon.png" style="display: inline-block; vertical-align: middle;">
  <h1 style="display: inline-block; vertical-align: middle;">clipboard-online</h1>
</div>

![GitHub release (latest by date)](https://img.shields.io/github/v/release/YanxinTang/clipboard-online)

clipboard-online 是一款可以帮你在 💻Linux（KDE 桌面）和 📱iOS/Android 之间分享剪切板的应用

## 文档

【[English](https://github.com/YanxinTang/clipboard-online/blob/master/README.md)】【[中文](https://github.com/YanxinTang/clipboard-online/blob/master/README_zh.md)】

## 下载

目前仅支持源码编译。

构建之前，请确保你已经安装：

- golang（≥ 1.21）。如果没有，可能你需要[这个](https://golang.org/dl/)
- KDE Plasma 桌面会话（Wayland 与 X11 会话均支持）
- 剪贴板工具：`sudo pacman -S wl-clipboard xclip`（Wayland 会话使用 wl-clipboard，X11 会话使用 xclip）
- 可选：`sudo pacman -S libheif` 启用 HEIC 照片解码

构建步骤：

- `git clone git@github.com:YanxinTang/clipboard-online.git`
- `cd clipboard-online`
- `./build.sh`
- 你可以在 `release` 目录下找到可执行文件（`release/clipboard-online`）

## 使用

KDE 系统托盘使用固定标识 `clipboard-online` 保存显示偏好。若从使用 `systray_<PID>` 标识的旧版本升级，请在重启应用后重新设置一次 `Configure System Tray` → `clipboard-online` → `Show only in popup`；后续重启会沿用该设置。

### iOS 用户

1. 在电脑上运行 `clipboard-online`（托盘图标可能收纳在系统托盘的折叠区）
2. iPhone 或 iPad 上安装快捷指令 （在 safari 中打开链接）
    - Copy:
      - iCloud: [https://www.icloud.com/shortcuts/f463a1e431c94c60b8a5c65305eb819f](https://www.icloud.com/shortcuts/f463a1e431c94c60b8a5c65305eb819f)
      - ![复制](./images/copy.png)
    - Paste:
      - iCloud: [https://www.icloud.com/shortcuts/90e7a2af70df4707a17dece8c263afc5](https://www.icloud.com/shortcuts/90e7a2af70df4707a17dece8c263afc5)
      - ![粘贴](./images/paste.png)

3. 设置 ip 地址和 authkey （默认是空字符串）。参考 [https://www.kejiwanjia.com/jiaocheng/zheteng/83030.html](https://www.kejiwanjia.com/jiaocheng/zheteng/83030.html)。
4. 玩的开心...😊

### Android 用户

1. HTTP Shortcuts: [https://meta.appinn.net/t/topic/20322](https://meta.appinn.net/t/topic/20322)

2. Tasker: [https://github.com/YanxinTang/clipboard-online/issues/26](https://github.com/YanxinTang/clipboard-online/issues/26)

非常感谢 [@xiozf](https://github.com/xiozf)，[@Befod](https://github.com/Befod) 及其他贡献者，谢谢你们为 Android 平台提供的支持！

## 配置

`clipboard-online` 将在运行路径下面创建两个文件： `config.json` and `log.txt`

你可以通过修改 `config.json` 来自定义配置

### `config.json`

- `port`
  - 类型: `string`
  - 默认: `"8086"`

- `logLevel`
  - 类型: `string`
  - 默认: `"warning"`
  - 可选: `"panic"`, `"fatal"`, `"error"`, `"warning"`, `"info"`, `"debug"`, `"trace"`

- `authkey`
  - type: `string`
  - default: `''`

- `authkeyExpiredTimeout`
  - type: `int64`
  - default: `30`

- `tempDir`
  - type: `string`
  - default: `./temp`

- `reserveHistory`
  - type: `Boolean`
  - default: `false`

- `notify`
  - type: `object`
  - children:
    - `copy`
      - type: `Bollean`
      - default: `false`
    - `paste`
      - type: `Boolean`
      - default: `false`

## API

### 公共 headers

#### 必选

- `X-API-Version`: indicates version of api

#### 可选

- `X-Client-Name`: indicates name of device
- `X-Auth`: hashed authkey. Value from `md5(config.authkey + timestamp/30)`

### 1. 获取 Windows 剪切板

> Request

- URL: `/`
- Method: `GET`

> Reponse

- Body: `json`

```json
// 200 ok

{
  "type": "text",
  "data": "clipboard text on the server"
}

{
  "type": "file",
  "data": [
    {
      "name": "filename",
      "content": "base64 string of file bytes"
    }
    ...
  ]
}

```

### 2. 设置 Windows 剪切板

> Request

- URL: `/`
- Method: `POST`
- Headers:
  - `X-Content-Type`: indicates type of request body content
    - `required`
    - values: `text`, `file`, `media`

- Body: `json`

For text:

```json
{
  "data": "text you want to set"
}
```

For file:

```json
{
  "data": [
    {
      "name": "filename",
      "base64": "base64 string of file bytes"
    }
  ]
}
```

响应的 body 为空。如果剪切板设置成功，状态码将返回 `200`
