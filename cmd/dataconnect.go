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
	marlinTypes "github.com/supragya/tendermint_connector/types"

	"github.com/supragya/tendermint_connector/marlin"

	// Tendermint Core Chains
	"github.com/supragya/tendermint_connector/chains"
)

// connectCmd represents the connect command
var dataconnectCmd = &cobra.Command{
	Use:   "dataconnect",
	Short: "Act as a connector between TM Core and Marlin Relay",
	Long:  `Act as a connector between TM Core and Marlin Relay`,
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

		nodeStatus, err := getRPCNodeStatus(rpcAddr)
		if err != nil {
			return
		}

		// Channels
		marlinTo := make(chan marlinTypes.MarlinMessage, 1000)
		marlinFrom := make(chan marlinTypes.MarlinMessage, 1000)

		nodeInfo := extractNodeInfo(nodeStatus)

		// TODO - is this style of invocation correct? can we wrap this? WAITGROUPS??? - v0.1 prerelease
		go marlin.RunDataConnectHandler(marlinAddr, marlinTo, marlinFrom)

		findAndRunDataConnectHandler(nodeInfo["nodeType"].(chains.NodeType), peerAddr, marlinTo, marlinFrom, isConnectionOutgoing, keyFile, listenPortPeer)
	},
}

func init() {
	rootCmd.AddCommand(dataconnectCmd)
	dataconnectCmd.Flags().StringVarP(&peerIP, "peerip", "i", "127.0.0.1", "Tendermint Core IP address")
	dataconnectCmd.Flags().IntVarP(&peerPort, "peerport", "p", 26656, "Tendermint Core peer connection port")
	dataconnectCmd.Flags().IntVarP(&rpcPort, "rpcport", "r", 26657, "Tendermint Core rpc port")
	dataconnectCmd.Flags().StringVarP(&keyFile, "keyfile", "k", "", "KeyFile to use for connection")
	dataconnectCmd.Flags().BoolVarP(&isConnectionOutgoing, "dial", "d", false, "Connector DIALs TMCore if flag is set, otherwise connector LISTENs for connections.")
	dataconnectCmd.Flags().StringVarP(&marlinIP, "marlinip", "m", "127.0.0.1", "Marlin TCP Bridge IP address")
	dataconnectCmd.Flags().IntVarP(&marlinPort, "marlinport", "n", 21901, "Marlin TCP Bridge port (default: 21901 IRIS, 22401 COSMOS)")
	dataconnectCmd.Flags().IntVarP(&listenPortPeer, "listenportpeer", "l", 21900, "Port on which Connector should listen for incoming connections from peer. (defaults: 21900 IRIS, 22400 COSMOS)")
}
