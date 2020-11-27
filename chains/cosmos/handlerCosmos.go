package cosmos

import (
	"bufio"
	// "bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"reflect"
	"runtime"
	"strconv"
	"time"
	"encoding/hex"
	"encoding/json"
	b64 "encoding/base64"
	"net/http"
	"io/ioutil"

	log "github.com/sirupsen/logrus"
	"github.com/hashicorp/golang-lru"
	"github.com/supragya/tendermint_connector/chains"
	"github.com/supragya/tendermint_connector/chains/cosmos/conn"
	cmn "github.com/supragya/tendermint_connector/chains/cosmos/libs/common"
	flow "github.com/supragya/tendermint_connector/chains/cosmos/libs/flowrate"
	marlinTypes "github.com/supragya/tendermint_connector/types"
	amino "github.com/tendermint/go-amino"
	"github.com/tendermint/tendermint/crypto/ed25519"

	// Protocols
	"github.com/supragya/tendermint_connector/marlin"
)

// ServicedTMCore is a string associated with each TM core handler
// to decipher which handler is to be attached.
var ServicedTMCore chains.NodeType = chains.NodeType{Version: "0.32.13", Network: "cosmoshub-3", ProtocolVersionApp: "0", ProtocolVersionBlock: "10", ProtocolVersionP2p: "7"}

// ---------------------- DATA CONNECT INTERFACE --------------------------------

func RunDataConnect(peerAddr string,
	marlinTo chan marlinTypes.MarlinMessage,
	marlinFrom chan marlinTypes.MarlinMessage,
	isConnectionOutgoing bool,
	keyFile string,
	listenPort int) {
	log.Info("Starting Cosmos-3 Tendermint Core Handler - mainnet")

	if keyFile != "" {
		isKeyFileUsed = true
		keyFileLocation = keyFile
	}

	for {
		handler, err := createTMHandler(peerAddr, "0.0.0.0:0", marlinTo, marlinFrom, isConnectionOutgoing, listenPort, true)

		if err != nil {
			log.Error("Error encountered while creating TM Handler: ", err)
			os.Exit(1)
		}

		if isConnectionOutgoing {
			err = handler.dialPeer()
		} else {
			err = handler.acceptPeer()
		}
		if err != nil {
			log.Error("Base Connection establishment with peer unsuccessful: ", err)
			goto REATTEMPT_CONNECTION
		}

		err = handler.upgradeConnectionAndHandshake()
		if err != nil {
			log.Error("Error while upgrading connection and handshaking with peer: ", err)
			goto REATTEMPT_CONNECTION
		}

		handler.beginServicing()

		select {
		case <-handler.signalConnError:
			handler.signalShutSend <- struct{}{}
			handler.signalShutRecv <- struct{}{}
			handler.signalShutThroughput <- struct{}{}
			goto REATTEMPT_CONNECTION
		}

	REATTEMPT_CONNECTION:
		handler.baseConnection.Close()
		handler.secretConnection.Close()
		log.Info("Error encountered with connection to the peer. Attempting reconnect post 1 second.")
		time.Sleep(1 * time.Second)
	}
}

func (h *TendermintHandler) dialPeer() error {
	var err error
	h.baseConnection, err = net.DialTimeout("tcp", h.peerAddr, 2000*time.Millisecond)
	if err != nil {
		return err
	}

	return nil
}

func (h *TendermintHandler) acceptPeer() error {
	log.Info("TMCore side listening for dials to ",
		string(hex.EncodeToString(h.privateKey.PubKey().Address())), "@<SYSTEM-IP-ADDR>:", h.listenPort)

	listener, err := net.Listen("tcp", "0.0.0.0:"+strconv.Itoa(h.listenPort))
	if err != nil {
		return err
	}

	h.baseConnection, err = listener.Accept()
	if err != nil {
		return err
	}

	return nil
}

func (h *TendermintHandler) upgradeConnectionAndHandshake() error {
	var err error
	h.secretConnection, err = conn.MakeSecretConnection(h.baseConnection, h.privateKey)
	if err != nil {
		return err
	}

	err = h.handshake()
	if err != nil {
		return err
	}

	log.Info("Established connection with TM peer [" +
		string(hex.EncodeToString(h.secretConnection.RemotePubKey().Address())) +
		"] a.k.a. " + h.peerNodeInfo.Moniker)
	return nil
}

func (h *TendermintHandler) handshake() error {
	var (
		errc                        = make(chan error, 2)
		ourNodeInfo DefaultNodeInfo = DefaultNodeInfo{
			ProtocolVersion{App: 0, Block: 10, P2P: 7},
			string(hex.EncodeToString(h.privateKey.PubKey().Address())),
			"tcp://127.0.0.1:20006", //TODO Correct this - v0.2 prerelease
			"cosmoshub-3",
			"0.32.13",
			[]byte{channelBc, channelCsSt, channelCsDc, channelCsVo,
				channelCsVs, channelMm, channelEv},
			"marlin-tendermint-connector",
			DefaultNodeInfoOther{"on", "tcp://0.0.0.0:26667"}, // TODO: Correct this - v0.2 prerelease
		}
	)

	go func(errc chan<- error, c net.Conn) {
		_, err := h.codec.MarshalBinaryLengthPrefixedWriter(c, ourNodeInfo)
		if err != nil {
			log.Error("Error encountered while sending handshake message")
		}
		errc <- err
	}(errc, h.secretConnection)
	go func(errc chan<- error, c net.Conn) {
		_, err := h.codec.UnmarshalBinaryLengthPrefixedReader(
			c,
			&h.peerNodeInfo,
			int64(10240), // 10KB MaxNodeInfoSize()
		)
		if err != nil {
			log.Error("Error encountered while recieving handshake message")
		}
		errc <- err
	}(errc, h.secretConnection)

	for i := 0; i < cap(errc); i++ {
		err := <-errc
		if err != nil {
			log.Error("Encountered error in handshake with TM core: ", err)
			return err
		}
	}
	return nil
}

