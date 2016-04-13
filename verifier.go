// Package security is responsible for verifying the integrity of received SQS messages
// before agent processes them. Currently, Neptune.io signs all the messages with a private key
// and Agent verifies the message signature with public key before processing the message.
package agent

import (
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/neptuneio/agent/logging"
)

const (
	certificateFileName = "neptuneio.crt"
)

// Global variable to hold Neptune.io's public key.
var publicKey *rsa.PublicKey

func init() {
	// Get the full path of the binary and pick the certificate file from the same directory.
	dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err == nil {
		certFilePath := filepath.Join(dir, certificateFileName)
		key, e := loadPublicKey(certFilePath)
		if e == nil {
			publicKey = key
		}
	}

	if publicKey == nil {
		fmt.Println("Could not load public key.")
		os.Exit(1)
	}
}

// Function to load Neptune.io's public key while booting up the agent. This public key will be used
// in message signature verification.
func loadPublicKey(path string) (*rsa.PublicKey, error) {
	data, err := ioutil.ReadFile(path)

	if err != nil {
		return nil, err
	}

	block, _ := pem.Decode(data)
	if block == nil {
		return nil, errors.New("No key is found.")
	}

	switch block.Type {
	case "CERTIFICATE":
		certificate, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, err
		} else {
			finalKey, ok := certificate.PublicKey.(*rsa.PublicKey)
			if ok {
				return finalKey, nil
			}
			return nil, errors.New("Could not get RSA public key from the cert file.")
		}
	default:
		return nil, fmt.Errorf("Unsupported key type %q", block.Type)
	}
}

// Function to verify the signature of given message and check if the received signature is same as computed one.
func VerifyMessage(message, signature string) (bool, error) {
	if publicKey == nil {
		logging.Error("Public key is null so cannot verify the message.", nil)
		return false, errors.New("Null public key")
	}

	sigData, err := base64.StdEncoding.DecodeString(signature)
	if err != nil {
		logging.Error("Could not decode the signature into binary.", logging.Fields{"error": err})
		return false, nil
	} else {
		hash := sha256.New()
		hash.Write([]byte(message))
		d := hash.Sum(nil)
		err := rsa.VerifyPKCS1v15(publicKey, crypto.SHA256, d, sigData)
		if err != nil {
			logging.Error("Could not verify the message.", logging.Fields{"error": err})
			return false, nil
		} else {
			return true, nil
		}
	}
}
