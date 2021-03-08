package iris

import (

	// "bytes"

	"bufio"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"time"

	"github.com/gogo/protobuf/proto"
	lru "github.com/hashicorp/golang-lru"
	log "github.com/sirupsen/logrus"
	"github.com/supragya/TendermintConnector/chains"
	"github.com/supragya/TendermintConnector/chains/iris/conn"
	"github.com/supragya/TendermintConnector/chains/iris/crypto/ed25519"
	"github.com/supragya/TendermintConnector/chains/iris/crypto/merkle"
	"github.com/supragya/TendermintConnector/chains/iris/libs/bits"
	flow "github.com/supragya/TendermintConnector/chains/iris/libs/flowrate"
	tmmath "github.com/supragya/TendermintConnector/chains/iris/libs/math"
	"github.com/supragya/TendermintConnector/chains/iris/libs/protoio"
	"github.com/supragya/TendermintConnector/chains/iris/libs/timer"
	tmcons "github.com/supragya/TendermintConnector/chains/iris/proto/tendermint/consensus"
	tmp2p "github.com/supragya/TendermintConnector/chains/iris/proto/tendermint/p2p"
	tmproto "github.com/supragya/TendermintConnector/chains/iris/proto/tendermint/types"
	"github.com/supragya/TendermintConnector/marlin"
	marlinTypes "github.com/supragya/TendermintConnector/types"
)

// ServicedTMCore is a string associated with each TM core handler
// to decipher which handler is to be attached.
var ServicedTMCore chains.NodeType = chains.NodeType{Version: "", Network: "irishub-1", ProtocolVersionApp: "0", ProtocolVersionBlock: "11", ProtocolVersionP2p: "8"}

// ---------------------- DATA CONNECT INTERFACE --------------------------------

func RunDataConnect(peerAddr string,
	marlinTo chan marlinTypes.MarlinMessage,
	marlinFrom chan marlinTypes.MarlinMessage,
	isConnectionOutgoing bool,
	keyFile string,
	listenPort int) {
	log.Info("Starting iris Tendermint Core Handler - Vanilla Tendermint")

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
		}
		// else {
		// 	err = handler.acceptPeer()
		// }
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

func (h *TendermintHandler) upgradeConnectionAndHandshake() error {
	log.Info("Handshaking procedure")
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
		errc                              = make(chan error, 2)
		ourNodeInfo tmp2p.DefaultNodeInfo = tmp2p.DefaultNodeInfo{
			tmp2p.ProtocolVersion{App: 0, Block: 11, P2P: 8},
			string(hex.EncodeToString(h.privateKey.PubKey().Address())),
			"tcp://127.0.0.1:20016", //TODO Correct this - v0.2 prerelease
			"irishub-1",
			"",
			[]byte{channelBc, channelCsSt, channelCsDc, channelCsVo,
				channelCsVs, channelMm, channelEv},
			"marlin-tendermint-connector",
			tmp2p.DefaultNodeInfoOther{"on", "tcp://0.0.0.0:26667"}, // TODO: Correct this - v0.2 prerelease
		}
	)

	go func(errc chan<- error, c net.Conn) {
		_, err := protoio.NewDelimitedWriter(c).WriteMsg(&ourNodeInfo)
		if err != nil {
			log.Error("Error encountered while sending handshake message ", err)
		}
		errc <- err
	}(errc, h.secretConnection)

	go func(errc chan<- error, c net.Conn) {
		protoReader := protoio.NewDelimitedReader(c, 10240)
		_, err := protoReader.ReadMsg(&h.peerNodeInfo)
		if err != nil {
			log.Error("Error encountered while recieving handshake message ", err)
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
		flushTimer:      timer.NewThrottleTimer("flush", 100*time.Millisecond),
		pingTimer:       time.NewTicker(30 * time.Second),
		pongTimeoutCh:   make(chan bool, 1),
	}

	// Start P2P Send and recieve routines + Status messages for message throughput
	go h.sendRoutine()
	go h.recvRoutine()
	go h.throughput.presentThroughput(5, h.signalShutThroughput)

	// Allow Irisnet messages from marlin Relay
	marlin.AllowServicedChainMessages(h.servicedChainId)
	return nil
}

