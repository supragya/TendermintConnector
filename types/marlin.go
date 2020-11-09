package types

type MarlinMessage struct {
	ChainID	uint32
	Channel uint32
	Data 	[]byte
}

// DANGER - Do not change mappings for serviced chains.
// These are vital for encoding / decoding to correct chains.
var ServicedChains = map[string]uint32{
	"irisnet-0.16.3-mainnet": 1,
}

const (
	ChannelConsensusState uint32 = 1
)