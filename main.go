package main

import (
	_ "embed"
	"os"
	"path/filepath"

	"fyne.io/systray"

	"github.com/YanxinTang/clipboard-online/action"
	"github.com/YanxinTang/clipboard-online/utils"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

//go:embed images/clipboard-icon.png
var trayIcon []byte

var app *Application

var execPath string
var config *Config
var mode string = "debug"
var version string = ""
var log = logrus.New()

func init() {
	execPath = filepath.Dir(os.Args[0])

	var err error
	configFilePath := filepath.Join(execPath, ConfigFile)
	config, err = loadConfig(configFilePath)
	if err != nil {
		log.WithError(err).Warn("failed to load config")
	}
	log.SetLevel(config.LogLevel)

	if mode == "debug" {
		log.SetFormatter(&logrus.TextFormatter{ForceColors: true})
	} else {
		logFilePath := filepath.Join(execPath, LogFile)
		f, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			log.WithError(err).Fatal("failed to open log file")
		}
		log.SetOutput(f)
		gin.SetMode(gin.ReleaseMode)
	}
}

func main() {
	app = NewApplication(config)
	defer app.BeforeExit()

	if err := utils.CreateDirectory(app.GetTempFilePath("")); err != nil {
		log.WithError(err).Fatal("failed to create temp directory")
	}

	log.Debug("start http server")
	app.RunHTTPServer()
	log.Debug("start app")
	// Linux uses the title as the tray ID; set it before registration so KDE can persist visibility.
	systray.SetTitle("clipboard-online")
	systray.Run(onReady, func() {})
}

func onReady() {
	systray.SetIcon(trayIcon)
	systray.SetTooltip("clipboard-online")
	if _, err := action.NewAutoRunMenuItem(); err != nil {
		log.WithError(err).Warn("failed to create auto run menu item")
	}
	systray.AddSeparator()
	action.NewExitMenuItem()
}
