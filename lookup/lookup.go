package lookup

import (
	"context"

	"japaneseparse/dictionary"
	"japaneseparse/model"
	"japaneseparse/tokenize"
)

type LexEntry = model.LexEntry

// Lookup performs a lexical lookup for tokens and returns LexEntry results.
// It delegates to the dictionary package so the pipeline reuses the same
// JMdict/ENAMDICT logic used by the demo and dictionary package.
func Lookup(ctx context.Context, tokens []model.Token) ([]LexEntry, error) {
	if tokens == nil {
		return nil, nil
	}

	// Convert model.Token -> tokenize.Token minimal shape expected by dictionary
	tkns := make([]tokenize.Token, len(tokens))
	for i, t := range tokens {
		tkns[i] = tokenize.Token{
			Text:          t.Text,
			Lemma:         t.Lemma,
			Reading:       t.Reading,
			Pronunciation: t.Pronunciation,
		}
	}

	dictEntries, err := dictionary.LookupDictionary(ctx, tkns)
	if err != nil {
		return nil, err
	}

	out := make([]LexEntry, len(tokens))
	for i, t := range tokens {
		defs := []string{}
		if i < len(dictEntries) {
			defs = dictEntries[i].Glosses
		}
		out[i] = LexEntry{Token: t, Readings: dictEntries[i].Readings, Definitions: defs}
	}
	return out, nil
}
