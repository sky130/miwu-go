package miot

import (
	"strings"
	"testing"

	"miwu-go/internal/miot/crypto"
)

const (
	data              = "1234567890abcdef"
	nonce             = "wugs9fcXuyVpR1bg"
	requestURL        = "https://api.io.mi.com/app/home/profile"
	expectSignedNonce = "R1DO8/6YPRXDcNQLb18ur9AOxsvTJ6JpZX12sdcsNfc="
	expectSignature   = "y0aw0lUxOfnGL0OpFQVRX1M4MnXhYG/41xzjMsoJblc="
	expectRequestPath = "/home/profile"
)

func TestSign(t *testing.T) {
	path := strings.ReplaceAll(requestURL, MiotBaseURL, "/")

	signedNonce, err := crypto.GenerateSignedNonce(data, nonce)
	if err != nil {
		t.Fatalf("sign nonce failure: %v\n", err)
	}
	if signedNonce != expectSignedNonce {
		t.Fatalf("signed nonce expect %s, but got %s", expectSignedNonce, signedNonce)
	}

	if path != expectRequestPath {
		t.Fatalf("request path expect %s, but got %s", expectRequestPath, path)
	}

	signature, err := crypto.GenerateSignature(path, signedNonce, nonce, data)
	if err != nil {
		t.Fatalf("generate signature failure: %v\n", err)
	}
	if signature != expectSignature {
		t.Errorf("signature expect %s, but got %s", expectSignature, signature)
	}
}
