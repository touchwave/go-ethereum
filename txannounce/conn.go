package txannounce

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
	"io"
	"sync"
)

var ErrConnClosed = errors.New("ws conn closed")

var supportTopics map[Topic]bool

func init() {
	supportTopics = map[Topic]bool{
		TopicNewTx:           true,
		TopicBlockedTxHashes: true,
	}
}

type Conn struct {
	c          *websocket.Conn
	subscribed map[Topic]bool

	feedCh chan struct {
		packet FeedPacket
		errCh  chan error
	}
	stopCh    chan struct{}
	closeOnce sync.Once
}

func NewConn(c *websocket.Conn) *Conn {
	return &Conn{
		c:          c,
		subscribed: make(map[Topic]bool),
		feedCh: make(chan struct {
			packet FeedPacket
			errCh  chan error
		}),
		stopCh: make(chan struct{}),
	}
}

// feed put packet into conn
func (c *Conn) feed(packet FeedPacket) chan error {
	errCh := make(chan error)
	c.feedCh <- struct {
		packet FeedPacket
		errCh  chan error
	}{packet, errCh}
	return errCh
}

// FeedResponse respond to client requests
func (c *Conn) FeedResponse(id int, ok bool, msg string) error {
	return <-c.feed(FeedPacket{
		T:    FeedTypeResponse,
		Data: ResponsePacket{id, ok, msg},
	})
}

// FeedNewTx relay new tx event to clients
func (c *Conn) FeedNewTx(txs types.Transactions) error {
	return <-c.feed(FeedPacket{
		T:    FeedTypeTransactions,
		Data: txs,
	})
}

// FeedBlockedTxHash relay new blocked tx hash to clients
func (c *Conn) FeedBlockedTxHash(hashes []common.Hash) error {
	return <-c.feed(FeedPacket{
		T:    FeedTypeBlockedTxHashes,
		Data: hashes,
	})
}

func (c *Conn) sendLoop() {
	for {
		select {
		case <-c.stopCh:
			break
		case m := <-c.feedCh:
			err := c.c.WriteJSON(m.packet)
			m.errCh <- err
		}
	}
}

func (c *Conn) onSubscribe(topic Topic, id int) error {
	if supportTopics[topic] {
		c.subscribed[topic] = true
		return c.FeedResponse(id, true, "subscribed topic: "+string(topic))
	}
	return c.FeedResponse(id, false, "unknown topic :"+string(topic))
}

// onUnsubscribe always respond ok
func (c *Conn) onUnsubscribe(topic Topic, id int) error {
	delete(c.subscribed, topic)
	return c.FeedResponse(id, true, "unsubscribed topic (unchecked): "+string(topic))
}

func (c *Conn) RecvLoop() error {
	for {
		var req RequestPacket
		err := c.c.ReadJSON(&req)
		if err != nil {
			log.Error("ws Request decode", "err", err, "remote", c.c.RemoteAddr())
			err = c.FeedResponse(-1, false, err.Error())
			if err != nil {
				return err
			}
		}
		switch req.Op {
		case ClientOPSubscribe:
			err = c.onSubscribe(req.Topic, req.Id)
			if err != nil {
				return err
			}
		case ClientOPUnsubscribe:
			err = c.onUnsubscribe(req.Topic, req.Id)
			if err != nil {
				return err
			}
		default:
			log.Error("Announce Server: unsupported request type", "remote", c.c.RemoteAddr())
		}
	}
}

// Serve blocked until conn closed
func (c *Conn) Serve() error {
	go c.sendLoop()
	err := c.RecvLoop()
	if !errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, io.EOF) {
		return ErrConnClosed
	}
	return nil
}

func (c *Conn) Stop() {
	// TODO: optimize
	c.closeOnce.Do(func() {
		log.Debug("Closing websocket connection", "remote", c.c.RemoteAddr())
		c.c.Close()
		close(c.stopCh)
	})
}
