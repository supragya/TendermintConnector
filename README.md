<p align="center">
  <img src="banner.png?raw=true" alt="Tendermint Connector Banner"/>
</p>

# Tendermint Connector
Tendermint Connector is a bridging application required for Tendermint based blockchains to interface with Marlin Protocol. Tendermint Connector connects as a peer to already running TM based blockchain and retrieves / pushes selective messages that the peer needs from / to Marlin Relays. Tendermint Connector also doubles up as a spam checker machanism for marlin relay.

## Serviced Blockchains
Currently, the following blockchains are serviced by **tendermint_connector**:
1. Irisnet Mainnet 0.16.3

## Building the application
Ensure that go is installed on the build system - ensure all is good using `go version`. This should return good version string. Proceed to build the application using:
```
make
make install
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
tendermint_connector --version
```

This should return you information such as this:
```
tendermint_connector version 0.1
+ [Tendermint Chain]   IRISNet Mainnet 0.16.3 (Consensus State transfers) v0.1
+ [Marlin TM Protocol] Marlin Tendermint Data Transfer Protocol v1
```

### tendermint_connector as a connector between TMCore and Marlin Relay
```
tendermint connect --marlindial
```
Tendermint connector will automatically try to connect using the default parameters.

You can learn more about the *tendermint_connector connect* command using `tendermint_connector connect -h`.

For configuring any of this for runtime, the following can be used for changed params:
```
tendermint_connector --server_address 127.0.0.2 --rpc_port 21222
```

### tendermint_connector as a spamfilter for Marlin Relay
```
tendermint spamfilter
```
Tendermint connector will run as a spam filter endpoint for verifying messages recieved at any relay endpoint. The expected format of message in spamfilter mode is proto3 encoded Marlin Tendermint Data Transfer Protocol messages. Spamfilter replies with a single byte (0 or 1) for whether message is spam (0), hence to be blocked or not (1), hence to be allowed.

You can learn more about the *tendermint_connector spamfilter* command using `tendermint_connector spamfilter -h`.

For configuring any of this for runtime, the following can be used for changed params:
```
tendermint_connector spamfilter --peerip 127.0.0.2 --rpcport 26667 --listenportmarlin 59004 
```

## Technical Overview
--- TECH OVERVIEW GOES HERE --