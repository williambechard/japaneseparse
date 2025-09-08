package dictionary

import (
	"context"
	"japaneseparse/model"
	"japaneseparse/tokenize"
)

func InitDictionaries(jmdictPath, enamdictPath string) error {
	// If LoadJMdict is not exported, inline its logic or export it
	return nil // TODO: Replace with actual loading logic
}

func DebugGlossaryFields() {
	// No-op or add debug logic if needed
}

func LookupDictionary(ctx context.Context, tokens []tokenize.Token) ([]model.DictionaryEntry, error) {
	entries := make([]model.DictionaryEntry, len(tokens))
	for i, t := range tokens {
		entries[i] = model.DictionaryEntry{
			Kanji:    []string{t.Text},
			Readings: []string{t.Reading},
			Glosses:  []string{"<no definition found>"},
			Source:   "none",
		}
	}
	return entries, nil
}
