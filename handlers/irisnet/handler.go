package irisnet

import (
	"encoding/hex"
	"net"
	"time"
	"os"
	"bufio"
	"io"
	"reflect"

	log "github.com/sirupsen/logrus"
	"github.com/supragya/tendermint_connector/handlers"
	"github.com/supragya/tendermint_connector/handlers/irisnet/conn"
	flow "github.com/supragya/tendermint_connector/handlers/irisnet/libs/flowrate"
	cmn "github.com/supragya/tendermint_connector/handlers/irisnet/libs/common"
	amino "github.com/tendermint/go-amino"
	"github.com/tendermint/tendermint/crypto/ed25519"
)

var ServicedTMCore handlers.NodeType = handlers.NodeType{Version: "0.32.2", Network: "irishub", ProtocolVersionApp: "2", ProtocolVersionBlock: "9", ProtocolVersionP2p: "5"}

func Run(peerAddr string) {
	log.Info("Starting Irisnet Tendermint Core Handler - 0.16.3-d83fc038-2-mainnet")

	log.Info("Attempting to open TCP connection to ", peerAddr)

	c, err := net.Dial("tcp", peerAddr)
	if err != nil {
		log.Error("Error opening TCP connection to ", peerAddr)
	} else {
		log.Info("Successfully opened TCP connection to ", peerAddr)
	}

	privateKey := ed25519.GenPrivKey()
	nodeIDString := string(hex.EncodeToString(privateKey.PubKey().Address()))

	log.Info("Generating ED25519 Key Pair for connector. Connector assuming nodeID: ", nodeIDString)

	secretConn, _ := conn.MakeSecretConnection(c, privateKey)
	remoteID := string(hex.EncodeToString(secretConn.RemotePubKey().Address()))

	log.Info("Initiating handshake with Node ", remoteID, " using amino codec")

	cdc := amino.NewCodec()
	nodeInfo, handshakeErr := handshake(secretConn, nodeIDString, cdc)

	if handshakeErr != nil {
		log.Error("Error encountered while handshaking")
	}

	log.Info("P2P handshake successful with ", remoteID, "; node has moniker: ", nodeInfo.Moniker)

	log.Info("Setting up persistent connectivity with TM Core")
	setupConnection(cdc, secretConn)

	time.Sleep(100000 * time.Second)
}

func setupConnection(cdc *amino.Codec, secretConn net.Conn) {
	// Register Packet Types
	RegisterPacket(cdc)

	// Create a P2P Connection
	p2pConnection := P2PConnection{
		conn:	secretConn,
		bufConnReader: bufio.NewReaderSize(secretConn, 65535),
		bufConnWriter: bufio.NewWriterSize(secretConn, 65535),
		sendMonitor: flow.New(0,0),
		recvMonitor: flow.New(0,0),
		send:	make(chan struct{}, 1),
		pong:	make(chan struct{}, 1),
		doneSendRoutine: make(chan struct{}, 1),
		quitSendRoutine: make(chan struct{}, 1),
		quitRecvRoutine: make(chan struct{}, 1),
		flushTimer: cmn.NewThrottleTimer("flush", 100 * time.Millisecond),
		pingTimer: time.NewTicker(30 * time.Second),
		pongTimeoutCh: make(chan bool, 1),
	}

	// Start P2P Send and recieve routines
	go p2pConnection.sendRoutine(cdc)
	go p2pConnection.recvRoutine(cdc)

}
// sendRoutine polls for packets to send from channels.
func (c *P2PConnection) sendRoutine(cdc *amino.Codec) {
	log.Info("Started sendRoutine")
	for {
		// var _n int64
	SELECTION:
		select {
		// case <-c.flushTimer.Ch:
		// 	// NOTE: flushTimer.Set() must be called every time
		// 	// something is written to .bufConnWriter.
		// 	c.flush()
		// case <-c.chStatsTimer.C:
		// 	for _, channel := range c.channels {
		// 		channel.updateStats()
		// 	}
		case <-c.pingTimer.C:
			_n, err := cdc.MarshalBinaryLengthPrefixedWriter(c.bufConnWriter, PacketPing{})
			if err != nil {
				break SELECTION
			}
			c.sendMonitor.Update(int(_n))
			c.pongTimer = time.AfterFunc(60 * time.Second, func() {
				select {
				case c.pongTimeoutCh <- true:
				default:
				}
			})
			c.flush()
		case timeout := <-c.pongTimeoutCh:
			if timeout {
				log.Error("Pong timeout, Probably Irisnode is dead. Closing Connector")
				os.Exit(3)
			} else {
				c.stopPongTimer()
			}
		case <-c.pong:
			_n, err := cdc.MarshalBinaryLengthPrefixedWriter(c.bufConnWriter, PacketPong{})
			if err != nil {
				break SELECTION
			}
			c.sendMonitor.Update(int(_n))
			c.flush()
		// case <-c.quitSendRoutine:
		// 	break FOR_LOOP
		// case <-c.send:
		// 	// Send some PacketMsgs
		// 	eof := c.sendSomePacketMsgs()
		// 	if !eof {
		// 		// Keep sendRoutine awake.
		// 		select {
		// 		case c.send <- struct{}{}:
		// 		default:
		// 		}
		// 	}
		}

	}

	log.Error("Closing sendRoutine")
	// Cleanup
	c.stopPongTimer()
	close(c.doneSendRoutine)
}

