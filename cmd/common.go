/*
Copyright © 2020 Supragya Raj <supragyaraj@gmail.com>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package cmd

import (
	"encoding/json"
	"io/ioutil"
	"net/http"

	log "github.com/sirupsen/logrus"
	"github.com/supragya/tendermint_connector/types"

	// Tendermint Core Chains
	"github.com/supragya/tendermint_connector/chains"
	"github.com/supragya/tendermint_connector/chains/irisnet"
)

var peerPort, rpcPort, marlinPort, listenPort int
var peerIP, marlinIP, keyFile, chain, fileLocation string
var isConnectionOutgoing, isGenerate bool

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
		"nodeType": chains.NodeType{
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

func invokeHandler(node chains.NodeType,
	peerAddr string,
	marlinTo chan types.MarlinMessage,
	marlinFrom chan types.MarlinMessage,
	isConnectionOutgoing bool,
	keyFile string,
	listenPort int) {
	log.Info("Trying to match ", node, " to available tendermint core handlers")

	switch node {
	case irisnet.ServicedTMCore:
		log.Info("Attaching Irisnet TM Handler to service given TM core")
		irisnet.Run(peerAddr, marlinTo, marlinFrom, isConnectionOutgoing, keyFile, listenPort)
	default:
		log.Error("Cannot find any handler for ", node)
		return
	}
}

func invokeSpamFilter(node chains.NodeType,
	rpcAddr string,
	marlinTo chan types.MarlinMessage,
	marlinFrom chan types.MarlinMessage) {
	log.Info("Trying to match ", node, " to available tendermint core spamfilters")

	switch node {
	case irisnet.ServicedTMCore:
		log.Info("Attaching Irisnet TM spamfilter")
		irisnet.RunSpamFilter(rpcAddr, marlinTo, marlinFrom)
	default:
		log.Error("Cannot find any spamfilter for ", node)
		return
	}
}