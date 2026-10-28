package crypto

import (
	"crypto/hmac"
	"crypto/md5"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strings"
)

const (
	longTempChars   = "1234567890abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	randomTempChars = "0123456789ABCDEF"
)

func GenerateSignedNonce(secret, nonce string) (string, error) {
	secretBytes, err := base64.StdEncoding.DecodeString(secret)
	if err != nil {
		return "", err
	}
	nonceBytes, err := base64.StdEncoding.DecodeString(nonce)
	if err != nil {
		return "", err
	}
	h := sha256.New()
	h.Write(secretBytes)
	h.Write(nonceBytes)
	return base64.StdEncoding.EncodeToString(h.Sum(nil)), nil
}

func GenerateSignature(uri, signedNonce, nonce, data string) (string, error) {
	key, err := base64.StdEncoding.DecodeString(signedNonce)
	if err != nil {
		return "", err
	}
	mac := hmac.New(sha256.New, key)
	sign := fmt.Sprintf("%s&%s&%s&data=%s", uri, signedNonce, nonce, data)
	mac.Write([]byte(sign))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil)), nil
}

func GetNonce() (string, error) {
	const n = 16
	const chars = longTempChars
	const limit = 256 - (256 % 62)
	out := make([]byte, 0, n)
	buf := make([]byte, n*2)
	for len(out) < n {
		if _, err := rand.Read(buf); err != nil {
			return "", err
		}
		for _, b := range buf {
			if int(b) < limit {
				out = append(out, chars[int(b)%62])
				if len(out) == n {
					break
				}
			}
		}
	}
	return string(out), nil
}

func GetRandomDeviceID() (string, error) {
	const n = 16
	const chars = randomTempChars
	const L = len(chars)
	const limit = 256 - (256 % L)
	out := make([]byte, 0, n)
	buf := make([]byte, n*2)
	for len(out) < n {
		if _, err := rand.Read(buf); err != nil {
			return "", err
		}
		for _, b := range buf {
			if int(b) < limit {
				out = append(out, chars[int(b)%L])
				if len(out) == n {
					return string(out), nil
				}
			}
		}
	}
	// unreachable, but keep the compiler happy
	return string(out), nil
}

func MD5(s string) string {
	h := fmt.Sprintf("%X", md5.Sum([]byte(s)))
	return h + strings.Repeat("0", 32-len(h))
}
