package main

import (
	"encoding/json"
	"fmt"
	"github.com/go-foreman/examples/pkg/sagas/usecase/subscription"
	"github.com/go-foreman/examples/pkg/sagas/usecase/subscription/contracts"
	"github.com/go-foreman/foreman/pubsub/message"
	"github.com/go-foreman/foreman/runtime/scheme"
	"github.com/go-foreman/foreman/saga"
	sagaContracts "github.com/go-foreman/foreman/saga/contracts"
	"github.com/google/uuid"
	streadwayAmqp "github.com/streadway/amqp"
	"log"
)

func main() {
	conn, err := streadwayAmqp.Dial("amqp://admin:admin123@127.0.0.1:5672")
	if err != nil {
		panic(err)
	}

	defer func() {
		if err := conn.Close(); err != nil {
			panic(err)
		}
	}()

	ch, err := conn.Channel()

	if err != nil {
		panic(err)
	}

	returns := make(chan streadwayAmqp.Return, 1)
	ch.NotifyReturn(returns)

	for i := 0; i < 10; i++ {
		uid := uuid.New().String()
		registerAccountSaga := &subscription.SubscribeSaga{
			BaseSaga: saga.BaseSaga{ObjectMeta: message.ObjectMeta{
				TypeMeta: scheme.TypeMeta{
					Kind:  "SubscribeSaga",
					Group: contracts.SubscriptionGroup.String(),
				},
			}},
			Email:        fmt.Sprintf("account-%s@github.com", uid),
			Currency:     "eur",
			Amount:       float32(i * 10),
			RetriesLimit: 1,
		}
		startSagaCmd := &sagaContracts.StartSagaCommand{
			ObjectMeta: message.ObjectMeta{
				TypeMeta: scheme.TypeMeta{
					Group: "systemSaga",
					Kind:  "StartSagaCommand",
				},
			},
			SagaUID: uuid.New().String(),
			Saga:    registerAccountSaga,
		}

		msgBytes, err := json.Marshal(startSagaCmd)
		if err != nil {
			panic(err)
		}

		if err := ch.Publish(
			"messagebus_exchange",
			"messagebus_exchange.eventAndCommands",
			false,
			false,
			streadwayAmqp.Publishing{
				ContentType: "application/json",
				Body:        msgBytes,
				Headers: map[string]interface{}{
					"uid":     uuid.New().String(),
					"traceId": fmt.Sprintf("trace-%d", i),
				},
			},
		); err != nil {
			panic(err)
		}

		select {
		case r, ok := <-returns:
			if ok {
				panic(fmt.Sprintf("Message with headers %v failed to send. Code: %d. Reason: %s", r.Headers, r.ReplyCode, r.ReplyText))
			}
		default:

		}

	}

	log.Println("Finished")
}
