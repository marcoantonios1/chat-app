package main

import (
	"fmt"
	"net/http"
	"os"

	"github.com/urfave/cli/v2"
	"github.com/marcoantonios1/chat-app/internal/server"
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

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("OK"))
	})

	http.HandleFunc("/message", server.HandleMessage)

	return http.ListenAndServe(":8080", nil)
}


