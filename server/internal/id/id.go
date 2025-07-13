package id

import (
	"crypto/rand"
	"log/slog"
	"math/big"
)

type Generator struct {
	used   map[string]struct{}
	char   []rune
	length int
	maxTry int
}

func NewGenerator(charset string) *Generator {
	return &Generator{
		used:   make(map[string]struct{}),
		char:   []rune(charset),
		length: 8, // Default length
		maxTry: 1000,
	}
}

var DefaultGenerator = NewGenerator("0123456789ABCDEFGHJKMNPQRSTUVWXYZ")

func (g *Generator) randomID() (string, error) {
	id := make([]rune, g.length)
	for i := range id {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(g.char))))
		if err != nil {
			return "", err
		}
		id[i] = g.char[n.Int64()]
	}
	return string(id), nil
}

func (g *Generator) Generate() string {
	for range g.maxTry {
		id, err := g.randomID()
		if err != nil {
			slog.Warn("failed to generate random ID", "error", err)
			continue
		}
		if _, exists := g.used[id]; !exists {
			g.used[id] = struct{}{}
			return id
		}
	}
	slog.Error("failed to generate unique ID after max attempts", "maxTry", g.maxTry)
	return "" // Failed to generate a unique ID after maxTry attempts
}

func (g *Generator) Exist(id string) bool {
	_, exists := g.used[id]
	return exists
}

func Generate() string {
	return DefaultGenerator.Generate()
}

func Exist(id string) bool {
	return DefaultGenerator.Exist(id)
}
