package parser

import (
	"japaneseparse/analyze"
	"japaneseparse/pkg/types"
)

type ParseResult struct {
	Text             string           `json:"text"`              // Original input
	SentenceID       string           `json:"sentence_id"`       // Unique identifier
	TokenCount       int              `json:"token_count"`       // Number of tokens
	DefinitionsFound int              `json:"definitions_found"` // Tokens with definitions
	Tokens           []types.Token    `json:"tokens"`            // Detailed analysis
	Clauses          []analyze.Clause `json:"clauses"`           // Sentence structure
	ProcessedAt      string           `json:"processed_at"`      // Processing timestamp
}
