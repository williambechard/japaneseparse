package analyzer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/williambechard/japaneseparse/analyze"
	"github.com/williambechard/japaneseparse/dictionary"
	"github.com/williambechard/japaneseparse/enamdict"
	"github.com/williambechard/japaneseparse/ingest"
	"github.com/williambechard/japaneseparse/internal/config"
	"github.com/williambechard/japaneseparse/kanji"
	"github.com/williambechard/japaneseparse/logger"
	"github.com/williambechard/japaneseparse/lookup"
	"github.com/williambechard/japaneseparse/model"
	"github.com/williambechard/japaneseparse/pkg/types"
	"github.com/williambechard/japaneseparse/tokenize"
)

// JapaneseAnalyzer handles the complete Japanese text analysis pipeline
type JapaneseAnalyzer struct {
	config      *config.Config
	initialized bool
	enamdictMap map[string]enamdict.EnamdictEntry
}

// New creates a new JapaneseAnalyzer instance
func New(cfg *config.Config) *JapaneseAnalyzer {
	return &JapaneseAnalyzer{
		config: cfg,
	}
}

// Initialize sets up dictionaries and dependencies
func (ja *JapaneseAnalyzer) Initialize() error {
	if ja.initialized {
		return nil
	}

	// Validate configuration
	if err := ja.config.Validate(); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	// Initialize logs directory if saving logs
	if ja.config.Output.SaveLogs {
		if err := logger.InitLogs(ja.config.Output.LogsDir); err != nil {
			return fmt.Errorf("failed to initialize logs: %w", err)
		}
	}

	// Load dictionaries
	if err := dictionary.InitDictionaries(ja.config.Dictionary.JMdictPath, ja.config.Dictionary.EnamdictPath); err != nil {
		return fmt.Errorf("failed to load dictionaries: %w", err)
	}

	// Load ENAMDICT
	enamdictMap, err := enamdict.LoadEnamdict(ja.config.Dictionary.EnamdictPath)
	if err != nil {
		return fmt.Errorf("failed to load ENAMDICT: %w", err)
	}
	ja.enamdictMap = enamdictMap

	// Initialize Kanjidic2
	if err := kanji.InitKanjidic2(ja.config.Dictionary.KanjidicPath); err != nil {
		return fmt.Errorf("failed to load Kanjidic2: %w", err)
	}

	// Start the tokenizer goroutine
	tokenize.StartTokenizer(context.Background())

	ja.initialized = true
	return nil
}

// AnalyzeText performs complete analysis of Japanese text
func (ja *JapaneseAnalyzer) AnalyzeText(text string) (*types.SentenceAnalysis, error) {
	if !ja.initialized {
		if err := ja.Initialize(); err != nil {
			return nil, fmt.Errorf("failed to initialize analyzer: %w", err)
		}
	}

	// Ingest sentence
	sentence, err := ingest.IngestSentence(text)
	if err != nil {
		return nil, fmt.Errorf("failed to ingest sentence: %w", err)
	}

	if ja.config.Debug {
		logger.Logf("DEBUG: Processing sentence ID: %s", sentence.ID)
	}

	// Tokenize
	tokens, err := ja.tokenize(&sentence)
	if err != nil {
		return nil, fmt.Errorf("failed to tokenize sentence: %w", err)
	}

	if ja.config.Debug {
		logger.Logf("DEBUG: Tokenization result for sentence ID %s: %+v", sentence.ID, tokens)
	}

	// Merge verb auxiliaries
	mergedTokens := tokenize.MergeVerbAuxiliaries(tokens)
	if ja.config.Debug {
		logger.Logf("DEBUG: Merged %d tokens into %d", len(tokens), len(mergedTokens))
	}

	// Dictionary lookup
	dictEntries, err := dictionary.LookupDictionary(context.Background(), mergedTokens)
	if err != nil {
		return nil, fmt.Errorf("dictionary lookup failed: %w", err)
	}

	// Enrich tokens with dictionary entries
	for i := range mergedTokens {
		mergedTokens[i].DictionaryEntry = dictEntries[i]
	}

	// Update furigana from dictionary
	mergedTokens = tokenize.UpdateFuriganaFromDictionary(mergedTokens)

	// Additional lookup enrichment
	lexEntries, err := lookup.Lookup(context.Background(), mergedTokens)
	if err != nil {
		return nil, fmt.Errorf("lookup enrichment failed: %w", err)
	}

	// Attach enriched dictionary entries
	for i := range mergedTokens {
		if i < len(lexEntries) && len(lexEntries[i].Definitions) > 0 {
			src := mergedTokens[i].DictionaryEntry.Source
			if src == "" {
				src = "lookup.go"
			}
			mergedTokens[i].DictionaryEntry = model.DictionaryEntry{
				Kanji:    []string{lexEntries[i].Token.Text},
				Readings: lexEntries[i].Readings,
				Glosses:  lexEntries[i].Definitions,
				Source:   src,
			}
		}
	}

	// Update furigana again after lookup enrichment
	mergedTokens = tokenize.UpdateFuriganaFromDictionary(mergedTokens)

	// Grammar analysis
	analysis, err := analyze.Analyze(context.Background(), sentence, lexEntries)
	if err != nil {
		return nil, fmt.Errorf("grammar analysis failed: %w", err)
	}

	// ENAMDICT fallback for tokens with no glosses
	ja.applyEnamdictFallback(mergedTokens)

	// Convert to output format
	result := ja.convertToSentenceAnalysis(&sentence, mergedTokens, &analysis)

	// Save logs if configured
	if ja.config.Output.SaveLogs {
		if err := ja.saveLogs(sentence.ID, tokens, mergedTokens, dictEntries, &analysis, result); err != nil {
			logger.Logf("WARNING: Failed to save logs: %v", err)
		}
	}

	return result, nil
}

