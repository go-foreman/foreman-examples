package contracts

import (
	"github.com/go-foreman/examples/pkg/sagas/usecase"
	"github.com/go-foreman/foreman/pubsub/message"
	"github.com/go-foreman/foreman/runtime/scheme"
)

const (
	AccountGroup scheme.Group = "account"
)

func init() {
	contractsList := []message.Object{
		&RegisterAccountCmd{},
		&AccountRegistered{},
		&RegistrationFailed{},
		&SendConfirmationCmd{},
		&ConfirmationSent{},
		&ConfirmationSendingFailed{},
		&AccountConfirmed{},
	}

	scheme.KnownTypesRegistryInstance.AddKnownTypes(AccountGroup, usecase.ConvertToSchemaObj(contractsList)...)
	usecase.DefaultSagasCollection.RegisterContracts(contractsList...)
}

type RegisterAccountCmd struct {
	message.ObjectMeta
	UID   string `json:"uid"`
	Email string `json:"name"`
}

type AccountRegistered struct {
	message.ObjectMeta
	UID string `json:"uid"`
}

type RegistrationFailed struct {
	message.ObjectMeta
	UID    string `json:"uid"`
	Reason string `json:"reason"`
}

type SendConfirmationCmd struct {
	message.ObjectMeta
	UID   string `json:"uid"`
	Email string `json:"email"`
}

type ConfirmationSent struct {
	message.ObjectMeta
	UID string `json:"uid"`
}

type ConfirmationSendingFailed struct {
	message.ObjectMeta
	UID    string `json:"uid"`
	Reason string `json:"reason"`
}

type AccountConfirmed struct {
	message.ObjectMeta
	UID string `json:"uid"`
}
