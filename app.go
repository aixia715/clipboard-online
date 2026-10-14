package main

import (
	"path"
	"path/filepath"
	"sync"

	"fyne.io/systray"
	"github.com/YanxinTang/clipboard-online/utils"
	"github.com/gin-gonic/gin"
)

type Application struct {
	config *Config
	wg     sync.WaitGroup
}

func (app *Application) RunHTTPServer() {
	app.wg.Add(1)
	go func() {
		engin := gin.New()
		setupRoute(engin)
		if err := engin.Run(":" + app.config.Port); err != nil {
			utils.NotifyError("HTTP Server 启动失败", "端口 "+app.config.Port+" 可能被占用")
			systray.Quit()
			log.WithError(err).Error("failed to start http server")
			return
		}
	}()
}

func (app *Application) StopHTTPServer() {
	app.wg.Done()
}

func (app *Application) BeforeExit() {
	app.StopHTTPServer()
}

func (app *Application) GetTempFilePath(filename string) string {
	if !filepath.IsAbs(app.config.TempDir) {
		// temp files path in exec path but not pwd
		tempAbsPath := path.Join(execPath, app.config.TempDir)
		return filepath.Join(tempAbsPath, filename)
	}
	return filepath.Join(app.config.TempDir, filename)
}

func NewApplication(config *Config) *Application {
	app := new(Application)
	app.config = config
	return app
}
