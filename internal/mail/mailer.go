package mail

import (
	"context"
	"fmt"
	"net/smtp"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/ayse-solmaz/azula/internal/config"
)

type Mailer interface {
	Send(ctx context.Context, to, subject, body string) error
}

type Memory struct {
	mu       sync.Mutex
	Messages []Message
}

type Message struct {
	To, Subject, Body string
	At                time.Time
}

func (m *Memory) Send(_ context.Context, to, subject, body string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Messages = append(m.Messages, Message{To: to, Subject: subject, Body: body, At: time.Now().UTC()})
	return nil
}

type FileMailer struct {
	dir string
}

func NewFile(dir string) *FileMailer {
	return &FileMailer{dir: dir}
}

func (f *FileMailer) Send(_ context.Context, to, subject, body string) error {
	if err := os.MkdirAll(f.dir, 0o750); err != nil {
		return err
	}
	safe := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '@' || r == '.' || r == '-' {
			return r
		}
		return '_'
	}, to)
	name := fmt.Sprintf("%d-%s.eml", time.Now().UTC().UnixNano(), safe)
	msg := fmt.Sprintf("To: %s\r\nSubject: %s\r\nContent-Type: text/plain; charset=utf-8\r\n\r\n%s\n", to, subject, body)
	return os.WriteFile(filepath.Join(f.dir, name), []byte(msg), 0o640)
}

type SMTPMailer struct {
	host, port, user, pass, from string
}

func (s *SMTPMailer) Send(_ context.Context, to, subject, body string) error {
	addr := s.host + ":" + s.port
	msg := []byte("To: " + to + "\r\nSubject: " + subject + "\r\nContent-Type: text/plain; charset=utf-8\r\n\r\n" + body)
	var auth smtp.Auth
	if s.user != "" {
		auth = smtp.PlainAuth("", s.user, s.pass, s.host)
	}
	return smtp.SendMail(addr, auth, s.from, []string{to}, msg)
}

type Chain []Mailer

func (c Chain) Send(ctx context.Context, to, subject, body string) error {
	var last error
	ok := false
	for _, m := range c {
		if m == nil {
			continue
		}
		if err := m.Send(ctx, to, subject, body); err != nil {
			last = err
			continue
		}
		ok = true
	}
	if ok {
		return nil
	}
	if last != nil {
		return last
	}
	return fmt.Errorf("no mailer configured")
}

func New(cfg config.Config) Mailer {
	var chain Chain
	if cfg.SMTPHost != "" {
		chain = append(chain, &SMTPMailer{
			host: cfg.SMTPHost, port: cfg.SMTPPort, user: cfg.SMTPUser, pass: cfg.SMTPPass, from: cfg.SMTPFrom,
		})
	}
	if cfg.MailOutboxDir != "" {
		chain = append(chain, NewFile(cfg.MailOutboxDir))
	}
	if len(chain) == 0 {
		return NewFile("./data/outbox")
	}
	return chain
}
