package irisnet

import (
	"net"
	"sync"

	amino "github.com/tendermint/go-amino"
	lru "github.com/hashicorp/golang-lru"
	"github.com/tendermint/tendermint/crypto/ed25519"
	"github.com/supragya/tendermint_connector/chains/irisnet/conn"
	marlinTypes "github.com/supragya/tendermint_connector/types"
)

type TendermintHandler struct {
	servicedChainId      uint32
	listenPort           int
	isConnectionOutgoing bool
	peerAddr             string
	rpcAddr              string
	privateKey           ed25519.PrivKeyEd25519
	codec                *amino.Codec
	baseConnection       net.Conn
	validatorCache		 *lru.TwoQueueCache
	maxValidHeight 			int64
	secretConnection     *conn.SecretConnection
	marlinTo             chan marlinTypes.MarlinMessage
	marlinFrom           chan marlinTypes.MarlinMessage
	channelBuffer        map[byte][]marlinTypes.PacketMsg
	peerNodeInfo         DefaultNodeInfo
	p2pConnection        P2PConnection
	throughput           throughPutData
	signalConnError      chan struct{}
	signalShutSend       chan struct{}
	signalShutRecv       chan struct{}
	signalShutThroughput chan struct{}
}

type throughPutData struct {
	isDataConnect bool
	toTMCore   map[string]uint32
	fromTMCore map[string]uint32
	spam	   map[string]uint32
	mu         sync.Mutex
}

type keyData struct {
	Chain            string
	IdString         string
	PrivateKeyString string
	PublicKeyString  string
	PrivateKey       [64]byte
	PublicKey        [32]byte
}

type Validator struct {
	PublicKey	ed25519.PubKeyEd25519
	Address		string
}