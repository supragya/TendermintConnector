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
tendermint_connector connect --marlindial --dial
```
Tendermint connector will automatically try to connect using the default parameters.

You can learn more about the *tendermint_connector connect* command using `tendermint_connector connect -h`.

For configuring any of this for runtime, the following can be used for changed params:
```
tendermint_connector --server_address 127.0.0.2 --rpc_port 21222
```

### tendermint_connector keyfiles for persistent peer connection between TMCore and tendermint_connector
```
tendermint_connector keyfile
```
Tendermint connector can act as a tendermint peer who listens for connections instead of dialing TMCore itself. For this, you may need a persistent node ID to put in `config.toml` file for your real tendermint node. This is of format: *ae239af43..9bd7@127.0.0.1:266657*. This is essentially **nodeID@IP:PORT**. A keyfile for tendermint_connector is a file which describes the nodeID for tendermint_connector to use across process runs; it provides tendermint_connector with keys for fulfilling it's job as the given nodeID.

For example, you can generate a keyfile for irisnet chain using the following command:
```
tendermint_connector keyfile --chain irisnet --filelocation irisnetkeys.json --generate
```

This would return logs similar to below:
```
[INFO]:2020-11-13 15:32:51 - Generating KeyPair for irisnet-0.16.3-mainnet
[INFO]:2020-11-13 15:32:51 - ID for node after generating KeyPair: 376378adce3c6e3fde8201a4926b4adae6cb72e0
[INFO]:2020-11-13 15:32:51 - Successfully written keyfile irisnetkeys.json
```

This would create a new file `irisnetkeys.json` which contains keys relevant to peer nodeID `376378adce3c6e3fde8201a4926b4adae6cb72e0`. You can now put `376378adce3c6e3fde8201a4926b4adae6cb72e0@127.0.0.1:59001` (for localhost) as a persistent peer of your real tendermint node by editing your *config.toml* or passing peer as a command line argument.

Subsequent runs of `tendermint_connector connect` would require this keyfile to be provided to the program using command such as follows:
```
tendermint_connector connect --keyfile irisnetkeys.json
```

You may wish to verify the integrity of generated keyfiles. This can be done by simply (no `--generate` flag):
```
tendermint_connector keyfile --keyfile irisnetkeys.json --chain irisnet
```

### tendermint_connector as a spamfilter for Marlin Relay
```
tendermint_connector spamfilter
```
Tendermint connector will run as a spam filter endpoint for verifying messages recieved at any relay endpoint. The expected format of message in spamfilter mode is proto3 encoded Marlin Tendermint Data Transfer Protocol messages. Spamfilter replies with a single byte (0 or 1) for whether message is spam (0), hence to be blocked or not (1), hence to be allowed.

You can learn more about the *tendermint_connector spamfilter* command using `tendermint_connector spamfilter -h`.

For configuring any of this for runtime, the following can be used for changed params:
```
tendermint_connector spamfilter --peerip 127.0.0.2 --rpcport 26667 --listenportmarlin 59004 
```

## Technical Overview
--- TECH OVERVIEW GOES HERE --