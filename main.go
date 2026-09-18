package main

//go:generate go run packaging/windows/generate.go

import (
	"context"
	"fmt"
	"os"

	"github.com/kivutar/goro/app"
	"github.com/kivutar/goro/config"
	"github.com/kivutar/goro/glog"
	"github.com/kivutar/goro/render"
)

func main() {
	cfg, err := config.LoadConfig(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	closeLog, err := glog.Configure(cfg.Log)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer closeLog()

	game, err := app.New(cfg)
	if err != nil {
		glog.Fatalf("%v", err)
	}

if cfg.Headless {
	err = render.RunHeadless(context.Background(), game, cfg.Window)
} else {
	err = render.Run(game, cfg.Window, cfg.Render, cfg.Capture)
}
if err != nil {
	glog.Fatalf("%v", err)
}

		glog.Fatalf("%v", err)
	}
}