func (h *TendermintHandler) sendRoutine() {
	protoWriter := protoio.NewDelimitedWriter(h.p2pConnection.bufConnWriter)
	for {
		var _n int
		var err error
	SELECTION:
		select {
		case <-h.p2pConnection.pong:
			// log.Info("Send Pong")
			_n, err = protoWriter.WriteMsg(mustWrapPacket(&tmp2p.PacketPong{}))
			if err != nil {
				log.Error("Failed to send PacketPong", "err", err)
				break SELECTION
			}
			h.p2pConnection.sendMonitor.Update(_n)
			h.flush()

		// Actual message packets from Marlin Relay (encoded in Marlin Tendermint Data Transfer Protocol v1)
		case marlinMsg := <-h.marlinFrom:
			switch marlinMsg.Channel {
			case channelCsSt:
				msgEncoded, err := h.getBytesFromMarlinMessage(&marlinMsg)
				if err != nil {
					log.Debug("Cannot get bytes from marlin to a valid Consensus Message: ", err)
				}
				msg, err := decodeMsg(msgEncoded)
				if err != nil {
					log.Debug("Cannot decode messages to CsSt: ", err)
				}
				switch msg.(type) {
				case *NewRoundStepMessage:
					for _, pkt := range marlinMsg.Packets {
						_n, err := protoWriter.WriteMsg(mustWrapPacket(&tmp2p.PacketMsg{
							ChannelID: int32(channelCsSt),
							EOF:       pkt.EOF == 1,
							Data:      pkt.Bytes,
						}))
						if err != nil {
							log.Error("Error occurred in sending data to TMCore: ", err)
							// h.signalConnError <- struct{}{}
						}
						h.p2pConnection.sendMonitor.Update(int(_n))
						err = h.p2pConnection.bufConnWriter.Flush()
						if err != nil {
							log.Error("Cannot flush buffer: ", err)
						}
					}
				}
			}
		}
	}
}

func (h *TendermintHandler) getBytesFromMarlinMessage(marlinMsg *marlinTypes.MarlinMessage) ([]byte, error) {
	var databuf []byte
	for _, pkt := range marlinMsg.Packets {
		databuf = append(databuf, pkt.Bytes...)
	}
	return databuf, nil
}

func (h *TendermintHandler) flush() {
	err := h.p2pConnection.bufConnWriter.Flush()
	if err != nil {
		log.Error("BufConnWriter flush failed", "err", err)
	}
}

