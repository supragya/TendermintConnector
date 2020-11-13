package irisnet

import (
	"bufio"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"reflect"
	"strconv"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/supragya/tendermint_connector/chains"
	"github.com/supragya/tendermint_connector/chains/irisnet/conn"
	cmn "github.com/supragya/tendermint_connector/chains/irisnet/libs/common"
	flow "github.com/supragya/tendermint_connector/chains/irisnet/libs/flowrate"
	marlinTypes "github.com/supragya/tendermint_connector/types"
	amino "github.com/tendermint/go-amino"
	"github.com/tendermint/tendermint/crypto/ed25519"
	"github.com/tendermint/tendermint/crypto/merkle"

	// Protocols
	"github.com/supragya/tendermint_connector/marlin"
)

// ServicedTMCore is a string associated with each TM core handler
// to decipher which handler is to be attached.
var ServicedTMCore chains.NodeType = chains.NodeType{Version: "0.32.2", Network: "irishub", ProtocolVersionApp: "2", ProtocolVersionBlock: "9", ProtocolVersionP2p: "5"}

// Run serves as the entry point for a TM Core handler when serving as a connector
func Run(peerAddr string,
	marlinTo chan marlinTypes.MarlinMessage,
	marlinFrom chan marlinTypes.MarlinMessage,
	isConnectionOutgoing bool,
	keyFile string,
	listenPort int) {
	log.Info("Starting Irisnet Tendermint Core Handler - 0.16.3-d83fc038-2-mainnet")

	if keyFile != "" {
		isKeyFileUsed = true
		keyFileLocation = keyFile
	}

	for {
		handler, err := createTMHandler(peerAddr, "0.0.0.0:0", marlinTo, marlinFrom, isConnectionOutgoing, listenPort)

		if err != nil {
			log.Error("Error encountered while creating TM Handler: ", err)
			os.Exit(1)
		}

		handler.peerState = tmPeerStateNotConnected

		if isConnectionOutgoing {
			err = handler.dialPeer()
		} else {
			err = handler.acceptPeer()
		}
		if err != nil {
			log.Error("Base Connection establishment with peer unsuccessful: ", err)
			goto REATTEMPT_CONNECTION
		}

		err = handler.upgradeConnectionAndHandshake()
		if err != nil {
			log.Error("Error while upgrading connection and handshaking with peer: ", err)
			goto REATTEMPT_CONNECTION
		}

		handler.beginServicing()

		select {
		case <-handler.signalConnError:
			handler.signalShutSend <- struct{}{}
			handler.signalShutRecv <- struct{}{}
			handler.signalShutThroughput <- struct{}{}
			handler.peerState = tmPeerStateShutdown
			goto REATTEMPT_CONNECTION
		}

	REATTEMPT_CONNECTION:
		log.Info("Error encountered with connection to the peer. Attempting reconnect post 1 second.")
		time.Sleep(1 * time.Second)
	}
}

func createTMHandler(peerAddr string,
	rpcAddr string,
	marlinTo chan marlinTypes.MarlinMessage,
	marlinFrom chan marlinTypes.MarlinMessage,
	isConnectionOutgoing bool,
	listenPort int) (TendermintHandler, error) {
	chainId, ok := marlinTypes.ServicedChains["irisnet-0.16.3-mainnet"]
	if !ok {
		return TendermintHandler{}, errors.New("Cannot find irisnet-0.16.3-mainnet in list of serviced chains by marlin connector")
	}

	privateKey := getPrivateKey()

	return TendermintHandler{
		servicedChainId:      chainId,
		listenPort:           listenPort,
		isConnectionOutgoing: isConnectionOutgoing,
		peerAddr:             peerAddr,
		rpcAddr:			  rpcAddr,
		privateKey:           privateKey,
		codec:                amino.NewCodec(),
		peerState:            tmPeerStateNotConnected,
		marlinTo:             marlinTo,
		marlinFrom:           marlinFrom,
		channelBuffer:        make(map[byte][]marlinTypes.PacketMsg),
		throughput: throughPutData{
			toTMCore:   make(map[string]uint32),
			fromTMCore: make(map[string]uint32),
		},
		signalConnError:      make(chan struct{}, 1),
		signalShutSend:       make(chan struct{}, 1),
		signalShutRecv:       make(chan struct{}, 1),
		signalShutThroughput: make(chan struct{}, 1),
	}, nil
}

