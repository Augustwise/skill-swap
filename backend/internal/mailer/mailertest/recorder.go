package mailertest

import (
	"context"
	"sync"

	"skillswap/backend/internal/mailer"
)

type Recorder struct {
	mu       sync.Mutex
	messages []mailer.Message
	Err      error
}

var _ mailer.IMailer = (*Recorder)(nil)

func (r *Recorder) Send(_ context.Context, message mailer.Message) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.Err != nil {
		return r.Err
	}
	r.messages = append(r.messages, message)
	return nil
}

func (r *Recorder) Count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.messages)
}

func (r *Recorder) Last() mailer.Message {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.messages) == 0 {
		return mailer.Message{}
	}
	return r.messages[len(r.messages)-1]
}
