package lookup

import (
	"context"
	"japaneseparse/model"
)

type LexEntry = model.LexEntry

func Lookup(ctx context.Context, tokens []model.Token) ([]LexEntry, error) {
	if tokens == nil {
		return nil, nil
	}
	out := make([]LexEntry, 0, len(tokens))
	for _, t := range tokens {
		out = append(out, LexEntry{Token: t, Readings: []string{}, Definitions: []string{}})
	}
	return out, nil
}