func (h *TendermintHandler) dialPeer() error {
	var err error
	h.baseConnection, err = net.DialTimeout("tcp", h.peerAddr, 2000*time.Millisecond)
	if err != nil {
		return err
	}

	return nil
}

func (h *TendermintHandler) acceptPeer() error {
	log.Info("Listening for dials to ",
		string(hex.EncodeToString(h.privateKey.PubKey().Address())), "@<SYSTEM-IP-ADDR>:", h.listenPort)

	listener, err := net.Listen("tcp", "0.0.0.0:"+strconv.Itoa(h.listenPort))
	if err != nil {
		return err
	}

	h.baseConnection, err = listener.Accept()
	if err != nil {
		return err
	}

	return nil
}

func (h *TendermintHandler) upgradeConnectionAndHandshake() error {
	var err error
	h.secretConnection, err = conn.MakeSecretConnection(h.baseConnection, h.privateKey)
	if err != nil {
		return err
	}

	err = h.handshake()
	if err != nil {
		return err
	}

	log.Info("Established connection with TM peer [" +
		string(hex.EncodeToString(h.secretConnection.RemotePubKey().Address())) +
		"] a.k.a. " + h.peerNodeInfo.Moniker)
	return nil
}

func (h *TendermintHandler) handshake() error {
	var (
		errc                        = make(chan error, 2)
		ourNodeInfo DefaultNodeInfo = DefaultNodeInfo{
			ProtocolVersion{App: 2, Block: 9, P2P: 5},
			string(hex.EncodeToString(h.privateKey.PubKey().Address())),
			"tcp://127.0.0.1:20006", //TODO Correct this
			"irishub",
			"0.32.2",
			[]byte{channelBc, channelCsSt, channelCsDC, channelCsVo,
				channelCsVs, channelMm, channelEv},
			"marlin-tendermint-connector",
			DefaultNodeInfoOther{"on", "tcp://0.0.0.0:26667"}, // TODO: Correct this
		}
	)

	go func(errc chan<- error, c net.Conn) {
		_, err := h.codec.MarshalBinaryLengthPrefixedWriter(c, ourNodeInfo)
		if err != nil {
			log.Error("Error encountered while sending handshake message")
		}
		errc <- err
	}(errc, h.secretConnection)
	go func(errc chan<- error, c net.Conn) {
		_, err := h.codec.UnmarshalBinaryLengthPrefixedReader(
			c,
			&h.peerNodeInfo,
			int64(10240), // 10KB MaxNodeInfoSize()
		)
		if err != nil {
			log.Error("Error encountered while recieving handshake message")
		}
		errc <- err
	}(errc, h.secretConnection)

	for i := 0; i < cap(errc); i++ {
		err := <-errc
		if err != nil {
			log.Error("Encountered error in handshake with TM core: ", err)
			return err
		}
	}
	return nil
}

func (h *TendermintHandler) beginServicing() error {
	// Register Messages
	RegisterPacket(h.codec)
	RegisterConsensusMessages(h.codec)

	// Create a P2P Connection
	h.p2pConnection = P2PConnection{
		conn:            h.secretConnection,
		bufConnReader:   bufio.NewReaderSize(h.secretConnection, 65535),
		bufConnWriter:   bufio.NewWriterSize(h.secretConnection, 65535),
		sendMonitor:     flow.New(0, 0),
		recvMonitor:     flow.New(0, 0),
		send:            make(chan struct{}, 1),
		pong:            make(chan struct{}, 1),
		doneSendRoutine: make(chan struct{}, 1),
		quitSendRoutine: make(chan struct{}, 1),
		quitRecvRoutine: make(chan struct{}, 1),
		flushTimer:      cmn.NewThrottleTimer("flush", 100*time.Millisecond),
		pingTimer:       time.NewTicker(30 * time.Second),
		pongTimeoutCh:   make(chan bool, 1),
	}

	// Start P2P Send and recieve routines + Status messages for message throughput
	go h.sendRoutine()
	go h.recvRoutine()
	go h.throughput.presentThroughput(5, h.signalShutThroughput)

	// Allow Irisnet messages from marlin Relay
	marlin.AllowServicedChainMessages(h.servicedChainId)
	return nil
}