func (c *P2PConnection) flush() {
	log.Debug("Flush", "conn", c)
	err := c.bufConnWriter.Flush()
	if err != nil {
		log.Error("P2PConnection send flush failed with error: ", err)
		os.Exit(3)
	}
}

func (c *P2PConnection) stopPongTimer() {
	if c.pongTimer != nil {
		_ = c.pongTimer.Stop()
		c.pongTimer = nil
	}
}

func (c *P2PConnection) recvRoutine(cdc *amino.Codec) {
	log.Info("Started recvRoutine")
FOR_LOOP:
	for {
		// Block until .recvMonitor says we can read.
		c.recvMonitor.Limit(20000, 5120000, true)

		// Peek into bufConnReader for debugging

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
		
		// Read packet type
		var packet Packet
		_n, err := cdc.UnmarshalBinaryLengthPrefixedReader(c.bufConnReader, &packet, int64(20000))
		c.recvMonitor.Update(int(_n))

		if err != nil {
			// stopServices was invoked and we are shutting down
			// receiving is excpected to fail since we will close the connection
			select {
			case <-c.quitRecvRoutine:
				break FOR_LOOP
			default:
			}

			if err == io.EOF {
				log.Info("Connection is closed @ recvRoutine (likely by the other side)", "conn", c)
			} else {
				log.Error("Connection failed @ recvRoutine (reading byte)", " conn ", c, " err ", err)
			}
			break FOR_LOOP
		}

		// Read more depending on packet type.
		switch pkt := packet.(type) {
		case PacketPing:
			// TODO: prevent abuse, as they cause flush()'s.
			// https://github.com/tendermint/tendermint/issues/1190
			log.Debug("Receive Ping")
			select {
			case c.pong <- PacketPong{}:
			default:
				// never block
			}
		case PacketPong:
			log.Debug("Receive Pong")
			select {
			case c.pongTimeoutCh <- false:
			default:
				// never block
			}
		case PacketMsg:
			switch pkt.ChannelID {
			case channelBc:
				log.Info("Recieved BlockChain Channel message")
			case channelCsSt:
				log.Info("Recieved Consensus State Channel message")
			case channelCsDC:
				log.Info("Recieved Consensus Data Channel message")
			case channelCsVo:
				log.Info("Recieved Consensus Vote Channel message")
			case channelCsVs:
				log.Info("Recieved Consensus Vote setbits Channel message")
			case channelMm:
				log.Info("Recieved Consensus Mempool Channel message")
			case channelEv:
				log.Info("Recieved Consensus Evidence Channel message")
			default:
				log.Error("Unknown ChannelID Message recieved. Cannot service this message")
			}
			// channel, ok := c.channelsIdx[pkt.ChannelID]
			// if !ok || channel == nil {
			// 	err := fmt.Errorf("Unknown channel %X", pkt.ChannelID)
			// 	log.Error("Connection failed @ recvRoutine", "conn", c, "err", err)
			// 	c.stopForError(err)
			// 	break FOR_LOOP
			// }

			// msgBytes, err := channel.recvPacketMsg(pkt)
			// if err != nil {
			// 	log.Error("Connection failed @ recvRoutine", "conn", c, "err", err)
			// 	c.stopForError(err)
			// 	break FOR_LOOP
			// }
			// if msgBytes != nil {
			// 	c.Logger.Debug("Received bytes", "chID", pkt.ChannelID, "msgBytes", fmt.Sprintf("%X", msgBytes))
			// 	// NOTE: This means the reactor.Receive runs in the same thread as the p2p recv routine
			// 	c.onReceive(pkt.ChannelID, msgBytes)
			// }
		default:
			log.Errorf("Unknown message type ", reflect.TypeOf(packet))
			log.Error("Connection failed @ recvRoutine", "conn", c, "err", err)
			break FOR_LOOP
		}
	}

	// Cleanup
	close(c.pong)
	for range c.pong {
		// Drain
	}
}


func handshake(
	c net.Conn,
	nodeIDString string,
	cdc *amino.Codec,
) (DefaultNodeInfo, error) {
	var (
		errc         = make(chan error, 2)
		peerNodeInfo DefaultNodeInfo
		ourNodeInfo  DefaultNodeInfo = DefaultNodeInfo{
			ProtocolVersion{App: 2, Block: 9, P2P: 5},
			nodeIDString,
			"tcp://127.0.0.1:20006",
			"irishub",
			"0.32.2",
			[]byte{
				channelBc,
				channelCsSt,
				channelCsDC,
				channelCsVo,
				channelCsVs,
				channelMm,
				channelEv,
			},
			"marlin-tendermint-connector",
			DefaultNodeInfoOther{"on", "tcp://0.0.0.0:26667"},
		}
	)

	go func(errc chan<- error, c net.Conn) {
		_, err := cdc.MarshalBinaryLengthPrefixedWriter(c, ourNodeInfo)
		if err != nil {
			log.Error("Error encountered while sending handshake message")
		}
		errc <- err
	}(errc, c)
	go func(errc chan<- error, c net.Conn) {
		_, err := cdc.UnmarshalBinaryLengthPrefixedReader(
			c,
			&peerNodeInfo,
			int64(10240), // 10KB MaxNodeInfoSize()
		)
		if err != nil {
			log.Error("Error encountered while recieving handshake message")
		}
		errc <- err
	}(errc, c)

	for i := 0; i < cap(errc); i++ {
		err := <-errc
		if err != nil {
			log.Error("Encountered error in handshake with TM core: ", err)
			return DefaultNodeInfo{}, err
		}
	}

	return peerNodeInfo, c.SetDeadline(time.Time{})
}
