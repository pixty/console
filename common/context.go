package common

// The Context is used for passing some request parameters between services.
// One of the examples could be the TxPersister
type Context struct {
	cf          *DefaultContextFactory
	txPersister TxPersister
}

// The Context factory interface provides methods for creating new contexts
type ContextFactory interface {
	NewContext() *Context
}

type DefaultContextFactory struct {
	Persister Persister `inject:"persister"`
}

func NewContextFactory() *DefaultContextFactory {
	return &DefaultContextFactory{}
}

// Constructs new Context
func (cf *DefaultContextFactory) NewContext() *Context {
	return &Context{cf: cf}
}

func (ctx *Context) TxPersister() TxPersister {
	if ctx.txPersister == nil {
		ctx.txPersister = ctx.cf.Persister.NewTxPersister()
	}
	return ctx.txPersister
}

func (ctx *Context) Close() {
	if ctx.txPersister != nil {
		ctx.txPersister.Close()
		ctx.txPersister = nil
	}
}
