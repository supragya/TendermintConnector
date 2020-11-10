package connector

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	log "github.com/sirupsen/logrus"
	"github.com/supragya/tendermint_connector/handlers"
	"github.com/supragya/tendermint_connector/types"

	"github.com/supragya/tendermint_connector/marlin"

	// Tendermint Core Handlers
	"github.com/supragya/tendermint_connector/handlers/defaulthandler"
	"github.com/supragya/tendermint_connector/handlers/irisnet"
)

func getRPCNodeStatus(rpcAddr string) (map[string]interface{}, error) {
	log.Info("Retrieving Information from RPC server")
	var data map[string]interface{}

	resp, err := http.Get("http://" + rpcAddr + "/status")
	if err != nil {
		log.Error("Cannot retrieve node information from RPC server. Is tendermint node running?")
		return data, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Error(err)
		return data, err
	}
	json.Unmarshal(body, &data)

	return data, nil
}

func extractNodeInfo(rpcNodeStatus map[string]interface{}) map[string]interface{} {
	nodeResult := rpcNodeStatus["result"].(map[string]interface{})["node_info"].(map[string]interface{})
	return map[string]interface{}{
		"nodeType": handlers.NodeType{
			Version:              nodeResult["version"].(string),
			Network:              nodeResult["network"].(string),
			ProtocolVersionApp:   nodeResult["protocol_version"].(map[string]interface{})["app"].(string),
			ProtocolVersionBlock: nodeResult["protocol_version"].(map[string]interface{})["block"].(string),
			ProtocolVersionP2p:   nodeResult["protocol_version"].(map[string]interface{})["p2p"].(string),
		},
		"moniker": nodeResult["moniker"],
		"id":      nodeResult["id"],
	}
}

func invokeHandler(node handlers.NodeType, 
					peerAddr string, 
					marlinTo chan<- types.MarlinMessage, 
					marlinFrom <-chan types.MarlinMessage,
					isConnectionOutgoing bool) {
	log.Info("Trying to match ", node, " to available tendermint core handlers")

	switch node {
	case irisnet.ServicedTMCore:
		log.Info("Attaching Irisnet TM Handler to service given TM core")
		irisnet.Run(peerAddr, marlinTo, marlinFrom, isConnectionOutgoing)
	case defaulthandler.ServicedTMCore:
		log.Info("Attaching Default TM Handler to the service given TM core")
		defaulthandler.Run()
	default:
		log.Error("Cannot find any handler for ", node)
		return
	}
}

func Connect(peerIP string,
			peerPort int, 
			rpcPort int, 
			marlinIP string,
			marlinPort int,
			isConnectionOutgoing bool) {
	peerAddr := fmt.Sprintf("%v:%v", peerIP, peerPort)
	rpcAddr := fmt.Sprintf("%v:%v", peerIP, rpcPort)
	marlinAddr := fmt.Sprintf("%v:%v", marlinIP, marlinPort)

	if isConnectionOutgoing {
		log.Info("Configuring to DIAL Peer (TMCore) connection address: ", peerAddr, "; rpc address: ", rpcAddr)
	} else {
		log.Info("Configuring to LISTEN Peer (TMCore) connection address: ", peerAddr, "; rpc address: ", rpcAddr)
	}
	log.Info("Marlin connection address: ", marlinAddr)

	nodeStatus, err := getRPCNodeStatus(rpcAddr)
	if err != nil {
		return
	}

	// Channels
	marlinTo := make(chan types.MarlinMessage, 1000)
	marlinFrom := make(chan types.MarlinMessage, 1000)

	nodeInfo := extractNodeInfo(nodeStatus)

	marlin.Run(marlinAddr, marlinTo, marlinFrom)

	invokeHandler(nodeInfo["nodeType"].(handlers.NodeType), peerAddr, marlinTo, marlinFrom, isConnectionOutgoing)
}
