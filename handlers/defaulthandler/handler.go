package defaulthandler

import (
	log "github.com/sirupsen/logrus"
	"github.com/supragya/tendermint_connector/handlers"
)

var ServicedTMCore handlers.NodeType = handlers.NodeType{Version: "0.32.8", Network: "test-chain-WZxV62", ProtocolVersionApp: "1", ProtocolVersionBlock: "10", ProtocolVersionP2p: "7"}

func Run() {
	log.Info("Starting Default Tendermint Core Handler")
}
