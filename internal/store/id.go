package store

import (
	"crypto/sha256"
	"fmt"
	"math"
	"math/big"
	"strings"
	"time"
)

const base36Alphabet = "0123456789abcdefghijklmnopqrstuvwxyz"

// encodeBase36 converts a byte slice to a base36 string of specified length.
func encodeBase36(data []byte, length int) string {
	num := new(big.Int).SetBytes(data)

	base := big.NewInt(36)
	zero := big.NewInt(0)
	mod := new(big.Int)

	chars := make([]byte, 0, length)
	for num.Cmp(zero) > 0 {
		num.DivMod(num, base, mod)
		chars = append(chars, base36Alphabet[mod.Int64()])
	}

	var result strings.Builder
	for i := len(chars) - 1; i >= 0; i-- {
		result.WriteByte(chars[i])
	}

	str := result.String()
	if len(str) < length {
		str = strings.Repeat("0", length-len(str)) + str
	}
	if len(str) > length {
		str = str[len(str)-length:]
	}
	return str
}

// generateHashID creates a hash-based ID with prefix-suffix format.
func generateHashID(prefix, title, description, creator string, timestamp time.Time, length, nonce int) string {
	content := fmt.Sprintf("%s|%s|%s|%d|%d", title, description, creator, timestamp.UnixNano(), nonce)
	hash := sha256.Sum256([]byte(content))

	var numBytes int
	switch length {
	case 3:
		numBytes = 2
	case 4:
		numBytes = 3
	case 5, 6:
		numBytes = 4
	case 7, 8:
		numBytes = 5
	default:
		numBytes = 3
	}

	shortHash := encodeBase36(hash[:numBytes], length)
	return fmt.Sprintf("%s-%s", prefix, shortHash)
}

// collisionProbability calculates P(collision) using birthday paradox approximation.
func collisionProbability(numIssues int, idLength int) float64 {
	const base = 36.0
	totalPossibilities := math.Pow(base, float64(idLength))
	exponent := -float64(numIssues*numIssues) / (2.0 * totalPossibilities)
	return 1.0 - math.Exp(exponent)
}

// computeAdaptiveLength determines the optimal ID length for the current issue count.
func computeAdaptiveLength(numIssues int) int {
	const (
		minLength            = 3
		maxLength            = 8
		maxCollisionProb     = 0.25
	)
	for length := minLength; length <= maxLength; length++ {
		if collisionProbability(numIssues, length) <= maxCollisionProb {
			return length
		}
	}
	return maxLength
}

// GenerateID creates a unique hash-based ID, checking for collisions against existing IDs.
func GenerateID(prefix, title, description string, existingIDs map[string]bool) string {
	now := time.Now()
	actor := "bd-lite"
	baseLength := computeAdaptiveLength(len(existingIDs))

	for length := baseLength; length <= 8; length++ {
		for nonce := 0; nonce < 10; nonce++ {
			candidate := generateHashID(prefix, title, description, actor, now, length, nonce)
			if !existingIDs[candidate] {
				return candidate
			}
		}
	}

	// Extremely unlikely fallback: timestamp-based
	return fmt.Sprintf("%s-%s", prefix, encodeBase36([]byte(fmt.Sprintf("%d", now.UnixNano())), 8))
}
