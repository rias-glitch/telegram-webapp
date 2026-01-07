package service

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

// ValidateTelegramInitData verifies Telegram WebApp init_data HMAC and checks
// that the auth_date is recent (within 1 hour) to mitigate replay attacks.
func ValidateTelegramInitData(initData, botToken string) (url.Values, bool) {
	values, err := url.ParseQuery(initData)
	if err != nil {
		return nil, false
	}

	hash := values.Get("hash")
	if hash == "" {
		return nil, false
	}
	values.Del("hash")

	var dataCheck []string
	for k, v := range values {
		dataCheck = append(dataCheck, k+"="+strings.Join(v, ""))
	}

	sort.Strings(dataCheck)
	dataString := strings.Join(dataCheck, "\n")

	secret := sha256.Sum256([]byte(botToken))
	h := hmac.New(sha256.New, secret[:])
	h.Write([]byte(dataString))

	calculated := h.Sum(nil)
	provided, err := hex.DecodeString(hash)
	if err != nil {
		return nil, false
	}

	if !hmac.Equal(calculated, provided) {
		return nil, false
	}

	// Freshness check: require auth_date within the last hour
	authDateStr := values.Get("auth_date")
	if authDateStr == "" {
		return nil, false
	}
	authDate, err := strconv.ParseInt(authDateStr, 10, 64)
	if err != nil {
		return nil, false
	}

	now := time.Now().Unix()
	// allow small clock skew, but reject anything older than 1 hour
	if now-authDate > 3600 || authDate-now > 300 {
		return nil, false
	}

	return values, true
}
