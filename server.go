package main

import (
	"bufio"
	"bytes"
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"image"
	_ "image/gif"
	"image/png"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/YanxinTang/clipboard-online/utils"
	"github.com/gin-gonic/gin"
	"github.com/lxn/walk"
	"github.com/sirupsen/logrus"
	"golang.org/x/image/bmp"
)

const (
	apiVersion = "1"
)

func setupRoute(engin *gin.Engine) {
	engin.Use(clientName(), logger(), gin.Recovery(), apiVersionChecker(), auth())
	engin.GET("/", getHandler)
	engin.POST("/", setHandler)
	engin.NoRoute(notFoundHandler)
}

func clientName() gin.HandlerFunc {
	return func(c *gin.Context) {
		urlEncodedClientName := c.GetHeader("X-Client-Name")
		clientName, err := url.PathUnescape(urlEncodedClientName)
		if err != nil || clientName == "" {
			clientName = "匿名设备"
		}
		c.Set("clientName", clientName)
		c.Next()
	}
}

func apiVersionChecker() gin.HandlerFunc {
	return func(c *gin.Context) {
		version := c.GetHeader("X-API-Version")
		if version == apiVersion {
			c.Next()
			return
		}
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "接口版本不匹配，请升级您的捷径",
		})
	}
}

func auth() gin.HandlerFunc {
	return func(c *gin.Context) {

		if app.config.Authkey == "" {
			c.Next()
			return
		}

		reqAuth := c.GetHeader("X-Auth")

		timestamp := time.Now().Unix()
		timeKey := timestamp / app.config.AuthkeyExpiredTimeout

		authCodeRaw := app.config.Authkey + "." + strconv.FormatInt(timeKey, 10)
		authCodeHash := md5.Sum([]byte(authCodeRaw))
		authCodeString := hex.EncodeToString(authCodeHash[:])

		if authCodeString == reqAuth {
			c.Next()
			return
		}

		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
			"error": "操作被拒绝：Authkey 验证失败",
		})
	}
}

func logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path
		start := time.Now()
		c.Next()
		duration := time.Since(start)
		clientIP := c.ClientIP()
		statusCode := c.Writer.Status()
		clientName := c.GetString("clientName")
		requestLogger := log.WithFields(logrus.Fields{
			"method":     c.Request.Method,
			"statusCode": statusCode,
			"clientIP":   clientIP,
			"path":       path,
			"duration":   duration,
			"clientName": clientName,
		})

		if statusCode >= http.StatusInternalServerError {
			requestLogger.Error()
		} else if statusCode >= http.StatusBadRequest {
			requestLogger.Warn()
		} else {
			requestLogger.Info()
		}
	}
}

type ResponseFile struct {
	Name    string `json:"name"`
	Content string `json:"content"`
}

type ResponseFiles []ResponseFile