func (h *TendermintHandler) beginServicing() error {
	// Register Messages
	RegisterPacket(h.codec)
	RegisterConsensusMessages(h.codec)

	// Create a P2P Connection
	h.p2pConnection = P2PConnection{
		conn:            h.secretConnection,
		bufConnReader:   bufio.NewReaderSize(h.secretConnection, 65535),
		bufConnWriter:   bufio.NewWriterSize(h.secretConnection, 65535),
		sendMonitor:     flow.New(0, 0),
		recvMonitor:     flow.New(0, 0),
		send:            make(chan struct{}, 1),
		pong:            make(chan struct{}, 1),
		doneSendRoutine: make(chan struct{}, 1),
		quitSendRoutine: make(chan struct{}, 1),
		quitRecvRoutine: make(chan struct{}, 1),
		flushTimer:      cmn.NewThrottleTimer("flush", 100*time.Millisecond),
		pingTimer:       time.NewTicker(30 * time.Second),
		pongTimeoutCh:   make(chan bool, 1),
	}

	// Start P2P Send and recieve routines + Status messages for message throughput
	go h.sendRoutine()
	go h.recvRoutine()
	go h.throughput.presentThroughput(5, h.signalShutThroughput)

	// Allow cosmos3 messages from marlin Relay
	marlin.AllowServicedChainMessages(h.servicedChainId)
	return nil
}

