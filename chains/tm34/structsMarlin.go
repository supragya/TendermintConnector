package tm34

import (
	"bufio"
	"net"
	"sync"
	"time"

	lru "github.com/hashicorp/golang-lru"
	"github.com/supragya/TendermintConnector/chains/tm34/conn"
	"github.com/supragya/TendermintConnector/chains/tm34/crypto/ed25519"
	"github.com/supragya/TendermintConnector/chains/tm34/libs/timer"
	tmp2p "github.com/supragya/TendermintConnector/chains/tm34/proto/tendermint/p2p"
	marlinTypes "github.com/supragya/TendermintConnector/types"
	"github.com/tendermint/go-amino"
	flow "github.com/tendermint/tendermint/libs/flowrate"
)

type TendermintHandler struct {
	servicedChainId      uint32
	listenPort           int
	isConnectionOutgoing bool
	peerAddr             string
	rpcAddr              string
	privateKey           ed25519.PrivKey
	codec                *amino.Codec
	baseConnection       net.Conn
	validatorCache       *lru.TwoQueueCache
	maxValidHeight       int64
	secretConnection     *conn.SecretConnection
	marlinTo             chan marlinTypes.MarlinMessage
	marlinFrom           chan marlinTypes.MarlinMessage
	channelBuffer        map[byte][]marlinTypes.PacketMsg
	peerNodeInfo         tmp2p.DefaultNodeInfo
	p2pConnection        P2PConnection
	throughput           throughPutData
	signalConnError      chan struct{}
	signalShutSend       chan struct{}
	signalShutRecv       chan struct{}
	signalShutThroughput chan struct{}
}

type P2PConnection struct {
	conn          net.Conn
	bufConnReader *bufio.Reader
	bufConnWriter *bufio.Writer
	sendMonitor   *flow.Monitor
	recvMonitor   *flow.Monitor
	send          chan struct{}
	pong          chan struct{}
	// channels      []*Channel
	// channelsIdx   map[byte]*Channel
	errored uint32

	// Closing quitSendRoutine will cause the sendRoutine to eventually quit.
	// doneSendRoutine is closed when the sendRoutine actually quits.
	quitSendRoutine chan struct{}
	doneSendRoutine chan struct{}

	// Closing quitRecvRouting will cause the recvRouting to eventually quit.
	quitRecvRoutine chan struct{}

	// used to ensure FlushStop and OnStop
	// are safe to call concurrently.
	stopMtx sync.Mutex

	flushTimer *timer.ThrottleTimer // flush writes as necessary but throttled.
	pingTimer  *time.Ticker         // send pings periodically

	// close conn if pong is not received in pongTimeout
	pongTimer     *time.Timer
	pongTimeoutCh chan bool // true - timeout, false - peer sent pong

	chStatsTimer *time.Ticker // update channel stats periodically

	created time.Time // time of creation

	_maxPacketMsgSize int
}

type throughPutData struct {
	isDataConnect bool
	toTMCore      map[string]uint32
	fromTMCore    map[string]uint32
	spam          map[string]uint32
	mu            sync.Mutex
}
