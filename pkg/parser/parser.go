package parser

import (
	"fmt"
	"strings"

	"japaneseparse/internal/analyzer"
	"japaneseparse/internal/config"
	"japaneseparse/model"
	"japaneseparse/pkg/types"
)

// Parser is the main interface for Japanese text analysis
// Designed to be embedded in larger applications as a component
type Parser struct {
	analyzer *analyzer.JapaneseAnalyzer
}

// Config represents the configuration options for the parser
type Config struct {
	// Dictionary paths - if empty, will use default paths
	JMdictPath   string // Path to JMdict dictionary file
	EnamdictPath string // Path to ENAMDICT proper names dictionary
	KanjidicPath string // Path to Kanjidic2 kanji dictionary

	// Logging options - typically disabled for library use
	SaveLogs bool   // Whether to save detailed processing logs
	LogsDir  string // Directory to save logs (if SaveLogs is true)

	// Processing options
	Debug   bool // Enable debug logging
	Verbose bool // Enable verbose output
}

// New creates a new Parser with default configuration
// Uses embedded dictionary files and no logging - ideal for library use
func New() (*Parser, error) {
	return NewWithConfig(&Config{
		JMdictPath:   "dict/JMdict_e",
		EnamdictPath: "dict/enamdict",
		KanjidicPath: "dict/kanjidic2.xml",
		SaveLogs:     false, // No logs by default for library use
		Debug:        false,
		Verbose:      false,
	})
}

// NewWithConfig creates a new Parser with custom configuration
func NewWithConfig(cfg *Config) (*Parser, error) {
	if cfg == nil {
		return New()
	}

	// Convert to internal config format
	internalConfig := &config.Config{
		Dictionary: config.DictionaryConfig{
			JMdictPath:   cfg.JMdictPath,
			EnamdictPath: cfg.EnamdictPath,
			KanjidicPath: cfg.KanjidicPath,
		},
		Output: config.OutputConfig{
			LogsDir:  cfg.LogsDir,
			SaveLogs: cfg.SaveLogs,
			Verbose:  cfg.Verbose,
		},
		Debug: cfg.Debug,
	}

	// Create analyzer
	analyzer := analyzer.New(internalConfig)
	if err := analyzer.Initialize(); err != nil {
		return nil, fmt.Errorf("failed to initialize parser: %w", err)
	}

	return &Parser{analyzer: analyzer}, nil
}

// ParseResult contains the complete analysis result
type ParseResult struct {
	Text             string         `json:"text"`              // Original input text
	SentenceID       string         `json:"sentence_id"`       // Unique identifier for this analysis
	DefinitionsFound int            `json:"definitions_found"` // Number of tokens with dictionary definitions
	Tokens           []model.Token  `json:"tokens"`            // Detailed token analysis
	Clauses          []types.Clause `json:"clauses"`           // Grammatical clause structure
	ProcessedAt      string         `json:"processed_at"`      // When the analysis was performed
}

// Parse analyzes Japanese text and returns the complete analysis
// This is the main method for your language interpreter to call
func (p *Parser) Parse(text string) (*ParseResult, error) {
	// Get full analysis from internal analyzer
	result, err := p.analyzer.AnalyzeText(text)
	if err != nil {
		return nil, fmt.Errorf("analysis failed: %w", err)
	}

	// Convert to simplified external format
	return p.convertToParseResult(text, result), nil
}

// ParseSimple returns just the essential information for basic processing
func (p *Parser) ParseSimple(text string) ([]model.Token, error) {
	result, err := p.Parse(text)
	if err != nil {
		return nil, err
	}
	return result.Tokens, nil
}

// GetReadings extracts just the readings for each word - useful for pronunciation
func (p *Parser) GetReadings(text string) ([]string, error) {
	tokens, err := p.ParseSimple(text)
	if err != nil {
		return nil, err
	}

	readings := make([]string, len(tokens))
	for i, token := range tokens {
		readings[i] = token.Reading
	}
	return readings, nil
}

// GetMeanings extracts meanings for each word - useful for translation
func (p *Parser) GetMeanings(text string) ([][]string, error) {
	tokens, err := p.ParseSimple(text)
	if err != nil {
		return nil, err
	}

	meanings := make([][]string, len(tokens))
	for i, token := range tokens {
		meanings[i] = token.Meanings
	}
	return meanings, nil
}