// tokenize handles the tokenization process
func (ja *JapaneseAnalyzer) tokenize(sentence *ingest.Sentence) ([]model.Token, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Send to tokenizer
	ingest.IngestChan <- *sentence

	// Wait for result
	for {
		select {
		case tokenized := <-tokenize.TokenizedChan:
			if tokenized.Sentence.ID == sentence.ID {
				return tokenized.Tokens, nil
			}
		case <-ctx.Done():
			return nil, fmt.Errorf("tokenization timeout")
		}
	}
}

// applyEnamdictFallback applies ENAMDICT as fallback for missing definitions
func (ja *JapaneseAnalyzer) applyEnamdictFallback(tokens []model.Token) {
	for i := range tokens {
		entry := tokens[i].DictionaryEntry
		if len(entry.Glosses) == 0 || (len(entry.Glosses) == 1 && entry.Glosses[0] == "<no definition found>") {
			lemma := tokens[i].Text
			reading := tokens[i].Reading
			hiraganaReading := katakanaToHiragana(reading)
			key := lemma + "|" + hiraganaReading

			if enamEntry, ok := ja.enamdictMap[key]; ok {
				tokens[i].DictionaryEntry = model.DictionaryEntry{
					Kanji:    []string{enamEntry.Lemma},
					Readings: []string{enamEntry.Reading},
					Glosses:  []string{enamEntry.Meaning},
					Source:   "ENAMDICT",
				}
			}
		}
	}
}

