package marlin

import (
	"bufio"
	"encoding/binary"
	"io"
	"net"
	"os"
	"time"

	proto "github.com/golang/protobuf/proto"
	log "github.com/sirupsen/logrus"

	marlinTypes "github.com/supragya/tendermint_connector/types"

	// Protocols
	wireProtocol "github.com/supragya/tendermint_connector/marlin/protocols/tmDataTransferProtocolv1"
)

var currentlyServicing uint32

// ALlowServicedChainMessages allows handlers to register a single chain
// to allow through marlin side connector. Initially there is no blockchain message
// allowed to passthrough. One can register the chainId to look for in messages
// correspoding to chains available in marlinTypes.servicedChains to allow.
func AllowServicedChainMessages(servicedChainId uint32) {
	currentlyServicing = servicedChainId
}

type MarlinHandler struct {
	marlinConn      *bufio.ReadWriter
	marlinAddr      string
	connectionType  string
	marlinTo        chan marlinTypes.MarlinMessage
	marlinFrom      chan marlinTypes.MarlinMessage
	signalConnError chan struct{}
	signalShutSend  chan struct{}
	signalShutRecv  chan struct{}
}

func createMarlinHandler(marlinAddr string,
	marlinTo chan marlinTypes.MarlinMessage,
	marlinFrom chan marlinTypes.MarlinMessage,
	connectionType string) (MarlinHandler, error) {
	return MarlinHandler{
		marlinAddr:      marlinAddr,
		marlinTo:        marlinTo,
		marlinFrom:      marlinFrom,
		connectionType:  connectionType,
		signalConnError: make(chan struct{}, 1),
		signalShutSend:  make(chan struct{}, 1),
		signalShutRecv:  make(chan struct{}, 1),
	}, nil
}

func (h *MarlinHandler) connectMarlin() error {
	if h.connectionType == "tcp" {
		log.Info("Marlin side connector attempting a TCP dial to ", h.marlinAddr)
		conn, err := net.DialTimeout("tcp", h.marlinAddr, 2000*time.Millisecond)
		if err != nil {
			return err
		}
		h.marlinConn = bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))
	} else if h.connectionType == "unix" {
		log.Info("Marlin side connector attempting a UDS listen ", h.marlinAddr)
		listener, err := net.Listen("unix", h.marlinAddr)
		if err != nil {
			return err
		}
		conn, err := listener.Accept()
		if err != nil {
			return err
		}
		h.marlinConn = bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))
	}
	return nil
}

// ---------------------- DATA CONNECT INTERFACE --------------------------------

// Run acts as the entry point to marlin side connection logic for data connect interface.
// Marlin Side Connector requires two channels for sending / recieving messages
// from peer side connector.
func RunDataConnectHandler(marlinAddr string,
	marlinTo chan marlinTypes.MarlinMessage,
	marlinFrom chan marlinTypes.MarlinMessage) {
	log.Info("Starting Marlin DataConnect Handler")

	for {
		handler, err := createMarlinHandler(marlinAddr, marlinTo, marlinFrom, "tcp")

		if err != nil {
			log.Error("Error encountered while creating Marlin Handler: ", err)
			os.Exit(1)
		}

		err = handler.connectMarlin()

		if err != nil {
			log.Error("Connection establishment with Marlin TCP Bridge unsuccessful: ", err)
			goto REATTEMPT_CONNECTION
		}

		handler.beginServicingDataConnect()

		select {
		case <-handler.signalConnError:
			// Close bound port - bug
			handler.signalShutSend <- struct{}{}
			handler.signalShutRecv <- struct{}{}
			goto REATTEMPT_CONNECTION
		}

	REATTEMPT_CONNECTION:
		log.Info("Error encountered with connection to the Marlin TCP Bridge. Attempting reconnect post 1 second.")
		time.Sleep(1 * time.Second)
	}
}

func (h *MarlinHandler) beginServicingDataConnect() {
	go h.sendRoutineConnection()
	go h.recvRoutineConnection()
}

