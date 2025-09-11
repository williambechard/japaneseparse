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
"logger/logger"
	"japaneseparse/ingest"
	"japaneseparse/kanji"
	"japaneseparse/model"
	"logger/logger"
	"github.com/ikawaha/kagome-dict/ipa"
	"github.com/ikawaha/kagome/v2/tokenizer"

"github.com/ikawaha/kagome-dict/ipa"
"github.com/ikawaha/kagome/v2/tokenizer"
)

// Helper: Generate rendaku (voiced) form for a hiragana string
var rendakuMap = map[rune]rune{
 'か': 'が', 'き': 'ぎ', 'く': 'ぐ', 'け': 'げ', 'こ': 'ご',
 'さ': 'ざ', 'し': 'じ', 'す': 'ず', 'せ': 'ぜ', 'そ': 'ぞ',
 'た': 'だ', 'ち': 'ぢ', 'つ': 'づ', 'て': 'で', 'と': 'ど',
 'は': 'ば', 'ひ': 'び', 'ふ': 'ぶ', 'へ': 'べ', 'ほ': 'ぼ',
}

func rendakuForm(s string) string {
 runes := []rune(s)
 if len(runes) == 0 {
  return s
 }
 if v, ok := rendakuMap[runes[0]]; ok {
  runes[0] = v
  return string(runes)
 }
 return s
}
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
			"japaneseparse/ingest"
			"japaneseparse/kanji"
			"japaneseparse/model"
			"logger/logger"
			"github.com/ikawaha/kagome-dict/ipa"
			"github.com/ikawaha/kagome/v2/tokenizer"
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
		// Force all kanji tokens to use formatFuriganaBracketsOnly(getFuriganaString(...)), matching demo logic
		if containsKanjiText {
			tokens[i].FuriganaText = formatFuriganaBracketsOnly(getFuriganaString(tokens[i].Text, tokens[i].Reading))
		} else {
			tokens[i].FuriganaText = tokens[i].Text
		}
		if containsKanjiLemma {
			tokens[i].FuriganaLemma = formatFuriganaBracketsOnly(getFuriganaString(tokens[i].Lemma, tokens[i].Reading))
		} else {
			tokens[i].FuriganaLemma = tokens[i].Lemma
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
					// Rendaku handling: Try voicing the first kana of any reading
					if j > 0 {
						// Always convert candidate to hiragana before rendaku
						krHiragana := katakanaToHiragana(kr)
						krRunesH := []rune(krHiragana)
						rendaku := rendakuForm(krHiragana)
						out := ""
						lastKanjiIdx := -1
						for i := len(pairs) - 1; i >= 0; i-- {
							if len(pairs[i][0]) > 0 && isKanji([]rune(pairs[i][0])[0]) {
								lastKanjiIdx = i
								break
							}
						}
						for i, pair := range pairs {
							if len(pair[0]) > 0 && isKanji([]rune(pair[0])[0]) {
								furigana := pair[1]
								if i == lastKanjiIdx && furigana == "" {
									// Assign remaining reading runes to last kanji
									used := 0
									for j, p := range pairs {
										if j == lastKanjiIdx {
											break
										}
										used += len([]rune(p[1]))
									}
									// Get the original reading from context (not available here, so rely on alignFuriganaAccurate patch)
									// This function assumes pairs already patched
								}
								out += "[" + furigana + "]"
							} else {
								out += pair[0]
							}
						}
						return out
								_ = logger.LogJSON(logPath, logID, map[string]interface{}{
									"event":   "candidate_chosen_rendaku",
									"surface": surface,
									"reading": reading,
									"kanji":   string(s),
									"chosen":  rendaku,
								})
								return append([][2]string{{string(s), rendaku}}, rest...), true
							}
						}
					}
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
	log.Printf("[FALLBACK-GREEDY] surface='%s', reading='%s', kanjiIndices=%v, segments=%v", surface, reading, kanjiIndices, segments)
	surfaceRunes := []rune(surface)
	readingRunes := []rune(katakanaToHiragana(reading)) // Ensure reading is always hiragana
	logSteps := make([]map[string]interface{}, 0)
	logPath := "logs"
	logID := "furigana_analysis"

 // Helper: Generate rendaku (voiced) form for a hiragana string
 rendakuMap := map[rune]rune{
	'か': 'が', 'き': 'ぎ', 'く': 'ぐ', 'け': 'げ', 'こ': 'ご',
	'さ': 'ざ', 'し': 'じ', 'す': 'ず', 'せ': 'ぜ', 'そ': 'ぞ',
	'た': 'だ', 'ち': 'ぢ', 'つ': 'づ', 'て': 'で', 'と': 'ど',
	'は': 'ば', 'ひ': 'び', 'ふ': 'ぶ', 'へ': 'べ', 'ほ': 'ぼ',
 }
 func rendakuForm(s string) string {
	runes := []rune(s)
	if len(runes) == 0 {
		return s
	}
	if v, ok := rendakuMap[runes[0]]; ok {
		runes[0] = v
		return string(runes)
	}
	return s
 }

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
				log.Printf("[KANJI-DEBUG] surface='%s', reading='%s', kanji='%s', candidates=%v, readingRunes=%v, k=%d", surface, reading, string(s), kanjiReadings, readingRunes, k)
				matched := false
				   for idx, kr := range kanjiReadings {
					// Always log candidate details
					krHiragana := katakanaToHiragana(kr)
					krRunesH := []rune(krHiragana)
					rendaku := rendakuForm(krHiragana)
					substr := ""
					if k+len(krRunesH) <= len(readingRunes) {
						substr = string(readingRunes[k:k+len(krRunesH)])
					}
					log.Printf("[RENDAKU-ATTEMPT] surface='%s', reading='%s', kanji='%s', candidate='%s', hiragana='%s', rendaku='%s', substring='%s'", surface, reading, string(s), kr, krHiragana, rendaku, substr)
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
				  _ = logger.LogJSON(logPath, logID, map[string.interface{}{
				   "event":   "candidate_chosen",
				   "surface": surface,
				   "reading": reading,
				   "kanji":   string(s),
				   "chosen":  krH,
				  })
				  return append([][2]string{{string(s), krH}}, rest...), true
				 }
				}
				// Always check rendaku for every candidate
				krHiragana := katakanaToHiragana(kr)
				krRunesH := []rune(krHiragana)
				rendaku := rendakuForm(krHiragana)
				substr := ""
				if k+len(krRunesH) <= len(readingRunes) {
					substr = string(readingRunes[k:k+len(krRunesH)])
				}
				log.Printf("[RENDAKU-ATTEMPT] surface='%s', reading='%s', kanji='%s', candidate='%s', hiragana='%s', rendaku='%s', substring='%s'", surface, reading, string(s), kr, krHiragana, rendaku, substr)
				if rendaku != krHiragana && substr == rendaku {
					log.Printf("[RENDAKU-MATCH] surface='%s', reading='%s', kanji='%s', rendaku='%s', reading_substring='%s'", surface, reading, string(s), rendaku, substr)
					// If all reading runes are consumed, always return success for the token
					if k+len(krRunesH) == len(readingRunes) {
						step["chosen"] = rendaku
						step["reason"] = "rendaku match (all reading consumed, always success)"
						logSteps = append(logSteps, step)
						matched = true
						_ = logger.LogJSON(logPath, logID, map[string.interface{}{
							"event":   "candidate_chosen_rendaku_final_always",
							"surface": surface,
							"reading": reading,
							"kanji":   string(s),
							"chosen":  rendaku,
						})
						return append([][2]string{{string(s), rendaku}}, nil...), true
					}
					rest, ok := recur(j+1, k+len(krRunesH))
					if ok {
						step["chosen"] = rendaku
						step["reason"] = "rendaku match (voiced first kana of reading)"
						logSteps = append(logSteps, step)
						matched = true
						_ = logger.LogJSON(logPath, logID, map[string.interface{}{
							"event":   "candidate_chosen_rendaku",
							"surface": surface,
							"reading": reading,
							"kanji":   string(s),
							"chosen":  rendaku,
						})
						return append([][2]string{{string(s), rendaku}}, rest...), true
					}
				}
	 // Rendaku handling: If not first kanji, try voiced form
	 if j > 0 && k+len(krRunes) <= len(readingRunes) {
		rendaku := rendakuForm(krH)
		if rendaku != krH && string(readingRunes[k:k+len(krRunes)]) == rendaku {
		 rest, ok := recur(j+1, k+len(krRunes))
		 if ok {
			step["chosen"] = rendaku
			step["reason"] = "rendaku match"
			logSteps = append(logSteps, step)
			_ = logger.LogJSON(logPath, logID, map[string]interface{}{
			 "event":   "candidate_chosen_rendaku",
			 "surface": surface,
			 "reading": reading,
			 "kanji":   string(s),
			 "chosen":  rendaku,
			})
			return append([][2]string{{string(s), rendaku}}, rest...), true
		 }
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
		// fallback to robust greedy substring matching
		fallbackSteps := map[string.interface{}{
			"reason":  "fallback to greedy alignment (substring match)",
			"surface": surface,
			"reading": reading,
		}
		logSteps = append(logSteps, fallbackSteps)
		// Force greedy split: assign reading to kanji only, split into N non-empty segments
		surfaceRunes := []rune(surface)
		readingRunes := []rune(katakanaToHiragana(reading))
		kanjiIndices := []int{}
		for i, s := range surfaceRunes {
			if isKanji(s) {
				kanjiIndices = append(kanjiIndices, i)
			}
		}
		nKanji := len(kanjiIndices)
		segments := make([]string, nKanji)
		pos := 0
		total := len(readingRunes)
		for i := 0; i < nKanji; i++ {
			minLen := total / nKanji
			extra := 0
			if i < total%nKanji {
				extra = 1
			}
			segLen := minLen + extra
			if pos+segLen > total {
				segLen = total - pos
			}
			segments[i] = string(readingRunes[pos : pos+segLen])
			pos += segLen
		}
		pairs = make([][2]string, len(surfaceRunes))
		ki := 0
		for i, s := range surfaceRunes {
			if isKanji(s) {
				pairs[i] = [2]string{string(s), segments[ki]}
				ki++
			} else if isKana(s) {
				pairs[i] = [2]string{string(s), string(s)}
			} else {
				pairs[i] = [2]string{string(s), ""}
			}
		}
	}
	// Patch: assign remaining reading runes to last kanji if its furigana is empty
	// Improved greedy: If all kanji have readings and the reading is a concatenation, split accordingly
	kanjiIndices := []int{}
	kanjiReadings := []string{}
	for idx, pair := range pairs {
		if len(pair[0]) > 0 && isKanji([]rune(pair[0])[0]) {
			kanjiIndices = append(kanjiIndices, idx)
			// Get all possible readings for this kanji
			readings := kanji.GetKanjiReadings([]rune(pair[0])[0])
			// Use the longest reading (greedy)
			longest := ""
			for _, r := range readings {
				rH := katakanaToHiragana(r)
				if len(rH) > len(longest) {
					longest = rH
				}
			}
			kanjiReadings = append(kanjiReadings, longest)
		}
	}
	// If the concatenation of kanjiReadings matches the reading, split accordingly
	joined := ""
	for _, r := range kanjiReadings {
		joined += r
	}
	readingH := katakanaToHiragana(reading)
	if len(kanjiIndices) > 1 && len(readingH) >= len(kanjiIndices) {
		// Greedily split readingH into N non-empty segments for N kanji
		segments := make([]string, len(kanjiIndices))
		pos := 0
		remaining := len([]rune(readingH))
		for i := 0; i < len(kanjiIndices); i++ {
			// Ensure at least 1 rune per segment, and distribute any remainder to earlier segments
			segLen := 1
			if i < remaining-len(kanjiIndices) {
				segLen += 1
			}
			if i < len(kanjiIndices)-1 {
				// Calculate max possible for this segment
				maxPossible := remaining - (len(kanjiIndices)-(i+1))
				if segLen < maxPossible {
					segLen = maxPossible
				}
			}
			if pos+segLen > len([]rune(readingH)) {
				segLen = len([]rune(readingH)) - pos
			}
			seg := string([]rune(readingH)[pos : pos+segLen])
			segments[i] = seg
			pos += segLen
			remaining = len([]rune(readingH)) - pos
		}
		for i, idx := range kanjiIndices {
			pairs[idx][1] = segments[i]
		}
	} else if len(pairs) > 0 {
		// Find last kanji index
		lastKanjiIdx := -1
		for i := len(pairs) - 1; i >= 0; i-- {
			if len(pairs[i][0]) > 0 && isKanji([]rune(pairs[i][0])[0]) {
				lastKanjiIdx = i
				break
			}
		}
		if lastKanjiIdx != -1 && pairs[lastKanjiIdx][1] == "" {
			// Compute remaining reading runes
			used := 0
			for i, pair := range pairs {
				if i == lastKanjiIdx {
					break
				}
				used += len([]rune(pair[1]))
			}
			remaining := string([]rune(katakanaToHiragana(reading))[used:])
			if remaining != "" {
				pairs[lastKanjiIdx][1] = remaining
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
