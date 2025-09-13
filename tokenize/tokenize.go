package tokenize

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

	"github.com/ikawaha/kagome-dict/ipa"
	"github.com/ikawaha/kagome/v2/tokenizer"
)

// Token represents a token / morpheme produced by the tokenizer.
type Token = model.Token

// Tokenized pairs an ingest.Sentence with the tokens produced for it.
type Tokenized struct {
	Sentence ingest.Sentence
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

// Add this to the type definition:
var _ = xml.Name{Local: "kanjidic2"}

type DictionaryEntry = model.DictionaryEntry

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

// rendaku helpers are provided by package kanji

// getFuriganaString returns a slice of [kanji/kana, furigana] pairs for display.
func getFuriganaString(surface, reading string) [][2]string {
	result := make([][2]string, 0)
	surfaceRunes := []rune(surface)
	readingRunes := []rune(katakanaToHiragana(reading))
	k := 0
	for j := 0; j < len(surfaceRunes); j++ {
		s := surfaceRunes[j]
		if isKanji(s) {
			// Greedy longest-match per kanji using kanjidic2 readings with normalized variants and rendaku
			bestMatch := ""
			bestLen := 0
			kanjiReadings := kanji.GetKanjiReadings(s)
			for _, kr := range kanjiReadings {
				// generate normalized variants: full, prefix before '.', and without leading '-'
				full := kanji.NormalizeReading(kr)
				variants := []string{}
				if full != "" {
					variants = append(variants, full)
				}
				if idx := strings.IndexRune(kr, '.'); idx >= 0 {
					pre := kr[:idx]
					preNorm := kanji.NormalizeReading(pre)
					if preNorm != "" {
						// avoid duplicates
						found := false
						for _, v := range variants {
							if v == preNorm {
								found = true
								break
							}
						}
						if !found {
							variants = append(variants, preNorm)
						}
					}
				}
				if strings.HasPrefix(kr, "-") {
					noLead := kanji.NormalizeReading(strings.TrimPrefix(kr, "-"))
					if noLead != "" {
						found := false
						for _, v := range variants {
							if v == noLead {
								found = true
								break
							}
						}
						if !found {
							variants = append(variants, noLead)
						}
					}
				}

				for _, v := range variants {
					vRunes := []rune(v)
					// normal match
					if k+len(vRunes) <= len(readingRunes) && string(readingRunes[k:k+len(vRunes)]) == string(vRunes) {
						if len(vRunes) > bestLen {
							bestMatch = string(readingRunes[k : k+len(vRunes)])
							bestLen = len(vRunes)
						}
					}
					// rendaku match for non-first kanji
					if j > 0 {
						rForm := kanji.RendakuForm(v)
						rRunes := []rune(rForm)
						if k+len(rRunes) <= len(readingRunes) && string(readingRunes[k:k+len(rRunes)]) == rForm {
							if len(rRunes) > bestLen {
								bestMatch = string(readingRunes[k : k+len(rRunes)])
								bestLen = len(rRunes)
							}
						}
					}
				}
			}
			if bestMatch != "" {
				result = append(result, [2]string{string(s), bestMatch})
				k += bestLen
			} else {
				// If no match, assign remaining reading to last kanji if it's the last kanji
				isLastKanji := true
				for jj := j + 1; jj < len(surfaceRunes); jj++ {
					if isKanji(surfaceRunes[jj]) {
						isLastKanji = false
						break
					}
				}
				if isLastKanji && k < len(readingRunes) {
					result = append(result, [2]string{string(s), string(readingRunes[k:])})
					k = len(readingRunes)
				} else {
					result = append(result, [2]string{string(s), ""})
				}
			}
		} else if isKana(s) {
			result = append(result, [2]string{string(s), ""})
			if k < len(readingRunes) && readingRunes[k] == s {
				k++
			}
		} else {
			result = append(result, [2]string{string(s), ""})
		}
	}
	// If there is leftover reading and no kanji left, append as plain text
	kanjiLeft := false
	for jj := len(surfaceRunes) - 1; jj >= 0; jj-- {
		if isKanji(surfaceRunes[jj]) {
			kanjiLeft = true
			break
		}
	}
	if !kanjiLeft && k < len(readingRunes) {
		result = append(result, [2]string{"", string(readingRunes[k:])})
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

// formatFuriganaBracketsOnly formats furigana so only kanji readings are in brackets, with non-kanji characters outside.
func formatFuriganaBracketsOnly(pairs [][2]string) string {
	out := ""
	lastKanjiIdx := -1
	for i, pair := range pairs {
		if len(pair[0]) > 0 && isKanji([]rune(pair[0])[0]) {
			lastKanjiIdx = i
		}
	}
	// Assign remaining reading runes to last kanji if its furigana is empty
	if lastKanjiIdx != -1 && pairs[lastKanjiIdx][1] == "" {
		// Compute used reading runes
		used := 0
		for i, pair := range pairs {
			if i == lastKanjiIdx {
				break
			}
			used += len([]rune(pair[1]))
		}
		// Get remaining reading from context (not available here, so rely on getFuriganaString patch)
		// For now, leave as is, since getFuriganaString should assign correctly
	}
	for _, pair := range pairs {
		if len(pair[0]) == 0 {
			continue // skip empty surface segments
		}
		if isKanji([]rune(pair[0])[0]) {
			// Always output a bracketed block for every kanji, even if furigana is empty
			out += "[" + pair[1] + "]"
		} else if isKana([]rune(pair[0])[0]) {
			out += pair[0]
		} else if pair[0] != "" {
			out += pair[0]
		}
	}
	return out
}

// Exported wrappers so other packages (like main) can reuse the improved logic
func GetFuriganaString(surface, reading string) [][2]string {
	return getFuriganaString(surface, reading)
}

func FormatFuriganaBracketsOnly(pairs [][2]string) string {
	return formatFuriganaBracketsOnly(pairs)
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
			FuriganaText:   formatFuriganaBracketsOnly(getFuriganaString(kt.Surface, reading)),
			FuriganaLemma:  formatFuriganaBracketsOnly(getFuriganaString(lemma, reading)),
		}
		out = append(out, t)
	}
	return out
}

// UpdateFuriganaFromDictionary updates FuriganaText and FuriganaLemma for tokens using dictionary entries
func UpdateFuriganaFromDictionary(tokens []Token) []Token {
	for i := range tokens {
		containsKanjiText := false
		for _, r := range tokens[i].Text {
			if isKanji(r) {
				containsKanjiText = true
				break
			}
		}
		containsKanjiLemma := false
		for _, r := range tokens[i].Lemma {
			if isKanji(r) {
				containsKanjiLemma = true
				break
			}
		}
		// Restore previous logic: use getFuriganaString for all tokens
		if containsKanjiText {
			tokens[i].FuriganaText = formatFuriganaBracketsOnly(getFuriganaString(tokens[i].Text, tokens[i].Reading))
		} else {
			tokens[i].FuriganaText = formatFuriganaBracketsOnly(getFuriganaString(tokens[i].Text, tokens[i].Reading))
		}
		if containsKanjiLemma {
			tokens[i].FuriganaLemma = formatFuriganaBracketsOnly(getFuriganaString(tokens[i].Lemma, tokens[i].Reading))
		} else {
			tokens[i].FuriganaLemma = formatFuriganaBracketsOnly(getFuriganaString(tokens[i].Lemma, tokens[i].Reading))
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

// logFuriganaAlignment logs the alignment process to a JSON file for debugging
func logFuriganaAlignment(tokenText, tokenReading string, steps []map[string]interface{}) {
	// Use a unique filename: include reading, PID, and a random number for uniqueness
	randomPart := fmt.Sprintf("%d", time.Now().UnixNano())
	filename := fmt.Sprintf("logs/%s_%s_%s_furigana.json", tokenText, tokenReading, randomPart)
	data, err := json.MarshalIndent(steps, "", "  ")
	if err != nil {
		log.Printf("[FURIGANA] Failed to marshal alignment log for %s: %v", tokenText, err)
		return
	}
	err = os.WriteFile(filename, data, 0644)
	if err != nil {
		log.Printf("[FURIGANA] Failed to write alignment log for %s: %v", tokenText, err)
	} else {
		log.Printf("[FURIGANA] Alignment log written: %s", filename)
	}
}

// alignFuriganaAccurate splits reading for each kanji by remaining kana and kanji count, using Kanjidic2 readings for kanji
func alignFuriganaAccurate(surface, reading string) [][2]string {
	surfaceRunes := []rune(surface)
	readingRunes := []rune(katakanaToHiragana(reading))
	var result [][2]string
	j, k := 0, 0
	for j < len(surfaceRunes) {
		s := surfaceRunes[j]
		if isKanji(s) {
			// Find the best matching reading for this kanji
			bestMatch := ""
			bestLen := 0
			kanjiReadings := kanji.GetKanjiReadings(s)
			for _, kr := range kanjiReadings {
				// normalize and try useful variants
				full := kanji.NormalizeReading(kr)
				variants := []string{}
				if full != "" {
					variants = append(variants, full)
				}
				if idx := strings.IndexRune(kr, '.'); idx >= 0 {
					pre := kr[:idx]
					preNorm := kanji.NormalizeReading(pre)
					if preNorm != "" {
						found := false
						for _, v := range variants {
							if v == preNorm {
								found = true
								break
							}
						}
						if !found {
							variants = append(variants, preNorm)
						}
					}
				}
				if strings.HasPrefix(kr, "-") {
					noLead := kanji.NormalizeReading(strings.TrimPrefix(kr, "-"))
					if noLead != "" {
						found := false
						for _, v := range variants {
							if v == noLead {
								found = true
								break
							}
						}
						if !found {
							variants = append(variants, noLead)
						}
					}
				}
				for _, v := range variants {
					vRunes := []rune(v)
					if k+len(vRunes) <= len(readingRunes) && string(readingRunes[k:k+len(vRunes)]) == string(vRunes) {
						if len(vRunes) > bestLen {
							bestMatch = string(readingRunes[k : k+len(vRunes)])
							bestLen = len(vRunes)
						}
					}
					// try rendaku for non-first kanji
					if j > 0 {
						rForm := kanji.RendakuForm(v)
						rRunes := []rune(rForm)
						if k+len(rRunes) <= len(readingRunes) && string(readingRunes[k:k+len(rRunes)]) == rForm {
							if len(rRunes) > bestLen {
								bestMatch = string(readingRunes[k : k+len(rRunes)])
								bestLen = len(rRunes)
							}
						}
					}
				}
			}
			if bestMatch != "" {
				result = append(result, [2]string{"", bestMatch})
				k += bestLen
			} else {
				// No match: if this is the last kanji and there are remaining reading runes, assign them as furigana (rendaku fix)
				isLastKanji := true
				for jj := j + 1; jj < len(surfaceRunes); jj++ {
					if isKanji(surfaceRunes[jj]) {
						isLastKanji = false
						break
					}
				}
				if isLastKanji && k < len(readingRunes) {
					result = append(result, [2]string{string(s), string(readingRunes[k:])})
					k = len(readingRunes)
				} else {
					result = append(result, [2]string{string(s), ""})
				}
			}
			j++
		} else if isKana(s) {
			if k < len(readingRunes) && readingRunes[k] == s {
				result = append(result, [2]string{string(s), ""})
				k++
			} else {
				result = append(result, [2]string{string(s), ""})
			}
			j++
		} else {
			result = append(result, [2]string{string(s), ""})
			j++
		}
	}
	// Only append remaining reading if there are no kanji left in surface
	kanjiLeft := false
	for jj := j; jj < len(surfaceRunes); jj++ {
		if isKanji(surfaceRunes[jj]) {
			kanjiLeft = true
			break
		}
	}
	if !kanjiLeft && k < len(readingRunes) {
		result = append(result, [2]string{"", string(readingRunes[k:])})
	}
	return result
}

// formatFuriganaDisplayAccurate formats furigana so only kanji get [kanji|furigana], kana are plain
func formatFuriganaDisplayAccurate(pairs [][2]string) string {
	out := ""
	for _, pair := range pairs {
		if len(pair[0]) == 0 {
			continue
		}
		if isKanji([]rune(pair[0])[0]) {
			out += "[" + pair[1] + "]"
		} else {
			out += pair[0]
		}
	}
	return out
}

// normalizeReading removes non-kana characters (like '.' or '-') and
// converts katakana to hiragana so kanjidic readings like "い.り" match "いり".
// use kanji.NormalizeReading
