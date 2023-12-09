package mailbox

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
)

type Mailbox interface {
	Connect() error
	Disconnect()
	WaitForMessages(context.Context) <-chan error
	Messages() <-chan *Message
}

type mailbox struct {
	username string
	password string
	server   string
	mailbox  string

	logger      *slog.Logger
	messages    chan *Message
	numMessages uint32

	client      *imapclient.Client
	idleCommand *imapclient.IdleCommand

	unliteralExpungeHandler chan uint32
	unliteralMailboxHandler chan imapclient.UnilateralDataMailbox
	unliteralFetchHandler   chan imapclient.FetchMessageData
}

func New(server, username, password, folder string) Mailbox {
	return &mailbox{
		username: username,
		password: password,
		server:   server,
		mailbox:  folder,
		logger:   slog.Default().With(slog.String("mailbox", folder)),
		messages: make(chan *Message),
	}
}

func (m *mailbox) Connect() error {
	client, err := imapclient.DialTLS(m.server, &imapclient.Options{
		DebugWriter: debugWriter{m},
		UnilateralDataHandler: &imapclient.UnilateralDataHandler{
			Expunge: func(seqNum uint32) {
				slog.Info("received unilateral expunge command", slog.Int("seqNum", int(seqNum)))

				if m.unliteralExpungeHandler != nil {
					m.unliteralExpungeHandler <- seqNum
				}
			},
			Mailbox: func(data *imapclient.UnilateralDataMailbox) {
				if m.unliteralMailboxHandler != nil {
					m.unliteralMailboxHandler <- *data
				}
			},
			Fetch: func(msg *imapclient.FetchMessageData) {
				if m.unliteralFetchHandler != nil {
					m.unliteralFetchHandler <- *msg
				}
			},
		},
	})

	if err != nil {
		return err
	}

	err = client.Login(m.username, m.password).Wait()
	if err != nil {
		client.Close()
		return err
	}

	data, err := client.Select(m.mailbox, nil).Wait()
	if err != nil {
		listMailboxes(client)
		client.Close()
		return err
	}

	m.client = client
	m.numMessages = data.NumMessages

	caps := m.client.Caps()
	m.logger.Info("connected",
		slog.Bool("supportsIMAP4rev1", caps.Has(imap.CapIMAP4rev1)),
		slog.Bool("supportsIMAP4rev2", caps.Has(imap.CapIMAP4rev2)),
		slog.Bool("supportsIdle", caps.Has(imap.CapIdle)),
		slog.Int("UIDNext", int(data.UIDNext)),
		slog.Int("numMessages", int(data.NumMessages)),
	)

	return nil
}

func (m *mailbox) WaitForMessages(ctx context.Context) <-chan error {
	done := make(chan error)

	go func() {
		defer close(done)

		if m.unliteralMailboxHandler == nil {
			m.unliteralMailboxHandler = make(chan imapclient.UnilateralDataMailbox)
			defer close(m.unliteralMailboxHandler)
		}

		var err error
		var recoverErr error

		for {
			if err != nil {
				if recoverErr == err {
					done <- fmt.Errorf("unrecoverable error; %w", recoverErr)
					return
				}

				slog.Error("error while waiting for messages, try to recover", slog.String("error", err.Error()))
				recoverErr = err
			}

			var idle *imapclient.IdleCommand
			idle, err = m.client.Idle()
			if err != nil {
				continue
			}

			var md imapclient.UnilateralDataMailbox

			select {
			case <-ctx.Done():
				return
			case md = <-m.unliteralMailboxHandler:
				if md.NumMessages == nil {
					continue
				}

				slog.Info("mailbox changed", slog.Int("numMessages", int(*md.NumMessages)))
				break
			}

			err = idle.Close()
			if err != nil {
				continue
			}

			err = idle.Wait()
			if err != nil {
				continue
			}

			if m.numMessages < *md.NumMessages {
				var seq imap.SeqSet
				seq, err = imap.ParseSeqSet(fmt.Sprintf("%d:%d", m.numMessages+1, *md.NumMessages))
				if err != nil {
					continue
				}

				err = m.fetch(seq)
				if err != nil {
					continue
				}
			}

			recoverErr = nil
			m.numMessages = *md.NumMessages
		}
	}()

	return done
}

func (m *mailbox) fetch(seq imap.SeqSet) error {
	fetchCmd := m.client.Fetch(seq, &imap.FetchOptions{
		BodyStructure: &imap.FetchItemBodyStructure{Extended: false},
		BodySection: []*imap.FetchItemBodySection{
			{},
		},
		Envelope:     true,
		Flags:        true,
		InternalDate: true,
		RFC822Size:   true,
		UID:          false,
		ModSeq:       false,
		ChangedSince: 0,
	})

	messages, err := fetchCmd.Collect()
	if err != nil {
		return err
	}

	for _, message := range messages {
		var body []byte
		for sec, val := range message.BodySection {
			if sec.Specifier == imap.PartSpecifierNone {
				body = val
			}
		}

		msg, err := NewMessage(body)
		if err != nil {
			slog.Error("failed to parse message", slog.String("error", err.Error()), slog.Int("seqNum", int(message.SeqNum)), slog.Int("len", len(body)))
			continue
		}

		m.messages <- msg
	}

	return nil
}

func (m *mailbox) Messages() <-chan *Message {
	return m.messages
}

func (m *mailbox) Disconnect() {
	if m.client != nil {
		m.client.Close()
		m.client = nil
	}

	close(m.messages)
}

func listMailboxes(client *imapclient.Client) error {
	listCmd := client.List("", "*", nil)
	err := listCmd.Wait()
	if err != nil {
		return err
	}

	mailboxes, err := listCmd.Collect()
	if err != nil {
		return err
	}

	slog.Info("available mailboxes:")
	for _, m := range mailboxes {
		var attrs []slog.Attr
		if m.Status != nil {
			attrs = []slog.Attr{
				slog.Int("messages", int(*m.Status.NumMessages)),
				slog.Int("unseen", int(*m.Status.NumUnseen)),
			}
		}

		slog.LogAttrs(context.Background(), slog.LevelInfo, m.Mailbox, attrs...)
	}

	return nil
}
