package irisnet

import (
	"time"
	"os"

	log "github.com/sirupsen/logrus"
	marlinTypes "github.com/supragya/tendermint_connector/types"

	// Protocols
	"github.com/supragya/tendermint_connector/marlin"
)

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

	log.Info("This is stopping just because of time sleep added. Remove this later on.")
	time.Sleep(10000 * time.Second)
}

func (h *TendermintHandler) beginServicingSpamFilter() error {
	log.Info("Running TM side spam filter")
	// Register Messages
	RegisterPacket(h.codec)
	RegisterConsensusMessages(h.codec)

	// allowMessage and blockMessage to marlin side connector
	allowMessage := marlinTypes.MarlinMessage{
		ChainID:	h.servicedChainId,
		Channel:	byte(0x01),
	}
	blockMessage := marlinTypes.MarlinMessage{
		ChainID:	h.servicedChainId,
		Channel:	byte(0x00),
	}

	for msg := range h.marlinFrom {
		switch msg.Channel {
		case channelBc:
			log.Debug("TMCore <-> Marlin Blockhain is not serviced")
			h.marlinTo <- blockMessage
		case channelCsSt:
			h.marlinTo <- allowMessage
		case channelCsDC:
			h.marlinTo <- blockMessage
			log.Debug("TMCore <-> Marlin Consensensus Data Channel is not serviced")
		case channelCsVo:
			h.marlinTo <- blockMessage
			log.Debug("TMCore <-> Marlin Consensensus Vote Channel is not serviced")
		case channelCsVs:
			h.marlinTo <- blockMessage
			log.Debug("TMCore <-> Marlin Consensensus Vote Set Bits Channel is not serviced")
		case channelMm:
			h.marlinTo <- blockMessage
			log.Debug("TMCore <-> Marlin Mempool Channel is not serviced")
		case channelEv:
			h.marlinTo <- blockMessage
			log.Debug("TMCore <-> MarlinEvidence Channel is not serviced")
		default:
			h.marlinTo <- blockMessage
		}
	}

	return nil
}