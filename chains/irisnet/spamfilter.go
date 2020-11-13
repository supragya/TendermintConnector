package irisnet

import (
	"time"
	"os"

	log "github.com/sirupsen/logrus"
	marlinTypes "github.com/supragya/tendermint_connector/types"
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

	err = handler.beginServicingSpamFilter()
	if err != nil {
		log.Error("Error encountered while servicing spam filter: ", err)
		os.Exit(1)
	}

	log.Info("This is stopping just because of time sleep added. Remove this later on.")
	time.Sleep(10000 * time.Second)
}

func (h *TendermintHandler) beginServicingSpamFilter() error {
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
		case channelCsSt:
			h.marlinTo <- allowMessage
		default:
			log.Error("node <- connector Not servicing undecipherable channel ", msg.Channel)
			_ = blockMessage
		}
	}

	return nil
}