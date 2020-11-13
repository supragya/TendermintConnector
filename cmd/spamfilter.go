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
	// log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/supragya/tendermint_connector/types"
	"github.com/supragya/tendermint_connector/marlin"

	// Tendermint Core Chains
	"github.com/supragya/tendermint_connector/chains"
	// "github.com/supragya/tendermint_connector/chains/irisnet"
)

// connectCmd represents the connect command
var spamFilterCmd = &cobra.Command{
	Use:   "spamfilter",
	Short: "Filter Spams on marlin relay",
	Long:  `Filter Spams on marlin relay`,
	Run: func(cmd *cobra.Command, args []string) {
		rpcAddr := fmt.Sprintf("%v:%v", peerIP, rpcPort)
		marlinAddr := fmt.Sprintf("%v:%v", marlinIP, marlinPort)
		nodeStatus, err := getRPCNodeStatus(rpcAddr)
		if err != nil {
			return
		}

		// Channels
		marlinTo := make(chan types.MarlinMessage, 1)
		marlinFrom := make(chan types.MarlinMessage, 1)

		nodeInfo := extractNodeInfo(nodeStatus)

		go marlin.Run(marlinAddr, marlinTo, marlinFrom, false)

		invokeSpamFilter(nodeInfo["nodeType"].(chains.NodeType), rpcAddr, marlinTo, marlinFrom)
	},
}

func init() {
	rootCmd.AddCommand(spamFilterCmd)
	spamFilterCmd.Flags().StringVarP(&peerIP, "peerip", "p", "127.0.0.1", "Tendermint RPC IP address")
	spamFilterCmd.Flags().IntVarP(&rpcPort, "rpcport", "r", 26657, "Tendermint Core rpc port")
	connectCmd.Flags().StringVarP(&marlinIP, "marlinip", "m", "127.0.0.1", "Marlin TCP Bridge IP address")
	connectCmd.Flags().IntVarP(&marlinPort, "marlinport", "n", 15003, "Marlin TCP Bridge IP port")
}
