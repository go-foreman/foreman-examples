package contracts

import (
	"github.com/go-foreman/examples/pkg/sagas/usecase"
	"github.com/go-foreman/foreman/pubsub/message"
	"github.com/go-foreman/foreman/runtime/scheme"
)

const (
	SubscriptionGroup scheme.Group = "subscription"
)

func init() {
	contractsList := []message.Object{
		&RegisterUserCmd{},
		&UserRegistered{},
		&RegistrationFailed{},

		&CreateInvoiceCmd{},
		&InvoiceCreated{},
		&InvoiceCreationFailed{},

		&CancelInvoiceCmd{},
		&InvoiceCancellationFailed{},
		&InvoiceCanceled{},

		&SendEmailCmd{},
		&EmailSent{},
		&SendingEmailFailed{},
	}

	scheme.KnownTypesRegistryInstance.AddKnownTypes(SubscriptionGroup, usecase.ConvertToSchemaObj(contractsList)...)
	usecase.DefaultSagasCollection.RegisterContracts(contractsList...)
}

type RegisterUserCmd struct {
	message.ObjectMeta
	Email string `json:"email"`
}

type UserRegistered struct {
	message.ObjectMeta
	UID string `json:"uid"`
}

type RegistrationFailed struct {
	message.ObjectMeta
	Email  string `json:"email"`
	Reason string `json:"reason"`
}

type CreateInvoiceCmd struct {
	message.ObjectMeta
	Email    string  `json:"email"`
	UserID   string  `json:"user_id"`
	Amount   float32 `json:"amount"`
	Currency string  `json:"currency"`
}

type InvoiceCreated struct {
	message.ObjectMeta
	ID string `json:"id"`
}

type InvoiceCreationFailed struct {
	message.ObjectMeta
	Reason string `json:"reason"`
}

type CancelInvoiceCmd struct {
	message.ObjectMeta
	InvoiceID string `json:"invoice_id"`
}

type InvoiceCanceled struct {
	message.ObjectMeta
	InvoiceID string `json:"invoice_id"`
}

type InvoiceCancellationFailed struct {
	message.ObjectMeta
	InvoiceID string `json:"invoice_id"`
}

type SendEmailCmd struct {
	message.ObjectMeta
	UserID    string `json:"user_id"`
	Email     string `json:"email"`
	InvoiceID string `json:"invoice_id"`
}

type EmailSent struct {
	message.ObjectMeta
	Email string `json:"email"`
}

type SendingEmailFailed struct {
	message.ObjectMeta
	Email  string `json:"email"`
	Reason string `json:"reason"`
}