func (h *TendermintHandler) sendRoutine() {
	log.Info("TMCore <- Connector Routine Started")

	for {
	SELECTION:
		select {
		case <-h.p2pConnection.pingTimer.C: // Send PING messages to TMCore
			_n, err := h.codec.MarshalBinaryLengthPrefixedWriter(h.p2pConnection.bufConnWriter, PacketPing{})
			if err != nil {
				break SELECTION
			}
			h.p2pConnection.sendMonitor.Update(int(_n))
			h.p2pConnection.pongTimer = time.AfterFunc(60*time.Second, func() {
				select {
				case h.p2pConnection.pongTimeoutCh <- true:
				default:
				}
			})

			err = h.p2pConnection.bufConnWriter.Flush()
			if err != nil {
				log.Error("Cannot flush buffer PingTimer: ", err)
				h.signalConnError <- struct{}{}
			}

		case <-h.p2pConnection.pong: // Send PONG messages to TMCore
			_n, err := h.codec.MarshalBinaryLengthPrefixedWriter(h.p2pConnection.bufConnWriter, PacketPong{})
			if err != nil {
				log.Error("Cannot send Pong message: ", err)
				break SELECTION
			}
			h.p2pConnection.sendMonitor.Update(int(_n))
			err = h.p2pConnection.bufConnWriter.Flush()
			if err != nil {
				log.Error("Cannot flush buffer: ", err)
				h.signalConnError <- struct{}{}
			}

		case timeout := <-h.p2pConnection.pongTimeoutCh: // Check if PONG messages are received in time
			if timeout {
				log.Error("Pong timeout, TM Core did not reply in time!")
				h.p2pConnection.stopPongTimer()
				h.signalConnError <- struct{}{}
			} else {
				h.p2pConnection.stopPongTimer()
			}

		case <-h.signalShutSend: // Signal to Shut down sendRoutine
			log.Info("node <- Connector Routine shutdown")
			h.p2pConnection.stopPongTimer()
			close(h.p2pConnection.doneSendRoutine)
			return

		case marlinMsg := <-h.marlinFrom: // Actual message packets from Marlin Relay (encoded in Marlin Tendermint Data Transfer Protocol v1)
			switch marlinMsg.Channel {
			case channelCsSt:
				msg, err:= h.decodeConsensusMsgFromChannelBuffer(marlinMsg.Packets)
				if err != nil {
					log.Debug("Cannot decode message recieved from marlin to a valid Consensus Message: ", err)
				} else {
					switch msg.(type) {
					case *NewRoundStepMessage:
						for _, pkt := range marlinMsg.Packets {
							_n, err := h.codec.MarshalBinaryLengthPrefixedWriter(
								h.p2pConnection.bufConnWriter,
								PacketMsg{
									ChannelID: byte(pkt.ChannelID),
									EOF:       byte(pkt.EOF),
									Bytes:     pkt.Bytes,
								})
							if err != nil {
								log.Error("Error occurred in sending data to TMCore: ", err)
								h.signalConnError <- struct{}{}
							}
							h.p2pConnection.sendMonitor.Update(int(_n))
							err = h.p2pConnection.bufConnWriter.Flush()
							if err != nil {
								log.Error("Cannot flush buffer: ", err)
								h.signalConnError <- struct{}{}
							}
						}
						h.throughput.putInfo("to", "+CsStNRS", uint32(len(marlinMsg.Packets)))
					default:
						h.throughput.putInfo("to", "-CsStUNK", uint32(len(marlinMsg.Packets)))
					}
				}
			case channelCsVo:
				msg, err:= h.decodeConsensusMsgFromChannelBuffer(marlinMsg.Packets)
				if err != nil {
					log.Debug("Cannot decode message recieved from marlin to a valid Consensus Message: ", err)
				} else {
					switch msg.(type) {
					case *VoteMessage:
						for _, pkt := range marlinMsg.Packets {
							_n, err := h.codec.MarshalBinaryLengthPrefixedWriter(
								h.p2pConnection.bufConnWriter,
								PacketMsg{
									ChannelID: byte(pkt.ChannelID),
									EOF:       byte(pkt.EOF),
									Bytes:     pkt.Bytes,
								})
							if err != nil {
								log.Error("Error occurred in sending data to TMCore: ", err)
								h.signalConnError <- struct{}{}
							}
							h.p2pConnection.sendMonitor.Update(int(_n))
							err = h.p2pConnection.bufConnWriter.Flush()
							if err != nil {
								log.Error("Cannot flush buffer: ", err)
								h.signalConnError <- struct{}{}
							}
						}
						h.throughput.putInfo("to", "+CsVoVOT", uint32(len(marlinMsg.Packets)))
					default:
						h.throughput.putInfo("to", "-CsVoUNK", uint32(len(marlinMsg.Packets)))
					}
				}
			case channelCsDc:
				msg, err:= h.decodeConsensusMsgFromChannelBuffer(marlinMsg.Packets)
				if err != nil {
					log.Debug("Cannot decode message recieved from marlin to a valid Consensus Message: ", err)
				} else {
					switch msg.(type) {
					case *ProposalMessage:
						for _, pkt := range marlinMsg.Packets {
							_n, err := h.codec.MarshalBinaryLengthPrefixedWriter(
								h.p2pConnection.bufConnWriter,
								PacketMsg{
									ChannelID: byte(pkt.ChannelID),
									EOF:       byte(pkt.EOF),
									Bytes:     pkt.Bytes,
								})
							if err != nil {
								log.Error("Error occurred in sending data to TMCore: ", err)
								h.signalConnError <- struct{}{}
							}
							h.p2pConnection.sendMonitor.Update(int(_n))
							err = h.p2pConnection.bufConnWriter.Flush()
							if err != nil {
								log.Error("Cannot flush buffer: ", err)
								h.signalConnError <- struct{}{}
							}
						}
						h.throughput.putInfo("to", "+CsDcPRP", uint32(len(marlinMsg.Packets)))
					case *ProposalPOLMessage:
						// Not serviced
					case *BlockPartMessage:
						for _, pkt := range marlinMsg.Packets {
							_n, err := h.codec.MarshalBinaryLengthPrefixedWriter(
								h.p2pConnection.bufConnWriter,
								PacketMsg{
									ChannelID: byte(pkt.ChannelID),
									EOF:       byte(pkt.EOF),
									Bytes:     pkt.Bytes,
								})
							if err != nil {
								log.Error("Error occurred in sending data to TMCore: ", err)
								h.signalConnError <- struct{}{}
							}
							h.p2pConnection.sendMonitor.Update(int(_n))
							err = h.p2pConnection.bufConnWriter.Flush()
							if err != nil {
								log.Error("Cannot flush buffer: ", err)
								h.signalConnError <- struct{}{}
							}
						}
						h.throughput.putInfo("to", "+CsDcBPM", uint32(len(marlinMsg.Packets)))
					default:
						h.throughput.putInfo("to", "-CsDcUNK", uint32(len(marlinMsg.Packets)))
					}
				}
			default:
				h.throughput.putInfo("to", "-UnkUNK", uint32(len(marlinMsg.Packets)))
				log.Debug("TMCore <- connector Not servicing undecipherable channel ", marlinMsg.Channel)
			}
		}
	}
}