func getHandler(c *gin.Context) {
	contentType, err := utils.Clipboard().ContentType()
	if err != nil {
		log.WithError(err).Info("failed to get content type of clipboard")
		c.Status(http.StatusBadRequest)
		return
	}

	if contentType == utils.TypeText {
		str, err := walk.Clipboard().Text()
		if err != nil {
			c.Status(http.StatusBadRequest)
			log.WithError(err).Warn("failed to get clipboard")
			return
		}
		log.Info("get clipboard text")
		c.JSON(http.StatusOK, gin.H{
			"type": "text",
			"data": str,
		})
		defer sendCopyNotification(log, c.GetString("clientName"), str)
		return
	}

	if contentType == utils.TypeBitmap {
		bmpBytes, err := utils.Clipboard().Bitmap()
		if err != nil {
			log.WithError(err).Warn("failed to get bmp bytes from clipboard")
		}

		bmpBytesReader := bytes.NewReader(bmpBytes)
		bmpImage, err := bmp.Decode(bmpBytesReader)
		if err != nil {
			log.WithError(err).Warn("failed to decode bmp")
			c.JSON(http.StatusBadRequest, gin.H{"error": "无法获取剪切板内容"})
			return
		}
		pngBytesBuffer := new(bytes.Buffer)
		if err = png.Encode(pngBytesBuffer, bmpImage); err != nil {
			log.WithError(err).Warn("failed to encode bmp as png")
		}

		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "无法获取剪切板内容"})
			return
		}

		responseFiles := make([]ResponseFile, 0, 1)
		responseFiles = append(responseFiles, ResponseFile{
			"clipboard.png",
			base64.StdEncoding.EncodeToString(pngBytesBuffer.Bytes()),
		})

		c.JSON(http.StatusOK, gin.H{
			"type": "file",
			"data": responseFiles,
		})
		defer sendCopyNotification(log, c.GetString("clientName"), "[图片媒体] 被复制")
		return
	}

	if contentType == utils.TypeFile {
		// get path of files from clipboard
		filenames, err := utils.Clipboard().Files()
		if err != nil {
			log.WithError(err).Warn("failed to get path of files from clipboard")
			c.Status(http.StatusBadRequest)
			return
		}

		responseFiles := make([]ResponseFile, 0, len(filenames))
		for _, path := range filenames {
			base64, err := readBase64FromFile(path)
			if err != nil {
				log.WithError(err).WithField("filepath", path).Warning("read base64 from file failed")
				continue
			}
			responseFiles = append(responseFiles, ResponseFile{filepath.Base(path), base64})
		}
		log.Info("get clipboard files")

		c.JSON(http.StatusOK, gin.H{
			"type": "file",
			"data": responseFiles,
		})
		defer sendCopyNotification(log, c.GetString("clientName"), "[文件] 被复制")
		return
	}
	c.JSON(http.StatusBadRequest, gin.H{"error": "无法识别剪切板内容"})
}

func readBase64FromFile(path string) (string, error) {
	fileBytes, err := ioutil.ReadFile(path)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(fileBytes), nil
}

// Set clipboard handler

// TextBody is a struct of request body when iOS send files to windows
type TextBody struct {
	Text string `json:"data"`
}

func setHandler(c *gin.Context) {
	if !app.config.ReserveHistory {
		cleanTempFiles()
	}

	contentType := c.GetHeader("X-Content-Type")
	if contentType == utils.TypeText {
		setTextHandler(c)
		return
	}

	setFileHandler(c)
}

func setTextHandler(c *gin.Context) {
	var body TextBody
	if err := c.ShouldBindJSON(&body); err != nil {
		log.WithError(err).Warn("failed to bind text body")
		c.Status(http.StatusBadRequest)
		return
	}

	if err := utils.Clipboard().SetText(body.Text); err != nil {
		log.WithError(err).Warn("failed to set clipboard")
		c.Status(http.StatusBadRequest)
		return
	}

	var notify string = "粘贴内容为空"
	if body.Text != "" {
		notify = body.Text
	}
	defer sendPasteNotification(log, c.GetString("clientName"), notify)
	log.WithField("text", body.Text).Info("set clipboard text")
	c.Status(http.StatusOK)
}

// FileBody is a struct of request body when iOS send files to windows
type FileBody struct {
	Files []File `json:"data"`
}

// File is a struct represtents request file
type File struct {
	Name   string `json:"name"` // filename
	Base64 string `json:"base64"`
	_bytes []byte `json:"-"` // don't use this directly. use *File.Bytes() to get bytes
}

// Bytes returns byte slice of file
func (f *File) Bytes() ([]byte, error) {
	if len(f._bytes) > 0 {
		return f._bytes, nil
	}
	fileBytes, err := base64.StdEncoding.DecodeString(f.Base64)
	if err != nil {
		return []byte{}, nil
	}
	f._bytes = fileBytes
	return fileBytes, nil
}

func setFileHandler(c *gin.Context) {
	contentType := c.GetHeader("X-Content-Type")

	var body FileBody
	if err := c.ShouldBindJSON(&body); err != nil {
		log.WithError(err).Warn("failed to bind file body")
		c.Status(http.StatusBadRequest)
		return
	}

	// 对于media类型，尝试解码为图片并写入剪贴板为CF_DIB
	if contentType == utils.TypeMedia {
		setMediaHandler(c, body)
		return
	}

	paths := make([]string, 0, len(body.Files))
	for _, file := range body.Files {
		if file.Name == "-" && file.Base64 == "-" {
			continue
		}
		path := utils.LatestFilename(app.GetTempFilePath(file.Name))
		fileBytes, err := file.Bytes()
		if err != nil {
			log.WithField("filename", file.Name).Warn("failed to read file bytes")
			continue
		}
		if err := newFile(path, fileBytes); err != nil {
			log.WithError(err).WithField("path", path).Warn("failed to create file")
			continue
		}
		paths = append(paths, path)
	}

	if app.config.ReserveHistory {
		setLastFilenames(nil)
	} else {
		setLastFilenames(paths)
	}

	if err := utils.Clipboard().SetFiles(paths); err != nil {
		log.WithError(err).Warn("failed to set clipboard")
		c.Status(http.StatusBadRequest)
		return
	}

	notify := "[文件] 已复制到剪贴板"
	defer sendPasteNotification(log, c.GetString("clientName"), notify)
	log.WithField("paths", paths).Info("set clipboard file")
	c.Status(http.StatusOK)
}

