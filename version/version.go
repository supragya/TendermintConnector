package version

import "bytes"

// Application version
var applicationVersion string = "0.1-rc-1"

// Supported Chains
var supportedChains = []string{
	"IRISNet Mainnet 0.16.3 (Consensus State transfers) v0.1",
}

// Marlin TM Encoder/Decoder Protocols
var marlinTMProtocols = []string{
	"Marlin Tendermint Data Transfer Protocol v1",
}

var RootCmdVersion string = prepareVersionString()

func prepareVersionString() string {
	var buffer bytes.Buffer
	buffer.WriteString(applicationVersion)
	for _, v := range supportedChains {
		buffer.WriteString("\n+ [Tendermint Chain]   " + v)

	}
	for _, v := range marlinTMProtocols {
		buffer.WriteString("\n+ [Marlin TM Protocol] " + v)
	}
	return buffer.String()
}
