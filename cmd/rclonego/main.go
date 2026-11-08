package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"github.com/DeliciousBuding/rclonego/internal/api"
	"github.com/DeliciousBuding/rclonego/internal/cli"
	"github.com/DeliciousBuding/rclonego/internal/core"
	"github.com/DeliciousBuding/rclonego/internal/engine"
	"github.com/DeliciousBuding/rclonego/internal/session"
	desktop "github.com/DeliciousBuding/rclonego/internal/ui"
)

func main() {
	runtime.GOMAXPROCS(min(runtime.NumCPU(), 4))
	args := os.Args[1:]
	if cli.IsCLI(args) || len(args) > 0 && args[0] != "serve" && (args[0] == "" || args[0][0] != '-') {
		core.AttachCLIConsole()
		os.Exit(cli.Run(args, os.Stdout, os.Stderr))
	}
	headless := len(args) > 0 && args[0] == "serve"
	if headless {
		core.AttachCLIConsole()
		args = args[1:]
	}
	flags := flag.NewFlagSet("rclonego", flag.ExitOnError)
	var dataDir, rclonePath, listen, config string
	flags.StringVar(&dataDir, "data", "", "private state directory")
	flags.StringVar(&rclonePath, "rclone", "", "rclone executable; missing engine keeps UI available")
	flags.StringVar(&listen, "api-listen", "127.0.0.1:0", "loopback API address")
	flags.StringVar(&config, "config", "", "rclone config; default is a new private application config")
	_ = flags.Parse(args)
	host, _, err := net.SplitHostPort(listen)
	if err != nil || host != "127.0.0.1" && host != "::1" && host != "localhost" {
		log.Fatal("API must listen on loopback")
	}
	if dataDir == "" {
		dataDir, err = session.DataDir()
		if err != nil {
			log.Fatal(err)
		}
	}
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		log.Fatal(err)
	}
	if !headless && !desktop.ClaimInstance(dataDir) {
		return
	}
	unlock, err := core.AcquireInstance(filepath.Join(dataDir, ".instance.lock"))
	if err != nil {
		log.Fatal("state is already managed by another instance")
	}
	defer unlock()
	listener, err := net.Listen("tcp", listen)
	if err != nil {
		log.Fatal(err)
	}
	defer listener.Close()
	if config == "" {
		config = filepath.Join(dataDir, "rclone.conf")
		if _, err := os.Stat(config); errors.Is(err, os.ErrNotExist) {
			if err := os.WriteFile(config, nil, 0o600); err != nil {
				log.Fatal(err)
			}
		}
	}
	token, err := session.Token(dataDir, "api-token")
	if err != nil {
		log.Fatal(err)
	}
	rcToken, err := session.Token(dataDir, "engine-token")
	if err != nil {
		log.Fatal(err)
	}
	if rclonePath == "" {
		if selected, err := (&engine.Manager{Dir: filepath.Join(dataDir, "engines")}).Selected(); err == nil {
			rclonePath = selected.Path
		} else {
			rclonePath, _ = exec.LookPath("rclone")
		}
	}
	store, err := core.OpenStore(dataDir)
	if err != nil {
		log.Fatal(err)
	}
	defer store.Close()
	app := core.NewApp(store, rclonePath, dataDir)
	app.ConfigureEngine(config, rcToken)
	app.Start()
	defer app.Close()
	handler, err := api.New(app, token)
	if err != nil {
		log.Fatal(err)
	}
	server := &http.Server{Handler: handler.Handler(), ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout: 15 * time.Second, WriteTimeout: 11 * time.Minute, IdleTimeout: 90 * time.Second}
	go func() {
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			log.Printf("local API: %v", err)
		}
	}()
	address := "http://" + listener.Addr().String()
	if err := session.Publish(dataDir, address); err != nil {
		log.Fatal(err)
	}
	defer os.Remove(filepath.Join(dataDir, "instance.json"))
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(ctx)
	}()
	if headless {
		fmt.Printf("RcloneGo API: %s\n", address)
		stop := make(chan os.Signal, 1)
		signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
		<-stop
		signal.Stop(stop)
		return
	}
	if err := desktop.Open(app, address); err != nil {
		log.Fatal(err)
	}
}
