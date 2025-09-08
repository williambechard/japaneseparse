package analyze

import (
	"context"
	"fmt"
	"japaneseparse/ingest"
	"japaneseparse/model"
)

type LexEntry = model.LexEntry

// Analysis represents the result of analyzing a sentence plus lexicon entries.
type Analysis struct {
	SentenceID    string      `json:"sentence_id"`
	TokenCount    int         `json:"token_count"`
	Definitions   int         `json:"definitions_found"`
	GrammarIssues []string    `json:"grammar_issues,omitempty"`
	Structure     interface{} `json:"structure,omitempty"`
}

// SemanticRole represents semantic roles in a clause.
type SemanticRole string

const (
	AgentRole    SemanticRole = "agent"
	PatientRole  SemanticRole = "patient"
	LocationRole SemanticRole = "location"
	TimeRole     SemanticRole = "time"
)

// ClauseRole represents grammatical roles in a clause.
type ClauseRole struct {
	Subject         *[]int                 `json:"subject,omitempty"` // indices in entries
	Object          *[]int                 `json:"object,omitempty"`
	IndirectObj     *[]int                 `json:"indirect_object,omitempty"`
	Adverbial       *[]int                 `json:"adverbial,omitempty"`
	Verb            *int                   `json:"verb,omitempty"`
	Auxiliaries     []int                  `json:"auxiliaries,omitempty"`
	Tokens          []int                  `json:"tokens"`
	NamedEntities   map[string][]int       `json:"named_entities,omitempty"` // type -> indices
	VerbLinks       map[string]*int        `json:"verb_links,omitempty"`     // role -> index
	SemanticRoles   map[SemanticRole][]int `json:"semantic_roles,omitempty"`
	EmbeddedClauses []struct {
		Start int
		End   int
	} `json:"embedded_clauses,omitempty"`
}

type ClauseType string

const (
	MainClause        ClauseType = "main"
	SubordinateClause ClauseType = "subordinate"
	RelativeClause    ClauseType = "relative"
	QuotedClause      ClauseType = "quoted"
)

type Clause struct {
	Start      int        `json:"start"`
	End        int        `json:"end"`
	Roles      ClauseRole `json:"roles"`
	Type       ClauseType `json:"type"`
	Connective string     `json:"connective,omitempty"`
}

// Analyze performs grammar/structure analysis over the lexicon entries.
func Analyze(ctx context.Context, sentence ingest.Sentence, entries []LexEntry) (Analysis, error) {
	if ctx.Err() != nil {
		fmt.Println("[ANALYZE] Context error:", ctx.Err())
		// Log and continue instead of returning
	}

	found := 0
	for _, e := range entries {
		if len(e.Definitions) > 0 {
			found++
		}
	}

	// Clause boundary detection: split at "。" and "、"
	var clauses []Clause
	clauseStart := 0
	for i, e := range entries {
		if e.Token.Text == "。" || e.Token.Text == "、" {
			clause := Clause{Start: clauseStart, End: i, Roles: ClauseRole{Tokens: make([]int, i-clauseStart)}}
			for j := clauseStart; j < i; j++ {
				clause.Roles.Tokens[j-clauseStart] = j
			}
			// Discourse/connective analysis: look for conjunctions before clause boundary
			if i > 0 {
				prev := entries[i-1].Token.Text
				if prev == "が" || prev == "ので" || prev == "から" || prev == "けど" || prev == "そして" || prev == "と" {
					clause.Connective = prev
				}
			}
			clauses = append(clauses, clause)
			clauseStart = i + 1
		}
	}
	// Add final clause if needed
	if clauseStart < len(entries) {
		clause := Clause{Start: clauseStart, End: len(entries), Roles: ClauseRole{Tokens: make([]int, len(entries)-clauseStart)}}
		for j := clauseStart; j < len(entries); j++ {
			clause.Roles.Tokens[j-clauseStart] = j
		}
		clauses = append(clauses, clause)
	}

	// For each clause, assign grammatical roles
	// ...existing code for grammatical role assignment...

	return Analysis{
		SentenceID:    sentence.ID,
		TokenCount:    len(entries),
		Definitions:   found,
		GrammarIssues: []string{},
		Structure:     map[string]interface{}{"clauses": clauses},
	}, nil
}
