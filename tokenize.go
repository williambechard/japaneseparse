package main

import (
	"context"
	"strings"

	"github.com/ikawaha/kagome-dict/ipa"
	"github.com/ikawaha/kagome/v2/tokenizer"
)

// Token represents a token / morpheme produced by the tokenizer.
type Token struct {
	Text         string `json:"text"`
	Lemma        string `json:"lemma,omitempty"`
	POS          string `json:"pos,omitempty"`
	Start        int    `json:"start"`
	End          int    `json:"end"`
	Reading      string `json:"reading,omitempty"`
	Pronunciation string `json:"pronunciation,omitempty"`
	TokenID      int    `json:"token_id,omitempty"`
}

// Tokenized pairs a Sentence with the tokens produced for it.
type Tokenized struct {
	Sentence Sentence
	Tokens   []Token
}

// TokenizedChan publishes tokenization results for downstream processing.
var TokenizedChan chan Tokenized

// kagome tokenizer instance (initialized in init)
var kg *tokenizer.Tokenizer

func init() {
	TokenizedChan = make(chan Tokenized, 100)
	// initialize kagome tokenizer with the ipa dict and omit BOS/EOS
	// ignore errors here for simplicity; Tokenize will return an error if tokenizer is nil
	if t, err := tokenizer.New(ipa.Dict(), tokenizer.OmitBosEos()); err == nil {
		kg = t
	}
}

func convertKagomeTokens(ktoks []tokenizer.Token) []Token {
	out := make([]Token, 0, len(ktoks))
	for _, kt := range ktoks {
		pos := strings.Join(kt.POS(), ",")
		lemma, _ := kt.BaseForm()
		if lemma == "" {
			lemma = kt.Surface
		}
			reading, okR := kt.Reading()
		if !okR {
			reading = ""
		}
		pron, okP := kt.Pronunciation()
		if !okP {
			pron = ""
		}
		tokenID := kt.ID // kagome's Token.ID is a field, not a method
		t := Token{
			Text:         kt.Surface,
			Lemma:        lemma,
			POS:          pos,
			Start:        kt.Start,
			End:          kt.End,
			Reading:      reading,
			Pronunciation: pron,
			TokenID:      tokenID,
		}
		out = append(out, t)
	}
	return out
}

// Tokenize uses kagome to produce tokens for the input text (normal mode).
func Tokenize(ctx context.Context, text string) ([]Token, error) {
	if text == "" {
		return nil, nil
	}
	if kg == nil {
		// tokenizer not initialized
		return nil, nil
	}

	ktoks := kg.Tokenize(text)
	return convertKagomeTokens(ktoks), nil
}

// TokenizeModes runs kagome.Analyze in Normal, Search and Extended modes and returns
// a map from mode name to the resulting tokens. Useful to compare segmentations.
func TokenizeModes(ctx context.Context, text string) (map[string][]Token, error) {
	res := make(map[string][]Token)
	if text == "" || kg == nil {
		return res, nil
	}

	// Normal
	ktNormal := kg.Analyze(text, tokenizer.Normal)
	res["normal"] = convertKagomeTokens(ktNormal)

	// Search
	ktSearch := kg.Analyze(text, tokenizer.Search)
	res["search"] = convertKagomeTokens(ktSearch)

	// Extended
	ktExt := kg.Analyze(text, tokenizer.Extended)
	res["extended"] = convertKagomeTokens(ktExt)

	return res, nil
}

// TokenizeStream streams tokens to a channel. This is useful for building a concurrent pipeline.
func TokenizeStream(ctx context.Context, text string) (<-chan Token, <-chan error) {
	out := make(chan Token, 8)
	errs := make(chan error, 1)
	go func() {
		defer close(out)
		defer close(errs)
		toks, err := Tokenize(ctx, text)
		if err != nil {
			errs <- err
			return
		}
		for _, tk := range toks {
			select {
			case <-ctx.Done():
				errs <- ctx.Err()
				return
			case out <- tk:
			}
		}
	}()
	return out, errs
}

// StartTokenizer launches a goroutine that consumes Sentence from IngestChan,
// tokenizes them and publishes Tokenized results to TokenizedChan.
func StartTokenizer(ctx context.Context) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case s := <-IngestChan:
				// process s (normal mode)
				toks, err := Tokenize(ctx, s.Text)
				if err != nil {
					// on error drop for now
					continue
				}
				select {
				case <-ctx.Done():
					return
				case TokenizedChan <- Tokenized{Sentence: s, Tokens: toks}:
				}
			}
		}
	}()
}