// sendRoutineConnection is the routine that reads data from peer side connector and makes it available to marlin relay
// Messages on wire are encoded with Marlin Tendermint Data Transfer Protocol v1 prepended with length of message
// Messages from peer connector on channel is defined using marlinTypes.marlinMessage
func (h *MarlinHandler) sendRoutineConnection() {
	log.Info("Marlin <- Connector Routine started")

	for msg := range h.marlinTo {
		select {
		case <-h.signalShutSend:
			h.marlinTo <- msg
			log.Info("Marlin <- Connector Routine shutdown")
			return
		default:
		}

		if msg.ChainID != currentlyServicing {
			log.Error("Marlin <- Connector Discarding message to marlin relay, chain is not allowed. Chain ID: ", msg.ChainID)
			continue
		}

		messageLen := make([]byte, 8)

		// Making PacketMsg Proto3 PacketMsg
		wirePackets := []*wireProtocol.PacketMsg{}
		for _, pkt := range msg.Packets {
			wirePackets = append(wirePackets, &wireProtocol.PacketMsg{
				Eof:       pkt.EOF,
				DataBytes: pkt.Bytes,
			})
		}

		sendData := &wireProtocol.TendermintMessage{
			ChainId: msg.ChainID,
			Channel: uint32(msg.Channel),
			Packets: wirePackets,
		}

		proto3Marshalled, err := proto.Marshal(sendData)
		if err != nil {
			log.Error("Marlin <- Connector Error encountered while marshalling sendData.")
			h.signalConnError <- struct{}{}
			return
		}

		binary.BigEndian.PutUint64(messageLen, uint64(len(proto3Marshalled)))

		encodedData := append(messageLen, proto3Marshalled...)

		n, err := h.marlinConn.Write(encodedData)
		if err != nil {
			log.Error("Marlin <- Connector Error encountered when writing to marlin TCP Bridge: ", err)
			h.signalConnError <- struct{}{}
			return
		} else if n < len(encodedData) {
			log.Error("Marlin <- Connector Too few data pushed to marlin TCP Bridge: ", n, " vs ", len(encodedData))
			h.signalConnError <- struct{}{}
			return
		}

		err = h.marlinConn.Flush()
		if err != nil {
			log.Error("Marlin <- Connector Error flushing marlinWriter buffer, err: ", err)
			h.signalConnError <- struct{}{}
			return
		}

		log.Debug("Marlin <- Connector transferred stat: ", len(encodedData), " ", encodedData)
	}
}

// recvRoutineConnection is the routine that read data from marlin relay and makes it available to peer side connector.
// Messages on wire are encoded with Marlin Tendermint Data Transfer Protocol v1 prepended with length of message
// Messages pushed to peer connector on channel is defined using marlinTypes.marlinMessage
func (h *MarlinHandler) recvRoutineConnection() {
	log.Info("Marlin -> Connector Routine started")

	for {
		select {
		case <-h.signalShutRecv:
			log.Info("Marlin -> Connector Routine shutdown")
			return
		default:
		}

		partialLen := make([]byte, 8)

		n, err := io.ReadFull(h.marlinConn, partialLen)
		if err != nil {
			// TODO - Return spam 0x00
			h.signalConnError <- struct{}{}
			return
		}

		messageLen := binary.BigEndian.Uint64(partialLen)

		var buffer = make([]byte, messageLen)
		n, err = io.ReadFull(h.marlinConn, buffer)
		if err != nil {
			log.Error("Marlin -> Connector Error reading message of size ", messageLen, " instead read ", n)
			h.signalConnError <- struct{}{}
			return
		}

		tmMessage := wireProtocol.TendermintMessage{}
		proto.Unmarshal(buffer, &tmMessage)
		log.Debug("Marlin -> Connector recieved message: ", buffer, " ", tmMessage)

		if tmMessage.GetChainId() == currentlyServicing && messageLen > 0 {
			internalPackets := []marlinTypes.PacketMsg{}
			for _, pkt := range tmMessage.GetPackets() {
				internalPackets = append(internalPackets, marlinTypes.PacketMsg{
					ChannelID: tmMessage.GetChannel(),
					EOF:       pkt.GetEof(),
					Bytes:     pkt.GetDataBytes(),
				})
			}
			message := marlinTypes.MarlinMessage{
				ChainID: tmMessage.GetChainId(),
				Channel: byte(tmMessage.GetChannel()),
				Packets: internalPackets,
			}
			select {
			case h.marlinFrom <- message:
			default:
				log.Warning("Too many messages in channel marlinFrom. Dropping oldest messags")
				_ = <-h.marlinFrom
				h.marlinFrom <- message
			}
		}
	}
}

// ---------------------- SPAM FILTER INTERFACE ------------------------------------

// Run acts as the entry point to marlin side connection logic for data connect interface.
// Marlin Side Connector requires two channels for sending / recieving messages
// from peer side connector.
func RunSpamFilterHandler(marlinUdsFile string,
	marlinTo chan marlinTypes.MarlinMessage,
	marlinFrom chan marlinTypes.MarlinMessage) {
	log.Info("Starting Marlin SpamFilter Handler")

	for {
		handler, err := createMarlinHandler(marlinUdsFile, marlinTo, marlinFrom, "unix")

		if err != nil {
			log.Error("Error encountered while creating Marlin Handler: ", err)
			os.Exit(1)
		}

		err = handler.connectMarlin()

		if err != nil {
			log.Error("Connection establishment with Marlin TCP Bridge unsuccessful: ", err)
			goto REATTEMPT_CONNECTION
		}

		handler.beginServicingSpamFilter()

		select {
		case <-handler.signalConnError:
			handler.signalShutSend <- struct{}{}
			handler.signalShutRecv <- struct{}{}
			goto REATTEMPT_CONNECTION
		}

	REATTEMPT_CONNECTION:
		log.Info("Error encountered with connection to the Marlin TCP Bridge. Attempting reconnect post 1 second.")
		time.Sleep(1 * time.Second)
	}
}

func (h *MarlinHandler) beginServicingSpamFilter() {
	go h.sendRoutineSpamFilter()
	go h.recvRoutineSpamFilter()
}

