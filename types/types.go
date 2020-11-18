package types

type MarlinMessage struct {
	ChainID  uint32
	Channel  byte        // ChannelID maps to TM channels for data tfr use, 0x01 for allow/0x00 for deny during spamcheck
	PacketId uint64      // Used only for spam checks
	Packets  []PacketMsg // Only available during data transfer
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
