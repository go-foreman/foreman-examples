package subscription

import (
	"time"

	"github.com/go-foreman/foreman/pubsub/endpoint"
	sagaContracts "github.com/go-foreman/foreman/saga/contracts"

	"github.com/go-foreman/examples/pkg/sagas/usecase"
	"github.com/go-foreman/examples/pkg/sagas/usecase/subscription/contracts"
	"github.com/go-foreman/foreman/log"
	"github.com/go-foreman/foreman/runtime/scheme"
	"github.com/go-foreman/foreman/saga"
)

func init() {
	scheme.KnownTypesRegistryInstance.AddKnownTypes(contracts.SubscriptionGroup, &SubscribeSaga{})
	usecase.DefaultSagasCollection.AddSaga(&SubscribeSaga{})
}

type SubscribeSaga struct {
	saga.BaseSaga //embeds ObjectMeta, EventHandlers() and SetSchema()

	// business data
	Email        string  `json:"email"`
	Currency     string  `json:"currency"`
	Amount       float32 `json:"amount"`
	RetriesLimit int     `json:"retries_limit"`

	// these fields will be set in runtime from received events as saga progresses
	UserID         string `json:"user_id"`
	InvoiceID      string `json:"invoice_id"`
	CurrentRetries int    `json:"current_retries"`
}

func (r *SubscribeSaga) Init() {
	r.
		AddEventHandler(&contracts.UserRegistered{}, r.UserRegistered).
		AddEventHandler(&contracts.RegistrationFailed{}, r.RegistrationFailed).
		AddEventHandler(&contracts.InvoiceCreated{}, r.InvoiceCreated).
		AddEventHandler(&contracts.InvoiceCreationFailed{}, r.InvoiceCreationFailed).
		AddEventHandler(&contracts.EmailSent{}, r.EmailSent).
		AddEventHandler(&contracts.SendingEmailFailed{}, r.EmailSendingFailed).
		AddEventHandler(&contracts.InvoiceCanceled{}, r.CanceledInvoice).
		AddEventHandler(&contracts.InvoiceCancellationFailed{}, r.InvoiceCancellationFailed)
}

func (r *SubscribeSaga) Start(execCtx saga.SagaContext) error {
	r.CurrentRetries = r.RetriesLimit
	execCtx.Logger().Log(log.InfoLevel, "Starting saga")
	execCtx.Dispatch(&contracts.RegisterUserCmd{
		Email: r.Email,
	})
	return nil
}

func (r *SubscribeSaga) Compensate(execCtx saga.SagaContext) error {
	execCtx.Logger().Log(log.InfoLevel, "Starting compensation...")
	execCtx.Dispatch(&contracts.CancelInvoiceCmd{
		InvoiceID: r.InvoiceID,
	})

	return nil
}

func (r *SubscribeSaga) Recover(execCtx saga.SagaContext) error {
	r.CurrentRetries = 1

	// let's push again last failed message  with a single retry
	if ev := execCtx.SagaInstance().Status().FailedOnEvent(); ev != nil {
		execCtx.Dispatch(ev)
	}

	execCtx.Logger().Log(log.InfoLevel, "Recovering saga, Retries limit was set to 1")

	return nil
}

func (r *SubscribeSaga) UserRegistered(execCtx saga.SagaContext) error {
	ev, _ := execCtx.Message().Payload().(*contracts.UserRegistered)

	execCtx.Logger().Logf(log.InfoLevel, "User %s registration successful", r.Email)

	r.UserID = ev.UID

	execCtx.Dispatch(&contracts.CreateInvoiceCmd{
		UserID:   ev.UID,
		Email:    r.Email,
		Amount:   r.Amount,
		Currency: r.Currency,
	})

	return nil
}

func (r *SubscribeSaga) RegistrationFailed(execCtx saga.SagaContext) error {
	ev, _ := execCtx.Message().Payload().(*contracts.RegistrationFailed)

	execCtx.Logger().Logf(log.ErrorLevel, "User %s registration failed. %s", r.Email, ev.Reason)

	if r.CurrentRetries > 0 {
		r.CurrentRetries--
		execCtx.Dispatch(&contracts.RegisterUserCmd{
			Email: r.Email,
		})

		return nil
	}

	execCtx.Logger().Log(log.ErrorLevel, "Saga failed. You can recover it or compensate by sending corresponding commands.")

	execCtx.SagaInstance().Fail(execCtx.Message().Payload())
	return nil
}

