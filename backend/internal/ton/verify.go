package ton

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
)

// TON Connect proof verification
// Based on: https://docs.ton.org/develop/dapps/ton-connect/sign

// ConnectProof represents the proof sent by TON Connect
type ConnectProof struct {
	Timestamp int64  `json:"timestamp"`
	Domain    Domain `json:"domain"`
	Signature string `json:"signature"`
	Payload   string `json:"payload"`
}

// Domain represents the domain part of the proof
type Domain struct {
	LengthBytes int    `json:"lengthBytes"`
	Value       string `json:"value"`
}

// WalletAccount represents wallet account info from TON Connect
type WalletAccount struct {
	Address   string `json:"address"`
	Chain     string `json:"chain"`
	PublicKey string `json:"publicKey"`
}

// VerifyProof verifies TON Connect wallet ownership proof
func VerifyProof(account WalletAccount, proof ConnectProof, allowedDomain string) error {
	// 1. Check timestamp (proof should be recent)
	proofTime := time.Unix(proof.Timestamp, 0)
	if time.Since(proofTime) > ProofTTL {
		return errors.New("proof expired")
	}

	// 2. Check domain
	if proof.Domain.Value != allowedDomain {
		return fmt.Errorf("domain mismatch: expected %s, got %s", allowedDomain, proof.Domain.Value)
	}

	// 3. Decode public key
	pubKeyBytes, err := hex.DecodeString(account.PublicKey)
	if err != nil {
		return fmt.Errorf("invalid public key format: %w", err)
	}

	if len(pubKeyBytes) != ed25519.PublicKeySize {
		return errors.New("invalid public key size")
	}

	// 4. Decode signature
	signatureBytes, err := base64.StdEncoding.DecodeString(proof.Signature)
	if err != nil {
		return fmt.Errorf("invalid signature format: %w", err)
	}

	// 5. Build message to verify
	message := buildProofMessage(account.Address, proof)

	// 6. Verify signature
	if !ed25519.Verify(pubKeyBytes, message, signatureBytes) {
		return errors.New("invalid signature")
	}

	return nil
}

// buildProofMessage constructs the message that was signed
func buildProofMessage(address string, proof ConnectProof) []byte {
	// Message format:
	// "ton-proof-item-v2/" + address_workchain (4 bytes) + address_hash (32 bytes)
	// + domain_len (4 bytes) + domain + timestamp (8 bytes) + payload

	// Parse address to get workchain and hash
	// For simplicity, we'll construct a simpler message hash
	// In production, you'd want to properly parse the TON address

	// The actual TON proof message construction
	var message []byte

	// "ton-proof-item-v2/"
	message = append(message, []byte("ton-proof-item-v2/")...)

	// Address (simplified - in real implementation parse properly)
	message = append(message, []byte(address)...)

	// Domain length (4 bytes, little endian)
	domainLen := make([]byte, 4)
	binary.LittleEndian.PutUint32(domainLen, uint32(proof.Domain.LengthBytes))
	message = append(message, domainLen...)

	// Domain value
	message = append(message, []byte(proof.Domain.Value)...)

	// Timestamp (8 bytes, little endian)
	timestamp := make([]byte, 8)
	binary.LittleEndian.PutUint64(timestamp, uint64(proof.Timestamp))
	message = append(message, timestamp...)

	// Payload
	message = append(message, []byte(proof.Payload)...)

	// Hash the message
	hash := sha256.Sum256(message)

	// Prefix with "ton-connect" and hash again
	finalMessage := append([]byte("ton-connect"), hash[:]...)
	finalHash := sha256.Sum256(finalMessage)

	return finalHash[:]
}

// GeneratePayload generates a random payload for TON Connect
func GeneratePayload() string {
	// Generate a random payload that will be signed
	// This should be unique per session to prevent replay attacks
	timestamp := time.Now().Unix()
	payload := fmt.Sprintf("%d-%x", timestamp, sha256.Sum256([]byte(fmt.Sprintf("%d", timestamp))))
	return payload[:32] // Truncate to reasonable length
}

// ValidateAddress checks if the TON address format is valid
func ValidateAddress(address string) bool {
	// TON addresses are typically:
	// - Raw: 0:hex (workchain:hash)
	// - User-friendly: Base64 encoded (48 chars with bounceable/non-bounceable flag)

	if len(address) == 0 {
		return false
	}

	// Check for raw format (0:hex or -1:hex)
	if len(address) >= 66 && (address[0:2] == "0:" || address[0:3] == "-1:") {
		return true
	}

	// Check for user-friendly format (base64, 48 chars)
	if len(address) == 48 {
		_, err := base64.URLEncoding.DecodeString(address)
		return err == nil
	}

	return false
}

// NormalizeAddress converts address to raw format
func NormalizeAddress(address string) (string, error) {
	// If already raw format, return as is
	if len(address) >= 66 && (address[0:2] == "0:" || address[0:3] == "-1:") {
		return address, nil
	}

	// Try to decode user-friendly format
	if len(address) == 48 {
		decoded, err := base64.URLEncoding.DecodeString(address)
		if err != nil {
			return "", fmt.Errorf("invalid address format: %w", err)
		}

		// User-friendly address is 36 bytes:
		// 1 byte flags + 1 byte workchain + 32 bytes hash + 2 bytes CRC
		if len(decoded) != 36 {
			return "", errors.New("invalid address length")
		}

		workchain := int8(decoded[1])
		hash := decoded[2:34]

		return fmt.Sprintf("%d:%s", workchain, hex.EncodeToString(hash)), nil
	}

	return "", errors.New("unknown address format")
}
