package main

import (
	"context"
	"go.uber.org/dig"
	"go.uber.org/zap"
	"os"
	"os/signal"
	"syscall"
	"time"
	"vault-injector/config"
	"vault-injector/internal/controller"
	"vault-injector/internal/http"
	"vault-injector/internal/k8s"
	"vault-injector/pkg/vault"
)

var (
	buildTime = "now"
	version   = "local_developer"
)

func main() {
	//cfg := config.GetCfg()
	ctx, cancelFunction := context.WithCancel(context.Background())
	//telega := telegram.NewTelegram(cfg)

	container := dig.New()
	container.Provide(config.GetCfg)                 //nolint:errcheck
	container.Provide(k8s.NewKubeRepo)               //nolint:errcheck
	container.Provide(k8s.NewKubeService)            //nolint:errcheck
	container.Provide(http.NewWebServer)             //nolint:errcheck
	container.Provide(controller.NewLoopController)  //nolint:errcheck
	container.Provide(controller.NewWatchController) //nolint:errcheck
	container.Provide(vault.NewVaultService)         //nolint:errcheck

	zap.S().Infof("vault-secret-syncer starting. Version: %s. (BuiltTime: %s)\n", version, buildTime)

	if err := container.Invoke(func(webServer http.WebServer) {
		webServer.Start()
	}); err != nil {
		zap.S().Fatal(err)
	}

	defer func() {
		zap.S().Info("Main Defer: canceling context")
		cancelFunction()
		time.Sleep(time.Second * 5)
	}()

	if err := container.Invoke(func(ctlList controller.List) {
		for _, ctl := range ctlList.Controllers {
			ctl.Start(ctx)
		}
	}); err != nil {
		zap.S().Fatal(err)
	}

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
	sigName := <-signals
	zap.S().Infof("Received SIGNAL - %s. Terminating...", sigName)

}