func (r *SubscribeSaga) InvoiceCreated(execCtx saga.SagaContext) error {
	ev, _ := execCtx.Message().Payload().(*contracts.InvoiceCreated)

	execCtx.Logger().Logf(log.InfoLevel, "Invoice %s created for user %s", ev.ID, r.Email)

	r.InvoiceID = ev.ID

	execCtx.Dispatch(&contracts.SendEmailCmd{
		UserID:    r.UserID,
		Email:     r.Email,
		InvoiceID: r.InvoiceID,
	})

	return nil
}

func (r *SubscribeSaga) InvoiceCreationFailed(execCtx saga.SagaContext) error {
	ev, _ := execCtx.Message().Payload().(*contracts.InvoiceCreationFailed)
	execCtx.Logger().Logf(log.ErrorLevel, "Failed to create invoice %s for user %s. %s", r.InvoiceID, r.Email, ev.Reason)

	// custom retry logic
	if r.CurrentRetries > 0 {
		r.CurrentRetries--

		execCtx.Dispatch(&contracts.CreateInvoiceCmd{
			UserID:   r.UserID,
			Email:    r.Email,
			Amount:   r.Amount,
			Currency: r.Currency,
		}, endpoint.WithDelay(time.Second*5)) //do not retry immediately, wait 5s

		return nil
	}

	// if all retries are used, invoice creation failed, need to mark this saga as failed one, the last failed message will be persisted into saga's state
	execCtx.SagaInstance().Fail(execCtx.Message().Payload())
	execCtx.Logger().Log(log.ErrorLevel, "Saga failed. You can recover it or compensate by sending corresponding commands.")

	return nil
}

func (r *SubscribeSaga) EmailSent(execCtx saga.SagaContext) error {
	execCtx.Logger().Logf(log.InfoLevel, "Email to %s was sent", r.Email)
	execCtx.Logger().Log(log.InfoLevel, "Saga completed")

	// all steps are processed successfully, mark this saga as completed.
	execCtx.SagaInstance().Complete()

	return nil
}

func (r *SubscribeSaga) EmailSendingFailed(execCtx saga.SagaContext) error {
	ev, _ := execCtx.Message().Payload().(*contracts.SendingEmailFailed)
	execCtx.Logger().Logf(log.ErrorLevel, "Failed to send email to %s. %s", r.Email, ev.Reason)

	if r.CurrentRetries > 0 {
		r.CurrentRetries--

		execCtx.Dispatch(&contracts.SendEmailCmd{
			UserID:    r.UserID,
			Email:     r.Email,
			InvoiceID: r.InvoiceID,
		})

		return nil
	}

	execCtx.SagaInstance().Fail(execCtx.Message().Payload())
	execCtx.Logger().Log(log.ErrorLevel, "Saga failed. You can recover it or compensate by sending corresponding commands.")

	// Depending on business logic these commands can be pushed on user action or
	// automatically. In this example at the end of failed process a compensation
	// command is dispatched and soon r.Compensate() will be triggered and saga
	// will go into 'compensating' status
	execCtx.Dispatch(&sagaContracts.CompensateSagaCommand{
		SagaUID: execCtx.SagaInstance().UID(),
	})

	return nil
}

func (r *SubscribeSaga) CanceledInvoice(execCtx saga.SagaContext) error {
	ev, _ := execCtx.Message().Payload().(*contracts.InvoiceCanceled)

	execCtx.Logger().Logf(log.InfoLevel, "Invoice %s canceled. Saga marked as completed", ev.InvoiceID)

	//@todo maybe send an email here about cancellation?

	execCtx.SagaInstance().Complete()

	return nil
}

func (r *SubscribeSaga) InvoiceCancellationFailed(execCtx saga.SagaContext) error {
	ev, _ := execCtx.Message().Payload().(*contracts.InvoiceCancellationFailed)
	execCtx.Logger().Logf(log.InfoLevel, "Invoice %s wasn't canceled. Saga marked as failed. Call your administrator and fix it :)", ev.InvoiceID)

	execCtx.SagaInstance().Fail(ev)

	return nil
}
