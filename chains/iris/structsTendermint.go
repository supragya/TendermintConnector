package iris

import (
	"time"

	"github.com/supragya/TendermintConnector/chains/iris/crypto"
	"github.com/supragya/TendermintConnector/chains/iris/crypto/merkle"
	"github.com/supragya/TendermintConnector/chains/iris/libs/bits"
	tmjson "github.com/supragya/TendermintConnector/chains/iris/libs/json"
	"github.com/supragya/TendermintConnector/chains/iris/libs/protoio"
	tmproto "github.com/supragya/TendermintConnector/chains/iris/proto/tendermint/types"
)

const (
	channelBc   = byte(0x40) //bc.BlockchainChannel,
	channelCsSt = byte(0x20) //cs.StateChannel,
	channelCsDc = byte(0x21) //cs.DataChannel,
	channelCsVo = byte(0x22) //cs.VoteChannel,
	channelCsVs = byte(0x23) //cs.VoteSetBitsChannel,
	channelMm   = byte(0x30) //mempl.MempoolChannel,
	channelEv   = byte(0x38) //evidence.EvidenceChannel,
)

func init() {
	tmjson.RegisterType(&NewRoundStepMessage{}, "tendermint/NewRoundStepMessage")
	tmjson.RegisterType(&NewValidBlockMessage{}, "tendermint/NewValidBlockMessage")
	tmjson.RegisterType(&ProposalMessage{}, "tendermint/Proposal")
	tmjson.RegisterType(&ProposalPOLMessage{}, "tendermint/ProposalPOL")
	tmjson.RegisterType(&BlockPartMessage{}, "tendermint/BlockPart")
	tmjson.RegisterType(&VoteMessage{}, "tendermint/Vote")
	tmjson.RegisterType(&HasVoteMessage{}, "tendermint/HasVote")
	tmjson.RegisterType(&VoteSetMaj23Message{}, "tendermint/VoteSetMaj23")
	tmjson.RegisterType(&VoteSetBitsMessage{}, "tendermint/VoteSetBits")
}

//-------------------------------------

// NewRoundStepMessage is sent for every step taken in the ConsensusState.
// For every height/round/step transition
type NewRoundStepMessage struct {
	Height                int64
	Round                 int32
	Step                  int8
	SecondsSinceStartTime int64
	LastCommitRound       int32
}

//-------------------------------------
type PartSetHeader struct {
	Total uint32 `json:"total"`
	Hash  []byte `json:"hash"`
}

// ToProto converts PartSetHeader to protobuf
func (psh *PartSetHeader) ToProto() tmproto.PartSetHeader {
	if psh == nil {
		return tmproto.PartSetHeader{}
	}

	return tmproto.PartSetHeader{
		Total: psh.Total,
		Hash:  psh.Hash,
	}
}

// NewValidBlockMessage is sent when a validator observes a valid block B in some round r,
// i.e., there is a Proposal for block B and 2/3+ prevotes for the block B in the round r.
// In case the block is also committed, then IsCommit flag is set to true.
type NewValidBlockMessage struct {
	Height             int64
	Round              int32
	BlockPartSetHeader PartSetHeader
	BlockParts         *bits.BitArray
	IsCommit           bool
}

//-------------------------------------
type BlockID struct {
	Hash          []byte        `json:"hash"`
	PartSetHeader PartSetHeader `json:"parts"`
}

func (blockID *BlockID) ToProto() tmproto.BlockID {
	if blockID == nil {
		return tmproto.BlockID{}
	}

	return tmproto.BlockID{
		Hash:          blockID.Hash,
		PartSetHeader: blockID.PartSetHeader.ToProto(),
	}
}

type Proposal struct {
	Type      tmproto.SignedMsgType
	Height    int64     `json:"height"`
	Round     int32     `json:"round"`     // there can not be greater than 2_147_483_647 rounds
	POLRound  int32     `json:"pol_round"` // -1 if null.
	BlockID   BlockID   `json:"block_id"`
	Timestamp time.Time `json:"timestamp"`
	Signature []byte    `json:"signature"`
}

// ProposalMessage is sent when a new block is proposed.
type ProposalMessage struct {
	Proposal *Proposal
}

//-------------------------------------

