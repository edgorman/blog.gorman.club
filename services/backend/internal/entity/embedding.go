package entity

import (
	"crypto/sha256"
	"encoding/hex"
	"time"
)

// Embedding is a post's position in meaning: a vector a text-embedding model gave its title and
// body. It is stored beside the post rather than on it (see the worker), and is only ever used to
// rank - who may read a post is still the post's own rule.
type Embedding struct {
	Slug    string
	OwnerID string
	Vector  []float32
	// ContentHash is the sha256 of the text that was embedded (EmbeddingText), so a write that did
	// not change it - visibility, tags - is recognised without calling the model again.
	ContentHash string
	// Model is the model id that produced Vector. Vectors from different models are not comparable,
	// so a changed model re-embeds a post even when its text is unchanged.
	Model     string
	CreatedAt time.Time
}

// EmbeddingText is what a post is embedded as: its title and body, which is all of what it says.
func EmbeddingText(blog Blog) string {
	return blog.Title + "\n\n" + blog.Content
}

// ContentHash is the hex sha256 of text.
func ContentHash(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}
