package marlin

import (
	"net"
	"bufio"
	"os"
	"io"
	// "io/ioutil"
	"time"
	"encoding/binary"

	log "github.com/sirupsen/logrus"
	proto "github.com/golang/protobuf/proto"

	// Protocols
	"github.com/supragya/tendermint_connector/marlin/protocols/marlinTMCSTfr1"
)

func recvRoutine() {
	log.Info("mrln -> connector Routine started")
	for {
		messageLenByte0, err1 := marlinConn.ReadByte()
		messageLenByte1, err2 := marlinConn.ReadByte()

		if err1 != nil || err2 != nil {
			log.Error("Error reading message size from the buffer")
		}

		messageLen := binary.LittleEndian.Uint16([]byte{messageLenByte0, messageLenByte1})

		var buffer = make([]byte, messageLen)
		n, err := io.ReadFull(marlinConn, buffer)
		if err != nil {
			log.Error("Error reading message of size ", messageLen, " instead read ", n)
		}
		
		tmMessage := marlinTMCSTfr1.TendermintMessage{}
		proto.Unmarshal(buffer, &tmMessage)
		log.Info("mrln -> connector recieved message: ", tmMessage.GetData())

		time.Sleep(200 * time.Millisecond)
	}
}

var marlinConn *bufio.ReadWriter

func ConnectMarlinBridge(marlinAddr string) {
	conn, err := net.Dial("tcp", marlinAddr)
	if err != nil{
		log.Error("Unable to connect to Marlin multicast TCP bridge at ", marlinAddr, ". Closing application")
		os.Exit(2)
	}

	marlinConn = bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))

	go recvRoutine()
}

func Send(chainID string, data []byte, encoderVersion uint16) {
	chainIDNum, ok := servicedChains[chainID]
	if !ok {
		log.Error("Cannot service, chainID unknown: ", chainID)
		return
	}

	messageLen := make([]byte, 2)
	switch encoderVersion {
	case marlinTMCSTfr1.Version:
		sendData := &marlinTMCSTfr1.TendermintMessage{
			ChainId: chainIDNum,
			Data:	data,
		}
		encodedData, err := proto.Marshal(sendData)
		if err != nil {
			log.Error("Error encountered while marshalling marlinTMSCTfr1 message.")
		}

		binary.LittleEndian.PutUint16(messageLen, uint16(len(encodedData)))
		encodedData = append(messageLen, encodedData...)
		
		n, err := marlinConn.Write(encodedData)
		if err != nil {
			log.Error("Error encountered when writing to marlin TCP Bridge: ", err)
			os.Exit(2)
		} else if n < len(encodedData) {
			log.Error("Too few data pushed to marlin TCP Bridge: ", n, " vs ", len(encodedData))
			os.Exit(2)
		}
		err = marlinConn.Flush()
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