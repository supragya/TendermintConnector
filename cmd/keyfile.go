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
	log "github.com/sirupsen/logrus"

	// Tendermint Core Chains
	// "github.com/supragya/tendermint_connector/chains"
	"github.com/supragya/tendermint_connector/chains/irisnet"

)

var chain, fileLocation string
var isGenerate bool

// connectCmd represents the connect command
var keyFileCmd = &cobra.Command{
	Use:   "keyfile",
	Short: "Generate a keyfile for specific blockchain",
	Long:  `Generate a keyfile for specific blockchain`,
	Run: func(cmd *cobra.Command, args []string) {
		switch chain {
		case irisnet.ServicedKeyFile:
			if isGenerate {
				irisnet.GenerateKeyFile(fileLocation)
			} else {
				irisnet.VerifyKeyFile(fileLocation)
			}
		default:
			log.Error("Unknown tendermint chain, can't generate or verify for ", chain)
		}
	},
}

func init() {
	rootCmd.AddCommand(keyFileCmd)
	keyFileCmd.Flags().StringVarP(&chain, "chain", "c", "", "Tendermint based chain for which to generate KeyFile")
	keyFileCmd.Flags().StringVarP(&fileLocation, "filelocation", "f", "", "File to generate/validate")
	keyFileCmd.Flags().BoolVarP(&isGenerate, "generate", "g", true, "Generate new file. If not set, validate given KeyFile")
}