func (h *TendermintHandler) recvRoutine() {
	log.Info("TMCore -> Connector Routine Started")
	protoReader := protoio.NewDelimitedReader(h.p2pConnection.bufConnReader, 2000)

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
		var packet tmp2p.Packet
		_n, err := protoReader.ReadMsg(&packet)

		h.p2pConnection.recvMonitor.Update(int(_n))

		// Unmarshalling test
		if err != nil {
			if err == io.EOF {
				log.Error("TMCore -> Connector Connection is closed (likely by the other side) ", err)
			} else {
				log.Error("TMCore -> Connector Connection failed (reading byte): ", err)
			}
			h.signalConnError <- struct{}{}
			break FOR_LOOP
		}

		switch pkt := packet.Sum.(type) {
		case *tmp2p.Packet_PacketPing:
			// TODO: prevent abuse, as they cause flush()'s.
			// https://github.com/tendermint/tendermint/issues/1190
			// log.Info("Receive Ping ", pkt)
			select {
			case h.p2pConnection.pong <- struct{}{}:
			default:
				// never block
			}
		case *tmp2p.Packet_PacketPong:
			log.Info("Receive Pong")
			select {
			case h.p2pConnection.pongTimeoutCh <- false:
			default:
				// never block
			}
		case *tmp2p.Packet_PacketMsg:
			var eof uint32 = 0
			if pkt.PacketMsg.EOF {
				eof = 1
			}
			h.channelBuffer[byte(pkt.PacketMsg.ChannelID)] = append(h.channelBuffer[byte(pkt.PacketMsg.ChannelID)],
				marlinTypes.PacketMsg{
					ChannelID: uint32(pkt.PacketMsg.ChannelID),
					EOF:       eof,
					Bytes:     pkt.PacketMsg.Data,
				})

			// In case of incomplete message
			if !pkt.PacketMsg.EOF {
				log.Debug("TMCore -> Connector partial ", byte(pkt.PacketMsg.ChannelID))
				break
			}

			// Reach here only in case message has EOF. At this point the previous
			// messages are in h.channelBuffer[channel-idx]
			switch byte(pkt.PacketMsg.ChannelID) {
			case channelBc:
				h.throughput.putInfo("from", "=BcMSG", 1)
				log.Debug("TMCore -> Connector Blockhain is not serviced")
				h.channelBuffer[channelBc] = h.channelBuffer[channelBc][:0]
			case channelCsSt:
				log.Debug("TMCore -> Connector CsSt servicing")
				submessage, err := h.serviceConsensusStateMessage()
				if err != nil {
					log.Warning("Could not service consensus state message due to err: ", err)
				}
				h.throughput.putInfo("from", submessage, 1)
				h.channelBuffer[channelCsSt] = h.channelBuffer[channelCsSt][:0]
			case channelCsDc:
				log.Debug("TMCore -> Connector CsDc servicing")
				submessage, err := h.serviceConsensusDataMessage()
				if err != nil {
					log.Warning("Could not service consensus data message due to err: ", err)
				}
				h.throughput.putInfo("from", submessage, 1)
				h.channelBuffer[channelCsDc] = h.channelBuffer[channelCsDc][:0]
			case channelCsVo:
				log.Debug("TMCore -> Connector CsDo servicing")
				submessage, err := h.serviceConsensusVoteMessage()
				if err != nil {
					log.Warning("Could not service consensus vote message due to err: ", err)
				}
				h.throughput.putInfo("from", submessage, 1)
				h.channelBuffer[channelCsVo] = h.channelBuffer[channelCsVo][:0]
			case channelCsVs:
				h.throughput.putInfo("from", "=CsVsVSB", 1)
				log.Debug("TMCore -> Connector Consensus Vote Set Bits Channel is not serviced")
				h.channelBuffer[channelCsVs] = h.channelBuffer[channelCsVs][:0]
			case channelMm:
				h.throughput.putInfo("from", "=MmMSG", 1)
				log.Debug("TMCore -> Connector Mempool Channel is not serviced")
				h.channelBuffer[channelMm] = h.channelBuffer[channelMm][:0]
			case channelEv:
				h.throughput.putInfo("from", "=EvMSG", 1)
				log.Debug("TMCore -> Connector Evidence Channel is not serviced")
				h.channelBuffer[channelEv] = h.channelBuffer[channelEv][:0]
			default:
				h.throughput.putInfo("from", "=UnkUNK", 1)
				log.Warning("TMCore -> Connector Unknown ChannelID Message recieved: ", pkt.PacketMsg.ChannelID)
				h.channelBuffer[byte(pkt.PacketMsg.ChannelID)] = h.channelBuffer[byte(pkt.PacketMsg.ChannelID)][:0]
			}
		}
	}

	// Cleanup
	close(h.p2pConnection.pong)
	for range h.p2pConnection.pong {
		// Drain
	}
}

func (h *TendermintHandler) getBytesFromChannelBuffer(chanbuf []marlinTypes.PacketMsg) []byte {
	var databuf []byte
	for _, pkt := range chanbuf {
		databuf = append(databuf, pkt.Bytes...)
	}
	return databuf
}

