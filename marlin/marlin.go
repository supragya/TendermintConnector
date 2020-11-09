package marlin

import (
	"net"
	"bufio"
	"os"
	"io"
	// "io/ioutil"
	// "time"
	"encoding/binary"

	log "github.com/sirupsen/logrus"
	proto "github.com/golang/protobuf/proto"

	"github.com/supragya/tendermint_connector/types"

	// Protocols
	wireProtocol "github.com/supragya/tendermint_connector/marlin/protocols/tmDataTransferProtocolv1"
)

var currentlyServicing uint32 = 0
var marlinConn *bufio.ReadWriter

// ALlowServicedChainMessages allows handlers to register a single chain
// to allow through marlin side connector. Initially there is no blockchain message
// allowed to passthrough. One can register the chainId to look for in messages
// correspoding to chains available in types.servicedChains to allow.
func AllowServicedChainMessages(servicedChainId uint32) {
	currentlyServicing = servicedChainId
}

// Run acts as the entry point to marlin side connection logic.
// Marlin Side Connector requires two channels for sending / recieving messages
// from peer side connector.
func Run(marlinAddr string, marlinTo <-chan types.MarlinMessage, marlinFrom chan<- types.MarlinMessage) {
	conn, err := net.Dial("tcp", marlinAddr)
	if err != nil{
		log.Error("Unable to connect to Marlin multicast TCP bridge at ", marlinAddr, ". Closing application")
		os.Exit(2)
	}

	marlinConn = bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))

	go sendRoutine(marlinTo)
	go recvRoutine(marlinFrom)
}

// sendRoutine is the routine that reads data from peer side connector and makes it available to marlin relay 
// Messages on wire are encoded with Marlin Tendermint Data Transfer Protocol v1 prepended with length of message
// Messages from peer connector on channel is defined using types.marlinMessage
func sendRoutine(marlinTo <-chan types.MarlinMessage) {
	log.Info("mrln <- connector Routine started")

	for msg := range marlinTo {
		if msg.ChainID != currentlyServicing {
			log.Error("mrln <- connector Discarding message to marlin relay, chain is not allowed. Chain ID: ", msg.ChainID)
			continue
		}

		messageLen := make([]byte, 2)
		sendData := &wireProtocol.TendermintMessage{
			ChainId: msg.ChainID,
			Channel: msg.Channel,
			Data:	msg.Data,
		}

		proto3Marshalled, err := proto.Marshal(sendData)
		if err != nil {
			log.Error("mrln <- connector Error encountered while marshalling sendData.")
			continue
		}

		binary.LittleEndian.PutUint16(messageLen, uint16(len(proto3Marshalled)))

		encodedData := append(messageLen, proto3Marshalled...)

		n, err := marlinConn.Write(encodedData)
		if err != nil {
			log.Error("mrln <- connector Error encountered when writing to marlin TCP Bridge: ", err)
			os.Exit(2)
		} else if n < len(encodedData) {
			log.Error("mrln <- connector Too few data pushed to marlin TCP Bridge: ", n, " vs ", len(encodedData))
			os.Exit(2)
		}

		err = marlinConn.Flush()
		if err != nil {
			log.Error("mrln <- connector Error flushing marlinWriter buffer, err: ", err)
			os.Exit(2)
		}

		log.Debug("mrln <- connector transferred stat: ", len(msg.Data), "/", n, "/", len(encodedData), " ", encodedData)
	}
}

// recvRoutine is the routine that read data from marlin relay and makes it available to peer side connector. 
// Messages on wire are encoded with Marlin Tendermint Data Transfer Protocol v1 prepended with length of message
// Messages pushed to peer connector on channel is defined using types.marlinMessage
func recvRoutine(marlinFrom chan<- types.MarlinMessage) {
	log.Info("mrln -> connector Routine started")
	for {
		messageLenByte0, err1 := marlinConn.ReadByte()
		messageLenByte1, err2 := marlinConn.ReadByte()

		if err1 != nil || err2 != nil {
			log.Error("mrln -> connector Error reading message size from the buffer")
		}

		messageLen := binary.LittleEndian.Uint16([]byte{messageLenByte0, messageLenByte1})

		var buffer = make([]byte, messageLen)
		n, err := io.ReadFull(marlinConn, buffer)
		if err != nil {
			log.Error("mrln -> connector Error reading message of size ", messageLen, " instead read ", n)
		}
		
		tmMessage := wireProtocol.TendermintMessage{}
		proto.Unmarshal(buffer, &tmMessage)
		log.Debug("mrln -> connector recieved message: ", buffer, " ", tmMessage)

		if tmMessage.GetChainId() == currentlyServicing && messageLen > 0 {
			marlinFrom <- types.MarlinMessage{
				ChainID: tmMessage.GetChainId(),
				Channel: tmMessage.GetChannel(),
				Data:	tmMessage.GetData(),
			}
		}
	}
}

