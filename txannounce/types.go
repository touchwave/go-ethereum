package txannounce

type FeedType int

const (
	FeedTypeTransactions FeedType = iota
	FeedTypeBlockedTxHashes
	FeedTypeResponse
)

// FeedPacket 服务器向客户端推送的数据类型
type FeedPacket struct {
	T    FeedType `json:"T"`
	Data any      `json:"Data"`
}

type RequestPacket struct {
	Op ClientOp `json:"op"`
	// 请求id
	Id    int   `json:"id"`
	Topic Topic `json:"topic"`
}

type ResponsePacket struct {
	// 响应的请求id
	Id      int    `json:"id"`
	Ok      bool   `json:"ok"`
	Message string `json:"message"`
}

type ClientOp int

const (
	ClientOPSubscribe ClientOp = iota
	ClientOPUnsubscribe
)

type Topic string

const (
	TopicNewTx           Topic = "newTx"
	TopicBlockedTxHashes       = "blockedTxHashes"
)
