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
	marlinTypes "github.com/supragya/tendermint_connector/types"

	// Tendermint Core Chains
	"github.com/supragya/tendermint_connector/chains"
	"github.com/supragya/tendermint_connector/chains/irisnet"
)

var peerPort, rpcPort, marlinPort, listenPortPeer int
var peerIP, marlinIP, keyFile, chain, fileLocation, marlinUdsFile string
var isConnectionOutgoing, isGenerate, isMarlinconnectionOutgoing bool

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

func findAndRunDataConnectHandler(node chains.NodeType,
	peerAddr string,
	marlinTo chan marlinTypes.MarlinMessage,
	marlinFrom chan marlinTypes.MarlinMessage,
	isConnectionOutgoing bool,
	keyFile string,
	listenPortPeer int) {
	log.Info("Trying to match ", node, " to available TMCore Data Connect handlers")

	switch node {
	case irisnet.ServicedTMCore:
		log.Info("Attaching Irisnet TM Handler to service given TM core")
		irisnet.RunDataConnect(peerAddr, marlinTo, marlinFrom, isConnectionOutgoing, keyFile, listenPortPeer)
	default:
		log.Error("Cannot find any handler for ", node)
		return
	}
}

func findAndRunSpamFilterHandler(node chains.NodeType,
	rpcAddr string,
	marlinTo chan marlinTypes.MarlinMessage,
	marlinFrom chan marlinTypes.MarlinMessage) {
	log.Info("Trying to match ", node, " to available TMCore Spamfilter handlers")

	switch node {
	case irisnet.ServicedTMCore:
		log.Info("Attaching Irisnet TM spamfilter")
		// TODO - Only one SpamFilter ?? should we spam more than one goroutine for this? Check if goroutine safe - v0.2 prerelease
		irisnet.RunSpamFilter(rpcAddr, marlinTo, marlinFrom)
	default:
		log.Error("Cannot find any spamfilter for ", node)
		return
	}
}