// convertToSentenceAnalysis converts internal types to public API types
func (ja *JapaneseAnalyzer) convertToSentenceAnalysis(sentence *ingest.Sentence, tokens []model.Token, analysis *analyze.Analysis) *types.SentenceAnalysis {
	// Convert tokens
	convertedTokens := make([]types.Token, len(tokens))
	for i, token := range tokens {
		convertedTokens[i] = types.Token{
			Text:             token.Text,
			Lemma:            token.Lemma,
			POS:              token.POS,
			POSEnglish:       token.POSEnglish,
			Start:            token.Start,
			End:              token.End,
			Reading:          token.Reading,
			Pronunciation:    token.Pronunciation,
			TokenID:          token.TokenID,
			InflectionType:   token.InflectionType,
			InflectionForm:   token.InflectionForm,
			FuriganaText:     token.FuriganaText,
			FuriganaLemma:    token.FuriganaLemma,
			Conjugation:      token.Conjugation,
			MergedIndices:    token.MergedIndices,
			ConjugationLabel: token.ConjugationLabel,
			DictionaryEntry: types.DictionaryEntry{
				Source:      token.DictionaryEntry.Source,
				Kanji:       token.DictionaryEntry.Kanji,
				Readings:    token.DictionaryEntry.Readings,
				Glosses:     token.DictionaryEntry.Glosses,
				POS:         token.DictionaryEntry.POS,
				Frequency:   token.DictionaryEntry.Frequency,
				IsName:      token.DictionaryEntry.IsName,
				IsCommon:    token.DictionaryEntry.IsCommon,
				OtherFields: token.DictionaryEntry.OtherFields,
			},
		}

		// Convert auxiliaries
		if len(token.Auxiliaries) > 0 {
			convertedTokens[i].Auxiliaries = make([]types.Token, len(token.Auxiliaries))
			for j, aux := range token.Auxiliaries {
				convertedTokens[i].Auxiliaries[j] = types.Token{
					Text:           aux.Text,
					Lemma:          aux.Lemma,
					POS:            aux.POS,
					POSEnglish:     aux.POSEnglish,
					Start:          aux.Start,
					End:            aux.End,
					Reading:        aux.Reading,
					Pronunciation:  aux.Pronunciation,
					TokenID:        aux.TokenID,
					InflectionType: aux.InflectionType,
					InflectionForm: aux.InflectionForm,
					FuriganaText:   aux.FuriganaText,
					FuriganaLemma:  aux.FuriganaLemma,
					DictionaryEntry: types.DictionaryEntry{
						Source:      aux.DictionaryEntry.Source,
						Kanji:       aux.DictionaryEntry.Kanji,
						Readings:    aux.DictionaryEntry.Readings,
						Glosses:     aux.DictionaryEntry.Glosses,
						POS:         aux.DictionaryEntry.POS,
						Frequency:   aux.DictionaryEntry.Frequency,
						IsName:      aux.DictionaryEntry.IsName,
						IsCommon:    aux.DictionaryEntry.IsCommon,
						OtherFields: aux.DictionaryEntry.OtherFields,
					},
				}
			}
		}
	}

	// Extract clauses from the structure interface
	var clauses []types.Clause
	if structMap, ok := analysis.Structure.(map[string]interface{}); ok {
		if clausesData, exists := structMap["clauses"]; exists {
			if clauseSlice, ok := clausesData.([]analyze.Clause); ok {
				clauses = make([]types.Clause, len(clauseSlice))
				for i, clause := range clauseSlice {
					clauses[i] = types.Clause{
						Start: clause.Start,
						End:   clause.End,
						Type:  string(clause.Type),
						Roles: types.ClauseRoles{
							Tokens:  clause.Roles.Tokens,
							Subject: clause.Roles.Subject,
							Object:  clause.Roles.Object,
							Verb:    clause.Roles.Verb,
						},
					}

					// Debugging: Log clause roles and type
					logger.Logf("DEBUG: Clause %d-%d, Type: %s, Roles: %v", clause.Start, clause.End, clause.Type, clause.Roles.Tokens)

					// Write clause analysis to a JSON file for debugging
					debugData := map[string]interface{}{
						"start": clause.Start,
						"end":   clause.End,
						"type":  clause.Type,
						"roles": clause.Roles.Tokens,
					}
					// Add debug logs before and after file writing
					logger.Logf("DEBUG: Attempting to write clause log to %s", fmt.Sprintf("%s/debug_clause_%d.json", ja.config.Output.LogsDir, i))
					// Normalize LogsDir to avoid trailing slashes
					logsDir := strings.TrimRight(ja.config.Output.LogsDir, "/\\")
					if logsDir == "" {
						logsDir = "logs"
					}
					// Write debug clause JSON into the logs directory with a clean id (no extension)
					_ = logger.LogJSON(logsDir, fmt.Sprintf("debug_clause_%d", i), debugData)
				}

				// Aggregate all clause data into a single grammar.json file
				// Ensure logsDir is defined for grammar.json aggregation
				logsDir := strings.TrimRight(ja.config.Output.LogsDir, "/\\")
				if logsDir == "" {
					logsDir = "logs" // Default to 'logs' if empty
				}
				// Enhance clause data with additional details
				var allClauses []map[string]interface{}
				for _, clause := range clauseSlice {
					clauseData := map[string]interface{}{
						"start": clause.Start,
						"end":   clause.End,
						"type":  clause.Type,
						"roles": clause.Roles.Tokens,
					}
					// Add metadata if available
					if clause.Roles.Subject != nil && len(*clause.Roles.Subject) > 0 {
						clauseData["subject"] = *clause.Roles.Subject
					}
					if clause.Roles.Object != nil && len(*clause.Roles.Object) > 0 {
						clauseData["object"] = *clause.Roles.Object
					}
					if clause.Roles.Verb != nil {
						clauseData["verb"] = *clause.Roles.Verb
					}
					allClauses = append(allClauses, clauseData)
				}
				logger.LogJSON(logsDir, "grammar", allClauses)

				// Debug logs for grammar.json generation
				logger.Logf("DEBUG: Writing grammar.json to %s", filepath.Join(logsDir, "grammar.json"))
				logger.Logf("DEBUG: Preparing to write grammar.json. Clause count: %d", len(allClauses))
				for i, clause := range allClauses {
					logger.Logf("DEBUG: Clause %d: %+v", i, clause)
				}
				if err := logger.LogJSON(logsDir, "grammar", allClauses); err != nil {
					logger.Logf("ERROR: Failed to write grammar.json: %v", err)
				} else {
					logger.Logf("DEBUG: Successfully wrote grammar.json")
				}
			}
		}
	}

	return &types.SentenceAnalysis{
		SentenceID:       sentence.ID,
		TokenCount:       len(convertedTokens),
		Tokens:           convertedTokens,
		ProcessedAt:      time.Now(),
		DefinitionsFound: analysis.Definitions,
		Analysis: types.GrammarAnalysis{
			SentenceID:       analysis.SentenceID,
			TokenCount:       analysis.TokenCount,
			DefinitionsFound: analysis.Definitions,
			Structure: types.Structure{
				Clauses: clauses,
			},
		},
	}
}