func (h *TendermintHandler) recvRoutine() {
	log.Info("TMCore -> Connector Routine Started")

FOR_LOOP:
	for {
		select {
		case <-h.signalShutRecv:
			log.Info("TMCore -> Connector Routine shutdown")
			break FOR_LOOP
		default:
		}
		h.p2pConnection.recvMonitor.Limit(20000, 5120000, true)

		/*
			Peek into bufConnReader for debugging

			if numBytes := c.bufConnReader.Buffered(); numBytes > 0 {
				bz, err := c.bufConnReader.Peek(cmn.MinInt(numBytes, 100))
				if err == nil {
					// return
				} else {
					log.Debug("Error peeking connection buffer ", "err ", err)
					// return nil
				}
				log.Info("Peek connection buffer ", "numBytes ", numBytes, " bz ", bz)
			}
		*/

		// Read packet type
		var packet Packet
		_n, err := h.codec.UnmarshalBinaryLengthPrefixedReader(
			h.p2pConnection.bufConnReader,
			&packet,
			int64(20000))

		h.p2pConnection.recvMonitor.Update(int(_n))

		// Unmarshalling test
		if err != nil {
			if err == io.EOF {
				log.Error("TMCore -> Connector Connection is closed (likely by the other side)")
			} else {
				log.Error("TMCore -> Connector Connection failed (reading byte): ", err)
			}
			h.signalConnError <- struct{}{}
			break FOR_LOOP
		}

		// Read more depending on packet type.
		switch pkt := packet.(type) {
		case PacketPing: // Received PING messages from TMCore
			select {
			case h.p2pConnection.pong <- PacketPong{}:
			default:
			}

		case PacketPong: // Received PONG messages from TMCore
			select {
			case h.p2pConnection.pongTimeoutCh <- false:
			default:
			}

		case PacketMsg: // Actual message packets from TMCore
			switch pkt.ChannelID {
			case channelBc:
				h.throughput.putInfo("from", "=BcMSG", 1)
				log.Debug("TMCore -> Connector Blockhain is not serviced")
			case channelCsSt:
				h.channelBuffer[channelCsSt] = append(h.channelBuffer[channelCsSt],
					marlinTypes.PacketMsg{
						ChannelID: uint32(pkt.ChannelID),
						EOF:       uint32(pkt.EOF),
						Bytes:     pkt.Bytes,
					})

				if pkt.EOF == byte(0x01) {
					msg, err := h.decodeConsensusMsgFromChannelBuffer(h.channelBuffer[channelCsSt])
					if err != nil {
						log.Error("Cannot decode message recieved from TMCore to a valid Consensus Message: ", err)
					} else {
						message := marlinTypes.MarlinMessage{
							ChainID: h.servicedChainId,
							Channel: channelCsSt,
							Packets: h.channelBuffer[channelCsSt],
						}

						switch msg.(type) {
						// Only NRS is sent forward
						case *NewRoundStepMessage:
							select {
							case h.marlinTo <- message:
							default:
								log.Warning("Too many messages in channel marlinTo. Dropping oldest messages")
								_ = <-h.marlinTo
								h.marlinTo <- message
							}
							select {
							case h.marlinFrom <- message:
							default:
								log.Warning("Too many messages in channel marlinFrom. Dropping oldest messages")
								_ = <-h.marlinFrom
								h.marlinFrom <- message
							}
							h.throughput.putInfo("from", "+CsStNRS", uint32(len(h.channelBuffer[channelCsSt])))
						case *NewValidBlockMessage:
							// h.throughput.putInfo("from", "=CsStNVB", uint32(len(h.channelBuffer[channelCsSt])))
						case *HasVoteMessage:
							// h.throughput.putInfo("from", "=CsStHVM", uint32(len(h.channelBuffer[channelCsSt])))
						case *VoteSetMaj23Message:
							// h.throughput.putInfo("from", "=CsStM23", uint32(len(h.channelBuffer[channelCsSt])))
						default:
							h.throughput.putInfo("from", "-CsStUNK", uint32(len(h.channelBuffer[channelCsSt])))
						}
					}
					h.channelBuffer[channelCsSt] = h.channelBuffer[channelCsSt][:0]
				}
			case channelCsDc:
				h.channelBuffer[channelCsDc] = append(h.channelBuffer[channelCsDc],
					marlinTypes.PacketMsg{
						ChannelID: uint32(pkt.ChannelID),
						EOF:       uint32(pkt.EOF),
						Bytes:     pkt.Bytes,
					})
				if pkt.EOF == byte(0x01) {
					msg, err := h.decodeConsensusMsgFromChannelBuffer(h.channelBuffer[channelCsDc])
					if err != nil {
						log.Error("Cannot decode message recieved from TMCore to a valid Consensus Message: ", err)
					} else {
						message := marlinTypes.MarlinMessage{
							ChainID: h.servicedChainId,
							Channel: channelCsDc,
							Packets: h.channelBuffer[channelCsDc],
						}

						switch msg.(type) {
						case *ProposalMessage:
							select {
							case h.marlinTo <- message:
							default:
								log.Warning("Too many messages in channel marlinTo. Dropping oldest messages")
								_ = <-h.marlinTo
								h.marlinTo <- message
							}
							h.throughput.putInfo("from", "+CsDcPRP", uint32(len(h.channelBuffer[channelCsDc])))
						case *ProposalPOLMessage:
							// Not serviced
						case *BlockPartMessage:
							select {
							case h.marlinTo <- message:
							default:
								log.Warning("Too many messages in channel marlinTo. Dropping oldest messages")
								_ = <-h.marlinTo
								h.marlinTo <- message
							}
							h.throughput.putInfo("from", "+CsDcBPM", uint32(len(h.channelBuffer[channelCsDc])))
						default:
							h.throughput.putInfo("from", "-CsDcMSG", uint32(len(h.channelBuffer[channelCsDc])))
						}
					}
					h.channelBuffer[channelCsDc] = h.channelBuffer[channelCsDc][:0]
					
				}
			case channelCsVo:
				h.channelBuffer[channelCsVo] = append(h.channelBuffer[channelCsVo],
					marlinTypes.PacketMsg{
						ChannelID: uint32(pkt.ChannelID),
						EOF:       uint32(pkt.EOF),
						Bytes:     pkt.Bytes,
					})
				if pkt.EOF == byte(0x01) {
					msg, err := h.decodeConsensusMsgFromChannelBuffer(h.channelBuffer[channelCsVo])
					if err != nil {
						log.Error("Cannot decode message recieved from TMCore to a valid Consensus Message: ", err)
					} else {
						message := marlinTypes.MarlinMessage{
							ChainID: h.servicedChainId,
							Channel: channelCsVo,
							Packets: h.channelBuffer[channelCsVo],
						}

						switch msg.(type) {
						case *VoteMessage:
							select {
							case h.marlinTo <- message:
							default:
								log.Warning("Too many messages in channel marlinTo. Dropping oldest messages")
								_ = <-h.marlinTo
								h.marlinTo <- message
							}
							h.throughput.putInfo("from", "+CsVoVOT", uint32(len(h.channelBuffer[channelCsVo])))
						default:
							h.throughput.putInfo("from", "-CsVoVOT", uint32(len(h.channelBuffer[channelCsVo])))
						}
					}
					h.channelBuffer[channelCsVo] = h.channelBuffer[channelCsVo][:0]
				}
			case channelCsVs:
				h.throughput.putInfo("from", "=CsVsVSB", 1)
				log.Debug("TMCore -> Connector Consensensus Vote Set Bits Channel is not serviced")
			case channelMm:
				h.throughput.putInfo("from", "=MmMSG", 1)
				log.Debug("TMCore -> Connector Mempool Channel is not serviced")
			case channelEv:
				h.throughput.putInfo("from", "=EvMSG", 1)
				log.Debug("TMCore -> Connector Evidence Channel is not serviced")
			default:
				h.throughput.putInfo("from", "=UnkUNK", 1)
				log.Warning("TMCore -> Connector Unknown ChannelID Message recieved: ", pkt.ChannelID)
			}

		default:
			log.Error("TMCore -> Connector Unknown message type ", reflect.TypeOf(packet))
			log.Error("TMCore -> Connector Connection failed: ", err)
			h.signalConnError <- struct{}{}
			break FOR_LOOP
		}
	}

	// Cleanup
	close(h.p2pConnection.pong)
	for range h.p2pConnection.pong {
		// Drain
	}
}

