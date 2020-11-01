package main

import (
	"encoding/json"
	"github.com/go-foreman/examples/pkg/sagas/handlers"
	"github.com/go-foreman/examples/pkg/sagas/usecase/account/contracts"
	foreman "github.com/go-foreman/foreman"
	"github.com/go-foreman/foreman/log"
	"github.com/go-foreman/foreman/pubsub/message"
	amqp2 "github.com/streadway/amqp"
	"io/ioutil"
	"os"
	"time"
)

func loadSomeDIContainer(bus *foreman.MessageBus, defaultLogger log.Logger) {
	tmpDir, err := ioutil.TempDir("", "confirmations")
	handleErr(err)
	accountHandler, err := handlers.NewAccountHandler(defaultLogger, tmpDir)
	handleErr(err)
	//here goes registration of handlers
	bus.Dispatcher().RegisterCmdHandler(&contracts.RegisterAccountCmd{}, accountHandler.RegisterAccount)
	bus.Dispatcher().RegisterCmdHandler(&contracts.SendConfirmationCmd{}, accountHandler.SendConfirmation)

	//and here we are going to run some application that will simulate user behaviour. Look into mailbox, click confirm and complete registration.
	go watchAndConfirmRegistration(tmpDir, defaultLogger)
}

func watchAndConfirmRegistration(dir string, logger log.Logger) {
	amqpConnection, err := amqp2.Dial("amqp://admin:admin123@127.0.0.1:5672")
	handleErr(err)

	amqpChannel, err := amqpConnection.Channel()
	handleErr(err)

	defer amqpConnection.Close()
	defer amqpChannel.Close()
	defer os.RemoveAll(dir)

	for {
		select {
		case <-time.After(time.Second * 4):
			files, err := ioutil.ReadDir(dir)
			handleErr(err)

			for _, info := range files {
				handleErr(err)
				filePath := dir + "/" + info.Name()
				uid, err := ioutil.ReadFile(filePath)
				handleErr(err)
				accountConfirmedEvent := &contracts.AccountConfirmed{UID: string(uid)}
				msgToDeliver := message.NewEventMessage(accountConfirmedEvent)
				msgBytes, err := json.Marshal(msgToDeliver)
				handleErr(err)
				err = amqpChannel.Publish(topicName, topicName+".confirmations", false, false, amqp2.Publishing{
					Headers: map[string]interface{}{
						"sagaId": info.Name(),
					},
					ContentType: "application/json",
					Body:        msgBytes,
				})
				handleErr(err)
				handleErr(os.Remove(filePath))
				logger.Logf(log.InfoLevel, "SagaId: %s. Sent msg that account %s confirmed", info.Name(), uid)
			}
			handleErr(err)

		}
	}
}