func (h *TendermintHandler) sendRoutine() {
	log.Info("TMCore <- Connector Routine Started")

	for {
	SELECTION:
		select {
		case <-h.p2pConnection.pingTimer.C: // Send PING messages to TMCore
			_n, err := h.codec.MarshalBinaryLengthPrefixedWriter(h.p2pConnection.bufConnWriter, PacketPing{})
			if err != nil {
				break SELECTION
			}
			h.p2pConnection.sendMonitor.Update(int(_n))
			h.p2pConnection.pongTimer = time.AfterFunc(60*time.Second, func() {
				select {
				case h.p2pConnection.pongTimeoutCh <- true:
				default:
				}
			})

			err = h.p2pConnection.bufConnWriter.Flush()
			if err != nil {
				log.Error("Cannot flush buffer PingTimer: ", err)
				h.signalConnError <- struct{}{}
			}

		case <-h.p2pConnection.pong: // Send PONG messages to TMCore
			_n, err := h.codec.MarshalBinaryLengthPrefixedWriter(h.p2pConnection.bufConnWriter, PacketPong{})
			if err != nil {
				log.Error("Cannot send Pong message: ", err)
				break SELECTION
			}
			h.p2pConnection.sendMonitor.Update(int(_n))
			err = h.p2pConnection.bufConnWriter.Flush()
			if err != nil {
				log.Error("Cannot flush buffer: ", err)
				h.signalConnError <- struct{}{}
			}

		case timeout := <-h.p2pConnection.pongTimeoutCh: // Check if PONG messages are received in time
			if timeout {
				log.Error("Pong timeout, TM Core did not reply in time!")
				h.p2pConnection.stopPongTimer()
				h.signalConnError <- struct{}{}
			} else {
				h.p2pConnection.stopPongTimer()
			}

		case <-h.signalShutSend: // Signal to Shut down sendRoutine
			log.Info("node <- Connector Routine shutdown")
			h.p2pConnection.stopPongTimer()
			close(h.p2pConnection.doneSendRoutine)
			return

		case msg := <-h.marlinFrom: // Actual message packets from Marlin Relay (encoded in Marlin Tendermint Data Transfer Protocol v1)
			switch msg.Channel {
			case channelCsSt:
				var sendAhead = true

				// Unmarshalling Test
				for _, pkt := range msg.Packets {
					var cmsg ConsensusMessage
					err := h.codec.UnmarshalBinaryBare(pkt.Bytes, &cmsg)
					if err != nil {
						sendAhead = false
					}
				}

				if sendAhead {
					for _, pkt := range msg.Packets {
						_n, err := h.codec.MarshalBinaryLengthPrefixedWriter(
							h.p2pConnection.bufConnWriter,
							PacketMsg{
								ChannelID: byte(pkt.ChannelID),
								EOF:       byte(pkt.EOF),
								Bytes:     pkt.Bytes,
							})
						if err != nil {
							log.Error("Error occurred in sending data to TMCore: ", err)
							h.signalConnError <- struct{}{}
						}
						h.p2pConnection.sendMonitor.Update(int(_n))
						err = h.p2pConnection.bufConnWriter.Flush()
						if err != nil {
							log.Error("Cannot flush buffer: ", err)
							h.signalConnError <- struct{}{}
						}
					}
					h.throughput.putInfo("to", "CsSt+", 1)
				}
				if !sendAhead {
					h.throughput.putInfo("to", "CsSt-", uint32(len(msg.Packets)))
				}
			default:
				log.Error("node <- connector Not servicing undecipherable channel ", msg.Channel)
			}
		}
	}
}

