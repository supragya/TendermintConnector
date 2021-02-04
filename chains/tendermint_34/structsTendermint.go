package irisnet

import (
	"net"
	"bufio"
	"sync"
	"fmt"
	"time"
	"errors"

	proto3 "github.com/golang/protobuf"
	cmn "github.com/supragya/tendermint_connector/chains/irisnet/libs/common"
	flow "github.com/supragya/tendermint_connector/chains/irisnet/libs/flowrate"
	"github.com/tendermint/tendermint/crypto/merkle"
)

type ProtocolVersion struct {
	P2P   uint64 `json:"p2p"`
	Block uint64 `json:"block"`
	App   uint64 `json:"app"`
}

type DefaultNodeInfo struct {
	ProtocolVersion ProtocolVersion `json:"protocol_version"`

	ID_        string `json:"id"`          // authenticated ideamino "github.com/tendermint/go-amino"ntifier
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

func RegisterPacket(cdc *proto3) {
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

func RegisterConsensusMessages(cdc *proto3) {
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
		(m.Height > 1 && m.LastCommitRound < -1) {
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
	channelCsDc = byte(0x21) //cs.DataChannel,
	channelCsVo = byte(0x22) //cs.VoteChannel,
	channelCsVs = byte(0x23) //cs.VoteSetBitsChannel,
	channelMm   = byte(0x30) //mempl.MempoolChannel,
	channelEv   = byte(0x38) //evidence.EvidenceChannel,
)
