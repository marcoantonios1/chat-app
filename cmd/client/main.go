// ...existing code...
package main

import (
	"bytes"
	"os"
	"text/template"

	"github.com/marcoantonios1/chat-app/internal/client"
	"github.com/urfave/cli/v2"
)

// template for formatted error output
var errTmpl = template.Must(template.New("err").Parse(
	"ERROR: {{.Command}}{{if .ID}} [id={{.ID}}]{{end}} - {{.Message}}\n",
))

func printError(cmd, id string, err error) {
	var buf bytes.Buffer
	_ = errTmpl.Execute(&buf, map[string]string{
		"Command": cmd,
		"ID":      id,
		"Message": err.Error(),
	})
	os.Stderr.Write(buf.Bytes())
}

// main entry point for the temporal server
func main() {
	app := buildCLI()
	_ = app.Run(os.Args)
}

func buildCLI() *cli.App {
	host := os.Getenv("CHAT_SERVER_HOST")
	if host == "" {
		host = "localhost:8080"
	}

	app := cli.NewApp()
	app.Name = "chatapp"
	app.Usage = "Client for chatapp"
	app.Version = "0.1.0"
	app.ArgsUsage = " "
	app.Flags = []cli.Flag{}
	app.Commands = []*cli.Command{
		{
			Name:  "send",
			Usage: "Send message to server",
			Flags: []cli.Flag{
				&cli.StringFlag{Name: "server", Value: "ws://" + host + "/message", Usage: "websocket server URL"},
				&cli.StringFlag{Name: "message", Aliases: []string{"m"}, Usage: "message to send"},
				&cli.StringFlag{Name: "id", Aliases: []string{"i"}, Usage: "Identification"},
				&cli.StringFlag{Name: "recipient", Aliases: []string{"r"}, Usage: "Recipient ID"},
			},
			Action: func(c *cli.Context) error {
				msg := c.String("message")
				id := c.String("id")
				recipient := c.String("recipient")
				if id == "" {
					printError("send", id, cli.Exit("provide an ID with --id", 2))
					return cli.Exit("provide an ID with --id", 2)
				}
				if msg == "" {
					if c.NArg() > 0 {
						msg = c.Args().Get(0)
					} else {
						printError("send", id, cli.Exit("provide a message with --message or as argument", 2))
						return cli.Exit("provide a message with --message or as argument", 2)
					}
				}
				if err := client.SendAndReceive(c.String("server"), msg, id, recipient); err != nil {
					printError("send", id, err)
					return cli.Exit(err.Error(), 1)
				}
				return nil
			},
		},
		{
			Name:  "register",
			Usage: "register id with server",
			Flags: []cli.Flag{
				&cli.StringFlag{Name: "server", Value: "http://" + host + "/register", Usage: "http server URL"},
				&cli.StringFlag{Name: "id", Aliases: []string{"i"}, Usage: "Identification"},
			},
			Action: func(c *cli.Context) error {
				id := c.String("id")
				if id == "" {
					printError("register", id, cli.Exit("provide an ID with --id", 2))
					return cli.Exit("provide an ID with --id", 2)
				}
				if err := client.Register(c.String("server"), id); err != nil {
					if err == client.ErrIDTaken {
						printError("register", id, err)
						return cli.Exit("id already taken; choose another", 2)
					}
					printError("register", id, err)
					return cli.Exit(err.Error(), 1)
				}
				return nil
			},
		},

		{
			Name:  "recieve",
			Usage: "recieve message from server",
			Flags: []cli.Flag{
				&cli.StringFlag{Name: "server", Value: "ws://localhost:8080/message", Usage: "websocket server URL"},
			},
			Action: func(c *cli.Context) error {
				if err := client.Listen(c.String("server")); err != nil {
					printError("recieve", "", err)
					return cli.Exit(err.Error(), 1)
				}
				return nil
			},
		},
	}
	return app
}
