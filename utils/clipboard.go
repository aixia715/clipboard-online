package utils

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/png"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	_ "image/gif"
	_ "image/jpeg"

	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"
)

const (
	TypeText    = "text"
	TypeFile    = "file"
	TypeMedia   = "media"
	TypeBitmap  = "bitmap"
	TypeUnknown = "unknown"
)

// textTargets 为文本读取的候选目标，按优先级排序
var textTargets = []string{
	"text/plain;charset=utf-8",
	"text/plain",
	"UTF8_STRING",
	"STRING",
	"TEXT",
}

const clipboardTimeout = 10 * time.Second

var clipboard ClipboardService

// Clipboard returns an object that provides access to the system clipboard.
func Clipboard() *ClipboardService {
	return &clipboard
}

// ClipboardService provides access to the system clipboard.
// Linux 下通过外部工具实现：Wayland 会话用 wl-copy/wl-paste，X11 会话用 xclip。
type ClipboardService struct{}

type clipboardBackend int

const (
	backendWayland clipboardBackend = iota
	backendX11
)

// detectBackend 探测剪贴板后端：
// WAYLAND_DISPLAY 非空 → wl-clipboard；否则 DISPLAY 非空 → xclip；都没有 → 报错
func detectBackend() (clipboardBackend, error) {
	if os.Getenv("WAYLAND_DISPLAY") != "" {
		return backendWayland, nil
	}
	if os.Getenv("DISPLAY") != "" {
		return backendX11, nil
	}
	return 0, errors.New("未检测到图形会话：WAYLAND_DISPLAY 与 DISPLAY 均未设置，无法访问剪贴板")
}

// checkTools 检查后端所需的外部剪贴板工具是否已安装
func checkTools(backend clipboardBackend) error {
	if backend == backendWayland {
		if _, err := exec.LookPath("wl-copy"); err != nil {
			return errors.New("未找到 wl-copy/wl-paste，请安装：sudo pacman -S wl-clipboard")
		}
		if _, err := exec.LookPath("wl-paste"); err != nil {
			return errors.New("未找到 wl-copy/wl-paste，请安装：sudo pacman -S wl-clipboard")
		}
		return nil
	}
	if _, err := exec.LookPath("xclip"); err != nil {
		return errors.New("未找到 xclip，请安装：sudo pacman -S xclip")
	}
	return nil
}

// listTargetsCmd 列出剪贴板当前可用目标（MIME 类型）
func (b clipboardBackend) listTargetsCmd() (name string, args []string) {
	if b == backendWayland {
		return "wl-paste", []string{"--list-types"}
	}
	return "xclip", []string{"-selection", "clipboard", "-o", "-t", "TARGETS"}
}

// readTargetCmd 构造读取指定目标数据的命令
func (b clipboardBackend) readTargetCmd(target string) (name string, args []string) {
	if b == backendWayland {
		return "wl-paste", []string{"--no-newline", "--type", target}
	}
	return "xclip", []string{"-selection", "clipboard", "-o", "-t", target}
}

// writeTargetCmd 构造以指定目标类型写入剪贴板的命令
func (b clipboardBackend) writeTargetCmd(target string) (name string, args []string) {
	if b == backendWayland {
		return "wl-copy", []string{"--type", target}
	}
	return "xclip", []string{"-selection", "clipboard", "-i", "-t", target}
}

// writeTextCmd 构造写入纯文本的命令（不带类型，由工具自行推断）
func (b clipboardBackend) writeTextCmd() (name string, args []string) {
	if b == backendWayland {
		return "wl-copy", nil
	}
	return "xclip", []string{"-selection", "clipboard", "-i"}
}

// readClipboard 执行读取剪贴板的外部命令，返回 stdout
func readClipboard(name string, args []string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), clipboardTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, commandError(name, args, ctx, err, stderr.String())
	}
	return stdout.Bytes(), nil
}

// writeClipboard 将 data 写入剪贴板外部命令的 stdin。
// wl-copy/xclip 写入后会 fork 到后台进程持有选区数据，fork 出的进程会继承
// 并长期持有 stderr 的管道 fd；若 stderr 为内存缓冲，cmd.Wait 会因等待
// 拷贝结束而永久阻塞（已实测）。因此 stderr 通过临时文件传递、stdout 置空
// （/dev/null）。stdin 经实测（10MB）由前台进程在 fork 前读完，管道安全。
func writeClipboard(data []byte, name string, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), clipboardTimeout)
	defer cancel()

	stderrFile, err := os.CreateTemp("", "clipboard-online-stderr-*")
	if err != nil {
		return fmt.Errorf("failed to create temp stderr file: %w", err)
	}
	defer os.Remove(stderrFile.Name())

	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdin = bytes.NewReader(data)
	cmd.Stderr = stderrFile
	if err := cmd.Run(); err != nil {
		stderrFile.Close()
		errMsg, _ := os.ReadFile(stderrFile.Name())
		return commandError(name, args, ctx, err, string(errMsg))
	}
	stderrFile.Close()
	return nil
}

