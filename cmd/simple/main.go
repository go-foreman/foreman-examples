package main

import (
	"context"
	"fmt"
	foreman "github.com/go-foreman/foreman"
	"github.com/go-foreman/foreman/log"
	"github.com/go-foreman/foreman/pubsub/endpoint"
	"github.com/go-foreman/foreman/pubsub/message"
	"github.com/go-foreman/foreman/pubsub/transport"
	foremanAmqp "github.com/go-foreman/foreman/pubsub/transport/amqp"
	"github.com/go-foreman/foreman/runtime/scheme"
	"time"
)

const (
	queueName = "messagebus"
	topicName = "messagebus_exchange"
)

var defaultLogger = log.DefaultLogger()

func main() {
	amqpTransport := foremanAmqp.NewTransport("amqp://admin:admin123@127.0.0.1:5672", defaultLogger)
	queue := foremanAmqp.Queue(queueName, false, false, false, false)
	topic := foremanAmqp.Topic(topicName, false, false, false, false)
	binds := foremanAmqp.QueueBind(topic.Name(), fmt.Sprintf("%s.#", topic.Name()), false)

	ctx := context.Background()

	if err := amqpTransport.Connect(ctx); err != nil {
		defaultLogger.Logf(log.ErrorLevel, "Error connecting to amqp. %s", err)
		panic(err)
	}

	if err := amqpTransport.CreateTopic(ctx, topic); err != nil {
		defaultLogger.Logf(log.ErrorLevel, "Error creating topic %s. %s", topic.Name(), err)
		panic(err)
	}

	if err := amqpTransport.CreateQueue(ctx, queue, binds); err != nil {
		defaultLogger.Logf(log.ErrorLevel, "Error creating queue %s. %s", queue.Name(), err)
		panic(err)
	}

	schemeRegistry := scheme.KnownTypesRegistryInstance
	marshaller := message.NewJsonMarshaller(schemeRegistry)

	amqpEndpoint := endpoint.NewAmqpEndpoint(
		fmt.Sprintf("%s_endpoint", queue.Name()),
		amqpTransport,
		transport.DeliveryDestination{
			DestinationTopic: topic.Name(),
			RoutingKey:       fmt.Sprintf("%s.eventAndCommands", topic.Name()),
		},
		marshaller,
	)

	bus, err := foreman.NewMessageBus(
		defaultLogger,
		marshaller,
		schemeRegistry,
		foreman.DefaultWithTransport(amqpTransport),
	)

	handleErr(err)

	//here all registrations are happening...

	//all types that go through message bus must be registered in schema
	bus.SchemeRegistry().AddKnownTypes(scheme.Group("some"), &SomeCommand{}, &SomeEvent{})

	//subscribe handler for its command and event
	h := &Handler{}
	bus.Dispatcher().SubscribeForCmd(&SomeCommand{}, h.handleSomeCommand)
	bus.Dispatcher().SubscribeForEvent(&SomeEvent{}, h.handleSomeEvent)

	//subscribe both messages for amqp endpoint, so execution context will know where to send replies with these types
	bus.Router().RegisterEndpoint(amqpEndpoint, &SomeEvent{}, &SomeCommand{})

	// normally this won't be part of your code, it's just to constantly generate some commands and send them into the message bus
	go func() {
		defaultLogger.Log(log.InfoLevel, "simulation will start in 3 sec")
		time.Sleep(time.Second * 3)
		simulation(ctx, defaultLogger, amqpEndpoint)
	}()

	defaultLogger.Log(log.FatalLevel, bus.Subscriber().Run(ctx, queue))
}

func handleErr(err error) {
	if err != nil {
		panic(err)
	}
}

func simulation(ctx context.Context, logger log.Logger, amqpEndpoint endpoint.Endpoint) {
	i := 0
	for {
		select {
		case <-ctx.Done():
			logger.Log(log.WarnLevel, "ctx was canceled, stopping simulation")
			return
		case <-time.After(time.Millisecond * 500):
			if err := amqpEndpoint.Send(ctx, message.NewOutcomingMessage(&SomeCommand{
				MyID: fmt.Sprintf("myid-%d", i),
			})); err != nil {
				panic(err)
			}
			i++

		}
	}
}