func (h *TendermintHandler) recvRoutine() {
	log.Info("TMCore -> Connector Routine Started")
	// var recvBuffer []byte

FOR_LOOP:
	for {
		select {
		case <-h.signalShutRecv:
			log.Info("TMCore -> Connector Routine shutdown")
			break FOR_LOOP
		default:
		}
		h.p2pConnection.recvMonitor.Limit(20000, 5120000, true)

		/*
			Peek into bufConnReader for debugging

			if numBytes := c.bufConnReader.Buffered(); numBytes > 0 {
				bz, err := c.bufConnReader.Peek(cmn.MinInt(numBytes, 100))
				if err == nil {
					// return
				} else {
					log.Debug("Error peeking connection buffer ", "err ", err)
					// return nil
				}
				log.Info("Peek connection buffer ", "numBytes ", numBytes, " bz ", bz)
			}
		*/

		// Read packet type
		var packet Packet
		_n, err := h.codec.UnmarshalBinaryLengthPrefixedReader(
			h.p2pConnection.bufConnReader,
			&packet,
			int64(20000))

		h.p2pConnection.recvMonitor.Update(int(_n))

		// Unmarshalling test
		if err != nil {
			if err == io.EOF {
				log.Error("TMCore -> Connector Connection is closed (likely by the other side)")
			} else {
				log.Error("TMCore -> Connector Connection failed (reading byte): ", err)
			}
			h.signalConnError <- struct{}{}
			break FOR_LOOP
		}

		// Read more depending on packet type.
		switch pkt := packet.(type) {
		case PacketPing: // Received PING messages from TMCore
			select {
			case h.p2pConnection.pong <- PacketPong{}:
			default:
			}

		case PacketPong: // Received PONG messages from TMCore
			select {
			case h.p2pConnection.pongTimeoutCh <- false:
			default:
			}

		case PacketMsg: // Actual message packets from TMCore
			switch pkt.ChannelID {
			case channelBc:
				log.Debug("TMCore -> Connector Blockhain is not serviced")
			case channelCsSt:

				h.channelBuffer[channelCsSt] = append(h.channelBuffer[channelCsSt],
					marlinTypes.PacketMsg{
						ChannelID: uint32(pkt.ChannelID),
						EOF:       uint32(pkt.EOF),
						Bytes:     pkt.Bytes,
					})

				if pkt.EOF == byte(0x01) {
					message := marlinTypes.MarlinMessage{
						ChainID: h.servicedChainId,
						Channel: channelCsSt,
						Packets: h.channelBuffer[channelCsSt],
					}

					// h.throughput.putInfo("from", 1, 0, (message), 0)
					select {
					case h.marlinTo <- message:
					default:
						log.Warning("Too many messages in channel marlinTo. Dropping oldest messages")
						_ = <-h.marlinTo
						h.marlinTo <- message
					}
					h.throughput.putInfo("from", "CsSt+", uint32(len(h.channelBuffer[channelCsSt])))
					h.channelBuffer[channelCsSt] = h.channelBuffer[channelCsSt][:0]
				}
			case channelCsDC:
				log.Debug("TMCore -> Connector Consensensus Data Channel is not serviced")
			case channelCsVo:
				log.Debug("TMCore -> Connector Consensensus Vote Channel is not serviced")
			case channelCsVs:
				log.Debug("TMCore -> Connector Consensensus Vote Set Bits Channel is not serviced")
			case channelMm:
				log.Debug("TMCore -> Connector Mempool Channel is not serviced")
			case channelEv:
				log.Debug("TMCore -> Connector Evidence Channel is not serviced")
			default:
				log.Warning("TMCore -> Connector Unknown ChannelID Message recieved: ", pkt.ChannelID)
			}

		default:
			log.Error("TMCore -> Connector Unknown message type ", reflect.TypeOf(packet))
			log.Error("TMCore -> Connector Connection failed: ", err)
			h.signalConnError <- struct{}{}
			break FOR_LOOP
		}
	}

	// Cleanup
	close(h.p2pConnection.pong)
	for range h.p2pConnection.pong {
		// Drain
	}
}

func (c *P2PConnection) stopPongTimer() {
	if c.pongTimer != nil {
		_ = c.pongTimer.Stop()
		c.pongTimer = nil
	}
}

func (t *throughPutData) putInfo(direction string, key string, count uint32) {
	t.mu.Lock()
	switch direction {
	case "to":
		t.toTMCore[key] = t.toTMCore[key] + count
	case "from":
		t.fromTMCore[key] = t.fromTMCore[key] + count
	}
	t.mu.Unlock()
}

func (t *throughPutData) presentThroughput(sec time.Duration, shutdownCh chan struct{}) {
	for {
		time.Sleep(sec * time.Second)

		select {
		case <-shutdownCh:
			return
		default:
		}
		t.mu.Lock()
		log.Info(fmt.Sprintf("[Stats] To TMCore: %v; From TMCore: %v", t.toTMCore, t.fromTMCore))

		t.toTMCore = make(map[string]uint32)
		t.fromTMCore = make(map[string]uint32)
		t.mu.Unlock()
	}
}

// -------- STRUCTS -------