// setMediaHandler 处理媒体类型：解码图片并写入剪贴板为图片格式(CF_DIB)
func setMediaHandler(c *gin.Context, body FileBody) {
	for _, file := range body.Files {
		if file.Name == "-" && file.Base64 == "-" {
			continue
		}

		fileBytes, err := file.Bytes()
		if err != nil {
			log.WithField("filename", file.Name).Warn("failed to read media file bytes")
			continue
		}

		// 同时保存文件到临时目录备份
		path := utils.LatestFilename(app.GetTempFilePath(file.Name))
		if err := newFile(path, fileBytes); err != nil {
			log.WithError(err).WithField("path", path).Warn("failed to save media file backup")
		}

		img, err := decodeMediaImage(fileBytes, file.Name)
		if err != nil {
			log.WithError(err).WithField("filename", file.Name).Warn("failed to decode media image, falling back to file mode")
			// 解码失败时回退到文件模式
			paths := []string{path}
			if err := utils.Clipboard().SetFiles(paths); err != nil {
				log.WithError(err).Warn("failed to set clipboard files (fallback)")
				c.Status(http.StatusBadRequest)
				return
			}
			notify := "[文件] 已复制到剪贴板 (图片解码失败)"
			defer sendPasteNotification(log, c.GetString("clientName"), notify)
			c.Status(http.StatusOK)
			return
		}

		if err := utils.Clipboard().SetImage(img); err != nil {
			log.WithError(err).Warn("failed to set clipboard image")
			c.Status(http.StatusBadRequest)
			return
		}

		notify := "[图片媒体] 已复制到剪贴板"
		defer sendPasteNotification(log, c.GetString("clientName"), notify)
		log.WithField("filename", file.Name).Info("set clipboard image")
		c.Status(http.StatusOK)
		return
	}

	c.Status(http.StatusBadRequest)
}

func notFoundHandler(c *gin.Context) {
	requestLogger := log.WithFields(logrus.Fields{"user_ip": c.Request.RemoteAddr})
	requestLogger.Info("404 not found")
	c.Status(http.StatusNotFound)
}

func sendCopyNotification(logger *logrus.Logger, client, notify string) {
	if app.config.Notify.Copy {
		sendNotification(logger, "复制", client, notify)
	}
}

func sendPasteNotification(logger *logrus.Logger, client, notify string) {
	if app.config.Notify.Paste {
		sendNotification(logger, "粘贴", client, notify)
	}
}

func sendNotification(logger *logrus.Logger, action, client, notify string) {
	if notify == "" {
		notify = action + "内容为空"
	}
	title := fmt.Sprintf("%s自 %s", action, client)
	if err := app.ni.ShowInfo(title, notify); err != nil {
		logger.WithError(err).WithField("notify", notify).Warn("failed to send notification")
	}
}

func setLastFilenames(filenames []string) {
	path := app.GetTempFilePath("_filename.txt")
	allFilenames := strings.Join(filenames, "\n")
	_ = ioutil.WriteFile(path, []byte(allFilenames), os.ModePerm)
}

func newFile(path string, bytes []byte) error {
	return ioutil.WriteFile(path, bytes, 0644)
}

func cleanTempFiles() {
	path := app.GetTempFilePath("_filename.txt")
	if utils.IsExistFile(path) {
		file, err := os.Open(path)
		if err != nil {
			log.WithError(err).WithField("path", path).Warn("failed to open temp file")
			return
		}
		defer file.Close()
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			delPath := scanner.Text()
			if err = os.Remove(delPath); err != nil {
				log.WithError(err).WithField("delPath", delPath).Warn("failed to delete specify path")
			}
		}
	}
}