// FormatHumanReadable returns a human-readable analysis (for debugging)
func (p *Parser) FormatHumanReadable(result *ParseResult) string {
	var output strings.Builder

	output.WriteString("=== Japanese Text Analysis ===\n")
	output.WriteString(fmt.Sprintf("Text: %s\n", result.Text))
	output.WriteString(fmt.Sprintf("Definitions found: %d\n\n", result.DefinitionsFound))

	output.WriteString("=== Token Analysis ===\n")
	for i, token := range result.Tokens {
		output.WriteString(fmt.Sprintf("%d. %s", i+1, token.Text))

		if token.Lemma != "" && token.Lemma != token.Text {
			output.WriteString(fmt.Sprintf(" (%s)", token.Lemma))
		}

		if token.Reading != "" {
			output.WriteString(fmt.Sprintf(" [%s]", token.Reading))
		}

		if token.POS != "" {
			output.WriteString(fmt.Sprintf(" <%s>", token.POS))
		}

		output.WriteString("\n")

		if token.Furigana != "" {
			output.WriteString(fmt.Sprintf("   Furigana: %s\n", token.Furigana))
		}

		if len(token.Meanings) > 0 {
			output.WriteString(fmt.Sprintf("   Meanings: %s\n", strings.Join(token.Meanings, "; ")))
		}

		if len(token.Conjugation) > 0 {
			output.WriteString(fmt.Sprintf("   Conjugation: %s\n", token.Conjugation))
		}

		if token.HasAuxiliaries {
			output.WriteString("   Auxiliaries: ")
			for j, aux := range token.Auxiliaries {
				if j > 0 {
					output.WriteString(" + ")
				}
				output.WriteString(aux.Text)
			}
			output.WriteString("\n")
		}

		output.WriteString("\n")
	}

	// Add grammatical analysis to human-readable output
	if len(result.Clauses) > 0 {
		output.WriteString("=== Grammatical Analysis ===\n")
		for i, clause := range result.Clauses {
			output.WriteString(fmt.Sprintf("Clause %d: tokens %d-%d", i+1, clause.Start, clause.End))
			output.WriteString(fmt.Sprintf("   TokenCount: %d\n", clause.End-clause.Start))
			output.WriteString(fmt.Sprintf("   Type: %s\n", clause.Type))
			output.WriteString(fmt.Sprintf("   Roles: %v\n", clause.Roles.Tokens))
			output.WriteString("\n")
		}
	}

	return output.String()
}

// convertToParseResult converts internal types to the simplified external API
func (p *Parser) convertToParseResult(originalText string, result *types.SentenceAnalysis) *ParseResult {
	// Convert tokens
	tokens := make([]model.Token, len(result.Tokens))
	for i, token := range result.Tokens {
		meanings := []string{}
		if len(token.DictionaryEntry.Glosses) > 0 {
			meanings = token.DictionaryEntry.Glosses
		}

		// Convert auxiliaries
		auxiliaries := make([]model.Token, len(token.Auxiliaries))
		for j, aux := range token.Auxiliaries {
			auxMeanings := []string{}
			if len(aux.DictionaryEntry.Glosses) > 0 {
				auxMeanings = aux.DictionaryEntry.Glosses
			}

			auxiliaries[j] = model.Token{
				Text:       aux.Text,
				Lemma:      aux.Lemma,
				Reading:    aux.Reading,
				POS:        aux.POS,
				Meanings:   auxMeanings,
				Furigana:   aux.FuriganaText,
				DictSource: aux.DictionaryEntry.Source,
			}
		}

		tokens[i] = model.Token{
			Text:           token.Text,
			Lemma:          token.Lemma,
			Reading:        token.Reading,
			POS:            token.POS,
			Position:       i,
			StartChar:      token.Start,
			EndChar:        token.End,
			Meanings:       meanings,
			Furigana:       token.FuriganaText,
			DictSource:     token.DictionaryEntry.Source,
			InflectionType: token.InflectionType,
			InflectionForm: token.InflectionForm,
			IsConjugated:   token.ConjugationLabel != "" || len(token.Auxiliaries) > 0,
			Conjugation:    []string{token.ConjugationLabel},
			Auxiliaries:    auxiliaries,
			MergedIndices:  token.MergedIndices,
			HasAuxiliaries: len(token.Auxiliaries) > 0,
		}
	}

	// Convert clauses
	clauses := make([]types.Clause, len(result.Analysis.Structure.Clauses))
	for i, clause := range result.Analysis.Structure.Clauses {
		clauses[i] = types.Clause{
			Start: clause.Start,
			End:   clause.End,
			Type:  clause.Type,
		}
	}

	return &ParseResult{
		Text:             originalText,
		SentenceID:       result.SentenceID,
		DefinitionsFound: result.DefinitionsFound,
		Tokens:           tokens,
		Clauses:          clauses,
		ProcessedAt:      result.ProcessedAt.Format("2006-01-02 15:04:05"),
	}
}
