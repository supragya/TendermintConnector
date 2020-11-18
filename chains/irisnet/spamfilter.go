package irisnet

import (
	"os"
	"time"

	log "github.com/sirupsen/logrus"
	marlinTypes "github.com/supragya/tendermint_connector/types"

	// Protocols
	"github.com/supragya/tendermint_connector/marlin"
)

// RunSpamFilter serves as the entry point for a TM Core handler when serving as a spamfilter
func RunSpamFilter(rpcAddr string,
	marlinTo chan marlinTypes.MarlinMessage,
	marlinFrom chan marlinTypes.MarlinMessage) {
	log.Info("Starting Irisnet Tendermint SpamFilter - 0.16.3-d83fc038-2-mainnet")

	handler, err := createTMHandler("0.0.0.0:0", rpcAddr, marlinTo, marlinFrom, false, 0)
	if err != nil {
		log.Error("Error encountered while creating TM Handler: ", err)
		os.Exit(1)
	}

	marlin.AllowServicedChainMessages(handler.servicedChainId)

	err = handler.beginServicingSpamFilter()
	if err != nil {
		log.Error("Error encountered while servicing spam filter: ", err)
		os.Exit(1)
	}

	// TODO - This is stopping just because of time sleep added. Remove this later on. - v0.1 prerelease
	time.Sleep(10000 * time.Second)
}

func (h *TendermintHandler) beginServicingSpamFilter() error {
	log.Info("Running TM side spam filter")
	// Register Messages
	RegisterPacket(h.codec)
	RegisterConsensusMessages(h.codec)

	// TODO - SpamFilter never has to consult RPC server currently - since only CsSt+ is supported, write for that. v0.2 prerelease

	for msg := range h.marlinFrom {
		switch msg.Channel {
		case channelBc:
			log.Debug("TMCore <-> Marlin Blockhain is not serviced")
			h.marlinTo <- h.spamVerdictMessage(msg, false)
		case channelCsSt:
			// Construct complete message from packets
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
				h.marlinTo <- h.spamVerdictMessage(msg, true)
			} else {
				h.marlinTo <- h.spamVerdictMessage(msg, false)
			}
			h.marlinTo <- h.spamVerdictMessage(msg, false)
		case channelCsDC:
			h.marlinTo <- h.spamVerdictMessage(msg, false)
			log.Debug("TMCore <-> Marlin Consensensus Data Channel is not serviced")
		case channelCsVo:
			h.marlinTo <- h.spamVerdictMessage(msg, false)
			log.Debug("TMCore <-> Marlin Consensensus Vote Channel is not serviced")
		case channelCsVs:
			h.marlinTo <- h.spamVerdictMessage(msg, false)
			log.Debug("TMCore <-> Marlin Consensensus Vote Set Bits Channel is not serviced")
		case channelMm:
			h.marlinTo <- h.spamVerdictMessage(msg, false)
			log.Debug("TMCore <-> Marlin Mempool Channel is not serviced")
		case channelEv:
			h.marlinTo <- h.spamVerdictMessage(msg, false)
			log.Debug("TMCore <-> MarlinEvidence Channel is not serviced")
		default:
			h.marlinTo <- h.spamVerdictMessage(msg, false)
		}
	}

	return nil
}

func (h *TendermintHandler) spamVerdictMessage(msg marlinTypes.MarlinMessage, allow bool) marlinTypes.MarlinMessage {
	if allow {
		return marlinTypes.MarlinMessage{
			ChainID:  h.servicedChainId,
			Channel:  byte(0x01),
			PacketId: msg.PacketId,
		}
	} else {
		return marlinTypes.MarlinMessage{
			ChainID:  h.servicedChainId,
			Channel:  byte(0x00),
			PacketId: msg.PacketId,
		}
	}
}
