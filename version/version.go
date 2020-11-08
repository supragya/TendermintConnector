package version

import "bytes"

// Application version
var applicationVersion string = "0.1"

// Supported Chains
var supportedChains = []string{
	"IRISNet Mainnet 0.16.3 (Consensus State transfers)",
}

// Marlin TM Encoder/Decoder Protocols
var marlinTMProtocols = []string{
	"marlinTMSTfr1 - v0.1 Marlin TM Consensus State Transfer Protocol",
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