// saveLogs saves intermediate and final results to log files
func (ja *JapaneseAnalyzer) saveLogs(sentenceID string, originalTokens, mergedTokens []model.Token, dictEntries []model.DictionaryEntry, analysis *analyze.Analysis, result *types.SentenceAnalysis) error {
	logDir := ja.config.Output.LogsDir

	// Save token data
	tokensOut := map[string]interface{}{
		"original_tokens": originalTokens,
		"merged_tokens":   mergedTokens,
	}
	if err := logger.LogJSON(logDir, sentenceID+"_tokens", tokensOut); err != nil {
		return fmt.Errorf("saving tokens: %w", err)
	}

	// Save enriched tokens
	if err := logger.LogJSON(logDir, sentenceID+"_enriched_tokens", mergedTokens); err != nil {
		return fmt.Errorf("saving enriched tokens: %w", err)
	}

	// Save dictionary results
	if err := logger.LogJSON(logDir, sentenceID+"_dict", dictEntries); err != nil {
		return fmt.Errorf("saving dictionary: %w", err)
	}

	// Save analysis
	if err := logger.LogJSON(logDir, sentenceID+"_analysis", analysis); err != nil {
		return fmt.Errorf("saving analysis: %w", err)
	}

	// Save final merged result
	mergedOutput := map[string]interface{}{
		"sentence_id": sentenceID,
		"token_count": len(mergedTokens),
		"tokens":      mergedTokens,
		"analysis":    analysis,
	}
	if err := logger.LogJSON(logDir, sentenceID+"_merged", mergedOutput); err != nil {
		return fmt.Errorf("saving merged output: %w", err)
	}

	// Save human-readable output
	humanReadable := ja.generateHumanReadableOutput(result, false)
	if err := ja.saveTextFile(logDir, sentenceID+"_human_readable_output", humanReadable); err != nil {
		return fmt.Errorf("saving human readable output: %w", err)
	}

	// Log the LogsDir path for debugging
	logger.Logf("DEBUG: LogsDir path: %s", ja.config.Output.LogsDir)
	// Check if the directory exists
	if _, err := os.Stat(ja.config.Output.LogsDir); os.IsNotExist(err) {
		logger.Logf("ERROR: LogsDir does not exist: %s", ja.config.Output.LogsDir)
	} else {
		logger.Logf("DEBUG: LogsDir exists: %s", ja.config.Output.LogsDir)
	}

	return nil
}

// katakanaToHiragana converts katakana to hiragana
func katakanaToHiragana(s string) string {
	runes := []rune(s)
	for i, r := range runes {
		if r >= 0x30A1 && r <= 0x30F6 {
			runes[i] = r - 0x60
		}
	}
	return string(runes)
}

