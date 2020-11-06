package irisnet

import (
	"bufio"
	"encoding/hex"
	"net"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/supragya/tendermint_connector/handlers"
	"github.com/supragya/tendermint_connector/handlers/irisnet/conn"
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
	bufConnReader := bufio.NewReaderSize(secretConn, 102400) // 100KB
	// bufConnWriter := bufio.NewWriterSize(c, 204800) // 200KB

	recvRoutine(bufConnReader)
}

func recvRoutine(rBuf *bufio.Reader) {
	for {
		log.Info("Input buffer size: ", rBuf.Buffered())
		time.Sleep(1 * time.Second)
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
				byte(0x40), //bc.BlockchainChannel,
				byte(0x20), //cs.StateChannel,
				byte(0x21), //cs.DataChannel,
				byte(0x22), //cs.VoteChannel,
				byte(0x23), //cs.VoteSetBitsChannel,
				byte(0x30), //mempl.MempoolChannel,
				byte(0x38), //evidence.EvidenceChannel,
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
