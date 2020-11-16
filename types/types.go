package types

type MarlinMessage struct {
	ChainID uint32
	Channel byte
	Packets []PacketMsg
}

type PacketMsg struct {
	ChannelID uint32
	EOF       uint32 // 1 means message ends here.
	Bytes     []byte
}

// DANGER - Do not change mappings for serviced chains.
// These are vital for encoding / decoding to correct chains.
var ServicedChains = map[string]uint32{
	"irisnet-0.16.3-mainnet": 1,
}