func (h *TendermintHandler) serviceConsensusStateMessage() (string, error) {
	ch := channelCsSt
	msgBytes := h.getBytesFromChannelBuffer(h.channelBuffer[ch])
	msg, err := decodeMsg(msgBytes)
	if err != nil {
		return "", err
	}

	switch msg.(type) {
	case *NewRoundStepMessage:
		message := marlinTypes.MarlinMessage{
			ChainID: h.servicedChainId,
			Channel: ch,
			Packets: h.channelBuffer[ch],
		}
		// Send to marlin side
		select {
		case h.marlinTo <- message:
		default:
			log.Warning("Too many messages in channel marlinTo. Dropping oldest messages")
			_ = <-h.marlinTo
			h.marlinTo <- message
		}
		// Reflect Back NRS message to get CsVoVOT messages
		select {
		case h.marlinFrom <- message:
		default:
			log.Warning("Too many messages in channel marlinFrom. Dropping oldest messages")
			_ = <-h.marlinFrom
			h.marlinFrom <- message
		}
		return "+CsStNRS", nil
	case *Proposal:
		log.Debug("Found proposal, not servicing")
		return "-CsStPRO", nil
	case *NewValidBlockMessage:
		log.Debug("Found NewValidBlock, not servicing")
		return "-CsStNVB", nil
	case *HasVoteMessage:
		log.Debug("Found HasVote, not servicing")
		return "-CsStHVM", nil
	case *VoteSetMaj23Message:
		log.Debug("Found SetMaj23, not servicing")
		return "-CsStM23", nil
	default:
		log.Warning("Unknown Consensus state message ", msg)
		return "-CsStXXX", nil
	}
}

func (h *TendermintHandler) serviceConsensusDataMessage() (string, error) {
	ch := channelCsDc
	msgBytes := h.getBytesFromChannelBuffer(h.channelBuffer[ch])
	msg, err := decodeMsg(msgBytes)
	if err != nil {
		return "", err
	}

	switch msg.(type) {
	case *ProposalMessage:
		message := marlinTypes.MarlinMessage{
			ChainID: h.servicedChainId,
			Channel: ch,
			Packets: h.channelBuffer[ch],
		}
		// Send to marlin side
		select {
		case h.marlinTo <- message:
		default:
			log.Warning("Too many messages in channel marlinTo. Dropping oldest messages")
			_ = <-h.marlinTo
			h.marlinTo <- message
		}
		return "+CsDcPRO", nil
	case *ProposalPOLMessage:
		log.Debug("Found Proposal POL, not servicing")
		return "-CsDcPOL", nil
	case *BlockPartMessage:
		log.Debug("Found BlockPart, not servicing")
		return "-CsDcBPM", nil
	default:
		log.Warning("Unknown Consensus data message ", msg)
		return "-CsDcXXX", nil
	}
}

func (h *TendermintHandler) serviceConsensusVoteMessage() (string, error) {
	ch := channelCsVo
	msgBytes := h.getBytesFromChannelBuffer(h.channelBuffer[ch])
	msg, err := decodeMsg(msgBytes)
	if err != nil {
		return "", err
	}

	switch msg.(type) {
	case *VoteMessage:
		message := marlinTypes.MarlinMessage{
			ChainID: h.servicedChainId,
			Channel: ch,
			Packets: h.channelBuffer[ch],
		}
		// Send to marlin side
		select {
		case h.marlinTo <- message:
		default:
			log.Warning("Too many messages in channel marlinTo. Dropping oldest messages")
			_ = <-h.marlinTo
			h.marlinTo <- message
		}
		return "+CsVoVOT", nil
	default:
		log.Warning("Unknown Consensus vote message ", msg)
		return "-CsVoXXX", nil
	}
}

