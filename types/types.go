package types

type MarlinMessage struct {
	ChainID  uint8
	Channel  byte        // ChannelID maps to TM channels for data tfr use, 0x01 for allow/0x00 for deny during spamcheck
	PacketId uint64      // Used only for spam checks
	Packets  []PacketMsg // Only available during data transfer
	Packets2 []PacketMsg // Only available during data transfer
}

type PacketMsg struct {
	EOF   uint32 // 1 means message ends here.
	Bytes []byte
}

// DANGER - Do not change mappings for serviced chains. APPEND ONLY
// These are vital for encoding / decoding to correct chains.
var ServicedChains = map[string]uint8{
	"irisnet-0.16.3-mainnet": 1,
	"cosmos-3-mainnet":       2,
	"tm34":                   3,
	"irisnet-1.0-mainnet":    6, // earlier used to be 4, made 6 for network changes
	"cosmoshub-4-mainnet":    7, // earlier used to be 5, made 7 for network changes
}
