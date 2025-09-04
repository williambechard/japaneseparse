package main

import (
	"context"
)

// LexEntry represents dictionary lookup information for a token.
type LexEntry struct {
	Token       Token    `json:"token"`
	Readings    []string `json:"readings,omitempty"`
	Definitions []string `json:"definitions,omitempty"`
}

// Lookup performs dictionary lookup for a slice of tokens. Placeholder implementation
// that returns a LexEntry per token with empty definitions. Replace with real dictionary
// lookups (JMdict, EDICT, web APIs) as needed.
func Lookup(ctx context.Context, tokens []Token) ([]LexEntry, error) {
	if tokens == nil {
		return nil, nil
	}
	out := make([]LexEntry, 0, len(tokens))
	for _, t := range tokens {
		out = append(out, LexEntry{Token: t, Readings: []string{}, Definitions: []string{}})
	}
	return out, nil
}

// LookupStream performs lookup on tokens received from a channel and sends results to an output channel.
func LookupStream(ctx context.Context, in <-chan Token) (<-chan LexEntry, <-chan error) {
	out := make(chan LexEntry, 8)
	errs := make(chan error, 1)
	go func() {
		defer close(out)
		defer close(errs)
		for tk := range in {
			select {
			case <-ctx.Done():
				errs <- ctx.Err()
				return
			default:
			}
			entries, err := Lookup(ctx, []Token{tk})
			if err != nil {
				errs <- err
				return
			}
			for _, e := range entries {
				select {
				case <-ctx.Done():
					errs <- ctx.Err()
					return
				case out <- e:
				}
			}
		}
	}()
	return out, errs
}
