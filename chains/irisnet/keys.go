package irisnet

import (
	"os"
	"io/ioutil"
	"encoding/json"
	"encoding/hex"

	log "github.com/sirupsen/logrus"
	"github.com/tendermint/tendermint/crypto/ed25519"
)

var ServicedKeyFile string = "irisnet"
var isKeyFileUsed, memoized bool
var keyFileLocation string
var privateKey ed25519.PrivKeyEd25519

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

	// TODO Check these conditions
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
			valid, err:= VerifyKeyFile(keyFileLocation)
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