// ---------------------- KEY GENERATION INTERFACE -----------------------------

var ServicedKeyFile string = "iris"
var isKeyFileUsed, memoized bool
var keyFileLocation string

var privateKey ed25519.PrivKey

// ---------------------- COMMON UTILITIES ---------------------------------

func createTMHandler(peerAddr string,
	rpcAddr string,
	marlinTo chan marlinTypes.MarlinMessage,
	marlinFrom chan marlinTypes.MarlinMessage,
	isConnectionOutgoing bool,
	listenPort int,
	isDataConnect bool) (TendermintHandler, error) {
	chainId, ok := marlinTypes.ServicedChains["irisnet-1.0-mainnet"]
	if !ok {
		return TendermintHandler{}, errors.New("Cannot find iris in list of serviced chains by marlin connector")
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
		// codec:                amino.NewCodec(),
		validatorCache: vCache,
		marlinTo:       marlinTo,
		marlinFrom:     marlinFrom,
		channelBuffer:  make(map[byte][]marlinTypes.PacketMsg),
		throughput: throughPutData{
			isDataConnect: isDataConnect,
			toTMCore:      make(map[string]uint32),
			fromTMCore:    make(map[string]uint32),
			spam:          make(map[string]uint32),
		},
		signalConnError:      make(chan struct{}, 1),
		signalShutSend:       make(chan struct{}, 1),
		signalShutRecv:       make(chan struct{}, 1),
		signalShutThroughput: make(chan struct{}, 1),
	}, nil
}