// commandError 统一命令错误信息，超时错误包装 context.DeadlineExceeded 以便调用方区分
func commandError(name string, args []string, ctx context.Context, err error, stderr string) error {
	disp := strings.TrimSpace(name + " " + strings.Join(args, " "))
	if ctx.Err() == context.DeadlineExceeded {
		return fmt.Errorf("%s 执行超时（%s）: %w", disp, clipboardTimeout, context.DeadlineExceeded)
	}
	if msg := strings.TrimSpace(stderr); msg != "" {
		return fmt.Errorf("%s failed: %s", disp, msg)
	}
	return fmt.Errorf("%s failed: %w", disp, err)
}

// parseTargets 解析目标列表输出（wl-paste 按行输出，xclip 兼容空白分隔）
func parseTargets(data []byte) []string {
	return strings.Fields(string(data))
}

// ContentType returns the type of current data of the clipboard.
// 列目标失败（空剪贴板、命令非零退出）→ (TypeUnknown, nil)，与 Windows 空剪贴板行为一致；
// 超时或环境/工具缺失 → 返回错误。
func (c *ClipboardService) ContentType() (string, error) {
	backend, err := detectBackend()
	if err != nil {
		return TypeUnknown, err
	}
	if err := checkTools(backend); err != nil {
		return TypeUnknown, err
	}

	out, err := readClipboard(backend.listTargetsCmd())
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return TypeUnknown, err
		}
		// 空剪贴板或命令非零退出
		return TypeUnknown, nil
	}
	targets := parseTargets(out)

	// 与 Windows Formats = {CF_HDROP, CF_DIBV5, CF_UNICODETEXT} 同序：文件 > 图片 > 文本
	if hasTarget(targets, "x-special/gnome-copied-files") || hasTarget(targets, "text/uri-list") {
		return TypeFile, nil
	}
	for _, t := range targets {
		if strings.HasPrefix(t, "image/") {
			return TypeBitmap, nil
		}
	}
	for _, t := range textTargets {
		if hasTarget(targets, t) {
			return TypeText, nil
		}
	}
	return TypeUnknown, nil
}

// Text returns the current text data of the clipboard.
// 按优先级选择第一个存在的文本目标读取。
func (c *ClipboardService) Text() (text string, err error) {
	backend, targets, err := c.listTargets()
	if err != nil {
		return "", err
	}
	for _, t := range textTargets {
		if !hasTarget(targets, t) {
			continue
		}
		out, err := readClipboard(backend.readTargetCmd(t))
		if err != nil {
			return "", err
		}
		return string(out), nil
	}
	return "", errors.New("剪贴板中没有文本内容")
}

// Image returns the current image data of the clipboard.
// 优先 image/png，否则第一个 image/ 前缀目标；字节经 image.Decode 解码。
func (c *ClipboardService) Image() (image.Image, error) {
	backend, targets, err := c.listTargets()
	if err != nil {
		return nil, err
	}

	target := ""
	if hasTarget(targets, "image/png") {
		target = "image/png"
	} else {
		for _, t := range targets {
			if strings.HasPrefix(t, "image/") {
				target = t
				break
			}
		}
	}
	if target == "" {
		return nil, errors.New("剪贴板中没有图片内容")
	}

	out, err := readClipboard(backend.readTargetCmd(target))
	if err != nil {
		return nil, err
	}
	img, _, err := image.Decode(bytes.NewReader(out))
	if err != nil {
		return nil, fmt.Errorf("failed to decode clipboard image (%s): %w", target, err)
	}
	return img, nil
}

