package analyze

import (
	"context"
	"fmt"
	"japaneseparse/ingest"
	"japaneseparse/logger"
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
	// Debugging: Log the entries being analyzed
	fmt.Printf("DEBUG: Entries for analysis: %+v\n", entries)

	// Debugging: Log the initial sentence structure
	fmt.Printf("DEBUG: Initial sentence: %+v\n", sentence)

	if ctx.Err() != nil {
		fmt.Println("[ANALYZE] Context error:", ctx.Err())
		// Log and continue instead of returning
	}

	found := 0
	for _, e := range entries {
		if len(e.Definitions) > 0 {
			// Exclude tokens like verbs from the definitions count
			if e.Token.Role == "subject" || e.Token.Role == "object" {
				found++
			}
		}
	}

	// Clause boundary detection: split at "。" and "、"
	var clauses []Clause
	clauseStart := 0
	for i, e := range entries {
		if e.Token.Text == "。" || e.Token.Text == "、" {
			clause := Clause{Start: clauseStart, End: i + 1, Roles: ClauseRole{Tokens: make([]int, i-clauseStart+1)}}
			for j := clauseStart; j <= i; j++ {
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
	// Enhance role assignment logic to better capture grammatical relationships
	for i := range clauses {
		clause := &clauses[i]

		// Assign subject, verb, and object roles based on token properties
		for j := clause.Start; j < clause.End; j++ {
			token := entries[j].Token

			// Debugging: Log token details and role field
			logger.Logf("DEBUG: Token %d details: %+v", j, token)
			logger.Logf("DEBUG: Token %d role: %s", j, token.Role)

			// Use the Role field to determine grammatical roles
			switch token.Role {
			case "subject":
				if clause.Roles.Subject == nil {
					clause.Roles.Subject = &[]int{}
				}
				*clause.Roles.Subject = append(*clause.Roles.Subject, j)
				logger.Logf("DEBUG: Assigned Subject role to token %d", j)
			case "object":
				if clause.Roles.Object == nil {
					clause.Roles.Object = &[]int{}
				}
				*clause.Roles.Object = append(*clause.Roles.Object, j)
				logger.Logf("DEBUG: Assigned Object role to token %d", j)
			case "verb":
				if clause.Roles.Verb == nil {
					clause.Roles.Verb = new(int)
					*clause.Roles.Verb = j
				}
				logger.Logf("DEBUG: Assigned Verb role to token %d", j)
			}
		}

		// Debugging: Log assigned roles for each clause
		fmt.Printf("DEBUG: Clause %d-%d, Roles: Subject=%v, Verb=%v, Object=%v\n",
			clause.Start, clause.End, clause.Roles.Subject, clause.Roles.Verb, clause.Roles.Object)

		// Set clause type based on connectives or other heuristics
		if clause.Connective != "" {
			clause.Type = SubordinateClause
		} else {
			clause.Type = MainClause
		}
	}

	return Analysis{
		SentenceID:    sentence.ID,
		TokenCount:    len(entries),
		Definitions:   found,
		GrammarIssues: []string{},
		Structure:     map[string]interface{}{"clauses": clauses},
	}, nil
}