func getPrivateKey() ed25519.PrivKey {
	// if !isKeyFileUsed {
	return ed25519.GenPrivKey()
	// }
	// else {
	// 	if !memoized {
	// 		valid, err := VerifyKeyFile(keyFileLocation)
	// 		if err != nil {
	// 			log.Error("Error verifying keyfile integrity: ", keyFileLocation)
	// 			os.Exit(1)
	// 		} else if !valid {
	// 			os.Exit(1)
	// 		}
	// 		log.Info("Accessing disk to extract info from KeyFile: ", keyFileLocation)
	// 		jsonFile, err := os.Open(keyFileLocation)
	// 		// if we os.Open returns an error then handle it
	// 		if err != nil {
	// 			log.Error("Error accessing file KeyFile: ", keyFileLocation, " error: ", err, ". exiting application.")
	// 			os.Exit(1)
	// 		}
	// 		defer jsonFile.Close()

	// 		byteValue, err := ioutil.ReadAll(jsonFile)
	// 		if err != nil {
	// 			log.Error("Error decoding KeyFile: ", keyFileLocation, " error: ", err, ". exiting application.")
	// 			os.Exit(1)
	// 		}
	// 		var key keyData
	// 		json.Unmarshal(byteValue, &key)
	// 		log.Info("Connector assumes for all connections henceforth the ID: ", key.IdString)
	// 		privateKey = key.PrivateKey
	// 		memoized = true
	// 	}
	// 	return privateKey
	// }
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

//-----------------------------------------------------------------------------
// Messages

// Message is a message that can be sent and received on the Reactor
type Message interface {
	// ValidateBasic() error // No restrictions
}

func decodeMsg(bz []byte) (msg Message, err error) {
	pb := &tmcons.Message{}
	if err = proto.Unmarshal(bz, pb); err != nil {
		return msg, err
	}

	return MsgFromProto(pb)
}

// MsgFromProto takes a consensus proto message and returns the native go type
func MsgFromProto(msg *tmcons.Message) (Message, error) {
	if msg == nil {
		return nil, errors.New("consensus: nil message")
	}
	var pb Message

	switch msg := msg.Sum.(type) {
	case *tmcons.Message_NewRoundStep:
		rs, err := tmmath.SafeConvertUint8(int64(msg.NewRoundStep.Step))
		// deny message based on possible overflow
		if err != nil {
			return nil, fmt.Errorf("denying message due to possible overflow: %w", err)
		}
		pb = &NewRoundStepMessage{
			Height:                msg.NewRoundStep.Height,
			Round:                 msg.NewRoundStep.Round,
			Step:                  int8(rs),
			SecondsSinceStartTime: msg.NewRoundStep.SecondsSinceStartTime,
			LastCommitRound:       msg.NewRoundStep.LastCommitRound,
		}
	case *tmcons.Message_NewValidBlock:
		pbPartSetHeader, err := PartSetHeaderFromProto(&msg.NewValidBlock.BlockPartSetHeader)
		if err != nil {
			return nil, fmt.Errorf("parts to proto error: %w", err)
		}

		pbBits := new(bits.BitArray)
		pbBits.FromProto(msg.NewValidBlock.BlockParts)

		pb = &NewValidBlockMessage{
			Height:             msg.NewValidBlock.Height,
			Round:              msg.NewValidBlock.Round,
			BlockPartSetHeader: *pbPartSetHeader,
			BlockParts:         pbBits,
			IsCommit:           msg.NewValidBlock.IsCommit,
		}
	case *tmcons.Message_Proposal:
		pbP, err := ProposalFromProto(&msg.Proposal.Proposal)
		if err != nil {
			return nil, fmt.Errorf("proposal msg to proto error: %w", err)
		}

		pb = &ProposalMessage{
			Proposal: pbP,
		}
	case *tmcons.Message_ProposalPol:
		pbBits := new(bits.BitArray)
		pbBits.FromProto(&msg.ProposalPol.ProposalPol)
		pb = &ProposalPOLMessage{
			Height:           msg.ProposalPol.Height,
			ProposalPOLRound: msg.ProposalPol.ProposalPolRound,
			ProposalPOL:      pbBits,
		}
	case *tmcons.Message_BlockPart:
		parts, err := PartFromProto(&msg.BlockPart.Part)
		if err != nil {
			return nil, fmt.Errorf("blockpart msg to proto error: %w", err)
		}
		pb = &BlockPartMessage{
			Height: msg.BlockPart.Height,
			Round:  msg.BlockPart.Round,
			Part:   parts,
		}
	case *tmcons.Message_Vote:
		vote, err := VoteFromProto(msg.Vote.Vote)
		if err != nil {
			return nil, fmt.Errorf("vote msg to proto error: %w", err)
		}

		pb = &VoteMessage{
			Vote: vote,
		}
	case *tmcons.Message_HasVote:
		pb = &HasVoteMessage{
			Height: msg.HasVote.Height,
			Round:  msg.HasVote.Round,
			Type:   msg.HasVote.Type,
			Index:  msg.HasVote.Index,
		}
	case *tmcons.Message_VoteSetMaj23:
		bi, err := BlockIDFromProto(&msg.VoteSetMaj23.BlockID)
		if err != nil {
			return nil, fmt.Errorf("voteSetMaj23 msg to proto error: %w", err)
		}
		pb = &VoteSetMaj23Message{
			Height:  msg.VoteSetMaj23.Height,
			Round:   msg.VoteSetMaj23.Round,
			Type:    msg.VoteSetMaj23.Type,
			BlockID: *bi,
		}
	case *tmcons.Message_VoteSetBits:
		bi, err := BlockIDFromProto(&msg.VoteSetBits.BlockID)
		if err != nil {
			return nil, fmt.Errorf("voteSetBits msg to proto error: %w", err)
		}
		bits := new(bits.BitArray)
		bits.FromProto(&msg.VoteSetBits.Votes)

		pb = &VoteSetBitsMessage{
			Height:  msg.VoteSetBits.Height,
			Round:   msg.VoteSetBits.Round,
			Type:    msg.VoteSetBits.Type,
			BlockID: *bi,
			Votes:   bits,
		}
	default:
		return nil, fmt.Errorf("consensus: message not recognized: %T", msg)
	}

	// if err := pb.ValidateBasic(); err != nil {
	// 	return nil, err
	// }

	return pb, nil
}

// FromProto sets a protobuf PartSetHeader to the given pointer
func PartSetHeaderFromProto(ppsh *tmproto.PartSetHeader) (*PartSetHeader, error) {
	if ppsh == nil {
		return nil, errors.New("nil PartSetHeader")
	}
	psh := new(PartSetHeader)
	psh.Total = ppsh.Total
	psh.Hash = ppsh.Hash

	return psh, nil
}

// FromProto sets a protobuf BlockID to the given pointer.
// It returns an error if the block id is invalid.
func BlockIDFromProto(bID *tmproto.BlockID) (*BlockID, error) {
	if bID == nil {
		return nil, errors.New("nil BlockID")
	}

	blockID := new(BlockID)
	ph, err := PartSetHeaderFromProto(&bID.PartSetHeader)
	if err != nil {
		return nil, err
	}

	blockID.PartSetHeader = *ph
	blockID.Hash = bID.Hash

	return blockID, nil
}

// FromProto sets a protobuf Proposal to the given pointer.
// It returns an error if the proposal is invalid.
func ProposalFromProto(pp *tmproto.Proposal) (*Proposal, error) {
	if pp == nil {
		return nil, errors.New("nil proposal")
	}

	p := new(Proposal)

	blockID, err := BlockIDFromProto(&pp.BlockID)
	if err != nil {
		return nil, err
	}

	p.BlockID = *blockID
	p.Type = pp.Type
	p.Height = pp.Height
	p.Round = pp.Round
	p.POLRound = pp.PolRound
	p.Timestamp = pp.Timestamp
	p.Signature = pp.Signature

	return p, nil
}

func PartFromProto(pb *tmproto.Part) (*Part, error) {
	if pb == nil {
		return nil, errors.New("nil part")
	}

	part := new(Part)
	proof, err := merkle.ProofFromProto(&pb.Proof)
	if err != nil {
		return nil, err
	}
	part.Index = pb.Index
	part.Bytes = pb.Bytes
	part.Proof = *proof

	return part, nil
}

// FromProto converts a proto generetad type to a handwritten type
// return type, nil if everything converts safely, otherwise nil, error
func VoteFromProto(pv *tmproto.Vote) (*Vote, error) {
	if pv == nil {
		return nil, errors.New("nil vote")
	}

	blockID, err := BlockIDFromProto(&pv.BlockID)
	if err != nil {
		return nil, err
	}

	vote := new(Vote)
	vote.Type = pv.Type
	vote.Height = pv.Height
	vote.Round = pv.Round
	vote.BlockID = *blockID
	vote.Timestamp = pv.Timestamp
	vote.ValidatorAddress = pv.ValidatorAddress
	vote.ValidatorIndex = pv.ValidatorIndex
	vote.Signature = pv.Signature

	return vote, nil
}

//----------------------------------------
// Packet

// mustWrapPacket takes a packet kind (oneof) and wraps it in a tmp2p.Packet message.
func mustWrapPacket(pb proto.Message) *tmp2p.Packet {
	var msg tmp2p.Packet

	switch pb := pb.(type) {
	case *tmp2p.Packet: // already a packet
		msg = *pb
	case *tmp2p.PacketPing:
		msg = tmp2p.Packet{
			Sum: &tmp2p.Packet_PacketPing{
				PacketPing: pb,
			},
		}
	case *tmp2p.PacketPong:
		msg = tmp2p.Packet{
			Sum: &tmp2p.Packet_PacketPong{
				PacketPong: pb,
			},
		}
	case *tmp2p.PacketMsg:
		msg = tmp2p.Packet{
			Sum: &tmp2p.Packet_PacketMsg{
				PacketMsg: pb,
			},
		}
	default:
		panic(fmt.Errorf("unknown packet type %T", pb))
	}

	return &msg
}
