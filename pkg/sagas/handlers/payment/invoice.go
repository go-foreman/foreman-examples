package payment

import (
	"github.com/go-foreman/examples/pkg/sagas/usecase/subscription/contracts"
	"github.com/go-foreman/examples/pkg/services/payment"
	foreman "github.com/go-foreman/foreman"
	"github.com/go-foreman/foreman/pubsub/message"
	"github.com/go-foreman/foreman/pubsub/message/execution"
)

type Handler struct {
	invoicingService *payment.InvoicingService
}

func NewHandler(mbus *foreman.MessageBus, invoicingService *payment.InvoicingService) *Handler {
	h := &Handler{invoicingService: invoicingService}

	mbus.Dispatcher().SubscribeForCmd(&contracts.CreateInvoiceCmd{}, h.CreateInvoice)
	mbus.Dispatcher().SubscribeForCmd(&contracts.CancelInvoiceCmd{}, h.CancelInvoice)

	return h
}

func (h Handler) CreateInvoice(execCtx execution.MessageExecutionCtx) error {
	createInvoiceCmd, _ := execCtx.Message().Payload().(*contracts.CreateInvoiceCmd)

	invoice, err := h.invoicingService.Create(execCtx.Context(), payment.Invoice{
		Amount:     createInvoiceCmd.Amount,
		Currency:   createInvoiceCmd.Currency,
		Email:      createInvoiceCmd.Email,
		CustomerID: createInvoiceCmd.UserID,
	})

	if err != nil {
		return execCtx.Send(message.NewOutcomingMessage(
			&contracts.InvoiceCreationFailed{
				Reason: err.Error(),
			},
			message.WithHeaders(execCtx.Message().Headers())),
		)
	}

	return execCtx.Send(message.NewOutcomingMessage(
		&contracts.InvoiceCreated{
			ID: invoice.ID,
		},
		message.WithHeaders(execCtx.Message().Headers())),
	)
}

func (h Handler) CancelInvoice(execCtx execution.MessageExecutionCtx) error {
	cancelInvoiceCmd, _ := execCtx.Message().Payload().(*contracts.CancelInvoiceCmd)

	if err := h.invoicingService.Cancel(execCtx.Context(), cancelInvoiceCmd.InvoiceID); err != nil {
		return execCtx.Send(message.NewOutcomingMessage(
			&contracts.InvoiceCancellationFailed{
				InvoiceID: cancelInvoiceCmd.InvoiceID,
			},
			message.WithHeaders(execCtx.Message().Headers()),
		),
		)
	}

	return execCtx.Send(message.NewOutcomingMessage(
		&contracts.InvoiceCanceled{
			InvoiceID: cancelInvoiceCmd.InvoiceID,
		},
		message.WithHeaders(execCtx.Message().Headers())),
	)
}
