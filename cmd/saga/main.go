package main

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/go-foreman/foreman/pubsub/subscriber"
	sagaSql "github.com/go-foreman/foreman/saga/sql"

	"io/ioutil"
	"net/http"

	emailHandler "github.com/go-foreman/examples/pkg/sagas/handlers/email"
	paymentHandler "github.com/go-foreman/examples/pkg/sagas/handlers/payment"
	userHandler "github.com/go-foreman/examples/pkg/sagas/handlers/user"
	"github.com/go-foreman/examples/pkg/sagas/usecase"
	"github.com/go-foreman/examples/pkg/services/email"
	"github.com/go-foreman/examples/pkg/services/payment"
	"github.com/go-foreman/examples/pkg/services/user"
	foreman "github.com/go-foreman/foreman"
	"github.com/go-foreman/foreman/log"
	"github.com/go-foreman/foreman/pubsub/endpoint"
	"github.com/go-foreman/foreman/pubsub/message"
	"github.com/go-foreman/foreman/pubsub/transport"
	foremanAmqp "github.com/go-foreman/foreman/pubsub/transport/amqp"
	"github.com/go-foreman/foreman/runtime/scheme"
	"github.com/go-foreman/foreman/saga"
	"github.com/go-foreman/foreman/saga/component"
	"github.com/go-foreman/foreman/saga/mutex"
	_ "github.com/go-sql-driver/mysql"

	_ "github.com/go-foreman/examples/pkg/sagas/usecase/subscription"
)

const (
	queueName = "messagebus"
	topicName = "messagebus_exchange"
)

var defaultLogger = log.DefaultLogger()

func main() {
	defaultLogger.SetLevel(log.InfoLevel)
	db, err := sql.Open("mysql", "root:root@tcp(127.0.0.1:3307)/foreman?charset=utf8&parseTime=True&timeout=30s")
	handleErr(err)
	db.SetMaxOpenConns(5)
	db.SetMaxIdleConns(5)

	sagaSqlWrapper := sagaSql.NewDB(db)

	amqpTransport := foremanAmqp.NewTransport("amqp://admin:admin123@127.0.0.1:5673", defaultLogger)
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

	amqpEndpoint := endpoint.NewAmqpEndpoint(fmt.Sprintf("%s_endpoint", queue.Name()), amqpTransport, transport.DeliveryDestination{DestinationTopic: topic.Name(), RoutingKey: fmt.Sprintf("%s.eventAndCommands", topic.Name())}, marshaller)

	httpMux := http.NewServeMux()

	sagaComponent := component.NewSagaComponent(
		func(scheme message.Marshaller) (saga.Store, error) {
			return saga.NewSQLSagaStore(sagaSqlWrapper, saga.MYSQLDriver, scheme)
		},
		mutex.NewSqlMutex(sagaSqlWrapper, saga.MYSQLDriver, defaultLogger),
		component.WithSagaApiServer(httpMux),
	)

	sagaComponent.RegisterSagaEndpoints(amqpEndpoint)
	sagaComponent.RegisterSagas(usecase.DefaultSagasCollection.Sagas()...)
	sagaComponent.RegisterContracts(usecase.DefaultSagasCollection.Contracts()...)

	sConfig := subscriber.DefaultConfig
	sConfig.WorkersCount = 100
	sConfig.PackageProcessingMaxTime = time.Second * 300

	bus, err := foreman.NewMessageBus(defaultLogger, marshaller, schemeRegistry, foreman.DefaultSubscriber(amqpTransport, subscriber.WithConfig(&sConfig)), foreman.WithComponents(sagaComponent))
	handleErr(err)

	//messagebus is ready to be used.
	//here we create services, handlers and inside of handler we will subscribe for commands
	provisionHandlers(bus)

	//start API server
	go func() {
		defaultLogger.Log(log.InfoLevel, "Started saga http server on :8080")
		defaultLogger.Log(log.FatalLevel, http.ListenAndServe(":8080", httpMux))
	}()

	//run subscriber
	defaultLogger.Log(log.FatalLevel, bus.Subscriber().Run(context.Background(), queue))
}

func provisionHandlers(bus *foreman.MessageBus) {
	userService := user.NewUserService()
	invoicingService := payment.NewInvoicingService()

	emailsDir, err := ioutil.TempDir("", "emails")
	handleErr(err)

	senderService := email.NewSenderService(emailsDir)

	userHandler.NewHandler(bus, userService)
	paymentHandler.NewHandler(bus, invoicingService)
	emailHandler.NewHandler(bus, senderService, userService, invoicingService)
}

func handleErr(err error) {
	if err != nil {
		panic(err)
	}
}
