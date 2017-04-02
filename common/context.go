package common

import (
	"sync"

	"golang.org/x/net/context"
)

const cCtxHolderKey = "ctxHolder"

type CtxHolder struct {
	ctx         context.Context
	persister   Persister
	txPersister TxPersister
	lock        sync.Mutex
}

func NewCtxHolder(ctx context.Context) (*CtxHolder, context.Context) {
	ch := new(CtxHolder)
	newCtx := context.WithValue(ctx, cCtxHolderKey, ch)
	ch.ctx = newCtx
	return ch, newCtx
}

func (ch *CtxHolder) WithPersister(persister Persister) *CtxHolder {
	ch.persister = persister
	return ch
}

func (ch *CtxHolder) TxPersister() TxPersister {
	if ch.txPersister == nil {
		ch.lock.Lock()
		if ch.txPersister == nil {
			ch.txPersister = ch.persister.NewTxPersister(ch.ctx)
		}
		ch.lock.Unlock()
	}
	return ch.txPersister
}

func (ch *CtxHolder) Ctx() context.Context {
	return ch.ctx
}

func GetTxPersister(ctx context.Context) TxPersister {
	ch, ok := ctx.Value(cCtxHolderKey).(*CtxHolder)
	if !ok {
		panic("Not acceptable usage of context!")
	}
	return ch.TxPersister()
}