type TendermintHandler struct {
	servicedChainId      uint32
	listenPort           int
	isConnectionOutgoing bool
	peerAddr             string
	rpcAddr				 string
	privateKey           ed25519.PrivKeyEd25519
	codec                *amino.Codec
	baseConnection       net.Conn
	secretConnection     *conn.SecretConnection
	peerState            int
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

// TODO: Check if we even have use for this??
const (
	tmPeerStateNotConnected = iota
	tmPeerStateBaseConnected
	tmPeerStateConnectionUpgraded
	tmPeerStateConnectionHandshakeComplete
	tmPeerStateConnectionError
	tmPeerStateShutdown
)

type throughPutData struct {
	toTMCore   map[string]uint32
	fromTMCore map[string]uint32
	mu         sync.Mutex
}

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

// Consensus Message
type ConsensusMessage interface {
	ValidateBasic() error
}

func RegisterConsensusMessages(cdc *amino.Codec) {
	cdc.RegisterInterface((*ConsensusMessage)(nil), nil)
	cdc.RegisterConcrete(&NewRoundStepMessage{}, "tendermint/NewRoundStepMessage", nil)
	cdc.RegisterConcrete(&NewValidBlockMessage{}, "tendermint/NewValidBlockMessage", nil)
	cdc.RegisterConcrete(&ProposalMessage{}, "tendermint/Proposal", nil)
	cdc.RegisterConcrete(&ProposalPOLMessage{}, "tendermint/ProposalPOL", nil)
	cdc.RegisterConcrete(&BlockPartMessage{}, "tendermint/BlockPart", nil)
	cdc.RegisterConcrete(&VoteMessage{}, "tendermint/Vote", nil)
	cdc.RegisterConcrete(&HasVoteMessage{}, "tendermint/HasVote", nil)
	cdc.RegisterConcrete(&VoteSetMaj23Message{}, "tendermint/VoteSetMaj23", nil)
	cdc.RegisterConcrete(&VoteSetBitsMessage{}, "tendermint/VoteSetBits", nil)
}

//-------------------------------------

// NewRoundStepMessage is sent for every step taken in the ConsensusState.
// For every height/round/step transition
type NewRoundStepMessage struct {
	Height                int64
	Round                 int
	Step                  uint8
	SecondsSinceStartTime int
	LastCommitRound       int
}

// ValidateBasic performs basic validation.
func (m *NewRoundStepMessage) ValidateBasic() error {
	if m.Height < 0 {
		return errors.New("Negative Height")
	}
	if m.Round < 0 {
		return errors.New("Negative Round")
	}
	// if !m.Step.IsValid() {
	// 	return errors.New("Invalid Step")
	// }

	// NOTE: SecondsSinceStartTime may be negative

	if (m.Height == 1 && m.LastCommitRound != -1) ||
		(m.Height > 1 && m.LastCommitRound < -1) { // TODO: #2737 LastCommitRound should always be >= 0 for heights > 1
		return errors.New("Invalid LastCommitRound (for 1st block: -1, for others: >= 0)")
	}
	return nil
}

// String returns a string representation.
func (m *NewRoundStepMessage) String() string {
	return fmt.Sprintf("[NewRoundStep H:%v R:%v S:%v LCR:%v]",
		m.Height, m.Round, m.Step, m.LastCommitRound)
}

//-------------------------------------

// NewValidBlockMessage is sent when a validator observes a valid block B in some round r,
//i.e., there is a Proposal for block B and 2/3+ prevotes for the block B in the round r.
// In case the block is also committed, then IsCommit flag is set to true.
type PartSetHeader struct {
	Total int          `json:"total"`
	Hash  cmn.HexBytes `json:"hash"`
}

type BitArray struct {
	mtx   sync.Mutex
	Bits  int      `json:"bits"`  // NOTE: persisted via reflect, must be exported
	Elems []uint64 `json:"elems"` // NOTE: persisted via reflect, must be exported
}

// Size returns the number of bits in the bitarray
func (bA *BitArray) Size() int {
	if bA == nil {
		return 0
	}
	return bA.Bits
}

type NewValidBlockMessage struct {
	Height           int64
	Round            int
	BlockPartsHeader PartSetHeader
	BlockParts       BitArray
	IsCommit         bool
}

// ValidateBasic performs basic validation.
func (m *NewValidBlockMessage) ValidateBasic() error {
	if m.Height < 0 {
		return errors.New("Negative Height")
	}
	if m.Round < 0 {
		return errors.New("Negative Round")
	}
	// if err := m.BlockPartsHeader.ValidateBasic(); err != nil {
	// 	return fmt.Errorf("Wrong BlockPartsHeader: %v", err)
	// }
	if m.BlockParts.Size() == 0 {
		return errors.New("Empty BlockParts")
	}
	if m.BlockParts.Size() != m.BlockPartsHeader.Total {
		return fmt.Errorf("BlockParts bit array size %d not equal to BlockPartsHeader.Total %d",
			m.BlockParts.Size(),
			m.BlockPartsHeader.Total)
	}
	// if m.BlockParts.Size() > types.MaxBlockPartsCount {
	// 	return errors.Errorf("BlockParts bit array is too big: %d, max: %d", m.BlockParts.Size(), types.MaxBlockPartsCount)
	// }
	return nil
}

// String returns a string representation.
func (m *NewValidBlockMessage) String() string {
	return fmt.Sprintf("[ValidBlockMessage H:%v R:%v BP:%v BA:%v IsCommit:%v]",
		m.Height, m.Round, m.BlockPartsHeader, m.BlockParts, m.IsCommit)
}

//-------------------------------------

type Proposal struct {
	Type      byte
	Height    int64     `json:"height"`
	Round     int       `json:"round"`
	POLRound  int       `json:"pol_round"` // -1 if null.
	BlockID   BlockID   `json:"block_id"`
	Timestamp time.Time `json:"timestamp"`
	Signature []byte    `json:"signature"`
}

type BlockID struct {
	Hash        cmn.HexBytes  `json:"hash"`
	PartsHeader PartSetHeader `json:"parts"`
}

// ProposalMessage is sent when a new block is proposed.
type ProposalMessage struct {
	Proposal Proposal
}

// ValidateBasic performs basic validation.
func (m *ProposalMessage) ValidateBasic() error {
	return nil
}

// String returns a string representation.
func (m *ProposalMessage) String() string {
	return fmt.Sprintf("[Proposal %v]", m.Proposal)
}

//-------------------------------------

// ProposalPOLMessage is sent when a previous proposal is re-proposed.
type ProposalPOLMessage struct {
	Height           int64
	ProposalPOLRound int
	ProposalPOL      *cmn.BitArray
}

// ValidateBasic performs basic validation.
func (m *ProposalPOLMessage) ValidateBasic() error {
	if m.Height < 0 {
		return errors.New("Negative Height")
	}
	if m.ProposalPOLRound < 0 {
		return errors.New("Negative ProposalPOLRound")
	}
	if m.ProposalPOL.Size() == 0 {
		return errors.New("Empty ProposalPOL bit array")
	}
	// if m.ProposalPOL.Size() > types.MaxVotesCount {
	// 	return errors.Errorf("ProposalPOL bit array is too big: %d, max: %d", m.ProposalPOL.Size(), types.MaxVotesCount)
	// }
	return nil
}

// String returns a string representation.
func (m *ProposalPOLMessage) String() string {
	return fmt.Sprintf("[ProposalPOL H:%v POLR:%v POL:%v]", m.Height, m.ProposalPOLRound, m.ProposalPOL)
}

//-------------------------------------

type Part struct {
	Index int                `json:"index"`
	Bytes cmn.HexBytes       `json:"bytes"`
	Proof merkle.SimpleProof `json:"proof"`

	// Cache
	hash []byte
}

// BlockPartMessage is sent when gossipping a piece of the proposed block.
type BlockPartMessage struct {
	Height int64
	Round  int
	Part   Part
}

// ValidateBasic performs basic validation.
func (m *BlockPartMessage) ValidateBasic() error {
	if m.Height < 0 {
		return errors.New("Negative Height")
	}
	if m.Round < 0 {
		return errors.New("Negative Round")
	}
	// if err := m.Part.ValidateBasic(); err != nil {
	// 	return fmt.Errorf("Wrong Part: %v", err)
	// }
	return nil
}

// String returns a string representation.
func (m *BlockPartMessage) String() string {
	return fmt.Sprintf("[BlockPart H:%v R:%v P:%v]", m.Height, m.Round, m.Part)
}

//-------------------------------------

// Vote represents a prevote, precommit, or commit vote from validators for
// consensus.
type Vote struct {
	Type             byte         `json:"type"`
	Height           int64        `json:"height"`
	Round            int          `json:"round"`
	BlockID          BlockID      `json:"block_id"` // zero if vote is nil.
	Timestamp        time.Time    `json:"timestamp"`
	ValidatorAddress cmn.HexBytes `json:"validator_address"`
	ValidatorIndex   int          `json:"validator_index"`
	Signature        []byte       `json:"signature"`
}

// VoteMessage is sent when voting for a proposal (or lack thereof).
type VoteMessage struct {
	Vote Vote
}

// ValidateBasic performs basic validation.
func (m *VoteMessage) ValidateBasic() error {
	return nil
}

// String returns a string representation.
func (m *VoteMessage) String() string {
	return fmt.Sprintf("[Vote %v]", m.Vote)
}

//-------------------------------------

// HasVoteMessage is sent to indicate that a particular vote has been received.

type HasVoteMessage struct {
	Height int64
	Round  int
	Type   byte
	Index  int
}

// ValidateBasic performs basic validation.
func (m *HasVoteMessage) ValidateBasic() error {
	if m.Height < 0 {
		return errors.New("Negative Height")
	}
	if m.Round < 0 {
		return errors.New("Negative Round")
	}
	// if !types.IsVoteTypeValid(m.Type) {
	// 	return errors.New("Invalid Type")
	// }
	if m.Index < 0 {
		return errors.New("Negative Index")
	}
	return nil
}

// String returns a string representation.
func (m *HasVoteMessage) String() string {
	return fmt.Sprintf("[HasVote VI:%v V:{%v/%02d/%v}]", m.Index, m.Height, m.Round, m.Type)
}

//-------------------------------------

// VoteSetMaj23Message is sent to indicate that a given BlockID has seen +2/3 votes.
type VoteSetMaj23Message struct {
	Height  int64
	Round   int
	Type    byte
	BlockID BlockID
}

// ValidateBasic performs basic validation.
func (m *VoteSetMaj23Message) ValidateBasic() error {
	if m.Height < 0 {
		return errors.New("Negative Height")
	}
	if m.Round < 0 {
		return errors.New("Negative Round")
	}
	// if !types.IsVoteTypeValid(m.Type) {
	// 	return errors.New("Invalid Type")
	// }
	// if err := m.BlockID.ValidateBasic(); err != nil {
	// 	return fmt.Errorf("Wrong BlockID: %v", err)
	// }
	return nil
}

// String returns a string representation.
func (m *VoteSetMaj23Message) String() string {
	return fmt.Sprintf("[VSM23 %v/%02d/%v %v]", m.Height, m.Round, m.Type, m.BlockID)
}

//-------------------------------------

// VoteSetBitsMessage is sent to communicate the bit-array of votes seen for the BlockID.
type VoteSetBitsMessage struct {
	Height  int64
	Round   int
	Type    byte
	BlockID BlockID
	Votes   *cmn.BitArray
}

// ValidateBasic performs basic validation.
func (m *VoteSetBitsMessage) ValidateBasic() error {
	if m.Height < 0 {
		return errors.New("Negative Height")
	}
	if m.Round < 0 {
		return errors.New("Negative Round")
	}
	// if !types.IsVoteTypeValid(m.Type) {
	// 	return errors.New("Invalid Type")
	// }
	// if err := m.BlockID.ValidateBasic(); err != nil {
	// 	return fmt.Errorf("Wrong BlockID: %v", err)
	// }
	// NOTE: Votes.Size() can be zero if the node does not have any
	// if m.Votes.Size() > types.MaxVotesCount {
	// 	return fmt.Errorf("Votes bit array is too big: %d, max: %d", m.Votes.Size(), types.MaxVotesCount)
	// }
	return nil
}

// String returns a string representation.
func (m *VoteSetBitsMessage) String() string {
	return fmt.Sprintf("[VSB %v/%02d/%v %v %v]", m.Height, m.Round, m.Type, m.BlockID, m.Votes)
}

const (
	channelBc   = byte(0x40) //bc.BlockchainChannel,
	channelCsSt = byte(0x20) //cs.StateChannel,
	channelCsDC = byte(0x21) //cs.DataChannel,
	channelCsVo = byte(0x22) //cs.VoteChannel,
	channelCsVs = byte(0x23) //cs.VoteSetBitsChannel,
	channelMm   = byte(0x30) //mempl.MempoolChannel,
	channelEv   = byte(0x38) //evidence.EvidenceChannel,
)
