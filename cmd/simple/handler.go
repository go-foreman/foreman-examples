package main

import (
	"fmt"
	"github.com/go-foreman/foreman/log"
	"github.com/go-foreman/foreman/pubsub/message"
	"github.com/go-foreman/foreman/pubsub/message/execution"
	"time"
)

type SomeCommand struct {
	message.ObjectMeta        //all types must have embedded ObjectMeta
	MyID               string `json:"my_id"`
}

type SomeEvent struct {
	message.ObjectMeta           //all types must have embedded ObjectMeta
	MyID               string    `json:"my_id"`
	HandledAt          time.Time `json:"handled_at"`
}

type Handler struct {
}

func (h Handler) handleSomeCommand(execCtx execution.MessageExecutionCtx) error {
	someCmd, _ := execCtx.Message().Payload().(*SomeCommand)

	execCtx.LogMessage(log.InfoLevel, fmt.Sprintf("Just received and handled command with MyID %s", someCmd.MyID))

	return execCtx.Send(message.NewOutcomingMessage(&SomeEvent{MyID: someCmd.MyID, HandledAt: time.Now()})) //reply with an event
}

func (h Handler) handleSomeEvent(execCtx execution.MessageExecutionCtx) error {
	ev, _ := execCtx.Message().Payload().(*SomeEvent)

	execCtx.LogMessage(log.InfoLevel, fmt.Sprintf("Received event that was a response to a handled command %s at %s", ev.MyID, ev.HandledAt))

	return nil
}
