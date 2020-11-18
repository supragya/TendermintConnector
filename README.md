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

## Tendermint Connector concepts
Tendermint Connector is an application that provides full support for tendermint to interplay with Marlin Relay Network. Consequently, there are many roles that Tendermint Connector needs to play with a few variations in all of them. Here are a few concepts that is essential in understanding of Tendermint Connector

### Organisation
Tendermint Connector is organised in primarily two separate modules - **Marlin Side Connector** and **Peer Side Connector**. Marlin side connector is a module that is tasked with interactions pertaining to marlin relay network - interactions with bridge or relay abci are both handled by marlin side connector. Peer side connector is tasked with interactions pertaining to TMCore of different blockchains - These include recieving messages from Tendermint Core, validating, asking Marlin Side Connector to put messages onto relay, deliver messages recieved from relay to TMCore and also, check for spam messages if there are any.

Consequently the connector can be thought of as follows:
<p align="center">
  <img src="tm_connector_diagram.png?raw=true" alt="Tendermint Connector diagram"/>
</p>

### Running modes
Tendermint Connector runs in two different modes:
1. **DataConnect mode**: Tendermint Connector acts as a connecting bridge between a real tendermint based blockchain application (Connector talks to real TMCore) and Marlin TCP bridge. Most users would like to use this mode. In this mode, connector connects as a peer to real TMCore is capable of both sending messages to marlin side connector and recieving from it while interacting with TMCore.

2. **SpamFilter mode**: Tendermint Connector acts as a spamfilter servicing application at marlin relay nodes and interacts with `tm_abci`. Relay operators would like to use this mode.

### marlinTo and marlinFrom
**marlinTo** and **marlinFrom** are two channels which act as a connecting buffer between the marlin side connector and peer side connector. Both of these are capable of holding upto 1000 messages in each direction and are supposed to make both sides (marlin side / peer side) be agnostic of wire protocols each of them use. In case of overflow, oldest messages are dropped and shown as warning by tendermint_connector.

### Selecting a Peer Side Connector
Tendermint Connector aims to provide support for many tendermint based blockchains. Consequently, there needs to be quite similar however different peer side connectors that are engineered to support a specific blockchain. 

Decision of which peer side connector to use is done during runtime and is based on the nodeInfo that TMCore provides. Based on Block version and chain id of node, Tendermint Connector chooses a peer side connector that best fits to service that particular blockchain.

Currently, there is only one peer side connector written and supported:
1. Irisnet Mainnet 0.16.3

### Peer side connector TCP connections
TMCore peers communicate with each other on TCP connections. Hence, if communication is to be established with real TMCore, one must dial the real TMCore or "be dialed" by the other side. Tendermint Core can do both - dial a real TMCore or be dialed. 

Dialing a TMCore is simpler - Tendermint Connector generates keypair on the fly before connecting to TMCore - allowing Tendermint Connector to have different IDs between connections. This is ideal - since it circumvents issues when TMCore blacklists an ID (of Tendermint Connector) as a bad actor - since tendermint connector connects with a new identity.

Being dialed however is a little tricky - Tendermint Connector needs to listen on a specific port with certain nodeID. In this case when Tendermint Connector is dialed, the Connector needs to have consistent ID across multiple connections and cannot use new generated IDs on the fly. This ID can be persisted on the disk to be used across connections and across process runs by saving it as a **keyfile** (Keyfiles save private key / public key pairs). An example keyfile is given in `keyfiles` directory here.

There are certain time when being dialed is preferred way - by adding the Tendermint Connector fake peer as **persistent_peer** or **unconditional_peer** of real TMCore (More on how to do this below).

### Marlin side connector
Tendermint Connector's marlin side connectivity is not always TCP. While in "dataconnect" mode, the marlin side connector interacts with Marlin Bridge. However, while in "spamfilter" mode, the marlin side connector relies on unix sockets to service spamfilter responses. Marlin side connector handles both in separate modes.


## Running the application
The tendermint_connector application provides the following features:
1. Full CLI interface (POSIX style flags)
2. Automatic Blockchain detection based on Node Info of peer (RPC access to real TMCore required)
3. Secured On the fly ED25519 key pair generation and handshake for every session in peerdial

For finding version info of the application, supported chains and supported encodings for marlin relay, run:
```
tendermint_connector --version
```

This should return you information such as this:
```
tendermint_connector version 0.1-rc-1
+ [Tendermint Chain]   IRISNet Mainnet 0.16.3 (Consensus State transfers) v0.1
+ [Marlin TM Protocol] Marlin Tendermint Data Transfer Protocol v1
```

### tendermint_connector as a dataconnector between TMCore and Marlin Relay
```
tendermint_connector dataconnect --dial
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
tendermint_connector spamfilter --peerip 127.0.0.2 --rpcport 26667
```