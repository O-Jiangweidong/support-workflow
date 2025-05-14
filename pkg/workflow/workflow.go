package workflow

import (
	"errors"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"support-workflow/pkg/config"
)

var (
	configPath = ""
)

func RunForever() {
	flag.StringVar(&configPath, "f", "config.yml", "config.yml path")

	config.Setup(configPath)
	httpServer := NewHttpServer()
	taskManager := &TaskManager{}

	go func() {
		if err := httpServer.Start(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("HTTP服务器启动失败: %v", err)
		}
	}()

	go taskManager.StartTasks()

	gracefulStop := make(chan os.Signal, 1)
	signal.Notify(gracefulStop, syscall.SIGTERM, syscall.SIGINT, syscall.SIGQUIT)
	<-gracefulStop
	log.Println("接收到终止信号，开始优雅关闭...")

	taskManager.Stop()
	httpServer.Stop()

	log.Println("所有服务已优雅关闭")
}
