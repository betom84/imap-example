package mailbox

import (
	"bytes"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/mail"
	"strings"
)

type Message struct {
	base  *mail.Message
	parts map[string][]byte
}

func NewMessage(rawBody []byte) (*Message, error) {
	base, err := mail.ReadMessage(bytes.NewReader(rawBody))
	if err != nil {
		return nil, err
	}

	var parts map[string][]byte
	contentType := strings.ToLower(base.Header.Get("Content-Type"))
	switch true {
	case strings.HasPrefix(contentType, "text/plain"):
		var b []byte
		b, err = io.ReadAll(base.Body)
		parts = map[string][]byte{"text/plain": b}

	case strings.HasPrefix(contentType, "multipart/alternative"):
		parts, err = parseMultipart(base)
	}

	if err != nil {
		return nil, err
	}

	msg := Message{base, parts}

	return &msg, nil
}

func parseMultipart(base *mail.Message) (map[string][]byte, error) {
	var err error
	var parts = make(map[string][]byte)

	_, params, err := mime.ParseMediaType(base.Header.Get("Content-Type"))
	if err != nil {
		return nil, fmt.Errorf("invalid multipart message type; %w", err)
	}

	r := multipart.NewReader(base.Body, params["boundary"])
	for {
		var n *multipart.Part

		n, err = r.NextPart()
		if err == io.EOF {
			err = nil
			break
		}

		if err != nil {
			break
		}

		contentType := strings.Split(n.Header.Get("Content-Type"), ";")
		if len(contentType) < 1 {
			continue
		}

		var content io.Reader
		contentEncoding := strings.ToLower(n.Header.Get("Content-Transfer-Encoding"))
		switch contentEncoding {
		case "quoted-printable":
			content = quotedprintable.NewReader(n)
		default:
			content = n
		}

		var cb []byte
		cb, err = io.ReadAll(content)
		if err != nil {
			continue
		}

		parts[contentType[0]] = cb
	}

	return parts, err
}

func (m *Message) Subject() (string, error) {
	dec := mime.WordDecoder{}
	return dec.DecodeHeader(m.base.Header.Get("Subject"))
}

func (m *Message) PlainText() (string, error) {
	c, ok := m.parts["text/plain"]
	if !ok {
		return "", fmt.Errorf("content type not found")
	}

	return string(c), nil
}