// generateHumanReadableOutput creates a human-readable text format of the analysis
func (ja *JapaneseAnalyzer) generateHumanReadableOutput(result *types.SentenceAnalysis, verbose bool) string {
	var output strings.Builder

	output.WriteString("=== Japanese Text Analysis ===\n")
	output.WriteString(fmt.Sprintf("Sentence ID: %s\n", result.SentenceID))
	output.WriteString(fmt.Sprintf("Tokens: %d\n", result.TokenCount))
	output.WriteString(fmt.Sprintf("Definitions found: %d\n", result.DefinitionsFound))
	output.WriteString(fmt.Sprintf("Processed at: %s\n\n", result.ProcessedAt.Format("2006-01-02 15:04:05")))

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

		// Show furigana if available
		if token.FuriganaText != "" {
			output.WriteString(fmt.Sprintf("   Furigana: %s\n", token.FuriganaText))
		}

		// Show dictionary information
		if len(token.DictionaryEntry.Glosses) > 0 {
			output.WriteString(fmt.Sprintf("   Meanings: %s\n", joinStrings(token.DictionaryEntry.Glosses, "; ")))
			if token.DictionaryEntry.Source != "" {
				output.WriteString(fmt.Sprintf("   Source: %s\n", token.DictionaryEntry.Source))
			}
		}

		// Show conjugation information
		if token.ConjugationLabel != "" {
			output.WriteString(fmt.Sprintf("   Conjugation: %s\n", token.ConjugationLabel))
		}

		// Show auxiliaries if present
		if len(token.Auxiliaries) > 0 {
			output.WriteString("   Auxiliaries: ")
			for j, aux := range token.Auxiliaries {
				if j > 0 {
					output.WriteString(" + ")
				}
				output.WriteString(aux.Text)
			}
			output.WriteString("\n")
		}

		if verbose {
			// Additional verbose information
			if token.InflectionType != "" {
				output.WriteString(fmt.Sprintf("   Inflection Type: %s\n", token.InflectionType))
			}
			if token.InflectionForm != "" {
				output.WriteString(fmt.Sprintf("   Inflection Form: %s\n", token.InflectionForm))
			}
			if len(token.MergedIndices) > 0 {
				output.WriteString(fmt.Sprintf("   Merged Indices: %v\n", token.MergedIndices))
			}
		}

		output.WriteString("\n")
	}

	// Show clause structure
	if len(result.Analysis.Structure.Clauses) > 0 {
		output.WriteString("=== Clause Structure ===\n")
		for i, clause := range result.Analysis.Structure.Clauses {
			output.WriteString(fmt.Sprintf("Clause %d: tokens %d-%d", i+1, clause.Start, clause.End))
			if clause.Type != "" {
				output.WriteString(fmt.Sprintf(" (%s)", clause.Type))
			}
			output.WriteString("\n")

			// Add roles and tokens
			if clause.Roles.Subject != nil {
				output.WriteString(fmt.Sprintf("   Subject: %v\n", *clause.Roles.Subject))
			}
			if clause.Roles.Object != nil {
				output.WriteString(fmt.Sprintf("   Object: %v\n", *clause.Roles.Object))
			}
			if clause.Roles.Verb != nil {
				output.WriteString(fmt.Sprintf("   Verb: %d\n", *clause.Roles.Verb))
			}
			output.WriteString(fmt.Sprintf("   Tokens: %v\n", clause.Roles.Tokens))

			// Debugging: Log roles for each clause
			logger.Logf("DEBUG: Clause %d roles: Subject=%v, Object=%v, Verb=%v, Tokens=%v", i+1, clause.Roles.Subject, clause.Roles.Object, clause.Roles.Verb, clause.Roles.Tokens)

			// Debugging: Log tokens mapped to clause roles
			logger.Logf("DEBUG: Mapping tokens to clause roles: Tokens=%v", clause.Roles.Tokens)
			if clause.Roles.Subject != nil {
				logger.Logf("DEBUG: Subject tokens: %v", *clause.Roles.Subject)
			} else {
				logger.Logf("DEBUG: No subject tokens assigned")
			}
			if clause.Roles.Object != nil {
				logger.Logf("DEBUG: Object tokens: %v", *clause.Roles.Object)
			} else {
				logger.Logf("DEBUG: No object tokens assigned")
			}
			if clause.Roles.Verb != nil {
				logger.Logf("DEBUG: Verb token: %d", *clause.Roles.Verb)
			} else {
				logger.Logf("DEBUG: No verb token assigned")
			}

			// Debugging: Log skipped tokens during clause role mapping
			for _, tokenID := range clause.Roles.Tokens {
				if tokenID == 0 {
					logger.Logf("DEBUG: Skipped token during clause role mapping: TokenID=%d", tokenID)
				}
			}
		}
		output.WriteString("\n")
	}

	// Debugging: Log roles before generating human-readable output
	for i, clause := range result.Analysis.Structure.Clauses {
		logger.Logf("DEBUG: Before output generation - Clause %d roles: Subject=%v, Object=%v, Verb=%v, Tokens=%v", i+1, clause.Roles.Subject, clause.Roles.Object, clause.Roles.Verb, clause.Roles.Tokens)
	}

	return output.String()
}

// saveTextFile saves text content to a file in the specified directory
func (ja *JapaneseAnalyzer) saveTextFile(logDir, filename, content string) error {
	filepath := fmt.Sprintf("%s/%s.txt", logDir, filename)
	return os.WriteFile(filepath, []byte(content), 0644)
}

// joinStrings joins string slice with separator (helper function for human readable output)
func joinStrings(strings []string, separator string) string {
	if len(strings) == 0 {
		return ""
	}
	if len(strings) == 1 {
		return strings[0]
	}

	result := strings[0]
	for i := 1; i < len(strings); i++ {
		result += separator + strings[i]
	}
	return result
}
