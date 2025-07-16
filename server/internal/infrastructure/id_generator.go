package infrastructure

import (
	"crypto/rand"
	"fmt"
	"math/big"
)

// Base58-like alphabet excluding easily confused characters (O, 0, l, I)
const (
	alphabet = "123456789ABCDEFGHJKLMNPQRSTUVWXYZ"
	idLength = 8
)

// DisplayExistenceChecker defines the interface for checking if a display ID exists.
type DisplayExistenceChecker interface {
	FindByID(id string) (any, bool)
}

// Base58IDGenerator implements the application.IDGenerator interface.
type Base58IDGenerator struct{}

// NewBase58IDGenerator creates a new Base58IDGenerator.
func NewBase58IDGenerator() *Base58IDGenerator {
	return &Base58IDGenerator{}
}

// GenerateRandomString generates a random string of a given length using the specified alphabet.
func GenerateRandomString(length int, alphabet string) (string, error) {
	bytes := make([]byte, length)
	for i := range length {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(alphabet))))
		if err != nil {
			return "", fmt.Errorf("failed to generate random number: %w", err)
		}
		bytes[i] = alphabet[num.Int64()]
	}
	return string(bytes), nil
}

// GenerateUniqueDisplayID generates a unique 8-character ID for a display.
// It retries until a unique ID is found using the provided checker.
func (g *Base58IDGenerator) GenerateUniqueDisplayID(checker DisplayExistenceChecker) (string, error) {
	for {
		id, err := GenerateRandomString(idLength, alphabet)
		if err != nil {
			return "", fmt.Errorf("failed to generate random ID string: %w", err)
		}

		// Check if the generated ID already exists
		if _, exists := checker.FindByID(id); !exists {
			return id, nil
		}
		// If it exists, loop and try again
	}
}
