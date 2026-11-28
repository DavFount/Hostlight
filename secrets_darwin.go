package main

/*
#cgo LDFLAGS: -framework Security -framework Foundation
#include <Security/SecBase.h>
#include <stdlib.h>
OSStatus hostlightReadWebhookKey(const char *account, unsigned char *key, int size);
OSStatus hostlightAddWebhookKey(const char *account, const unsigned char *key, int size);
*/
import "C"

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"unsafe"
)

const (
	macWebhookSecretFormat = "Hostlight:macOS:1:"
	macWebhookKeyAccount   = "encryption-key-v1"
)

func protectSecret(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("Empty protected secret")
	}
	aead, err := macWebhookCipher(macWebhookKeyAccount, true)
	if err != nil {
		return nil, err
	}
	header := []byte(macWebhookSecretFormat)
	return aead.Seal(header, nil, data, []byte(macWebhookSecretFormat)), nil
}

func unprotectSecret(data []byte) ([]byte, error) {
	if !bytes.HasPrefix(data, []byte(macWebhookSecretFormat)) {
		return nil, fmt.Errorf("These protected webhook settings were not created on macOS")
	}
	aead, err := macWebhookCipher(macWebhookKeyAccount, false)
	if err != nil {
		return nil, err
	}
	plaintext, err := aead.Open(nil, nil, data[len(macWebhookSecretFormat):], []byte(macWebhookSecretFormat))
	if err != nil {
		return nil, fmt.Errorf("The macOS Keychain key could not decrypt these webhook settings")
	}
	return plaintext, nil
}

func macWebhookCipher(account string, create bool) (cipher.AEAD, error) {
	keyAccount := C.CString(account)
	defer C.free(unsafe.Pointer(keyAccount))
	var key [32]byte
	defer clear(key[:])
	keyPointer := (*C.uchar)(&key[0])
	keySize := C.int(len(key))
	status := C.hostlightReadWebhookKey(keyAccount, keyPointer, keySize)
	if status == C.errSecItemNotFound && create {
		if _, err := rand.Read(key[:]); err != nil {
			return nil, err
		}
		status = C.hostlightAddWebhookKey(keyAccount, keyPointer, keySize)
		if status == C.errSecDuplicateItem {
			// Another save created the key first. Always use the stored key.
			status = C.hostlightReadWebhookKey(keyAccount, keyPointer, keySize)
		}
	}
	if status == C.errSecItemNotFound {
		return nil, fmt.Errorf("The webhook encryption key is missing from this user's macOS Keychain")
	}
	if status != C.errSecSuccess {
		return nil, fmt.Errorf("Hostlight could not access this user's macOS Keychain (status %d)", status)
	}
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, err
	}
	return cipher.NewGCMWithRandomNonce(block)
}
