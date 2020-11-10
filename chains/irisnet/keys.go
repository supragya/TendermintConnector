package irisnet

import (
	"io/ioutil"
	"encoding/json"
	"encoding/hex"

	log "github.com/sirupsen/logrus"
	"github.com/tendermint/tendermint/crypto/ed25519"
)

var ServicedKeyFile string = "irisnet"

type keyData struct {
	Chain		string
	IdString	string
	PrivateKeyString	string
	PublicKeyString		string
	PrivateKey	[64]byte
	PublicKey	[32]byte
}

func GenerateKeyFile(fileLocation string) {
	log.Info("Generating KeyPair for irisnet-0.16.3-mainnet")

	privateKey := ed25519.GenPrivKey()
	publicKey := privateKey.PubKey()

	key := keyData{
		Chain:		"irisnet-0.16.3-mainnet",
		IdString: string(hex.EncodeToString(publicKey.Address())),
		PrivateKeyString: string(hex.EncodeToString(privateKey[:])),
		PublicKeyString: string(hex.EncodeToString(publicKey.Bytes())),
		PrivateKey: privateKey,
		PublicKey: publicKey.(ed25519.PubKeyEd25519),
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

func VerifyKeyFile(fileLocation string) {
	log.Error("Yet to implement this :P")
}