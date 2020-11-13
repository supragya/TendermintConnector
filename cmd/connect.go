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
	"github.com/supragya/tendermint_connector/types"

	"github.com/supragya/tendermint_connector/marlin"

	// Tendermint Core Chains
	"github.com/supragya/tendermint_connector/chains"
)

// connectCmd represents the connect command
var connectCmd = &cobra.Command{
	Use:   "connect",
	Short: "Act as a connector between TM Core and Marlin Relay",
	Long:  `Act as a connector between TM Core and Marlin Relay`,
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

func connect() {
	peerAddr := fmt.Sprintf("%v:%v", peerIP, peerPort)
	rpcAddr := fmt.Sprintf("%v:%v", peerIP, rpcPort)
	marlinAddr := fmt.Sprintf("%v:%v", marlinIP, marlinPort)

	if keyFile != "" && isConnectionOutgoing {
		log.Warning("TMCore connector is using a KeyFile to connect to TMCore peer in DIAL mode." +
			" KeyFiles are useful to connect with peer in LISTEN mode in most use cases since peer would dial a specific peer which connector listens to." +
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

	go marlin.Run(marlinAddr, marlinTo, marlinFrom, false)

	invokeHandler(nodeInfo["nodeType"].(chains.NodeType), peerAddr, marlinTo, marlinFrom, isConnectionOutgoing, keyFile, listenPort)
}
