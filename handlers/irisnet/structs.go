package irisnet

import (
	"net"
	"bufio"
	"time"
	"sync"
	"fmt"
	flow "github.com/supragya/tendermint_connector/handlers/irisnet/libs/flowrate"
	cmn "github.com/supragya/tendermint_connector/handlers/irisnet/libs/common"
	amino "github.com/tendermint/go-amino"
)

type ProtocolVersion struct {
	P2P   uint64 `json:"p2p"`
	Block uint64 `json:"block"`
	App   uint64 `json:"app"`
}

type DefaultNodeInfo struct {
	ProtocolVersion ProtocolVersion `json:"protocol_version"`

	// Authenticate
	// TODO: replace with NetAddress
	ID_        string `json:"id"`          // authenticated identifier
	ListenAddr string `json:"listen_addr"` // accepting incoming

	// Check compatibility.
	// Channels are HexBytes so easier to read as JSON
	Network  string       `json:"network"`  // network/chain ID
	Version  string       `json:"version"`  // major.minor.revision
	Channels cmn.HexBytes `json:"channels"` // channels this node knows about

	// ASCIIText fields
	Moniker string               `json:"moniker"` // arbitrary moniker
	Other   DefaultNodeInfoOther `json:"other"`   // other application specific data
}

// DefaultNodeInfoOther is the misc. applcation specific data
type DefaultNodeInfoOther struct {
	TxIndex    string `json:"tx_index"`
	RPCAddress string `json:"rpc_address"`
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
	errored       uint32

	// Closing quitSendRoutine will cause the sendRoutine to eventually quit.
	// doneSendRoutine is closed when the sendRoutine actually quits.
	quitSendRoutine chan struct{}
	doneSendRoutine chan struct{}

	// Closing quitRecvRouting will cause the recvRouting to eventually quit.
	quitRecvRoutine chan struct{}

	// used to ensure FlushStop and OnStop
	// are safe to call concurrently.
	stopMtx sync.Mutex

	flushTimer *cmn.ThrottleTimer // flush writes as necessary but throttled.
	pingTimer  *time.Ticker       // send pings periodically

	// close conn if pong is not received in pongTimeout
	pongTimer     *time.Timer
	pongTimeoutCh chan bool // true - timeout, false - peer sent pong

	chStatsTimer *time.Ticker // update channel stats periodically

	created time.Time // time of creation

	_maxPacketMsgSize int
}


//----------------------------------------
// Packet

type Packet interface {
	AssertIsPacket()
}

func RegisterPacket(cdc *amino.Codec) {
	cdc.RegisterInterface((*Packet)(nil), nil)
	cdc.RegisterConcrete(PacketPing{}, "tendermint/p2p/PacketPing", nil)
	cdc.RegisterConcrete(PacketPong{}, "tendermint/p2p/PacketPong", nil)
	cdc.RegisterConcrete(PacketMsg{}, "tendermint/p2p/PacketMsg", nil)
}

func (_ PacketPing) AssertIsPacket() {}
func (_ PacketPong) AssertIsPacket() {}
func (_ PacketMsg) AssertIsPacket()  {}

type PacketPing struct {
}

type PacketPong struct {
}

type PacketMsg struct {
	ChannelID byte
	EOF       byte // 1 means message ends here.
	Bytes     []byte
}

func (mp PacketMsg) String() string {
	return fmt.Sprintf("PacketMsg{%X:%X T:%X}", mp.ChannelID, mp.Bytes, mp.EOF)
}

const (
	channelBc =	byte(0x40) //bc.BlockchainChannel,
	channelCsSt =	byte(0x20) //cs.StateChannel,
	channelCsDC =	byte(0x21) //cs.DataChannel,
	channelCsVo =	byte(0x22) //cs.VoteChannel,
	channelCsVs =	byte(0x23) //cs.VoteSetBitsChannel,
	channelMm =	byte(0x30) //mempl.MempoolChannel,
	channelEv =	byte(0x38) //evidence.EvidenceChannel,
)