<p align="center">
  <img src="banner.png?raw=true" alt="Tendermint Connector Banner"/>
</p>

# Tendermint Connector
Tendermint connector is a bridging application required for Tendermint based blockchains to interface with Marlin Protocol. Tendermint Connector connects as a peer to already running TM based blockchain and retrieves / pushes selective messages that the peer needs from / to Marlin Relays.

## Serviced Blockchains
Currently, the following blockchains are serviced by **tendermint_connector**:
1. IrisNet Mainnet 0.16.3

## Building the application
Ensure that go is installed on the build system - ensure all is good using `go version`. This should return good version string. Proceed to build the application using:
```
cd build
go build ..
```

## Running the application
The tendermint_connector application provides the following features:
1. Full CLI interface (POSIX style flags)
2. Automatic Blockchaing Detection based on Node Info of peer
3. Secured On the fly ED25519 key pair generation and handshake for every session
4. Amino Codec / Proto3 based interfaces supported
5. Marlin Relay send / recieved proto3 messages - option for multiple encoding versions available

For finding version info of the application, supported chains and supported encodings for marlin relay, run:
```
./tendermint_connector --version
```

This should return you information such as this:
```
tendermint_connector version 0.1
+ [Tendermint Chain]   IRISNet Mainnet 0.16.3 (Consensus State transfers)
+ [Marlin TM Protocol] marlinTMSTfr1 - v0.1 Marlin TM Consensus State Transfer Protocol
```

Connect to a Tendermint node using
```
./tendermint connect
```
Tendermint connector will automatically try to connect using the following default information (available by `tendermint_connector connect --help`):
- Peer Node Port: 26656
- Peer Node RPC : 26657
- Peer Node IP  : 127.0.0.1
- Marlin Port   : 15003

For configuring any of this for runtime, the following can be used for changed params:
```
./tendermint_connector --server_address 127.0.0.2 --rpc_port 21222
```

