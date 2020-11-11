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
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/spf13/cobra"
	log "github.com/sirupsen/logrus"
	"github.com/supragya/tendermint_connector/types"

	"github.com/supragya/tendermint_connector/marlin"

	// Tendermint Core Chains
	"github.com/supragya/tendermint_connector/chains"
	"github.com/supragya/tendermint_connector/chains/irisnet"

)

var peerPort, rpcPort, marlinPort, listenPort int
var peerIP, marlinIP, keyFile string
var isConnectionOutgoing bool

// connectCmd represents the connect command
var connectCmd = &cobra.Command{
	Use:   "connect",
	Short: "Connect to a TM Core",
	Long:  `Connect to a TM Core`,
	Run: func(cmd *cobra.Command, args []string) {
		connect()
	},
}

func init() {
	rootCmd.AddCommand(connectCmd)
	connectCmd.Flags().StringVarP(&peerIP, "peerip", "p", "127.0.0.1", "Tendermint Core IP address")
	connectCmd.Flags().IntVarP(&peerPort, "connectport", "c", 26656, "Tendermint Core peer connection port")
	connectCmd.Flags().IntVarP(&rpcPort, "rpcport", "r", 26657, "Tendermint Core rpc port")
	connectCmd.Flags().StringVarP(&keyFile, "keyfile", "k", "", "KeyFile that Connector should use to connect to peer. If set, keypair in KeyFile will be used for all connections, else new KeyPair is generated on the fly.")
	connectCmd.Flags().BoolVarP(&isConnectionOutgoing, "dial", "d", false, "Connector DIALs TMCore if flag is set, otherwise connector LISTENs for connections")
	connectCmd.Flags().StringVarP(&marlinIP, "marlinip", "m", "127.0.0.1", "Marlin TCP Bridge IP address")
	connectCmd.Flags().IntVarP(&marlinPort, "marlinport", "n", 15003, "Marlin TCP Bridge IP port")
	connectCmd.Flags().IntVarP(&listenPort, "listenport", "l", 59001, "Port on which Connector should listen for incoming connections from peer. Only applicable for LISTEN mode.")
}


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
					marlinTo chan<- types.MarlinMessage, 
					marlinFrom <-chan types.MarlinMessage,
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

func connect() {
	peerAddr := fmt.Sprintf("%v:%v", peerIP, peerPort)
	rpcAddr := fmt.Sprintf("%v:%v", peerIP, rpcPort)
	marlinAddr := fmt.Sprintf("%v:%v", marlinIP, marlinPort)

	if keyFile != "" && isConnectionOutgoing {
		log.Warning("TMCore connector is using a KeyFile to connect to TMCore peer in DIAL mode." +
					" KeyFiles are useful to connect with peer in LISTEN mode in most use cases." + 
					" Configuring KeyFile usage in DIAL mode may lead to unsuccessful connections if peer blacklists connector's ID." +
					" It is advised that you let connector use anonymous identities if possible.")
		time.Sleep(5 * time.Second) // Sleep so that warning message is clearly read
	}

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

	invokeHandler(nodeInfo["nodeType"].(chains.NodeType), peerAddr, marlinTo, marlinFrom, isConnectionOutgoing, keyFile, listenPort)
}