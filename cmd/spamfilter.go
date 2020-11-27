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
	"github.com/supragya/tendermint_connector/marlin"
	"github.com/supragya/tendermint_connector/types"

	// Tendermint Core Chains
	"github.com/supragya/tendermint_connector/chains"
)

// connectCmd represents the connect command
var spamFilterCmd = &cobra.Command{
	Use:   "spamfilter",
	Short: "Filter Spams on marlin relay",
	Long:  `Filter Spams on marlin relay`,
	Run: func(cmd *cobra.Command, args []string) {
		rpcAddr := fmt.Sprintf("%v:%v", peerIP, rpcPort)
		nodeStatus, err := getRPCNodeStatus(rpcAddr)
		if err != nil {
			return
		}

		// Channels
		marlinTo := make(chan types.MarlinMessage, 1000)
		marlinFrom := make(chan types.MarlinMessage, 1000)

		nodeInfo := extractNodeInfo(nodeStatus)

		// TODO - is this style of invocation correct? can we wrap this? WAITGROUPS??? - v0.1 prerelease
		go marlin.RunSpamFilterHandler(marlinUdsFile, marlinTo, marlinFrom)

		findAndRunSpamFilterHandler(nodeInfo["nodeType"].(chains.NodeType), rpcAddr, marlinTo, marlinFrom)
	},
}

func init() {
	rootCmd.AddCommand(spamFilterCmd)
	spamFilterCmd.Flags().StringVarP(&peerIP, "peerip", "p", "127.0.0.1", "Tendermint RPC IP address")
	spamFilterCmd.Flags().IntVarP(&rpcPort, "rpcport", "r", 26657, "Tendermint Core rpc port")
	spamFilterCmd.Flags().StringVarP(&marlinUdsFile, "marlinudsfile", "u", "/tmp/tm.ipc", "Marlin UDS file location")
}