func (h *TendermintHandler) decodeConsensusMsgFromChannelBuffer(chanbuf []marlinTypes.PacketMsg) (ConsensusMessage, error) {
	var databuf []byte
	var msg ConsensusMessage
	var err error
	for _, pkt := range chanbuf {
		databuf = append(databuf, pkt.Bytes...)
	}
	if len(databuf) > 1048576 {
		return msg, errors.New("Message is larger than 1MB. Cannot decode")
	}
	err = h.codec.UnmarshalBinaryBare(databuf, &msg)
	return msg, err
}

func (c *P2PConnection) stopPongTimer() {
	if c.pongTimer != nil {
		_ = c.pongTimer.Stop()
		c.pongTimer = nil
	}
}

// ---------------------- SPAM FILTER INTERFACE --------------------------------

// RunSpamFilter serves as the entry point for a TM Core handler when serving as a spamfilter
func RunSpamFilter(rpcAddr string,
	marlinTo chan marlinTypes.MarlinMessage,
	marlinFrom chan marlinTypes.MarlinMessage) {
	log.Info("Starting Cosmos3 Tendermint SpamFilter - 0.16.3-d83fc038-2-mainnet")

	handler, err := createTMHandler("0.0.0.0:0", rpcAddr, marlinTo, marlinFrom, false, 0, false)
	if err != nil {
		log.Error("Error encountered while creating TM Handler: ", err)
		os.Exit(1)
	}

	marlin.AllowServicedChainMessages(handler.servicedChainId)

	RegisterPacket(handler.codec)
	RegisterConsensusMessages(handler.codec)

	coreCount := runtime.NumCPU()
	multiple := 2
	log.Info("Runtime found number of CPUs on machine to be ", coreCount, ". Hence, running ", multiple*coreCount, " spamfilter handlers.")
	
	for i := 0; i < multiple*coreCount; i++ {
		go handler.beginServicingSpamFilter(i)
	}

	handler.throughput.presentThroughput(5, handler.signalShutThroughput)
}

