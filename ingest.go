package main

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Sentence represents an ingested Japanese sentence and metadata.
type Sentence struct {
	ID        string    `json:"id"`
	Text      string    `json:"text"`
	CreatedAt time.Time `json:"created_at"`
}

// IngestChan is a channel where ingested sentences are published for downstream processing.
// Other packages or goroutines can receive from this channel to process sentences.
var IngestChan chan Sentence

func init() {
	// buffered channel to decouple producer and consumers
	IngestChan = make(chan Sentence, 100)
}

// generateID creates a short random hex id. Falls back to a timestamp string on error.
func generateID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

// IngestSentence is the ingest entrypoint. It trims the input, validates it, constructs
// a Sentence object and publishes it to IngestChan asynchronously. It returns the
// created Sentence or an error if the input was invalid.
func IngestSentence(text string) (Sentence, error) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return Sentence{}, errors.New("empty sentence")
	}

	s := Sentence{
		ID:        generateID(),
		Text:      trimmed,
		CreatedAt: time.Now().UTC(),
	}

	// publish asynchronously so callers are not blocked
	go func(sent Sentence) {
		select {
		case IngestChan <- sent:
			// published successfully
		default:
			// channel is full; drop silently for now (could log or expand buffer)
		}
	}(s)

	return s, nil
}
