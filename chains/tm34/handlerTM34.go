package tm34

import (

	// "bytes"

	"bufio"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"os"
	"time"

	lru "github.com/hashicorp/golang-lru"
	log "github.com/sirupsen/logrus"
	"github.com/supragya/TendermintConnector/chains"
	"github.com/supragya/TendermintConnector/chains/tm34/conn"
	"github.com/supragya/TendermintConnector/chains/tm34/crypto/ed25519"
	"github.com/supragya/TendermintConnector/chains/tm34/libs/protoio"
	"github.com/supragya/TendermintConnector/chains/tm34/libs/timer"
	tmp2p "github.com/supragya/TendermintConnector/chains/tm34/proto/tendermint/p2p"
	"github.com/supragya/TendermintConnector/marlin"
	marlinTypes "github.com/supragya/TendermintConnector/types"
	flow "github.com/tendermint/tendermint/libs/flowrate"
	// Protocols
)

// ServicedTMCore is a string associated with each TM core handler
// to decipher which handler is to be attached.
var ServicedTMCore chains.NodeType = chains.NodeType{Version: "0.34.7", Network: "tmv34", ProtocolVersionApp: "1", ProtocolVersionBlock: "11", ProtocolVersionP2p: "8"}

// ---------------------- DATA CONNECT INTERFACE --------------------------------

func RunDataConnect(peerAddr string,
	marlinTo chan marlinTypes.MarlinMessage,
	marlinFrom chan marlinTypes.MarlinMessage,
	isConnectionOutgoing bool,
	keyFile string,
	listenPort int) {
	log.Info("Starting TM34 Tendermint Core Handler - Vanilla Tendermint")

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
			tmp2p.ProtocolVersion{App: 2, Block: 9, P2P: 5},
			string(hex.EncodeToString(h.privateKey.PubKey().Address())),
			"tcp://127.0.0.1:20016", //TODO Correct this - v0.2 prerelease
			"tmv34",
			"0.34.7",
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
	// go h.sendRoutine()
	// go h.recvRoutine()
	go h.throughput.presentThroughput(5, h.signalShutThroughput)

	// Allow Irisnet messages from marlin Relay
	marlin.AllowServicedChainMessages(h.servicedChainId)
	return nil
}

// ---------------------- KEY GENERATION INTERFACE -----------------------------

var ServicedKeyFile string = "tm34"
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
	chainId, ok := marlinTypes.ServicedChains["tm34"]
	if !ok {
		return TendermintHandler{}, errors.New("Cannot find tm34 in list of serviced chains by marlin connector")
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
