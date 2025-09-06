package main

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"

	"github.com/ikawaha/kagome-dict/ipa"
	"github.com/ikawaha/kagome/v2/tokenizer"
)

// Token represents a token / morpheme produced by the tokenizer.
type Token struct {
	Text             string          `json:"text"`
	Lemma            string          `json:"lemma,omitempty"`
	POS              string          `json:"pos,omitempty"`
	Start            int             `json:"start"`
	End              int             `json:"end"`
	Reading          string          `json:"reading,omitempty"`
	Pronunciation    string          `json:"pronunciation,omitempty"`
	TokenID          int             `json:"token_id,omitempty"`
	Conjugation      []string        `json:"conjugation,omitempty"`
	Auxiliaries      []Token         `json:"auxiliaries,omitempty"`
	MergedIndices    []int           `json:"merged_indices,omitempty"`
	ConjugationLabel string          `json:"conjugation_label,omitempty"`
	InflectionType   string          `json:"inflection_type,omitempty"`
	InflectionForm   string          `json:"inflection_form,omitempty"`
	DictionaryEntry  DictionaryEntry `json:"dictionary_entry,omitempty"`
	FuriganaText     string          `json:"furigana_text,omitempty"`
	FuriganaLemma    string          `json:"furigana_lemma,omitempty"`
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

var (
	kanjiReadingMap     map[rune][]string
	kanjiReadingMapOnce sync.Once
)

// Kanjidic2Kanji represents a single kanji entry in Kanjidic2
// Kanjidic2Root represents the root of the Kanjidic2 XML
type Kanjidic2Kanji struct {
	Literal        string `xml:"literal"`
	ReadingMeaning struct {
		RMGroup []struct {
			Reading []struct {
				Value string `xml:",chardata"`
				Type  string `xml:"r_type,attr"`
			} `xml:"reading"`
		} `xml:"rmgroup"`
	} `xml:"reading_meaning"`
}

type Kanjidic2Root struct {
	Kanji []Kanjidic2Kanji `xml:"character"`
}

// InitKanjidic2 parses kanjidic2.xml and builds kanji→readings map
func InitKanjidic2(path string) error {
	var err error
	kanjiReadingMapOnce.Do(func() {
		kanjiReadingMap = make(map[rune][]string)
		f, fileErr := os.Open(path)
		if fileErr != nil {
			log.Printf("Failed to open kanjidic2.xml: %v", fileErr)
			return
		}
		defer f.Close()
		var root Kanjidic2Root
		d := xml.NewDecoder(f)
		if decodeErr := d.Decode(&root); decodeErr != nil {
			log.Printf("Failed to parse kanjidic2.xml: %v", decodeErr)
			return
		}
		for _, k := range root.Kanji {
			if len(k.Literal) != 1 {
				continue
			}
			var readings []string
			for _, group := range k.ReadingMeaning.RMGroup {
				for _, r := range group.Reading {
					if r.Type == "ja_on" || r.Type == "ja_kun" {
						readings = append(readings, r.Value)
					}
				}
			}
			kanjiRune := rune(k.Literal[0])
			kanjiReadingMap[kanjiRune] = readings
			if kanjiRune == '秋' || kanjiRune == '田' {
				log.Printf("Loaded readings for %c: %v", kanjiRune, readings)
			}
		}
		log.Printf("Kanjidic2 loaded: %d kanji entries", len(kanjiReadingMap))
		// Dump the full map for debugging
		for k, v := range kanjiReadingMap {
			log.Printf("Map: %c -> %v", k, v)
		}
	})
	return err
}

// GetKanjiReadings returns readings for a kanji rune, with logging
func GetKanjiReadings(r rune) []string {
	if kanjiReadingMap == nil {
		log.Printf("kanjiReadingMap is nil when looking up %c", r)
		return nil
	}
	readings := kanjiReadingMap[r]
	if readings == nil {
		log.Printf("No readings found for kanji %c", r)
	} else {
		log.Printf("Readings for kanji %c: %v", r, readings)
	}
	return readings
}

func init() {
	TokenizedChan = make(chan Tokenized, 100)
	// initialize kagome tokenizer with the ipa dict and omit BOS/EOS
	// ignore errors here for simplicity; Tokenize will return an error if tokenizer is nil
	if t, err := tokenizer.New(ipa.Dict(), tokenizer.OmitBosEos()); err == nil {
		kg = t
	}
}

func isKanji(r rune) bool {
	return r >= 0x4E00 && r <= 0x9FFF
}

// isKana returns true if rune is Hiragana or Katakana
func isKana(r rune) bool {
	return (r >= 0x3040 && r <= 0x309F) || (r >= 0x30A0 && r <= 0x30FF)
}

// getFuriganaString returns a slice of [kanji/kana, furigana] pairs for display.
func getFuriganaString(surface, reading string) [][2]string {
	result := make([][2]string, 0)
	surfaceRunes := []rune(surface)
	readingRunes := []rune(katakanaToHiragana(reading))
	j, k := 0, 0
	for j < len(surfaceRunes) {
		s := surfaceRunes[j]
		if isKanji(s) {
			// Greedily match reading for this kanji
			startK := k
			for k < len(readingRunes) && (j+1 == len(surfaceRunes) || !isKana(surfaceRunes[j+1]) || readingRunes[k] != surfaceRunes[j+1]) {
				k++
			}
			result = append(result, [2]string{string(s), string(readingRunes[startK:k])})
			j++
		} else if isKana(s) {
			result = append(result, [2]string{string(s), string(s)})
			j++
			k++
		} else {
			result = append(result, [2]string{string(s), ""})
			j++
		}
	}
	return result
}

// katakanaToHiragana converts katakana to hiragana for furigana display
func katakanaToHiragana(s string) string {
	runes := []rune(s)
	for i, r := range runes {
		if r >= 0x30A1 && r <= 0x30F6 {
			runes[i] = r - 0x60
		}
	}
	return string(runes)
}

// formatFuriganaDisplay formats the furigana pairs for display (e.g., [kanji|furigana] or HTML ruby tags)
func formatFuriganaDisplay(pairs [][2]string) string {
	out := ""
	for _, pair := range pairs {
		if pair[1] != "" {
			out += "[" + pair[0] + "|" + pair[1] + "]"
		} else {
			out += pair[0]
		}
	}
	return out
}

// getFuriganaFromDictionary tries to align kanji and reading using JMdict entry if available
func getFuriganaFromDictionary(surface string, entry DictionaryEntry) string {
	if len(entry.Kanji) == 0 || len(entry.Readings) == 0 {
		return ""
	}
	kanji := entry.Kanji[0]
	reading := entry.Readings[0]
	if kanji != surface {
		// Only use dictionary furigana if kanji matches surface
		return ""
	}
	// Use dictionary reading for word-level furigana grouping
	surfaceRunes := []rune(kanji)
	readingRunes := []rune(katakanaToHiragana(reading))
	// Try to split reading proportionally by kanji/kana blocks
	result := make([][2]string, 0)
	kanjiCount := 0
	for _, r := range surfaceRunes {
		if isKanji(r) {
			kanjiCount++
		}
	}
	j, k := 0, 0
	for j < len(surfaceRunes) {
		s := surfaceRunes[j]
		if isKanji(s) {
			startK := k
			remainingKanji := 0
			for jj := j + 1; jj < len(surfaceRunes); jj++ {
				if isKanji(surfaceRunes[jj]) {
					remainingKanji++
				}
			}
			remainingReading := len(readingRunes) - k
			segLen := 1
			if remainingKanji > 0 {
				segLen = remainingReading / (remainingKanji + 1)
				if segLen < 1 {
					segLen = 1
				}
			} else {
				segLen = remainingReading
			}
			endK := k + segLen
			if endK > len(readingRunes) {
				endK = len(readingRunes)
			}
			result = append(result, [2]string{"", string(readingRunes[startK:endK])})
			k = endK
			j++
		} else if isKana(s) {
			if k < len(readingRunes) && readingRunes[k] == s {
				result = append(result, [2]string{"", string(s)})
				k++
			} else {
				result = append(result, [2]string{"", ""})
			}
			j++
		} else {
			result = append(result, [2]string{"", ""})
			j++
		}
	}
	// Format as [segment] blocks
	out := ""
	for _, pair := range result {
		if pair[1] != "" {
			out += "[" + pair[1] + "]"
		}
	}
	return out
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
		tokenID := kt.ID
		features := kt.Features()
		infType, infForm := "", ""
		if len(features) > 5 {
			infType = features[4]
			infForm = features[5]
		}
		t := Token{
			Text:           kt.Surface,
			Lemma:          lemma,
			POS:            pos,
			Start:          kt.Start,
			End:            kt.End,
			Reading:        reading,
			Pronunciation:  pron,
			TokenID:        tokenID,
			InflectionType: infType,
			InflectionForm: infForm,
			FuriganaText:   formatFuriganaDisplay(getFuriganaString(kt.Surface, reading)),
			FuriganaLemma:  formatFuriganaDisplay(getFuriganaString(lemma, reading)),
		}
		out = append(out, t)
	}
	return out
}

