package encrypt

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"syscall"

	"golang.org/x/crypto/openpgp"
	"golang.org/x/crypto/openpgp/armor"
	"golang.org/x/crypto/openpgp/packet"
	"golang.org/x/crypto/ssh/terminal"
)

// PGP holds the private key/pass and one message (may be encrypted/decrypted) at a time
type PGP struct {
	PrivateKey []byte
	Passphrase openpgp.PromptFunction
	Message    []byte
	Encrypted  bool
}

var entityList openpgp.EntityList

func NewPGP(k []byte, p openpgp.PromptFunction, m []byte, e bool) *PGP {

	r := new(PGP)
	r.PrivateKey = k
	r.Message = m
	r.Encrypted = e

	if p != nil {
		r.Passphrase = p
	} else {
		r.Passphrase = passPrompt(r)
	}

	return r
}

func shellPrompt() []byte {
	fmt.Print("Enter passphrase: ")
	passphraseByte, err := terminal.ReadPassword(int(syscall.Stdin))
	if err == nil {
		fmt.Println("")
	}

	return passphraseByte
}

func passPrompt(p *PGP) openpgp.PromptFunction {

	f := func(keys []openpgp.Key, symmetric bool) (pass []byte, err error) {
		for _, k := range keys {
			passphrase := shellPrompt()

			if err := k.PrivateKey.Decrypt(passphrase); err != nil {
				continue
			} else {
				return passphrase, nil
			}

		}
		return nil, err
	}

	return openpgp.PromptFunction(f)
}

// WriteFile writes the encrypted message to a new file, fails on existing files
func (f *PGP) WriteFile(repoPath string, filename string) error {
	if len(f.Message) == 0 {
		return fmt.Errorf("The message content has not been loaded")
	}

	if !f.Encrypted {
		return fmt.Errorf("Not allowed to write unencrypted content to a file")
	}

	p := path.Join(repoPath, filename)

	o, err := os.Open(p)
	if err == nil {
		o.Close()
		return fmt.Errorf("File already exists")
	}
	o.Close()

	d, err := os.Create(p)
	if err != nil {
		return fmt.Errorf("Unable to create the file: %s", err)
	}

	if err := d.Chmod(os.FileMode(0600)); err != nil {
		return fmt.Errorf("Unable to change permissions on file to 0600: %s", err)
	}

	if _, err := d.Write(f.Message); err != nil {
		return fmt.Errorf("Unable to write to file: %s", err)
	}

	return nil
}

//Keyring builds a pgp keyring based upon the users' private key
func (f *PGP) Keyring() error {
	passphraseByte := shellPrompt()

	s := bytes.NewReader([]byte(f.PrivateKey))
	block, err := armor.Decode(s)
	if err != nil {
		return fmt.Errorf("Not an armor encoded PGP private key: %s", err)
	} else if block.Type != openpgp.PrivateKeyType {
		return fmt.Errorf("Not a OpenPGP private key: %s", err)
	}

	entity, err := openpgp.ReadEntity(packet.NewReader(block.Body))
	if err != nil {
		return fmt.Errorf("Unable to read armor decoded key: %s", err)
	}

	if entity.PrivateKey != nil && entity.PrivateKey.Encrypted {
		err := entity.PrivateKey.Decrypt(passphraseByte)
		if err != nil {
			return fmt.Errorf("Failed to decrypt main private key: %s", err)
		}
	}

	for _, subkey := range entity.Subkeys {
		subkey.PrivateKey.Decrypt(passphraseByte)
	}

	entityList = append(entityList, entity)

	return nil
}

// Decrypt a message
func (f *PGP) Decrypt() error {
	if !f.Encrypted {
		return fmt.Errorf("The message is not encrypted")
	}

	block, err := armor.Decode(bytes.NewReader([]byte(f.Message)))
	if err != nil {
		return fmt.Errorf("Invalid PGP message or not armor encoded: %s", err)
	}
	if block.Type != "PGP MESSAGE" {
		return fmt.Errorf("This file is not a PGP message: %s", err)
	}

	md, err := openpgp.ReadMessage(block.Body, entityList, f.Passphrase, nil)
	if err != nil {
		return fmt.Errorf("Unable to decrypt the message: %s", err)
	}

	message, err := ioutil.ReadAll(md.UnverifiedBody)
	if err != nil {
		return fmt.Errorf("Unable to convert the decrypted message to a string: %s", err)
	}

	f.Encrypted = false
	f.Message = message

	return nil
}

// Encrypt a message
func (f *PGP) Encrypt() error {
	if f.Encrypted {
		return fmt.Errorf("The message is encrypted already")
	}

	var w bytes.Buffer

	b, err := armor.Encode(&w, "PGP MESSAGE", nil)
	if err != nil {
		return fmt.Errorf("Unable to armor encode")
	}

	e, err := openpgp.Encrypt(b, entityList, nil, nil, nil)
	if err != nil {
		return fmt.Errorf("Unable to load keyring for encryption: %s", err)
	}

	v, err := e.Write(f.Message)
	if err != nil {
		return fmt.Errorf("%s, ints buffered: %v", err, v)
	}

	if err := e.Close(); err != nil {
		return err
	}

	if err := b.Close(); err != nil {
		return err
	}

	message, err := ioutil.ReadAll(&w)
	if err != nil {
		return err
	}

	f.Encrypted = true
	f.Message = message

	return nil
}
