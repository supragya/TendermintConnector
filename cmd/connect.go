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
	"github.com/spf13/cobra"
	connector "github.com/supragya/tendermint_connector/connector"
)

var peerPort, rpcPort, marlinPort int
var serverAddr string

// connectCmd represents the connect command
var connectCmd = &cobra.Command{
	Use:   "connect",
	Short: "Connect to a TM Core",
	Long:  `Connect to a TM Core`,
	Run: func(cmd *cobra.Command, args []string) {
		connector.Connect(peerPort, rpcPort, marlinPort, serverAddr)
	},
}

func init() {
	rootCmd.AddCommand(connectCmd)
	connectCmd.Flags().IntVarP(&peerPort, "peer_port", "p", 26656, "Tendermint Core peer port")
	connectCmd.Flags().IntVarP(&rpcPort, "rpc_port", "r", 26657, "Tendermint Core rpc port")
	connectCmd.Flags().IntVarP(&marlinPort, "marlin_port", "m", 15003, "Marlin multicastTCPBridge port")
	connectCmd.Flags().StringVarP(&serverAddr, "server_address", "s", "127.0.0.1", "Tendermint Core IP address / MarlinTCPBridge IP address")
}
