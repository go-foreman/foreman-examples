package user

import (
	"github.com/go-foreman/examples/pkg/sagas/usecase/subscription/contracts"
	"github.com/go-foreman/examples/pkg/services/user"
	foreman "github.com/go-foreman/foreman"
	"github.com/go-foreman/foreman/pubsub/message"
	"github.com/go-foreman/foreman/pubsub/message/execution"
)

type Handler struct {
	userService *user.UserService
}

func NewHandler(mbus *foreman.MessageBus, userService *user.UserService) *Handler {
	h := &Handler{userService: userService}

	mbus.Dispatcher().SubscribeForCmd(&contracts.RegisterUserCmd{}, h.RegisterUser)

	return h
}

func (h Handler) RegisterUser(execCtx execution.MessageExecutionCtx) error {
	registerCmd, _ := execCtx.Message().Payload().(*contracts.RegisterUserCmd)

	usr, err := h.userService.Register(execCtx.Context(), user.User{
		Email: registerCmd.Email,
	})

	if err != nil {
		return execCtx.Send(message.NewOutcomingMessage(
			&contracts.RegistrationFailed{
				Email:  registerCmd.Email,
				Reason: err.Error(),
			},
			message.WithHeaders(execCtx.Message().Headers())),
		)
	}

	return execCtx.Send(message.NewOutcomingMessage(
		&contracts.UserRegistered{
			UID: usr.ID,
		},
		message.WithHeaders(execCtx.Message().Headers())),
	)
}