// Files returns the current file paths of the clipboard.
// 优先读 x-special/gnome-copied-files，否则读 text/uri-list；仅保留 file:// URI 对应的本地路径。
func (c *ClipboardService) Files() (filenames []string, err error) {
	backend, targets, err := c.listTargets()
	if err != nil {
		return nil, err
	}

	if hasTarget(targets, "x-special/gnome-copied-files") {
		out, err := readClipboard(backend.readTargetCmd("x-special/gnome-copied-files"))
		if err != nil {
			return nil, err
		}
		return parseGnomeCopiedFiles(out), nil
	}
	if hasTarget(targets, "text/uri-list") {
		out, err := readClipboard(backend.readTargetCmd("text/uri-list"))
		if err != nil {
			return nil, err
		}
		return parseURIList(out), nil
	}
	return nil, errors.New("剪贴板中没有文件内容")
}

// listTargets 探测后端并返回当前剪贴板目标列表
func (c *ClipboardService) listTargets() (clipboardBackend, []string, error) {
	backend, err := detectBackend()
	if err != nil {
		return 0, nil, err
	}
	if err := checkTools(backend); err != nil {
		return 0, nil, err
	}
	out, err := readClipboard(backend.listTargetsCmd())
	if err != nil {
		return 0, nil, err
	}
	return backend, parseTargets(out), nil
}

// hasTarget 判断目标列表中是否含指定目标
func hasTarget(targets []string, target string) bool {
	for _, t := range targets {
		if t == target {
			return true
		}
	}
	return false
}

// parseURIList 解析 text/uri-list 数据，返回其中的本地文件路径。
// 每行一个 URI，# 开头为注释行；仅保留 file:// scheme，其余静默跳过。
func parseURIList(data []byte) []string {
	var paths []string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if p := fileURIPath(line); p != "" {
			paths = append(paths, p)
		}
	}
	return paths
}

// parseGnomeCopiedFiles 解析 x-special/gnome-copied-files 数据，返回其中的本地文件路径。
// 首行为 copy/cut 操作标记（跳过），其余每行一个 URI。
func parseGnomeCopiedFiles(data []byte) []string {
	var paths []string
	for i, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if i == 0 || line == "" {
			continue
		}
		if p := fileURIPath(line); p != "" {
			paths = append(paths, p)
		}
	}
	return paths
}

// fileURIPath 解析单个 URI，仅接受 file:// scheme，返回自动百分号解码后的本地路径
func fileURIPath(uri string) string {
	u, err := url.Parse(uri)
	if err != nil || u.Scheme != "file" {
		return ""
	}
	return u.Path
}

// buildURIList 将文件路径列表构造为 text/uri-list 内容：
// 每行一个 file:// URI，换行连接、尾部补换行。
func buildURIList(paths []string) string {
	var b strings.Builder
	for _, p := range paths {
		u := url.URL{Scheme: "file", Path: p}
		b.WriteString(u.String())
		b.WriteString("\n")
	}
	return b.String()
}

// SetText sets the current text data of the clipboard.
func (c *ClipboardService) SetText(s string) error {
	backend, err := detectBackend()
	if err != nil {
		return err
	}
	if err := checkTools(backend); err != nil {
		return err
	}
	name, args := backend.writeTextCmd()
	return writeClipboard([]byte(s), name, args)
}

// SetFiles sets the current file drop data of the clipboard.
// 构造 text/uri-list 写入（wl-copy 单类型限制下选 KDE 原生格式；
// KDE 的 KUrlMimeData/KIO 解码它，缺 x-kde-cutselection 即“复制”语义）。
// file URI 的路径必须是绝对路径（RFC 8089）：相对路径会被解析成 host
// 而非路径，导致粘贴端无法定位文件，因此写入前先转为绝对路径。
func (c *ClipboardService) SetFiles(paths []string) error {
	backend, err := detectBackend()
	if err != nil {
		return err
	}
	if err := checkTools(backend); err != nil {
		return err
	}

	absPaths := make([]string, 0, len(paths))
	for _, p := range paths {
		if abs, err := filepath.Abs(p); err == nil {
			absPaths = append(absPaths, abs)
		} else {
			absPaths = append(absPaths, p)
		}
	}

	name, args := backend.writeTargetCmd("text/uri-list")
	return writeClipboard([]byte(buildURIList(absPaths)), name, args)
}

// SetImage sets the clipboard content to an image.
// 图片编码为 PNG 后以 image/png 目标类型写入。
func (c *ClipboardService) SetImage(img image.Image) error {
	backend, err := detectBackend()
	if err != nil {
		return err
	}
	if err := checkTools(backend); err != nil {
		return err
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return fmt.Errorf("failed to encode image as png: %w", err)
	}
	name, args := backend.writeTargetCmd("image/png")
	return writeClipboard(buf.Bytes(), name, args)
}
