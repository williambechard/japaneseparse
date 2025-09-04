package main

import (
	"context"
)

// Analysis represents the result of analyzing a sentence plus lexicon entries.
type Analysis struct {
	SentenceID    string      `json:"sentence_id"`
	TokenCount     int         `json:"token_count"`
	Definitions    int         `json:"definitions_found"`
	GrammarIssues  []string    `json:"grammar_issues,omitempty"`
	Structure      interface{} `json:"structure,omitempty"`
}

// Analyze performs a simple analysis over the lexicon entries. Replace with
// richer grammar checking and structure inference later.
func Analyze(ctx context.Context, sentence Sentence, entries []LexEntry) (Analysis, error) {
	select {
	case <-ctx.Done():
		return Analysis{}, ctx.Err()
	default:
	}

	found := 0
	for _, e := range entries {
		if len(e.Definitions) > 0 {
			found++
		}
	}

	return Analysis{
		SentenceID:   sentence.ID,
		TokenCount:    len(entries),
		Definitions:   found,
		GrammarIssues: []string{},
	}, nil
}
