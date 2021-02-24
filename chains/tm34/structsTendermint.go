package tm34

const (
	channelBc   = byte(0x40) //bc.BlockchainChannel,
	channelCsSt = byte(0x20) //cs.StateChannel,
	channelCsDc = byte(0x21) //cs.DataChannel,
	channelCsVo = byte(0x22) //cs.VoteChannel,
	channelCsVs = byte(0x23) //cs.VoteSetBitsChannel,
	channelMm   = byte(0x30) //mempl.MempoolChannel,
	channelEv   = byte(0x38) //evidence.EvidenceChannel,
)