func (h *TendermintHandler) beginServicingSpamFilter(id int) {
	log.Info("Running TM side spam filter handler ", id)
	// Register Messages

	// TODO - SpamFilter never has to consult RPC server currently - since only CsSt+ is supported, write for that. v0.2 prerelease

	for marlinMsg := range h.marlinFrom {
		switch marlinMsg.Channel {
		case channelBc:
			h.throughput.putInfo("spam", "-CsBc", 1)
			log.Debug("TMCore <-> Marlin Blockhain is not serviced")
			h.marlinTo <- h.spamVerdictMessage(marlinMsg, false)
		case channelCsSt:
			msg, err := h.decodeConsensusMsgFromChannelBuffer(marlinMsg.Packets)
			if err != nil {
				h.throughput.putInfo("spam", "-CsStUNK", uint32(len(marlinMsg.Packets)))
				h.marlinTo <- h.spamVerdictMessage(marlinMsg, false)
			} else {
				switch msg.(type) {
				case *NewRoundStepMessage:
					h.throughput.putInfo("spam", "+CsStNRS", uint32(len(marlinMsg.Packets)))
					h.marlinTo <- h.spamVerdictMessage(marlinMsg, true)
				default:
					h.throughput.putInfo("spam", "-CsStUNK", uint32(len(marlinMsg.Packets)))
					h.marlinTo <- h.spamVerdictMessage(marlinMsg, false)
				}
			}
		case channelCsVo:
			msg, err := h.decodeConsensusMsgFromChannelBuffer(marlinMsg.Packets)
			if err != nil {
				h.throughput.putInfo("spam", "-CsVoUNK", uint32(len(marlinMsg.Packets)))
				h.marlinTo <- h.spamVerdictMessage(marlinMsg, false)
			} else {
				switch msg.(type) {
				case *VoteMessage:
					if h.thoroughMessageCheck(msg) {
						h.marlinTo <- h.spamVerdictMessage(marlinMsg, true)
						h.throughput.putInfo("spam", "+CsVoVOT", uint32(len(marlinMsg.Packets)))
					} else {
						h.marlinTo <- h.spamVerdictMessage(marlinMsg, false)
						h.throughput.putInfo("spam", "-CsVoVOT", uint32(len(marlinMsg.Packets)))
					}
				default:
					h.throughput.putInfo("spam", "-CsVoUNK", uint32(len(marlinMsg.Packets)))
					h.marlinTo <- h.spamVerdictMessage(marlinMsg, false)
				}
			}
		case channelCsDc:
			msg, err := h.decodeConsensusMsgFromChannelBuffer(marlinMsg.Packets)
			if err != nil {
				h.throughput.putInfo("spam", "-CsDcUNK", uint32(len(marlinMsg.Packets)))
				h.marlinTo <- h.spamVerdictMessage(marlinMsg, false)
			} else {
				switch msg.(type) {
				case *ProposalMessage:
					if h.thoroughMessageCheck(msg) {
						h.marlinTo <- h.spamVerdictMessage(marlinMsg, true)
						h.throughput.putInfo("spam", "+CsDcPRO", uint32(len(marlinMsg.Packets)))
					} else {
						h.marlinTo <- h.spamVerdictMessage(marlinMsg, false)
						h.throughput.putInfo("spam", "-CsDcPRO", uint32(len(marlinMsg.Packets)))
					}
				case *BlockPartMessage:
					if h.thoroughMessageCheck(msg) {
						h.marlinTo <- h.spamVerdictMessage(marlinMsg, true)
						h.throughput.putInfo("spam", "+CsDcBPM", uint32(len(marlinMsg.Packets)))
					} else {
						h.marlinTo <- h.spamVerdictMessage(marlinMsg, false)
						h.throughput.putInfo("spam", "-CsDcBPM", uint32(len(marlinMsg.Packets)))
					}
				default:
					h.throughput.putInfo("spam", "-CsVoUNK", uint32(len(marlinMsg.Packets)))
					h.marlinTo <- h.spamVerdictMessage(marlinMsg, false)
				}
			}
		case channelCsVs:
			h.throughput.putInfo("spam", "-CsVs", 1)
			h.marlinTo <- h.spamVerdictMessage(marlinMsg, false)
			log.Debug("TMCore <-> Marlin Consensensus Vote Set Bits Channel is not serviced")
		case channelMm:
			h.throughput.putInfo("spam", "-CsMm", 1)
			h.marlinTo <- h.spamVerdictMessage(marlinMsg, false)
			log.Debug("TMCore <-> Marlin Mempool Channel is not serviced")
		case channelEv:
			h.throughput.putInfo("spam", "-CsEv", 1)
			h.marlinTo <- h.spamVerdictMessage(marlinMsg, false)
			log.Debug("TMCore <-> MarlinEvidence Channel is not serviced")
		default:
			h.throughput.putInfo("spam", "-UnkUNK", 1)
			h.marlinTo <- h.spamVerdictMessage(marlinMsg, false)
		}
	}
}

func (h *TendermintHandler) thoroughMessageCheck(msg ConsensusMessage) bool {
	switch msg.(type) {
	case *VoteMessage:
		if validator, ok := h.getValidators(msg.(*VoteMessage).Vote.Height); ok {
			vidx := msg.(*VoteMessage).Vote.ValidatorIndex
			vaddr := msg.(*VoteMessage).Vote.ValidatorAddress.String()
			if vidx >= len(validator) || vaddr != validator[vidx].Address ||
			   !validator[vidx].PublicKey.VerifyBytes(msg.(*VoteMessage).Vote.SignBytes("cosmoshub-3", h.codec), msg.(*VoteMessage).Vote.Signature) {
				return false
			}
			return true
		}
		return false
	case *BlockPartMessage:
		// Cache hash verification, needs Proposal message support
		return false
	case *ProposalMessage:
		// if _, ok := h.getValidators(msg.(*ProposalMessage).Proposal.Height); ok {
		// 	// Check signature, add to map so that BPM messages can be verified
		// 	return true
		// }
		return false
	default:
		return false
	}
}

func (vote *Vote) SignBytes(chainID string, cdc *amino.Codec) []byte {
	bz, err := cdc.MarshalBinaryLengthPrefixed(CanonicalizeVote(chainID, vote))
	if err != nil {
		panic(err)
	}
	return bz
}

func (h *TendermintHandler) getValidators(height int64) ([]Validator, bool) {
	if height+10 < h.maxValidHeight {
		// Don't service messages too old
		return []Validator{}, false
	} else if h.validatorCache.Contains(height) {
		value, ok := h.validatorCache.Get(height)
		return value.([]Validator), ok
	} else {
		// log.Info("Asked about height: ", height)
		response, err := http.Get("http://"+h.rpcAddr+"/validators?height="+strconv.Itoa((int)(height)))
		defer response.Body.Close()
		if err != nil {
			log.Error("Error while sending request to get validators at height: ", height, " err: ", err)
			return []Validator{}, false
		} else {
			bodyBytes, err := ioutil.ReadAll(response.Body)
			if err != nil {
				log.Error("Error while parsing request to get validators at height: ", height, " err: ", err)
				return []Validator{}, false
			}
			var jsonResult map[string]interface{}
			json.Unmarshal(bodyBytes, &jsonResult)
			// verify interface for errors
			if _, errorFieldFound := jsonResult["error"]; errorFieldFound {
				return []Validator{}, false
			}
			validatorInfo := jsonResult["result"].(map[string]interface{})["validators"].([]interface{})

			var validatorSet []Validator
			for _, v := range(validatorInfo) {
				if v.(map[string]interface{})["pub_key"].(map[string]interface{})["type"] != "tendermint/PubKeyEd25519" {
					log.Error("Not all keys of validators are tendermint/PubKeyEd25519. Cannot continue with this validator set from TMCore")
					return []Validator{}, false
				}
				decodedSlice, err := b64.StdEncoding.DecodeString(v.(map[string]interface{})["pub_key"].(map[string]interface{})["value"].(string))
				if err != nil {
					return []Validator{}, false
				}
				var decodedArray [32]byte
				copy(decodedArray[:], decodedSlice[:32])
				validatorSet = append(validatorSet, 
					Validator{
						PublicKey: ed25519.PubKeyEd25519(decodedArray), 
						Address: v.(map[string]interface{})["address"].(string),
					})
			}
			h.validatorCache.Add(height, validatorSet)
			
			h.maxValidHeight = height
			return validatorSet, true
		}
	}
}