// ProposalPOLMessage is sent when a previous proposal is re-proposed.
type ProposalPOLMessage struct {
	Height           int64
	ProposalPOLRound int32
	ProposalPOL      *bits.BitArray
}

//-------------------------------------
type Part struct {
	Index uint32       `json:"index"`
	Bytes []byte       `json:"bytes"`
	Proof merkle.Proof `json:"proof"`
}

// BlockPartMessage is sent when gossipping a piece of the proposed block.
type BlockPartMessage struct {
	Height int64
	Round  int32
	Part   *Part
}

//-------------------------------------
type Vote struct {
	Type             tmproto.SignedMsgType `json:"type"`
	Height           int64                 `json:"height"`
	Round            int32                 `json:"round"`    // assume there will not be greater than 2_147_483_647 rounds
	BlockID          BlockID               `json:"block_id"` // zero if vote is nil.
	Timestamp        time.Time             `json:"timestamp"`
	ValidatorAddress crypto.Address        `json:"validator_address"`
	ValidatorIndex   int32                 `json:"validator_index"`
	Signature        []byte                `json:"signature"`
}

// VoteMessage is sent when voting for a proposal (or lack thereof).
type VoteMessage struct {
	Vote *Vote
}

// ToProto converts the handwritten type to proto generated type
// return type, nil if everything converts safely, otherwise nil, error
func (vote *Vote) ToProto() *tmproto.Vote {
	if vote == nil {
		return nil
	}

	return &tmproto.Vote{
		Type:             vote.Type,
		Height:           vote.Height,
		Round:            vote.Round,
		BlockID:          vote.BlockID.ToProto(),
		Timestamp:        vote.Timestamp,
		ValidatorAddress: vote.ValidatorAddress,
		ValidatorIndex:   vote.ValidatorIndex,
		Signature:        vote.Signature,
	}
}

//-------------------------------------

// HasVoteMessage is sent to indicate that a particular vote has been received.
type HasVoteMessage struct {
	Height int64
	Round  int32
	Type   tmproto.SignedMsgType
	Index  int32
}

//-------------------------------------

// VoteSetMaj23Message is sent to indicate that a given BlockID has seen +2/3 votes.
type VoteSetMaj23Message struct {
	Height  int64
	Round   int32
	Type    tmproto.SignedMsgType
	BlockID BlockID
}

//-------------------------------------

// VoteSetBitsMessage is sent to communicate the bit-array of votes seen for the BlockID.
type VoteSetBitsMessage struct {
	Height  int64
	Round   int32
	Type    tmproto.SignedMsgType
	BlockID BlockID
	Votes   *bits.BitArray
}

//-------------------------------------

// CanonicalizeVote transforms the given PartSetHeader to a CanonicalPartSetHeader.
func CanonicalizePartSetHeader(psh tmproto.PartSetHeader) tmproto.CanonicalPartSetHeader {
	return tmproto.CanonicalPartSetHeader(psh)
}

func CanonicalizeBlockID(bid tmproto.BlockID) *tmproto.CanonicalBlockID {
	rbid, err := BlockIDFromProto(&bid)
	if err != nil {
		panic(err)
	}
	var cbid *tmproto.CanonicalBlockID
	if rbid == nil {
		cbid = nil
	} else {
		cbid = &tmproto.CanonicalBlockID{
			Hash:          bid.Hash,
			PartSetHeader: CanonicalizePartSetHeader(bid.PartSetHeader),
		}
	}

	return cbid
}

// CanonicalizeVote transforms the given Vote to a CanonicalVote, which does
// not contain ValidatorIndex and ValidatorAddress fields.
func CanonicalizeVote(chainID string, vote *tmproto.Vote) tmproto.CanonicalVote {
	return tmproto.CanonicalVote{
		Type:      vote.Type,
		Height:    vote.Height,       // encoded as sfixed64
		Round:     int64(vote.Round), // encoded as sfixed64
		BlockID:   CanonicalizeBlockID(vote.BlockID),
		Timestamp: vote.Timestamp,
		ChainID:   chainID,
	}
}

func VoteSignBytes(chainID string, vote *tmproto.Vote) []byte {
	pb := CanonicalizeVote(chainID, vote)
	bz, err := protoio.MarshalDelimited(&pb)
	if err != nil {
		panic(err)
	}

	return bz
}
