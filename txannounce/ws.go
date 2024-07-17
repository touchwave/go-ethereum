package txannounce

import (
	"context"
	"errors"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/log"
	"github.com/gorilla/websocket"
	"net/http"
	"sync"
)

var upgrader = websocket.Upgrader{}

type wsServer struct {
	srv      *http.Server
	conns    map[string]*Conn
	connLock sync.RWMutex
}

func newWS(addr string) *wsServer {
	w := &wsServer{conns: make(map[string]*Conn)}
	srv := &http.Server{
		Addr:    addr,
		Handler: w,
	}
	w.srv = srv
	return w
}

func (s *wsServer) DispatchNewTxsEvent(e core.NewTxsEvent) {
	s.connLock.RLock()
	defer s.connLock.RUnlock()
	for addr, conn := range s.conns {
		if !conn.subscribed[TopicNewTx] {
			continue
		}
		err := conn.FeedNewTx(e.Txs)
		if err != nil {
			log.Error(err.Error(), "addr", addr)
		}
	}
}

func (s *wsServer) DispatchChainHeadEvent(e core.ChainHeadEvent) {
	s.connLock.RLock()
	defer s.connLock.RUnlock()
	hashes := make([]common.Hash, len(e.Block.Transactions()))
	for i, tx := range e.Block.Transactions() {
		hashes[i] = tx.Hash()
	}
	for addr, conn := range s.conns {
		if !conn.subscribed[TopicBlockedTxHashes] {
			continue
		}
		err := conn.FeedBlockedTxHash(hashes)
		if err != nil {
			log.Error(err.Error(), "addr", addr)
		}
	}
}

func (s *wsServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Error("wsServer upgrade:", err)
	}
	log.Info("new conn:", "addr", c.RemoteAddr())
	conn := NewConn(c)
	s.AddConn(conn)
}

func (s *wsServer) AddConn(conn *Conn) {
	s.connLock.Lock()
	defer s.connLock.Unlock()
	s.conns[conn.c.RemoteAddr().String()] = conn

	go func() {
		err := conn.Serve()
		if err != nil {
			if !errors.Is(err, ErrConnClosed) {
				s.RemoveConn(conn, false)
				log.Error(err.Error())
			} else {
				s.RemoveConn(conn, true)
				log.Info(err.Error())
			}
		} else {
			log.Warn("conn goroutine stops strangely, it will never happen under normal cases")
		}
	}()
}

func (s *wsServer) RemoveConn(conn *Conn, closeConn bool) {
	if closeConn {
		conn.Stop()
	}
	s.connLock.Lock()
	defer s.connLock.Unlock()
	delete(s.conns, conn.c.RemoteAddr().String())
}

func (s *wsServer) ListenAndServe() error {
	return s.srv.ListenAndServe()
}

func (s *wsServer) Shutdown() error {
	s.connLock.Lock()
	defer s.connLock.Unlock()
	for _, conn := range s.conns {
		conn.Stop()
	}
	return s.srv.Shutdown(context.Background())
}
