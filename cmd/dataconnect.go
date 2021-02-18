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
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	marlinTypes "github.com/supragya/TendermintConnector/types"

	"github.com/supragya/TendermintConnector/marlin"

	// Tendermint Core Chains
	"github.com/supragya/TendermintConnector/chains"
	"github.com/supragya/TendermintConnector/chains/cosmos"
	"github.com/supragya/TendermintConnector/chains/irisnet"
)

// connectCmd represents the connect command
var dataconnectCmd = &cobra.Command{
	Use:   "dataconnect",
	Short: "Act as a connector between Marlin Relay and " + compilationChain,
	Long:  `Act as a connector between Marlin Relay and ` + compilationChain,
	Run: func(cmd *cobra.Command, args []string) {
		peerAddr := fmt.Sprintf("%v:%v", peerIP, peerPort)
		rpcAddr := fmt.Sprintf("%v:%v", peerIP, rpcPort)
		marlinAddr := fmt.Sprintf("%v:%v", marlinIP, marlinPort)

		if keyFile != "" && isConnectionOutgoing {
			log.Warning("TMCore connector is using a KeyFile to connect to TMCore peer in DIAL mode." +
				" KeyFiles are useful to connect with peer in LISTEN mode in most use cases since peer would dial a specific peer which connector listens to." +
				" Configuring KeyFile usage in DIAL mode may lead to unsuccessful connections if peer blacklists connector's ID." +
				" It is advised that you let connector use anonymous identities if possible.")
			time.Sleep(3 * time.Second) // Sleep so that warning message is clearly read
		}

		if isConnectionOutgoing {
			log.Info("Configuring to DIAL Peer (TMCore) connection address: ", peerAddr, "; rpc address: ", rpcAddr)
		} else {
			log.Info("Configuring to LISTEN Peer (TMCore) connection address: ", peerAddr, "; rpc address: ", rpcAddr)
		}
		log.Info("Marlin connection address: ", marlinAddr)

		// Channels
		marlinTo := make(chan marlinTypes.MarlinMessage, 1000)
		marlinFrom := make(chan marlinTypes.MarlinMessage, 1000)

		// TODO - is this style of invocation correct? can we wrap this? WAITGROUPS??? - v0.1 prerelease
		go marlin.RunDataConnectHandler(marlinAddr, marlinTo, marlinFrom, direction)

		if doRpcSanity {
			log.Info("Doing RPC sanity!")
			nodeStatus, err := getRPCNodeStatus(rpcAddr)
			if err != nil {
				return
			}
			nodeInfo := extractNodeInfo(nodeStatus)
			findAndRunDataConnectHandler(nodeInfo["nodeType"].(chains.NodeType), peerAddr, marlinTo, marlinFrom, isConnectionOutgoing, keyFile, listenPortPeer)
		} else if compilationChain == "iris" {
			findAndRunDataConnectHandler(irisnet.ServicedTMCore, peerAddr, marlinTo, marlinFrom, isConnectionOutgoing, keyFile, listenPortPeer)
		} else if compilationChain == "cosmos" {
			findAndRunDataConnectHandler(cosmos.ServicedTMCore, peerAddr, marlinTo, marlinFrom, isConnectionOutgoing, keyFile, listenPortPeer)
		} else {
			panic("Unknown chain. Exiting")
		}
	},
}

func init() {
	rootCmd.AddCommand(dataconnectCmd)
	if compilationChain == "iris" {
		dataconnectCmd.Flags().StringVarP(&peerIP, "peerip", "i", "127.0.0.1", "Iris node IP address")
		dataconnectCmd.Flags().IntVarP(&peerPort, "peerport", "p", 26656, "Iris node peer connection port")
		dataconnectCmd.Flags().IntVarP(&rpcPort, "rpcport", "r", 26657, "Iris node rpc port")
		dataconnectCmd.Flags().BoolVarP(&doRpcSanity, "rpcsanity", "s", false, "Validated node information prior to connecting to TMCore. (RPC Sanity)")
		dataconnectCmd.Flags().StringVarP(&keyFile, "keyfile", "k", "", "KeyFile to use for connection")
		dataconnectCmd.Flags().BoolVarP(&isConnectionOutgoing, "dial", "d", false, "Connector DIALs TMCore (iris node) if flag is set, otherwise connector LISTENs for connections.")
		dataconnectCmd.Flags().StringVarP(&marlinIP, "marlinip", "m", "127.0.0.1", "Marlin TCP Bridge IP address")
		dataconnectCmd.Flags().IntVarP(&marlinPort, "marlinport", "n", 21901, "Marlin TCP Bridge port")
		dataconnectCmd.Flags().StringVarP(&direction, "direction", "e", "both", "Direction of connection [both/producer/consumer]")
		dataconnectCmd.Flags().IntVarP(&listenPortPeer, "listenportpeer", "l", 21900, "Port on which Connector should listen for incoming connections from iris peer")
	} else if compilationChain == "cosmos" {
		dataconnectCmd.Flags().StringVarP(&peerIP, "peerip", "i", "127.0.0.1", "Gaia node IP address")
		dataconnectCmd.Flags().IntVarP(&peerPort, "peerport", "p", 26656, "Gaia node peer connection port")
		dataconnectCmd.Flags().IntVarP(&rpcPort, "rpcport", "r", 26657, "Gaia node rpc port")
		dataconnectCmd.Flags().BoolVarP(&doRpcSanity, "rpcsanity", "s", false, "Validate node information prior to connecting to TMCore. (RPC Sanity)")
		dataconnectCmd.Flags().StringVarP(&keyFile, "keyfile", "k", "", "KeyFile to use for connection")
		dataconnectCmd.Flags().BoolVarP(&isConnectionOutgoing, "dial", "d", false, "Connector DIALs TMCore (gaia node) if flag is set, otherwise connector LISTENs for connections.")
		dataconnectCmd.Flags().StringVarP(&marlinIP, "marlinip", "m", "127.0.0.1", "Marlin TCP Bridge IP address")
		dataconnectCmd.Flags().IntVarP(&marlinPort, "marlinport", "n", 22401, "Marlin TCP Bridge port")
		dataconnectCmd.Flags().StringVarP(&direction, "direction", "e", "both", "Direction of connection [both/producer/consumer]")
		dataconnectCmd.Flags().IntVarP(&listenPortPeer, "listenportpeer", "l", 22400, "Port on which Connector should listen for incoming connections from cosmos peer")
	}
}
