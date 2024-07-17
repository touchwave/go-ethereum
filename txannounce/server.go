package txannounce

import (
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/log"
)

type txSub interface {
	SubscribeTransactions(ch chan<- core.NewTxsEvent, reorgs bool) event.Subscription
}
type blkSub interface {
	SubscribeChainHeadEvent(ch chan<- core.ChainHeadEvent) event.Subscription
}

type AnnounceServer struct {
	pool txSub
	bc   blkSub

	txch            chan core.NewTxsEvent
	blkch           chan core.ChainHeadEvent
	stop            chan struct{}
	txSubscription  event.Subscription
	blkSubscription event.Subscription

	ws *wsServer
}

func NewAnnounceServer(pool txSub, bc blkSub) *AnnounceServer {
	as := &AnnounceServer{
		pool: pool,
		bc:   bc,

		txch:  make(chan core.NewTxsEvent),
		blkch: make(chan core.ChainHeadEvent),
		stop:  make(chan struct{}),

		ws: newWS(":7856"),
	}
	return as
}

func (a *AnnounceServer) subscribeEvents() {
	a.txSubscription = a.pool.SubscribeTransactions(a.txch, true)
	a.blkSubscription = a.bc.SubscribeChainHeadEvent(a.blkch)
}
func (a *AnnounceServer) unSubscribeEvents() {
	a.txSubscription.Unsubscribe()
	a.blkSubscription.Unsubscribe()
}

func (a *AnnounceServer) listen() error {
	return a.ws.ListenAndServe()
}

func (a *AnnounceServer) loop() {
	for {
		select {
		case txe := <-a.txch:
			log.Debug("new txs received", "len", len(txe.Txs))
			a.ws.DispatchNewTxsEvent(txe)
		case blke := <-a.blkch:
			log.Debug("blocks received", "tx len", len(blke.Block.Transactions()))
			a.ws.DispatchChainHeadEvent(blke)
		case <-a.stop:
			return
		}
	}
}

func (a *AnnounceServer) Start() error {
	a.subscribeEvents()
	go a.loop()
	go a.listen()
	return nil
}
func (a *AnnounceServer) Stop() error {
	log.Debug("stop AnnounceServer")
	a.unSubscribeEvents()
	close(a.stop)
	err := a.ws.Shutdown()
	if err != nil {
		return err
	}
	return nil
}