// decodeHEIFWithExtern 使用外部 heif_converter.exe 转换 HEIF 文件为 PNG
// 创建临时输入/输出文件，调用 exe 进行转换，读取结果并用标准 PNG decoder 解码
func decodeHEIFWithExtern(fileBytes []byte, filename string) (image.Image, error) {
	// 检查 exe 文件是否存在
	heifExePath := filepath.Join(execPath, "heif_converter.exe")
	if _, err := os.Stat(heifExePath); err != nil {
		return nil, fmt.Errorf("heif_converter.exe not found: %w", err)
	}

	// 创建临时输入文件
	tempDir := os.TempDir()
	tempInputFile, err := ioutil.TempFile(tempDir, "heif_input_*.heif")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp input file: %w", err)
	}
	tempInputPath := tempInputFile.Name()
	defer os.Remove(tempInputPath)

	// 写入输入数据
	if _, err := tempInputFile.Write(fileBytes); err != nil {
		tempInputFile.Close()
		return nil, fmt.Errorf("failed to write temp input file: %w", err)
	}
	tempInputFile.Close()

	// 设置文件权限
	if err := os.Chmod(tempInputPath, 0644); err != nil {
		return nil, fmt.Errorf("failed to set input file permission: %w", err)
	}

	// 创建临时输出文件路径
	tempOutputFile, err := ioutil.TempFile(tempDir, "heif_output_*.png")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp output file: %w", err)
	}
	tempOutputPath := tempOutputFile.Name()
	tempOutputFile.Close()
	defer os.Remove(tempOutputPath)

	// 调用 heif_converter.exe 进行转换
	cmd := exec.Command(heifExePath, tempInputPath, tempOutputPath)

	// 捕获 stderr 用于错误记录
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	// 执行转换
	if err := cmd.Run(); err != nil {
		errMsg := strings.TrimSpace(stderr.String())
		return nil, fmt.Errorf("heif_converter.exe failed (exit code %v): %s", err, errMsg)
	}

	// 读取转换后的 PNG 文件
	pngBytes, err := ioutil.ReadFile(tempOutputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read converted PNG: %w", err)
	}

	// 用标准 PNG decoder 解码
	pngReader := bytes.NewReader(pngBytes)
	img, err := png.Decode(pngReader)
	if err != nil {
		return nil, fmt.Errorf("failed to decode PNG from converter: %w", err)
	}

	return img, nil
}

// decodeStdImageSafe 使用标准解码器解码图片格式（PNG、JPEG、GIF、BMP）
func decodeStdImageSafe(fileBytes []byte) (image.Image, error) {
	imgReader := bytes.NewReader(fileBytes)
	img, _, err := image.Decode(imgReader)
	if err != nil {
		return nil, fmt.Errorf("failed to decode standard image format: %w", err)
	}
	return img, nil
}

// decodeMediaImage 根据文件格式选择合适的解码器来解码图片
// 对于 HEIF 格式，使用外部 exe 转换；其他格式使用标准解码器
func decodeMediaImage(fileBytes []byte, filename string) (image.Image, error) {
	// 获取文件扩展名并转换为小写
	ext := strings.ToLower(filepath.Ext(filename))
	byteLength := len(fileBytes)

	logger := log.WithFields(logrus.Fields{
		"filename":   filename,
		"ext":        ext,
		"byteLength": byteLength,
	})

	// 检查是否为 HEIF 格式
	if ext == ".heic" || ext == ".heif" {
		logger.Info("attempting HEIF decoder via external converter (heif_converter.exe)")

		img, err := decodeHEIFWithExtern(fileBytes, filename)
		if err != nil {
			logger.WithError(err).Error("✗ external HEIF converter failed")
			return nil, err
		}

		logger.Info("✓ decoded by external HEIF converter")
		return img, nil
	}

	// 非 HEIF 文件使用标准解码器
	img, err := decodeStdImageSafe(fileBytes)
	if err != nil {
		logger.WithError(err).Error("failed to decode standard image format")
		return nil, err
	}

	logger.Info("✓ decoded by standard image decoder")
	return img, nil
}
