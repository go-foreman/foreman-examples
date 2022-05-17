package email

import (
	"context"
	"io/ioutil"
	"path"
)

type Sender struct {
	emailsDir string
}

func NewSenderService(emailsDir string) *Sender {
	return &Sender{emailsDir: emailsDir}
}

func (s Sender) Send(ctx context.Context, email string, body []byte) error {
	return ioutil.WriteFile(path.Join(s.emailsDir, email), body, 0600)
}