func (h *TendermintHandler) spamVerdictMessage(msg marlinTypes.MarlinMessage, allow bool) marlinTypes.MarlinMessage {
	if allow {
		return marlinTypes.MarlinMessage{
			ChainID:  h.servicedChainId,
			Channel:  byte(0x01),
			PacketId: msg.PacketId,
		}
	} else {
		return marlinTypes.MarlinMessage{
			ChainID:  h.servicedChainId,
			Channel:  byte(0x00),
			PacketId: msg.PacketId,
		}
	}
}

// ---------------------- KEY GENERATION INTERFACE -----------------------------

var ServicedKeyFile string = "cosmos-3"
var isKeyFileUsed, memoized bool
var keyFileLocation string
var privateKey ed25519.PrivKeyEd25519

func GenerateKeyFile(fileLocation string) {
	log.Info("Generating KeyPair for cosmos-3-mainnet")

	privateKey := ed25519.GenPrivKey()
	publicKey := privateKey.PubKey()

	key := keyData{
		Chain:            "cosmos-3-mainnet",
		IdString:         string(hex.EncodeToString(publicKey.Address())),
		PrivateKeyString: string(hex.EncodeToString(privateKey[:])),
		PublicKeyString:  string(hex.EncodeToString(publicKey.Bytes())),
		PrivateKey:       privateKey,
		PublicKey:        publicKey.(ed25519.PubKeyEd25519),
	}

	log.Info("ID for node after generating KeyPair: ", key.IdString)

	encodedJson, err := json.MarshalIndent(&key, "", "    ")
	if err != nil {
		log.Error("Error generating KeyFile: ", err)
	}
	err = ioutil.WriteFile(fileLocation, encodedJson, 0644)
	if err != nil {
		log.Error("Error generating KeyFile: ", err)
	}

	log.Info("Successfully written keyfile ", fileLocation)
}

func VerifyKeyFile(fileLocation string) (bool, error) {
	log.Info("Accessing disk to extract info from KeyFile: ", fileLocation)
	jsonFile, err := os.Open(fileLocation)
	// if we os.Open returns an error then handle it
	if err != nil {
		log.Error("Error accessing file KeyFile: ", fileLocation, " error: ", err, ". exiting application.")
		os.Exit(1)
	}
	defer jsonFile.Close()

	byteValue, err := ioutil.ReadAll(jsonFile)
	if err != nil {
		log.Error("Error decoding KeyFile: ", fileLocation, " error: ", err, ". exiting application.")
		os.Exit(1)
	}
	var key keyData
	json.Unmarshal(byteValue, &key)

	// TODO Check these conditions, add more checks - v0.2 prerelease
	if string(hex.EncodeToString(key.PrivateKey[:])) == key.PrivateKeyString {
		log.Info("Integrity for KeyFile: ", fileLocation, " checked. Integrity OK.")
		return true, nil
	} else {
		log.Error("Integrity for KeyFile: ", fileLocation, " checked. Integrity NOT OK.")
		return false, nil
	}
}

func getPrivateKey() ed25519.PrivKeyEd25519 {
	if !isKeyFileUsed {
		return ed25519.GenPrivKey()
	} else {
		if !memoized {
			valid, err := VerifyKeyFile(keyFileLocation)
			if err != nil {
				log.Error("Error verifying keyfile integrity: ", keyFileLocation)
				os.Exit(1)
			} else if !valid {
				os.Exit(1)
			}
			log.Info("Accessing disk to extract info from KeyFile: ", keyFileLocation)
			jsonFile, err := os.Open(keyFileLocation)
			// if we os.Open returns an error then handle it
			if err != nil {
				log.Error("Error accessing file KeyFile: ", keyFileLocation, " error: ", err, ". exiting application.")
				os.Exit(1)
			}
			defer jsonFile.Close()

			byteValue, err := ioutil.ReadAll(jsonFile)
			if err != nil {
				log.Error("Error decoding KeyFile: ", keyFileLocation, " error: ", err, ". exiting application.")
				os.Exit(1)
			}
			var key keyData
			json.Unmarshal(byteValue, &key)
			log.Info("Connector assumes for all connections henceforth the ID: ", key.IdString)
			privateKey = key.PrivateKey
			memoized = true
		}
		return privateKey
	}
}

// ---------------------- COMMON UTILITIES ---------------------------------


