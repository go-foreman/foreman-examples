package email

import (
	"fmt"
	"github.com/go-foreman/examples/pkg/sagas/usecase/subscription/contracts"
	"github.com/go-foreman/examples/pkg/services/email"
	"github.com/go-foreman/examples/pkg/services/payment"
	"github.com/go-foreman/examples/pkg/services/user"
	foreman "github.com/go-foreman/foreman"
	"github.com/go-foreman/foreman/pubsub/message"
	"github.com/go-foreman/foreman/pubsub/message/execution"
)

type Handler struct {
	sender         *email.Sender
	userService    *user.UserService
	invoiceService *payment.InvoicingService
}

func NewHandler(mbus *foreman.MessageBus, sender *email.Sender, userService *user.UserService, invoiceService *payment.InvoicingService) *Handler {
	h := &Handler{
		sender:         sender,
		userService:    userService,
		invoiceService: invoiceService,
	}

	mbus.Dispatcher().SubscribeForCmd(&contracts.SendEmailCmd{}, h.SendEmail)

	return h
}

func (h Handler) SendEmail(execCtx execution.MessageExecutionCtx) error {
	sendEmailCmd, _ := execCtx.Message().Payload().(*contracts.SendEmailCmd)

	usr, err := h.userService.GetUser(execCtx.Context(), sendEmailCmd.UserID)
	if err != nil {
		return execCtx.Send(message.NewOutcomingMessage(
			&contracts.SendingEmailFailed{
				Email:  sendEmailCmd.Email,
				Reason: err.Error(),
			},
			message.WithHeaders(execCtx.Message().Headers()),
		),
		)
	}

	if usr == nil {
		return execCtx.Send(message.NewOutcomingMessage(
			&contracts.SendingEmailFailed{
				Email:  sendEmailCmd.Email,
				Reason: "User does not exist",
			},
			message.WithHeaders(execCtx.Message().Headers())),
		)
	}

	invoice, err := h.invoiceService.Get(execCtx.Context(), sendEmailCmd.InvoiceID)
	if err != nil {
		return execCtx.Send(message.NewOutcomingMessage(
			&contracts.SendingEmailFailed{
				Email:  sendEmailCmd.Email,
				Reason: err.Error(),
			},
			message.WithHeaders(execCtx.Message().Headers())),
		)
	}

	if invoice == nil {
		return execCtx.Send(message.NewOutcomingMessage(
			&contracts.SendingEmailFailed{
				Email:  sendEmailCmd.Email,
				Reason: fmt.Sprintf("Invoice %s does not exist", sendEmailCmd.InvoiceID),
			},
			message.WithHeaders(execCtx.Message().Headers())),
		)
	}

	messageBodyTemplate := `Hello %s,
Invoice details: 
	Amount - %f,
	Currency- %s
`
	messageBody := fmt.Sprintf(messageBodyTemplate, usr.Email, invoice.Amount, invoice.Currency)

	if err := h.sender.Send(execCtx.Context(), sendEmailCmd.Email, []byte(messageBody)); err != nil {
		return execCtx.Send(message.NewOutcomingMessage(
			&contracts.SendingEmailFailed{
				Email:  sendEmailCmd.Email,
				Reason: err.Error(),
			},
			message.WithHeaders(execCtx.Message().Headers())),
		)
	}

	return execCtx.Send(message.NewOutcomingMessage(
		&contracts.EmailSent{
			Email: sendEmailCmd.Email,
		},
		message.WithHeaders(execCtx.Message().Headers())),
	)
}
