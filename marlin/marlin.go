package marlin

import (
	"net"
	"bufio"
	"os"
	"encoding/binary"

	log "github.com/sirupsen/logrus"
	proto "github.com/golang/protobuf/proto"

	// Protocols
	"github.com/supragya/tendermint_connector/marlin/protocols/marlinTMCSTfr1"
)

func recvRoutine() {
	log.Info("mrln -> connector Routine started")
}

var marlinReader *bufio.Reader
var marlinWriter *bufio.Writer

func ConnectMarlinBridge(marlinAddr string) {
	conn, err := net.Dial("tcp", marlinAddr)
	if err != nil{
		log.Error("Unable to connect to Marlin multicast TCP bridge at ", marlinAddr, ". Closing application")
		os.Exit(2)
	}

	marlinReader = bufio.NewReader(conn)
	marlinWriter = bufio.NewWriter(conn)
	go recvRoutine()
}

func Send(chainID string, data []byte, encoderVersion uint16) {
	chainIDNum, ok := servicedChains[chainID]
	if !ok {
		log.Error("Cannot service, chainID unknown: ", chainID)
		return
	}

	encoderVersionBinary := make([]byte, 2)
	switch encoderVersion {
	case marlinTMCSTfr1.Version:
		binary.LittleEndian.PutUint16(encoderVersionBinary, marlinTMCSTfr1.Version)
		sendData := &marlinTMCSTfr1.TendermintMessage{
			ChainId: chainIDNum,
			Data:	data,
		}
		encodedData, err := proto.Marshal(sendData)
		if err != nil {
			log.Error("Error encountered while marshalling marlinTMSCTfr1 message.")
		}

		encodedData = append(encoderVersionBinary, encodedData...)
		
		n, err := marlinWriter.Write(encodedData)
		if err != nil {
			log.Error("Error encountered when writing to marlin TCP Bridge: ", err)
			os.Exit(2)
		} else if n < len(encodedData) {
			log.Error("Too few data pushed to marlin TCP Bridge: ", n, " vs ", len(encodedData))
			os.Exit(2)
		}
		err = marlinWriter.Flush()
		if err != nil {
			log.Error("Error flushing marlinWriter buffer, err: ", err)
			os.Exit(2)
		}
		log.Info("mrln <- connector <marlinTMSTfr1> for ", chainID, " stat: ", len(data), "/", n, "/", len(encodedData))
	default:
		log.Error("Encoder Version ", encoderVersion, " for marlin Protocol unknown. Not encoding and sending further")
	}
}

// DANGER - Do not change mappings for serviced chains.
// These are vital for encoding / decoding to correct chains.
var servicedChains = map[string]uint32{
	"irisnet-0.16.3-mainnet": 1,
}