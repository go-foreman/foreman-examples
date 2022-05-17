package subscription

import (
	"fmt"

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
	saga.BaseSaga

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
	execCtx.LogMessage(log.InfoLevel, fmt.Sprintf("Starting saga %s", execCtx.SagaInstance().UID()))
	execCtx.Dispatch(&contracts.RegisterUserCmd{
		Email: r.Email,
	})
	return nil
}

func (r *SubscribeSaga) Compensate(execCtx saga.SagaContext) error {
	execCtx.LogMessage(log.InfoLevel, "Starting compensation...")
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

	execCtx.LogMessage(log.InfoLevel, fmt.Sprintf("Recovering saga %s, Retries limit was set to 1", execCtx.SagaInstance().UID()))

	return nil
}

func (r *SubscribeSaga) UserRegistered(execCtx saga.SagaContext) error {
	ev, _ := execCtx.Message().Payload().(*contracts.UserRegistered)

	execCtx.LogMessage(log.InfoLevel, fmt.Sprintf("User %s registration successful", r.Email))

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

	execCtx.LogMessage(log.ErrorLevel, fmt.Sprintf("User %s registration failed. %s", r.Email, ev.Reason))

	if r.CurrentRetries > 0 {
		r.CurrentRetries--
		execCtx.Dispatch(&contracts.RegisterUserCmd{
			Email: r.Email,
		})

		return nil
	}

	execCtx.LogMessage(log.ErrorLevel, "Saga failed. You can recover it or compensate by sending corresponding commands.")

	execCtx.SagaInstance().Fail(execCtx.Message().Payload())
	return nil
}

func (r *SubscribeSaga) InvoiceCreated(execCtx saga.SagaContext) error {
	ev, _ := execCtx.Message().Payload().(*contracts.InvoiceCreated)

	execCtx.LogMessage(log.InfoLevel, fmt.Sprintf("Invoice %s created for user %s", ev.ID, r.Email))

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
	execCtx.LogMessage(log.ErrorLevel, fmt.Sprintf("Failed to create invoice %s for user %s. %s", r.InvoiceID, r.Email, ev.Reason))

	if r.CurrentRetries > 0 {
		r.CurrentRetries--

		execCtx.Dispatch(&contracts.CreateInvoiceCmd{
			UserID:   r.UserID,
			Email:    r.Email,
			Amount:   r.Amount,
			Currency: r.Currency,
		})

		return nil
	}

	execCtx.SagaInstance().Fail(execCtx.Message().Payload())
	execCtx.LogMessage(log.ErrorLevel, "Saga failed. You can recover it or compensate by sending corresponding commands.")

	return nil
}

func (r *SubscribeSaga) EmailSent(execCtx saga.SagaContext) error {
	execCtx.LogMessage(log.InfoLevel, fmt.Sprintf("Email to %s was sent", r.Email))
	execCtx.LogMessage(log.InfoLevel, "Saga completed")

	execCtx.SagaInstance().Complete()

	return nil
}

func (r *SubscribeSaga) EmailSendingFailed(execCtx saga.SagaContext) error {
	ev, _ := execCtx.Message().Payload().(*contracts.SendingEmailFailed)
	execCtx.LogMessage(log.ErrorLevel, fmt.Sprintf("Failed to send email to %s. %s", r.Email, ev.Reason))

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
	execCtx.LogMessage(log.ErrorLevel, "Saga failed. You can recover it or compensate by sending corresponding commands.")

	return nil
}

func (r *SubscribeSaga) CanceledInvoice(execCtx saga.SagaContext) error {
	ev, _ := execCtx.Message().Payload().(*contracts.InvoiceCanceled)

	execCtx.LogMessage(log.InfoLevel, fmt.Sprintf("Invoice %s canceled. Saga marked as completed", ev.InvoiceID))

	//@todo maybe send an email here about cancellation?

	execCtx.SagaInstance().Complete()

	return nil
}

func (r *SubscribeSaga) InvoiceCancellationFailed(execCtx saga.SagaContext) error {
	ev, _ := execCtx.Message().Payload().(*contracts.InvoiceCancellationFailed)
	execCtx.LogMessage(log.InfoLevel, fmt.Sprintf("Invoice %s wasn't canceled. Saga marked as failed. Call your administrator and fix it :)", ev.InvoiceID))

	execCtx.SagaInstance().Fail(ev)

	return nil
}
