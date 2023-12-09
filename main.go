package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"

	"github.com/betom84/imap-example/mailbox"
)

var (
	username = flag.String("username", "", "IMAP username")
	password = flag.String("password", "", "IMAP password")
	server   = flag.String("server", "", "IMAP server")
)

func main() {
	flag.Parse()
	if username == nil || password == nil || server == nil {
		panic(fmt.Errorf("unsufficient parameters"))
	}

	initLogger()

	sig := make(chan os.Signal)
	signal.Notify(sig, os.Interrupt, os.Kill)

	mb := mailbox.New(*server, *username, *password, "inbox")
	err := mb.Connect()
	if err != nil {
		panic(err)
	}
	defer mb.Disconnect()

	waitCtx, idleCancel := context.WithCancel(context.Background())
	waitStopped := mb.WaitForMessages(waitCtx)

	fmt.Println("Waiting for new messages..")

	for {
		select {
		case s := <-sig:
			slog.Info(fmt.Sprintf("signal %s recieved, shutting down\n", s))
			idleCancel()

		case err := <-waitStopped:
			if err != nil {
				slog.Error("stopped waiting for messages with error", slog.String("error", err.Error()))
			} else {
				slog.Info("stopped waiting for messages")
			}

			return

		case m := <-mb.Messages():
			subject, _ := m.Subject()
			body, _ := m.PlainText()

			fmt.Println("You've got a messages!")
			fmt.Printf("Subject:\n%q\nBody:\n%q", subject, body)
		}
	}
}

func initLogger() {
	logger := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		AddSource: false,
		Level:     slog.LevelDebug,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			return a
		},
	})

	slog.SetDefault(slog.New(logger))
}
