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
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/supragya/TendermintConnector/chains/cosmos"
	"github.com/supragya/TendermintConnector/chains/iris"
	"github.com/supragya/TendermintConnector/marlin"
	"github.com/supragya/TendermintConnector/types"
	// Tendermint Core Chains
	// "github.com/supragya/TendermintConnector/chains"
)

// connectCmd represents the connect command
var spamFilterCmd = &cobra.Command{
	Use:   "spamfilter",
	Short: "Filter Spams on marlin relay",
	Long:  `Filter Spams on marlin relay`,
	Run: func(cmd *cobra.Command, args []string) {
		rpcAddr := fmt.Sprintf("%v:%v", peerIP, rpcPort)
		marlinAddr := fmt.Sprintf("%v:%v", "0.0.0.0", marlinPort)
		log.Info("TMCore RPC address: ", rpcAddr)
		_, err := getRPCNodeStatus(rpcAddr)
		if err != nil {
			return
		}

		// Channels
		marlinTo := make(chan types.MarlinMessage, 1000)
		marlinFrom := make(chan types.MarlinMessage, 1000)

		// nodeInfo := extractNodeInfo(nodeStatus)

		go marlin.RunSpamFilterHandler(marlinAddr, marlinTo, marlinFrom)

		if compilationChain == "iris" {
			findAndRunSpamFilterHandler(iris.ServicedTMCore, rpcAddr, marlinTo, marlinFrom)
		} else if compilationChain == "cosmos" {
			findAndRunSpamFilterHandler(cosmos.ServicedTMCore, rpcAddr, marlinTo, marlinFrom)
		} else {
			panic("Unknown chain. Exiting")
		}
	},
}

func init() {
	rootCmd.AddCommand(spamFilterCmd)
	if compilationChain == "iris" {
		spamFilterCmd.Flags().StringVarP(&peerIP, "peerip", "p", "127.0.0.1", "Iris light client RPC IP address")
		spamFilterCmd.Flags().IntVarP(&rpcPort, "rpcport", "r", 21504, "Iris light client rpc port")
		spamFilterCmd.Flags().IntVarP(&marlinPort, "marlinport", "u", 21505, "Marlin TCP socket on localhost to interface with ABCI")
	} else if compilationChain == "cosmos" {
		spamFilterCmd.Flags().StringVarP(&peerIP, "peerip", "p", "127.0.0.1", "Gaia light client RPC IP address")
		spamFilterCmd.Flags().IntVarP(&rpcPort, "rpcport", "r", 22004, "Gaia light client rpc port")
		spamFilterCmd.Flags().IntVarP(&marlinPort, "marlinport", "u", 22005, "Marlin TCP socket on localhost to interface with ABCI")
	}
}
