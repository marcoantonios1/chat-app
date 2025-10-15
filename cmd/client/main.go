package main

import (
	"os"

	"github.com/marcoantonios1/chat-app/internal/client"
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
	app.Usage = "Client for chatapp"
	app.Version = "0.1.0"
	app.ArgsUsage = " "
	app.Flags = []cli.Flag{}
	app.Commands = []*cli.Command{
		{
			Name:  "send",
			Usage: "Send message to server",
			Flags: []cli.Flag{
				&cli.StringFlag{Name: "server", Value: "ws://localhost:8080/message", Usage: "websocket server URL"},
				&cli.StringFlag{Name: "message", Aliases: []string{"m"}, Usage: "message to send"},
				&cli.StringFlag{Name: "id", Aliases: []string{"i"}, Usage: "Identification"},
				&cli.StringFlag{Name: "recipient", Aliases: []string{"r"}, Usage: "Recipient ID"},
			},
			Action: func(c *cli.Context) error {
				msg := c.String("message")
				id := c.String("id")
				recipient := c.String("recipient")
				if id == "" {
					return cli.Exit("provide an ID with --id", 2)
				}
				if msg == "" {
					if c.NArg() > 0 {
						msg = c.Args().Get(0)
					} else {
						return cli.Exit("provide a message with --message or as argument", 2)
					}
				}
				return client.SendAndReceive(c.String("server"), msg, id, recipient)
			},
		},
		{
			Name:  "register",
			Usage: "register id with server",
			Flags: []cli.Flag{
				&cli.StringFlag{Name: "server", Value: "http://localhost:8080/register", Usage: "http server URL"},
				&cli.StringFlag{Name: "id", Aliases: []string{"i"}, Usage: "Identification"},
			},
			Action: func(c *cli.Context) error {
				id := c.String("id")
				if id == "" {
					return cli.Exit("provide an ID with --id", 2)
				}
				return client.Register(c.String("server"), id)
			},
		},

		{
			Name:  "recieve",
			Usage: "recieve message from server",
			Flags: []cli.Flag{
				&cli.StringFlag{Name: "server", Value: "ws://localhost:8080/message", Usage: "websocket server URL"},
			},
			Action: func(c *cli.Context) error {
				return client.Listen(c.String("server"))
			},
		},
	}
	return app
}
