package mailbox

import (
	"log/slog"
	"strings"
)

type debugWriter struct {
	mailbox *mailbox
}

func (w debugWriter) Write(b []byte) (int, error) {
	msg := string(b)
	msg = strings.ReplaceAll(msg, w.mailbox.password, "***")

	slog.Debug(msg)

	return len(b), nil
}