// sendRoutineSpamFilter is the routine that reads data from peer side connector and makes it available to marlin relay
// Messages on wire are single bytes per spam filter request and are either byte(0x00) for block and byte(0x01) for allow
// If message is recieved on 0x01 channel, it is allow request, else it is deny request
// Messages from peer connector on channel is defined using marlinTypes.marlinMessage
func (h *MarlinHandler) sendRoutineSpamFilter() {
	log.Info("Marlin <- Connector (Spamfilter) Routine started")
	messageId := make([]byte, 8)

	for msg := range h.marlinTo {
		select {
		case <-h.signalShutSend:
			h.marlinTo <- msg
			log.Info("Marlin <- Connector Routine shutdown")
			return
		default:
		}

		if msg.ChainID != currentlyServicing {
			log.Error("Marlin <- Connector Discarding message to marlin relay, chain is not allowed. Chain ID: ", msg.ChainID)
			continue
		}

		log.Info("Got spam check results as: ", msg.PacketId, " ", msg.Channel)
		binary.BigEndian.PutUint64(messageId, uint64(msg.PacketId))
		encodedData := append(messageId, msg.Channel)
		// msg.Channel is byte(0x00) from TMCore handler if message is marked spam
		// msg.Channel is byte(0x01) from TMCore handler if message is allowed
		n, err := h.marlinConn.Write(encodedData)
		if err != nil {
			log.Error("Marlin <- Connector Error encountered when writing to marlin UDS interface: ", err)
			h.signalConnError <- struct{}{}
			return
		} else if n < len(encodedData) {
			log.Error("Marlin <- Connector Too few data pushed to marlin UDS interface: ", n, " vs ", len(encodedData))
			h.signalConnError <- struct{}{}
			return
		}

		err = h.marlinConn.Flush()
		if err != nil {
			log.Error("Marlin <- Connector Error flushing marlinWriter buffer, err: ", err)
			h.signalConnError <- struct{}{}
			return
		}
	}
}

// recvRoutineSpamFilter is the routine that read data from marlin relay and makes it available to peer side connector.
// Messages on wire are encoded with Marlin Tendermint Data Transfer Protocol v1 prepended with length of message
// Messages pushed to peer connector on channel is defined using marlinTypes.marlinMessage
func (h *MarlinHandler) recvRoutineSpamFilter() {
	log.Info("Marlin -> Connector (Spamfilter) Routine started")
	for {
		select {
		case <-h.signalShutRecv:
			log.Info("Marlin -> Connector Routine shutdown")
			return
		default:
		}

		partialId := make([]byte, 8)
		partialLen := make([]byte, 8)
		n, err := io.ReadFull(h.marlinConn, partialId)
		if err != nil {
			// TODO - Return spam 0x00
			h.signalConnError <- struct{}{}
			return
		}
		n, err = io.ReadFull(h.marlinConn, partialLen)
		if err != nil {
			// TODO - Return spam 0x00
			h.signalConnError <- struct{}{}
			return
		}
		messageId := binary.BigEndian.Uint64(partialId)
		messageLen := binary.BigEndian.Uint64(partialLen)

		log.Info("Recieved for check marlinside: ", messageId, " len ", messageLen, " ", partialId, " ", partialLen)

		var buffer = make([]byte, messageLen)
		n, err = io.ReadFull(h.marlinConn, buffer)
		if err != nil {
			log.Error("Marlin -> Connector Error reading message of size ", messageLen, " instead read ", n)
			// TODO - Return spam 0x00
			h.signalConnError <- struct{}{}
			return
		}

		tmMessage := wireProtocol.TendermintMessage{}
		err = proto.Unmarshal(buffer, &tmMessage)

		if err != nil {
			log.Warning("proto3 deciphering failed! returning disallow")
			h.marlinTo <- marlinTypes.MarlinMessage{
				ChainID:  currentlyServicing,
				Channel:  byte(0x00),
				PacketId: messageId,
			}
		} else {
			log.Info("proto3 deciphering was good: ", tmMessage)
			if tmMessage.GetChainId() == currentlyServicing && messageLen > 0 {
				internalPackets := []marlinTypes.PacketMsg{}
				for _, pkt := range tmMessage.GetPackets() {
					internalPackets = append(internalPackets, marlinTypes.PacketMsg{
						ChannelID: tmMessage.GetChannel(),
						EOF:       pkt.GetEof(),
						Bytes:     pkt.GetDataBytes(),
					})
				}
				message := marlinTypes.MarlinMessage{
					ChainID:  tmMessage.GetChainId(),
					Channel:  byte(tmMessage.GetChannel()),
					PacketId: messageId,
					Packets:  internalPackets,
				}
				select {
				case h.marlinFrom <- message:
					log.Info("Pushed it to TM for checks")
				default:
					log.Warning("Too many messages in channel marlinFrom. Dropping oldest messags")
					_ = <-h.marlinFrom
					h.marlinFrom <- message
				}
			}
		}
	}
}
