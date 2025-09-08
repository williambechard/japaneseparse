package main

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"japaneseparse/ingest"
	"japaneseparse/kanji"
	"japaneseparse/model"

	"logger/logger"

	"github.com/ikawaha/kagome-dict/ipa"
	"github.com/ikawaha/kagome/v2/tokenizer"
)

// Tokenized pairs an ingest.Sentence with the tokens produced for it.
type Tokenized struct {
	Sentence ingest.Sentence
	Tokens   []model.Token
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

// Add this to the type definition:
var _ = xml.Name{Local: "kanjidic2"}

// InitKanjidic2 parses kanjidic2.xml and builds kanji→readings map
func InitKanjidic2(path string) error {
	var err error
	kanjiReadingMapOnce.Do(func() {
		kanjiReadingMap = make(map[rune][]string)
		var loadedKanji []string
		f, fileErr := os.Open(path)
		if fileErr != nil {
			log.Printf("Failed to open kanjidic2.xml: %v", fileErr)
			return
		}
		defer f.Close()

		// Use xml.Decoder to find <character> elements directly, skipping any wrapper
		d := xml.NewDecoder(f)
		for {
			tok, tokenErr := d.Token()
			if tokenErr == io.EOF {
				break
			}
			if tokenErr != nil {
				log.Printf("Failed to parse kanjidic2.xml: %v", tokenErr)
				return
			}
			switch se := tok.(type) {
			case xml.StartElement:
				if se.Name.Local == "character" {
					var k Kanjidic2Kanji
					if decodeErr := d.DecodeElement(&k, &se); decodeErr != nil {
						log.Printf("Failed to decode character: %v", decodeErr)
						continue
					}
					if utf8.RuneCountInString(k.Literal) != 1 {
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
					kanjiRune, _ := utf8.DecodeRuneInString(k.Literal)
					kanjiReadingMap[kanjiRune] = readings
					if len(loadedKanji) < 10 {
						loadedKanji = append(loadedKanji, fmt.Sprintf("%c: %v", kanjiRune, readings))
					}
					if kanjiRune == '秋' || kanjiRune == '田' {
						log.Printf("Loaded readings for %c: %v", kanjiRune, readings)
					}
				}
			}
		}
		log.Printf("First 10 kanji loaded: %v", loadedKanji)
		log.Printf("Kanjidic2 loaded: %d kanji entries", len(kanjiReadingMap))
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

func convertKagomeTokens(ktoks []tokenizer.Token) []model.Token {
	out := make([]model.Token, 0, len(ktoks))
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
		t := model.Token{
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

// UpdateFuriganaFromDictionary updates FuriganaText and FuriganaLemma for tokens using Kanjidic2 alignment
func UpdateFuriganaFromDictionary(tokens []model.Token) []model.Token {
	for i := range tokens {
		log.Printf("[FURIGANA-DEBUG] Processing token: surface='%s', reading='%s'", tokens[i].Text, tokens[i].Reading)
		pairsText, stepsText := alignFuriganaAccurate(tokens[i].Text, tokens[i].Reading)
		pairsLemma, stepsLemma := alignFuriganaAccurate(tokens[i].Lemma, tokens[i].Reading)
		// Always use Kanjidic2 alignment for any token containing kanji
		containsKanjiText := false
		for _, r := range []rune(tokens[i].Text) {
			if isKanji(r) {
				containsKanjiText = true
				break
			}
		}
		containsKanjiLemma := false
		for _, r := range []rune(tokens[i].Lemma) {
			if isKanji(r) {
				containsKanjiLemma = true
				break
			}
		}
		if containsKanjiText {
			tokens[i].FuriganaText = formatFuriganaDisplayAccurate(pairsText)
		} else {
			// fallback to dictionary furigana for pure kana/non-kanji
			dict := tokens[i].DictionaryEntry
			ft := getFuriganaFromDictionary(tokens[i].Text, dict)
			tokens[i].FuriganaText = ft
		}
		if containsKanjiLemma {
			tokens[i].FuriganaLemma = formatFuriganaDisplayAccurate(pairsLemma)
		} else {
			dict := tokens[i].DictionaryEntry
			fl := getFuriganaFromDictionary(tokens[i].Lemma, dict)
			tokens[i].FuriganaLemma = fl
		}
		logFuriganaAlignment(tokens[i].Text, tokens[i].Reading, stepsText, pairsText)
		logFuriganaAlignment(tokens[i].Lemma, tokens[i].Reading, stepsLemma, pairsLemma)
	}
	return tokens
}

// MergeVerbAuxiliaries scans tokens and merges verb+auxiliary sequences into a single token.
func MergeVerbAuxiliaries(tokens []model.Token) []model.Token {
	var out []model.Token
	i := 0
	for i < len(tokens) {
		tk := tokens[i]
		if strings.HasPrefix(tk.POS, "動詞") {
			// collect auxiliaries following the verb
			auxs := []model.Token{}
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
func Tokenize(ctx context.Context, text string) ([]model.Token, error) {
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
func TokenizeModes(ctx context.Context, text string) (map[string][]model.Token, error) {
	res := make(map[string][]model.Token)
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
func TokenizeStream(ctx context.Context, text string) (<-chan model.Token, <-chan error) {
	out := make(chan model.Token, 8)
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
		log.Println("[StartTokenizer] Goroutine started, waiting for sentences...")
		for {
			select {
			case <-ctx.Done():
				log.Println("[StartTokenizer] Context done, exiting goroutine.")
				return
			case s := <-ingest.IngestChan:
				log.Printf("[StartTokenizer] Received sentence: ID=%s, Text=%s", s.ID, s.Text)
				toks, err := Tokenize(ctx, s.Text)
				if err != nil {
					log.Printf("[StartTokenizer] Tokenize error: %v", err)
					continue
				}
				log.Printf("[StartTokenizer] Tokenized %d tokens for sentence ID=%s", len(toks), s.ID)
				select {
				case <-ctx.Done():
					log.Println("[StartTokenizer] Context done after tokenization, exiting goroutine.")
					return
				case TokenizedChan <- Tokenized{Sentence: s, Tokens: toks}:
					log.Printf("[StartTokenizer] Published tokenized result for sentence ID=%s", s.ID)
				}
			}
		}
	}()
}

// sanitizeFilename ensures log filenames are safe and unique
func sanitizeFilename(s string) string {
	var out strings.Builder
	for _, r := range s {
		if r <= 127 && r != '/' && r != '\\' && r != ':' && r != '*' && r != '?' && r != '"' && r != '<' && r != '>' && r != '|' {
			out.WriteRune(r)
		} else {
			out.WriteString(fmt.Sprintf("_%X_", r))
		}
	}
	return out.String()
}

// logFuriganaAlignment logs the alignment process to a JSON file for debugging
func logFuriganaAlignment(tokenText, tokenReading string, steps []map[string]interface{}, result [][2]string) {
	timestamp := time.Now().UnixNano()
	textSafe := sanitizeFilename(tokenText)
	readingSafe := sanitizeFilename(tokenReading)
	logFile := fmt.Sprintf("logs/%s_%s_%d_furigana.json", textSafe, readingSafe, timestamp)
	log.Printf("[FURIGANA-LOG] Writing furigana log for token: surface='%s', reading='%s', filename='%s'", tokenText, tokenReading, logFile)
	logData := map[string]interface{}{
		"surface": tokenText,
		"reading": tokenReading,
		"steps":   steps,
		"result":  result,
	}
	data, _ := json.MarshalIndent(logData, "", "  ")
	_ = os.WriteFile(logFile, data, 0644)
}

// alignFuriganaAccurate returns both alignment pairs and log steps
func alignFuriganaAccurate(surface, reading string) ([][2]string, []map[string]interface{}) {
	surfaceRunes := []rune(surface)
	readingRunes := []rune(katakanaToHiragana(reading)) // Ensure reading is always hiragana
	logSteps := make([]map[string]interface{}, 0)
	logPath := "logs"
	logID := "furigana_analysis"

	var recur func(j, k int) ([][2]string, bool)
	recur = func(j, k int) ([][2]string, bool) {
		if j >= len(surfaceRunes) {
			if k == len(readingRunes) {
				return make([][2]string, 0), true
			}
			return nil, false
		}
		s := surfaceRunes[j]
		step := map[string]interface{}{
			"kanji":             string(s),
			"reading_pos":       k,
			"remaining_reading": string(readingRunes[k:]),
			"candidates":        []string{},
			"chosen":            "",
			"reason":            "",
		}
		if isKanji(s) {
			kanjiReadings := kanji.GetKanjiReadings(s)
			step["candidates"] = kanjiReadings
			_ = logger.LogJSON(logPath, logID, map[string]interface{}{
				"event":             "kanji_candidates",
				"surface":           surface,
				"reading":           reading,
				"kanji":             string(s),
				"candidates":        kanjiReadings,
				"remaining_reading": string(readingRunes[k:]),
			})
			matched := false
			for _, kr := range kanjiReadings {
				krH := katakanaToHiragana(kr) // Ensure candidate is hiragana
				krRunes := []rune(krH)
				_ = logger.LogJSON(logPath, logID, map[string]interface{}{
					"event":             "try_candidate",
					"surface":           surface,
					"reading":           reading,
					"kanji":             string(s),
					"candidate":         krH,
					"reading_substring": string(readingRunes[k : k+len(krRunes)]),
				})
				if k+len(krRunes) <= len(readingRunes) && string(readingRunes[k:k+len(krRunes)]) == krH {
					rest, ok := recur(j+1, k+len(krRunes))
					if ok {
						step["chosen"] = krH
						step["reason"] = "substring match"
						logSteps = append(logSteps, step)
						_ = logger.LogJSON(logPath, logID, map[string]interface{}{
							"event":   "candidate_chosen",
							"surface": surface,
							"reading": reading,
							"kanji":   string(s),
							"chosen":  krH,
						})
						return append([][2]string{{string(s), krH}}, rest...), true
					}
				}
			}
			if !matched {
				step["chosen"] = ""
				if len(kanjiReadings) == 0 {
					step["reason"] = "no readings found for kanji"
				} else {
					step["reason"] = "no candidate matched reading substring"
				}
				logSteps = append(logSteps, step)
				_ = logger.LogJSON(logPath, logID, map[string]interface{}{
					"event":   "no_candidate_match",
					"surface": surface,
					"reading": reading,
					"kanji":   string(s),
				})
				return append([][2]string{{string(s), ""}}, nil...), false
			}
		} else if isKana(s) {
			if k < len(readingRunes) && readingRunes[k] == s {
				step["chosen"] = string(s)
				step["reason"] = "kana matches reading"
				logSteps = append(logSteps, step)
				rest, ok := recur(j+1, k+1)
				if ok {
					return append([][2]string{{string(s), string(s)}}, rest...), true
				}
			} else {
				step["chosen"] = ""
				step["reason"] = "kana does not match reading"
				logSteps = append(logSteps, step)
				rest, ok := recur(j+1, k)
				if ok {
					return append([][2]string{{string(s), ""}}, rest...), true
				}
			}
			return nil, false
		} else {
			step["chosen"] = ""
			step["reason"] = "non-kanji/kana character"
			logSteps = append(logSteps, step)
			rest, ok := recur(j+1, k)
			if ok {
				return append([][2]string{{string(s), ""}}, rest...), true
			}
			return nil, false
		}
		// Ensure all code paths return
		return nil, false
	}

	pairs, ok := recur(0, 0)
	if !ok {
		_ = logger.LogJSON(logPath, logID, map[string]interface{}{
			"event":   "fallback_greedy_alignment",
			"surface": surface,
			"reading": reading,
		})
		// fallback to greedy
		fallbackSteps := map[string]interface{}{
			"reason":  "fallback to greedy alignment",
			"surface": surface,
			"reading": reading,
		}
		logSteps = append(logSteps, fallbackSteps)
		pairs = make([][2]string, 0)
		surfaceRunes := []rune(surface)
		readingRunes := []rune(katakanaToHiragana(reading))
		j, k := 0, 0
		for j < len(surfaceRunes) {
			s := surfaceRunes[j]
			step := map[string]interface{}{
				"kanji":             string(s),
				"reading_pos":       k,
				"remaining_reading": string(readingRunes[k:]),
				"chosen":            "",
				"reason":            "",
			}
			if isKanji(s) {
				kanjiReadings := kanji.GetKanjiReadings(s)
				longestMatch := ""
				for _, kr := range kanjiReadings {
					krH := katakanaToHiragana(kr)
					krRunes := []rune(krH)
					if k+len(krRunes) <= len(readingRunes) && string(readingRunes[k:k+len(krRunes)]) == krH {
						if len(krH) > len(longestMatch) {
							longestMatch = krH
						}
					}
				}
				if longestMatch != "" {
					step["chosen"] = longestMatch
					step["reason"] = "greedy longest match"
					pairs = append(pairs, [2]string{string(s), longestMatch})
					k += len([]rune(longestMatch))
				} else {
					step["chosen"] = ""
					step["reason"] = "no match in greedy"
					pairs = append(pairs, [2]string{string(s), ""})
				}
				logSteps = append(logSteps, step)
				j++
			} else if isKana(s) {
				if k < len(readingRunes) && readingRunes[k] == s {
					step["chosen"] = string(s)
					step["reason"] = "kana matches reading"
					pairs = append(pairs, [2]string{string(s), string(s)})
					k++
				} else {
					step["chosen"] = ""
					step["reason"] = "kana does not match reading"
					pairs = append(pairs, [2]string{string(s), ""})
				}
				logSteps = append(logSteps, step)
				j++
			} else {
				step["chosen"] = ""
				step["reason"] = "non-kanji/kana character"
				pairs = append(pairs, [2]string{string(s), ""})
				logSteps = append(logSteps, step)
				j++
			}
		}
	}
	// Log alignment for debugging
	logFuriganaAlignment(surface, reading, logSteps, pairs)
	return pairs, logSteps
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
