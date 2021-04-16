package usecase

import (
	"github.com/go-foreman/foreman/pubsub/message"
	"github.com/go-foreman/foreman/runtime/scheme"
	"github.com/go-foreman/foreman/saga"
)

var DefaultSagasCollection = SagasCollection{}

type SagasCollection struct {
	sagas     []saga.Saga
	contracts []message.Object
}

func (c *SagasCollection) AddSaga(s saga.Saga) {
	c.sagas = append(c.sagas, s)
}

func (c *SagasCollection) Sagas() []saga.Saga {
	return c.sagas
}

func (c *SagasCollection) RegisterContracts(p ...message.Object) {
	if len(p) > 0 {
		c.contracts = append(c.contracts, p...)
	}
}

func (c *SagasCollection) Contracts() []message.Object {
	return c.contracts
}

func ConvertToSchemaObj(objs []message.Object) []scheme.Object {
	res := make([]scheme.Object, len(objs))
	for i, o := range objs {
		res[i] = o
	}
	return res
}
