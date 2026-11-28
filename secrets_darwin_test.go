package main

import (
	"bytes"
	"os"
	"os/exec"
	"testing"

	"github.com/google/uuid"
)

func TestMacWebhookKeychain(t *testing.T) {
	if os.Getenv("HOSTLIGHT_KEYCHAIN_SMOKE") != "1" {
		t.Skip("Requires an unlocked macOS login Keychain. Set HOSTLIGHT_KEYCHAIN_SMOKE=1 locally.")
	}

	account := "hostlight-test-" + uuid.NewString()
	writer, err := macWebhookCipher(account, true)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		out, err := exec.Command("/usr/bin/security", "delete-generic-password",
			"-s", "com.wails.hostlight.webhooks", "-a", account).CombinedOutput()
		if err != nil {
			t.Errorf("remove temporary Keychain item: %v (%s)", err, out)
		}
	})

	secret := []byte("fixture webhook credential")
	header := []byte(macWebhookSecretFormat)
	encrypted := writer.Seal(nil, nil, secret, header)
	if bytes.Contains(encrypted, secret) {
		t.Fatal("encrypted webhook contains the plaintext credential")
	}
	reader, err := macWebhookCipher(account, false)
	if err != nil {
		t.Fatal(err)
	}
	plain, err := reader.Open(nil, nil, encrypted, header)
	if err != nil || !bytes.Equal(plain, secret) {
		t.Fatalf("open credential with retrieved Keychain key: %v", err)
	}
	encrypted[len(encrypted)-1] ^= 1
	if _, err := reader.Open(nil, nil, encrypted, header); err == nil {
		t.Fatal("accepted tampered webhook credentials")
	}
}