func createTMHandler(peerAddr string,
	rpcAddr string,
	marlinTo chan marlinTypes.MarlinMessage,
	marlinFrom chan marlinTypes.MarlinMessage,
	isConnectionOutgoing bool,
	listenPort int,
	isDataConnect bool) (TendermintHandler, error) {
	chainId, ok := marlinTypes.ServicedChains["cosmos-3-mainnet"]
	if !ok {
		return TendermintHandler{}, errors.New("Cannot find cosmos-3-mainnet in list of serviced chains by marlin connector")
	}

	privateKey := getPrivateKey()

	vCache, err := lru.New2Q(500)
	if err != nil {
		return TendermintHandler{}, err
	}

	return TendermintHandler{
		servicedChainId:      chainId,
		listenPort:           listenPort,
		isConnectionOutgoing: isConnectionOutgoing,
		peerAddr:             peerAddr,
		rpcAddr:              rpcAddr,
		privateKey:           privateKey,
		codec:                amino.NewCodec(),
		validatorCache:		  vCache,
		marlinTo:             marlinTo,
		marlinFrom:           marlinFrom,
		channelBuffer:        make(map[byte][]marlinTypes.PacketMsg),
		throughput: throughPutData{
			isDataConnect: isDataConnect,
			toTMCore:   make(map[string]uint32),
			fromTMCore: make(map[string]uint32),
			spam:		make(map[string]uint32),
		},
		signalConnError:      make(chan struct{}, 1),
		signalShutSend:       make(chan struct{}, 1),
		signalShutRecv:       make(chan struct{}, 1),
		signalShutThroughput: make(chan struct{}, 1),
	}, nil
}

func (t *throughPutData) putInfo(direction string, key string, count uint32) {
	t.mu.Lock()
	switch direction {
	case "to":
		t.toTMCore[key] = t.toTMCore[key] + count
	case "from":
		t.fromTMCore[key] = t.fromTMCore[key] + count
	case "spam":
		t.spam[key] = t.spam[key] + count
	}
	t.mu.Unlock()
}

func (t *throughPutData) presentThroughput(sec time.Duration, shutdownCh chan struct{}) {
	for {
		time.Sleep(sec * time.Second)

		select {
		case <-shutdownCh:
			return
		default:
		}
		t.mu.Lock()
		if t.isDataConnect {
			log.Info(fmt.Sprintf("[DataConnect stats] To TMCore %v\tFrom TMCore %v", t.toTMCore, t.fromTMCore))
		} else {
			log.Info(fmt.Sprintf("[SpamFilter stats] Served %v", t.spam))
		}
		t.toTMCore = make(map[string]uint32)
		t.fromTMCore = make(map[string]uint32)
		t.spam = make(map[string]uint32)
		t.mu.Unlock()
	}
}


// --- EXTRAS


// Canonical* wraps the structs in types for amino encoding them for use in SignBytes / the Signable interface.
// TimeFormat is used for generating the sigs
// const TimeFormat = time.RFC3339Nano

type CanonicalBlockID struct {
	Hash        cmn.HexBytes
	PartsHeader CanonicalPartSetHeader
}

type CanonicalPartSetHeader struct {
	Hash  cmn.HexBytes
	Total int
}

type CanonicalProposal struct {
	Type      byte // type alias for byte
	Height    int64         `binary:"fixed64"`
	Round     int64         `binary:"fixed64"`
	POLRound  int64         `binary:"fixed64"`
	BlockID   CanonicalBlockID
	Timestamp time.Time
	ChainID   string
}

type CanonicalVote struct {
	Type      byte // type alias for byte
	Height    int64         `binary:"fixed64"`
	Round     int64         `binary:"fixed64"`
	BlockID   CanonicalBlockID
	Timestamp time.Time
	ChainID   string
}

//-----------------------------------
// Canonicalize the structs

func CanonicalizeBlockID(blockID BlockID) CanonicalBlockID {
	return CanonicalBlockID{
		Hash:        blockID.Hash,
		PartsHeader: CanonicalizePartSetHeader(blockID.PartsHeader),
	}
}

func CanonicalizePartSetHeader(psh PartSetHeader) CanonicalPartSetHeader {
	return CanonicalPartSetHeader{
		psh.Hash,
		psh.Total,
	}
}

func CanonicalizeProposal(chainID string, proposal *Proposal) CanonicalProposal {
	return CanonicalProposal{
		Type:      byte(0x20),
		Height:    proposal.Height,
		Round:     int64(proposal.Round), // cast int->int64 to make amino encode it fixed64 (does not work for int)
		POLRound:  int64(proposal.POLRound),
		BlockID:   CanonicalizeBlockID(proposal.BlockID),
		Timestamp: proposal.Timestamp,
		ChainID:   chainID,
	}
}

func CanonicalizeVote(chainID string, vote *Vote) CanonicalVote {
	return CanonicalVote{
		Type:      vote.Type,
		Height:    vote.Height,
		Round:     int64(vote.Round), // cast int->int64 to make amino encode it fixed64 (does not work for int)
		Timestamp: vote.Timestamp,
		BlockID:   CanonicalizeBlockID(vote.BlockID),
		ChainID:   chainID,
	}
}

// // CanonicalTime can be used to stringify time in a canonical way.
// func CanonicalTime(t time.Time) string {
// 	// Note that sending time over amino resets it to
// 	// local time, we need to force UTC here, so the
// 	// signatures match
// 	return tmtime.Canonical(t).Format(TimeFormat)
// }
