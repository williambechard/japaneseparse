package main

import (
	"context"
	"strings"
)

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

// ClauseType represents the type of a clause.
type ClauseType string

const (
	MainClause        ClauseType = "main"
	SubordinateClause ClauseType = "subordinate"
	RelativeClause    ClauseType = "relative"
	QuotedClause      ClauseType = "quoted"
)

// Clause represents a segmented clause with role labels.
type Clause struct {
	Start      int        `json:"start"`
	End        int        `json:"end"`
	Roles      ClauseRole `json:"roles"`
	Type       ClauseType `json:"type"`
	Connective string     `json:"connective,omitempty"`
}

// Analyze performs grammar/structure analysis over the lexicon entries.
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
	for idx := range clauses {
		c := &clauses[idx]
		var subject, object, indirectObj, adverbial []int
		namedEntities := map[string][]int{}
		verbLinks := map[string]*int{}
		semanticRoles := map[SemanticRole][]int{}
		var lastNounPhrase []int
		var lastAdjPhrase []int
		var embeddedClauses []struct {
			Start int
			End   int
		}
		inEmbedded := false
		embeddedStart := 0
		for i := 0; i < len(c.Roles.Tokens); i++ {
			j := c.Roles.Tokens[i]
			entry := entries[j].Token
			// Expanded noun phrase detection
			if entry.POS != "" && (strings.HasPrefix(entry.POS, "名詞") || strings.HasPrefix(entry.POS, "数") || strings.HasPrefix(entry.POS, "接頭詞") || strings.HasPrefix(entry.POS, "形容詞")) {
				lastNounPhrase = append(lastNounPhrase, j)
				if strings.HasPrefix(entry.POS, "形容詞") {
					lastAdjPhrase = append(lastAdjPhrase, j)
				}
				// Named entity detection
				if strings.Contains(entry.POS, "固有名詞") {
					namedEntities["organization"] = append(namedEntities["organization"], j)
				}
				if strings.Contains(entry.POS, "地域") {
					namedEntities["location"] = append(namedEntities["location"], j)
					semanticRoles[LocationRole] = append(semanticRoles[LocationRole], j)
				}
				if strings.Contains(entry.POS, "数") {
					namedEntities["number"] = append(namedEntities["number"], j)
					semanticRoles[TimeRole] = append(semanticRoles[TimeRole], j)
				}
			} else {
				// Particle after noun phrase
				if entry.Text == "は" || entry.Text == "が" {
					if len(lastNounPhrase) > 0 {
						subject = append(subject, lastNounPhrase...)
						verbLinks["subject"] = &lastNounPhrase[0]
						semanticRoles[AgentRole] = append(semanticRoles[AgentRole], lastNounPhrase...)
						lastNounPhrase = nil
						lastAdjPhrase = nil
					}
				}
				if entry.Text == "を" {
					if len(lastNounPhrase) > 0 {
						object = append(object, lastNounPhrase...)
						verbLinks["object"] = &lastNounPhrase[0]
						semanticRoles[PatientRole] = append(semanticRoles[PatientRole], lastNounPhrase...)
						lastNounPhrase = nil
						lastAdjPhrase = nil
					}
				}
				if entry.Text == "に" {
					if len(lastNounPhrase) > 0 {
						indirectObj = append(indirectObj, lastNounPhrase...)
						verbLinks["indirect_object"] = &lastNounPhrase[0]
						lastNounPhrase = nil
						lastAdjPhrase = nil
					}
				}
				if entry.Text == "で" || entry.Text == "と" {
					if len(lastNounPhrase) > 0 {
						adverbial = append(adverbial, lastNounPhrase...)
						verbLinks["adverbial"] = &lastNounPhrase[0]
						lastNounPhrase = nil
						lastAdjPhrase = nil
					}
				}
				// Embedded clause detection (simple: quotes or relative clause marker)
				if entry.Text == "「" {
					inEmbedded = true
					embeddedStart = j
				}
				if entry.Text == "」" && inEmbedded {
					inEmbedded = false
					embeddedClauses = append(embeddedClauses, struct {
						Start int
						End   int
					}{embeddedStart, j})
				}
				if entry.Text == "の" && i+1 < len(c.Roles.Tokens) {
					next := entries[c.Roles.Tokens[i+1]].Token
					if next.POS != "" && strings.HasPrefix(next.POS, "名詞") {
						// treat as relative clause boundary
						embeddedClauses = append(embeddedClauses, struct {
							Start int
							End   int
						}{j - 1, j + 1})
					}
				}
				// Reset noun/adj phrase if not a noun/adj/particle
				if entry.POS != "" && !strings.HasPrefix(entry.POS, "名詞") && !strings.HasPrefix(entry.POS, "数") && !strings.HasPrefix(entry.POS, "接頭詞") && !strings.HasPrefix(entry.POS, "形容詞") {
					lastNounPhrase = nil
					lastAdjPhrase = nil
				}
			}
			// Verb chain handling
			if entry.POS != "" && strings.HasPrefix(entry.POS, "動詞") {
				c.Roles.Verb = &j
				// If previous token is also verb, treat as verb chain
				if i > 0 {
					prev := entries[c.Roles.Tokens[i-1]].Token
					if prev.POS != "" && strings.HasPrefix(prev.POS, "動詞") {
						// label as verb chain
						c.Roles.Auxiliaries = append(c.Roles.Auxiliaries, c.Roles.Tokens[i-1])
					}
				}
			}
			if entry.POS != "" && strings.HasPrefix(entry.POS, "助動詞") {
				c.Roles.Auxiliaries = append(c.Roles.Auxiliaries, j)
			}
		}
		if len(subject) > 0 {
			c.Roles.Subject = &subject
		}
		if len(object) > 0 {
			c.Roles.Object = &object
		}
		if len(indirectObj) > 0 {
			c.Roles.IndirectObj = &indirectObj
		}
		if len(adverbial) > 0 {
			c.Roles.Adverbial = &adverbial
		}
		if len(namedEntities) > 0 {
			c.Roles.NamedEntities = namedEntities
		}
		if len(verbLinks) > 0 {
			c.Roles.VerbLinks = verbLinks
		}
		if len(semanticRoles) > 0 {
			c.Roles.SemanticRoles = semanticRoles
		}
		if len(embeddedClauses) > 0 {
			c.Roles.EmbeddedClauses = embeddedClauses
		}
		// Clause type labeling
		c.Type = MainClause
		for _, j := range c.Roles.Tokens {
			entry := entries[j].Token
			// Quoted clause
			if entry.Text == "「" {
				c.Type = QuotedClause
			}
			// Relative clause (simple: "の" followed by noun)
			if entry.Text == "の" && j+1 < c.End {
				next := entries[j+1].Token
				if next.POS != "" && strings.HasPrefix(next.POS, "名詞") {
					c.Type = RelativeClause
				}
			}
			// Subordinate clause (conjunctions)
			if entry.Text == "ので" || entry.Text == "から" || entry.Text == "けど" || entry.Text == "が" {
				c.Type = SubordinateClause
			}
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
