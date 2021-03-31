package main

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/go-foreman/examples/pkg/sagas/usecase"
	_ "github.com/go-foreman/examples/pkg/sagas/usecase/account"
	foreman "github.com/go-foreman/foreman"
	"github.com/go-foreman/foreman/log"
	"github.com/go-foreman/foreman/pubsub/endpoint"
	transportPackage "github.com/go-foreman/foreman/pubsub/transport/pkg"
	"github.com/go-foreman/foreman/pubsub/transport/plugins/amqp"
	"github.com/go-foreman/foreman/runtime/scheme"
	"github.com/go-foreman/foreman/saga"
	"github.com/go-foreman/foreman/saga/component"
	"github.com/go-foreman/foreman/saga/mutex"
	_ "github.com/go-sql-driver/mysql"
	"net/http"
)

const (
	queueName = "messagebus"
	topicName = "messagebus_exchange"
)

var defaultLogger = log.DefaultLogger()

func main() {
	db, err := sql.Open("mysql", "root:root@tcp(127.0.0.1:3306)/foreman?charset=utf8&parseTime=True&timeout=30s")
	handleErr(err)
	db.SetMaxOpenConns(100)
	db.SetMaxIdleConns(100)

	amqpTransport := amqp.NewTransport("amqp://admin:admin123@127.0.0.1:5672", defaultLogger)
	queue := amqp.Queue(queueName, false, false, false, false)
	topic := amqp.Topic(topicName, false, false, false, false)
	binds := amqp.QueueBind(topic.Name(), fmt.Sprintf("%s.#", topic.Name()), false)

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

	amqpEndpoint := endpoint.NewAmqpEndpoint(fmt.Sprintf("%s_endpoint", queue.Name()), amqpTransport, transportPackage.DeliveryDestination{DestinationTopic: topic.Name(), RoutingKey: fmt.Sprintf("%s.eventAndCommands", topic.Name())})

	httpMux := http.NewServeMux()

	sagaComponent := component.NewSagaComponent(
		func(scheme scheme.KnownTypesRegistry) (saga.Store, error) {
			return saga.NewMysqlSagaStore(db, scheme)
		},
		mutex.NewMysqlSqlMutex(db),
		component.WithSagaApiServer(httpMux),
	)

	sagaComponent.RegisterSagaEndpoints(amqpEndpoint)
	sagaComponent.RegisterSagas(usecase.DefaultSagasCollection.Sagas()...)
	sagaComponent.RegisterContracts(usecase.DefaultSagasCollection.Contracts()...)

	bus, err := foreman.NewMessageBus(defaultLogger, foreman.DefaultWithTransport(amqpTransport), foreman.WithSchemeRegistry(scheme.KnownTypesRegistryInstance), foreman.WithComponents(sagaComponent))

	handleErr(err)

	//messagebus is ready to be used.
	//here we load our container with all handlers, business entities etc
	loadSomeDIContainer(bus, defaultLogger)

	//start API server
	go func() {
		defaultLogger.Log(log.InfoLevel, "Started saga http server on :8080")
		defaultLogger.Log(log.FatalLevel, http.ListenAndServe(":8080", httpMux))
	}()

	//run subscriber
	defaultLogger.Log(log.FatalLevel, bus.Subscriber().Run(context.Background(), queue))
}

func handleErr(err error) {
	if err != nil {
		panic(err)
	}
}
