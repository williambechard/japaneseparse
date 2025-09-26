package ingest

import (
	"testing"
	"time"
)

func TestGenerateID(t *testing.T) {
	id1 := generateID()
	id2 := generateID()

	if id1 == id2 {
		t.Errorf("Expected unique IDs, but got duplicates: %s and %s", id1, id2)
	}

	if len(id1) == 0 || len(id2) == 0 {
		t.Errorf("Expected non-empty IDs, but got: '%s' and '%s'", id1, id2)
	}
}

func TestIngestSentence(t *testing.T) {
	// Test empty sentence
	_, err := IngestSentence("")
	if err == nil {
		t.Errorf("Expected error for empty sentence, but got nil")
	}

	// Test valid sentence
	sentenceText := "これはテストです。"
	s, err := IngestSentence(sentenceText)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if s.Text != sentenceText {
		t.Errorf("Expected text '%s', but got '%s'", sentenceText, s.Text)
	}

	if len(s.ID) == 0 {
		t.Errorf("Expected non-empty ID, but got '%s'", s.ID)
	}

	// Test channel publishing
	select {
	case published := <-IngestChan:
		if published.ID != s.ID || published.Text != s.Text {
			t.Errorf("Published sentence does not match: expected %+v, got %+v", s, published)
		}
	case <-time.After(1 * time.Second):
		t.Errorf("Timed out waiting for sentence to be published to IngestChan")
	}
}
