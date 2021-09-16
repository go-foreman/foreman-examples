package main

import (
	"encoding/json"
	"fmt"
	"github.com/go-foreman/examples/pkg/sagas/usecase/account"
	"github.com/go-foreman/examples/pkg/sagas/usecase/account/contracts"
	"github.com/go-foreman/foreman/pubsub/message"
	"github.com/go-foreman/foreman/runtime/scheme"
	"github.com/go-foreman/foreman/saga"
	sagaContracts "github.com/go-foreman/foreman/saga/contracts"
	"github.com/google/uuid"
	streadwayAmqp "github.com/streadway/amqp"
)

func main() {
	conn, err := streadwayAmqp.Dial("amqp://admin:admin123@127.0.0.1:5672")
	if err != nil {
		panic(err)
	}

	ch, err := conn.Channel()

	if err != nil {
		panic(err)
	}
	for i := 0; i < 1000; i++ {
		uid := uuid.New().String()
		registerAccountSaga := &account.RegisterAccountSaga{
			BaseSaga: saga.BaseSaga{ObjectMeta: message.ObjectMeta{
				TypeMeta: scheme.TypeMeta{
					Kind:  "RegisterAccountSaga",
					Group: contracts.AccountGroup.String(),
				},
			}},
			UID:          uid,
			Email:        fmt.Sprintf("account-%s@github.com", uid),
			RetriesLimit: 1,
		}
		startSagaCmd := &sagaContracts.StartSagaCommand{
			ObjectMeta: message.ObjectMeta{
				TypeMeta: scheme.TypeMeta{
					Group: "systemSaga",
					Kind: "StartSagaCommand",
				},
			},
			SagaUID:   uuid.New().String(),
			Saga:     registerAccountSaga,
		}

		msgBytes, err := json.Marshal(startSagaCmd)
		if err != nil {
			panic(err)
		}

		err = ch.Publish(
			"messagebus_exchange",
			"messagebus_exchange.eventAndCommands",
			false,
			false,
			streadwayAmqp.Publishing{
				ContentType: "application/json",
				Body:        msgBytes,
				Headers: map[string]interface{}{
					"uid": uuid.New().String(),
					"traceId": "sometraceid",
				},
			},
		)
		if err != nil {
			panic(err)
		}
	}
}