// UpdateFuriganaFromDictionary updates FuriganaText and FuriganaLemma for tokens using dictionary entries
func UpdateFuriganaFromDictionary(tokens []Token) []Token {
	for i := range tokens {
		dict := tokens[i].DictionaryEntry
		ft := getFuriganaFromDictionary(tokens[i].Text, dict)
		fl := getFuriganaFromDictionary(tokens[i].Lemma, dict)
		if ft != "" {
			tokens[i].FuriganaText = ft
		} else {
			tokens[i].FuriganaText = formatFuriganaDisplayAccurate(alignFuriganaAccurate(tokens[i].Text, tokens[i].Reading))
		}
		if fl != "" {
			tokens[i].FuriganaLemma = fl
		} else {
			tokens[i].FuriganaLemma = formatFuriganaDisplayAccurate(alignFuriganaAccurate(tokens[i].Lemma, tokens[i].Reading))
		}
	}
	return tokens
}

// MergeVerbAuxiliaries scans tokens and merges verb+auxiliary sequences into a single token.
func MergeVerbAuxiliaries(tokens []Token) []Token {
	var out []Token
	i := 0
	for i < len(tokens) {
		tk := tokens[i]
		if strings.HasPrefix(tk.POS, "動詞") {
			// collect auxiliaries following the verb
			auxs := []Token{}
			indices := []int{tk.Start}
			j := i + 1
			for j < len(tokens) && (strings.HasPrefix(tokens[j].POS, "助動詞") ||
				strings.HasPrefix(tokens[j].POS, "動詞,非自立") ||
				strings.HasPrefix(tokens[j].POS, "動詞,接尾")) {
				auxs = append(auxs, tokens[j])
				indices = append(indices, tokens[j].Start)
				j++
			}
			if len(auxs) > 0 {
				// merge
				mergedText := tk.Text
				mergedReading := tk.Reading
				mergedPron := tk.Pronunciation
				conjugation := []string{}
				for _, aux := range auxs {
					mergedText += aux.Text
					mergedReading += aux.Reading
					mergedPron += aux.Pronunciation
					conjugation = append(conjugation, aux.Lemma)
				}
				merged := tk
				merged.Text = mergedText
				merged.Reading = mergedReading
				merged.Pronunciation = mergedPron
				merged.End = auxs[len(auxs)-1].End
				merged.Conjugation = conjugation
				merged.Auxiliaries = auxs
				merged.MergedIndices = indices
				merged.ConjugationLabel = getConjugationLabel(conjugation)
				out = append(out, merged)
				i = j
				continue
			}
		}
		out = append(out, tk)
		i++
	}
	return out
}

