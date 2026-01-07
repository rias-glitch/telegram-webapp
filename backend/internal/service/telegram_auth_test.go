package service

import (
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
)

// buildInitData builds a valid init_data string for tests using the same
// algorithm as ValidateTelegramInitData.
func buildInitData(t *testing.T, botToken string, fields map[string]string) string {
    t.Helper()
    var parts []string
    for k, v := range fields {
        parts = append(parts, k+"="+v)
    }
    sort.Strings(parts)
    dataString := strings.Join(parts, "\n")

    secret := sha256.Sum256([]byte(botToken))
    h := hmacNew(secret[:], []byte(dataString))
    hash := hex.EncodeToString(h)

    // assemble query: include original fields and hash
    vals := url.Values{}
    for k, v := range fields {
        vals.Add(k, v)
    }
    vals.Add("hash", hash)
    return vals.Encode()
}

// hmacNew is a small helper duplicating the HMAC-SHA256 used in production code.
func hmacNew(key, data []byte) []byte {
    h := sha256.New()
    // simple HMAC implementation for test (do not replace production code)
    // HMAC(K, m) = H((K^opad) || H((K^ipad)||m))
    blockSize := 64
    if len(key) > blockSize {
        tmp := sha256.Sum256(key)
        key = tmp[:]
    }
    if len(key) < blockSize {
        pad := make([]byte, blockSize-len(key))
        key = append(key, pad...)
    }
    ipad := make([]byte, blockSize)
    opad := make([]byte, blockSize)
    for i := 0; i < blockSize; i++ {
        ipad[i] = key[i] ^ 0x36
        opad[i] = key[i] ^ 0x5c
    }
    h.Reset()
    h.Write(ipad)
    h.Write(data)
    inner := h.Sum(nil)

    h2 := sha256.New()
    h2.Write(opad)
    h2.Write(inner)
    return h2.Sum(nil)
}

func TestValidateTelegramInitData_Valid(t *testing.T) {
    botToken := "test-bot-token"
    fields := map[string]string{
        "auth_date": strconv.FormatInt(time.Now().Unix(), 10),
        "user":      `{"id":1,"username":"u","first_name":"F"}`,
    }

    initData := buildInitData(t, botToken, fields)

    vals, ok := ValidateTelegramInitData(initData, botToken)
    if !ok {
        t.Fatalf("expected valid init data")
    }
    if vals.Get("user") == "" {
        t.Fatalf("expected user field in values")
    }
}

func TestValidateTelegramInitData_Tampered(t *testing.T) {
    botToken := "test-bot-token"
    fields := map[string]string{
        "auth_date": strconv.FormatInt(time.Now().Unix(), 10),
        "user":      `{"id":1,"username":"u","first_name":"F"}`,
    }
    initData := buildInitData(t, botToken, fields)

    // tamper with data by appending an extra field (will break the hash)
    tampered := initData + "&x=1"

    _, ok := ValidateTelegramInitData(tampered, botToken)
    if ok {
        t.Fatalf("expected tampered init data to be invalid")
    }
}
