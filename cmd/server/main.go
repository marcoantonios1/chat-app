package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/marcoantonios1/chat-app/internal/server"
	"github.com/urfave/cli/v2"
)

// main entry point for the temporal server
func main() {
	app := buildCLI()
	_ = app.Run(os.Args)
}

func buildCLI() *cli.App {
	app := cli.NewApp()
	app.Name = "chatapp"
	app.Usage = "Server for chatapp"
	app.Version = "0.1.0"
	app.ArgsUsage = " "
	app.Flags = []cli.Flag{}
	app.Commands = []*cli.Command{
		{
			Name:  "start",
			Usage: "Start the chat server",
			Action: func(c *cli.Context) error {
				return startServer()
			},
		},
	}
	return app
}

func startServer() error {
    fmt.Println("🚀 Starting chat server on :8080...")

	go server.RunHub()

    mux := http.NewServeMux()
    mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("OK")) })
    mux.HandleFunc("/message", server.HandleMessage)
    mux.HandleFunc("/register", server.HandleRegister)

    srv := &http.Server{Addr: ":8080", Handler: mux}

    // graceful shutdown
    idleConnsClosed := make(chan struct{})
    go func() {
        sig := make(chan os.Signal, 1)
        signal.Notify(sig, os.Interrupt)
        <-sig
        ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
        defer cancel()
        _ = srv.Shutdown(ctx)
        close(idleConnsClosed)
    }()

    if err := srv.ListenAndServe(); err != http.ErrServerClosed {
        return err
    }
    <-idleConnsClosed
    return nil
}
