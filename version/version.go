package version

import "bytes"

// Application version
var applicationVersion string = "0.0.0"

// Build commit
var buildCommit string = "0x0000"

// Build time
var buildTime string = "Mon Dec 21 13:26:38 UTC 2020"

// Supported Chains
var supportedChains = []string{
	"Tendermint v0.34.7",
}

// Marlin TM Encoder/Decoder Protocols
var marlinTMProtocols = []string{
	"Marlin Tendermint Data Transfer Protocol v1",
}

var RootCmdVersion string = prepareVersionString()

func prepareVersionString() string {
	var buffer bytes.Buffer
	buffer.WriteString(applicationVersion + " build " + buildCommit)
	buffer.WriteString("\nCompiled on: " + buildTime)
	for _, v := range supportedChains {
		buffer.WriteString("\n+ [Tendermint Chain]   " + v)

	}
	for _, v := range marlinTMProtocols {
		buffer.WriteString("\n+ [Marlin TM Protocol] " + v)
	}
	return buffer.String()
}
