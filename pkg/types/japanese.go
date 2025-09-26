package types

import "time"

// SentenceAnalysis represents the complete analysis output for a Japanese sentence
type SentenceAnalysis struct {
	SentenceID       string          `json:"sentence_id"`
	TokenCount       int             `json:"token_count"`
	Tokens           []Token         `json:"tokens"`
	Analysis         GrammarAnalysis `json:"analysis"`
	ProcessedAt      time.Time       `json:"processed_at,omitempty"`
	DefinitionsFound int             `json:"definitions_found,omitempty"`
}

// Token represents a morphological token/morpheme with enriched information
type Token struct {
	Text           string `json:"text"`
	Lemma          string `json:"lemma,omitempty"`
	POS            string `json:"pos,omitempty"`
	Start          int    `json:"start"`
	End            int    `json:"end"`
	Reading        string `json:"reading,omitempty"`
	Pronunciation  string `json:"pronunciation,omitempty"`
	TokenID        int    `json:"token_id,omitempty"`
	InflectionType string `json:"inflection_type,omitempty"`
	InflectionForm string `json:"inflection_form,omitempty"`

	// Enhanced features
	DictionaryEntry DictionaryEntry `json:"dictionary_entry,omitempty"`
	FuriganaText    string          `json:"furigana_text,omitempty"`
	FuriganaLemma   string          `json:"furigana_lemma,omitempty"`

	// Conjugation and auxiliary information
	Conjugation      []string `json:"conjugation,omitempty"`
	Auxiliaries      []Token  `json:"auxiliaries,omitempty"`
	MergedIndices    []int    `json:"merged_indices,omitempty"`
	ConjugationLabel string   `json:"conjugation_label,omitempty"`

	// Added fields for grammatical roles and relationships
	Role     string `json:"role,omitempty"`     // Grammatical role (e.g., subject, object, predicate)
	Relation []int  `json:"relation,omitempty"` // Token IDs this token relates to
}

// DictionaryEntry represents dictionary lookup information
type DictionaryEntry struct {
	Source      string                 `json:"source,omitempty"`
	Kanji       []string               `json:"kanji,omitempty"`
	Readings    []string               `json:"readings,omitempty"`
	Glosses     []string               `json:"glosses,omitempty"`
	POS         []string               `json:"pos,omitempty"`
	Frequency   int                    `json:"frequency,omitempty"`
	IsName      bool                   `json:"is_name,omitempty"`
	IsCommon    bool                   `json:"is_common,omitempty"`
	OtherFields map[string]interface{} `json:"other_fields,omitempty"`
}

// GrammarAnalysis represents the grammatical structure analysis
type GrammarAnalysis struct {
	SentenceID       string    `json:"sentence_id"`
	TokenCount       int       `json:"token_count"`
	DefinitionsFound int       `json:"definitions_found"`
	Structure        Structure `json:"structure"`
}

// Structure represents the grammatical structure of the sentence
type Structure struct {
	Clauses []Clause `json:"clauses"`
}

// Clause represents a grammatical clause within the sentence
type Clause struct {
	Start int         `json:"start"`
	End   int         `json:"end"`
	Type  string      `json:"type"`
	Roles ClauseRoles `json:"roles"`
}

// ClauseRoles represents the roles of tokens within a clause
type ClauseRoles struct {
	Tokens  []int  `json:"tokens"`
	Subject *[]int `json:"subject,omitempty"`
	Object  *[]int `json:"object,omitempty"`
	Verb    *int   `json:"verb,omitempty"`
}

// LexEntry represents a lexical entry during analysis
type LexEntry struct {
	Token       Token    `json:"token"`
	Readings    []string `json:"readings,omitempty"`
	Definitions []string `json:"definitions,omitempty"`
}

// Sentence represents the input sentence for processing
type Sentence struct {
	ID   string `json:"id"`
	Text string `json:"text"`
}

// AnalyzerConfig holds configuration for the Japanese text analyzer
type AnalyzerConfig struct {
	DictPath     string `json:"dict_path"`
	EnamdictPath string `json:"enamdict_path"`
	KanjidicPath string `json:"kanjidic_path"`
	OutputDir    string `json:"output_dir"`
	SaveLogs     bool   `json:"save_logs"`
	Debug        bool   `json:"debug"`
}