// getConjugationLabel maps auxiliary lemma sequences to a human-readable conjugation label.
func getConjugationLabel(auxs []string) string {
	if len(auxs) == 1 {
		if auxs[0] == "ます" {
			return "polite"
		}
		if auxs[0] == "た" {
			return "past"
		}
	}
	if len(auxs) == 2 {
		if auxs[0] == "ます" && auxs[1] == "た" {
			return "polite past"
		}
	}
	return ""
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

// logFuriganaAlignment logs the alignment process to a JSON file for debugging
func logFuriganaAlignment(tokenText, tokenReading string, steps []map[string]interface{}) {
	logFile := fmt.Sprintf("logs/%s_furigana.json", tokenText)
	data, _ := json.MarshalIndent(steps, "", "  ")
	_ = os.WriteFile(logFile, data, 0644)
}

// alignFuriganaAccurate splits reading for each kanji by remaining kana and kanji count, using Kanjidic2 readings for kanji
func alignFuriganaAccurate(surface, reading string) [][2]string {
	surfaceRunes := []rune(surface)
	// Always convert the token's reading to hiragana
	readingRunes := []rune(katakanaToHiragana(reading))
	result := make([][2]string, 0)
	j, k := 0, 0
	logSteps := make([]map[string]interface{}, 0)
	for j < len(surfaceRunes) {
		s := surfaceRunes[j]
		step := map[string]interface{}{
			"kanji":             string(s),
			"reading_pos":       k,
			"remaining_reading": string(readingRunes[k:]),
			"candidates":        []string{},
			"chosen":            "",
		}
		if isKanji(s) {
			kanjiReadings := GetKanjiReadings(s)
			step["candidates"] = kanjiReadings
			longestMatch := ""
			for _, kr := range kanjiReadings {
				// Always convert candidate reading to hiragana
				krH := katakanaToHiragana(kr)
				krRunes := []rune(krH)
				if k+len(krRunes) <= len(readingRunes) && string(readingRunes[k:k+len(krRunes)]) == krH {
					if len(krH) > len(longestMatch) {
						longestMatch = krH
					}
				}
			}
			if len(kanjiReadings) == 0 {
				step["chosen"] = ""
				result = append(result, [2]string{string(s), ""})
				logSteps = append(logSteps, step)
				j++
				continue
			}
			if longestMatch != "" {
				result = append(result, [2]string{string(s), longestMatch})
				step["chosen"] = longestMatch
				k += len([]rune(longestMatch))
			} else {
				step["chosen"] = ""
				result = append(result, [2]string{string(s), ""})
			}
			logSteps = append(logSteps, step)
			j++
		} else if isKana(s) {
			if k < len(readingRunes) && readingRunes[k] == s {
				result = append(result, [2]string{string(s), string(s)})
				step["chosen"] = string(s)
				k++
			} else {
				result = append(result, [2]string{string(s), ""})
				step["chosen"] = ""
			}
			logSteps = append(logSteps, step)
			j++
		} else {
			result = append(result, [2]string{string(s), ""})
			step["chosen"] = ""
			logSteps = append(logSteps, step)
			j++
		}
	}
	// Log alignment for debugging
	logFuriganaAlignment(surface, reading, logSteps)
	return result
}

// formatFuriganaDisplayAccurate formats furigana so only kanji get [kanji|furigana], kana are plain
func formatFuriganaDisplayAccurate(pairs [][2]string) string {
	out := ""
	for _, pair := range pairs {
		if isKanji([]rune(pair[0])[0]) && pair[1] != "" {
			out += "[" + pair[0] + "|" + pair[1] + "]"
		} else {
			out += pair[0]
		}
	}
	return out
}
