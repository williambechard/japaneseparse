package tokenize

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/ikawaha/kagome-dict/ipa"
	"github.com/ikawaha/kagome/v2/tokenizer"

	"github.com/williambechard/japaneseparse/ingest"
	"github.com/williambechard/japaneseparse/kanji"
	"github.com/williambechard/japaneseparse/logger"
	"github.com/williambechard/japaneseparse/model"
)

// Ensure Token is imported from the centralized model package
// import (
// 	"github.com/williambechard/japaneseparse/model"
// )

// Use the centralized Token struct
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

// InitKanjidic2 parses kanjidic2.xml and builds kanji→readings map
func InitKanjidic2(path string) error {
	var err error
	kanjiReadingMapOnce.Do(func() {
		kanjiReadingMap = make(map[rune][]string)
		var loadedKanji []string
		f, fileErr := os.Open(path)
		if fileErr != nil {
			logger.Logf("Failed to open kanjidic2.xml: %v", fileErr)
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
				logger.Logf("Failed to parse kanjidic2.xml: %v", tokenErr)
				return
			}
			switch se := tok.(type) {
			case xml.StartElement:
				if se.Name.Local == "character" {
					var k Kanjidic2Kanji
					if decodeErr := d.DecodeElement(&k, &se); decodeErr != nil {
						logger.Logf("Failed to decode character: %v", decodeErr)
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
						logger.Logf("Loaded readings for %c: %v", kanjiRune, readings)
					}
				}
			}
		}
		logger.Logf("First 10 kanji loaded: %v", loadedKanji)
		logger.Logf("Kanjidic2 loaded: %d kanji entries", len(kanjiReadingMap))
	})
	return err
}

// GetKanjiReadings returns readings for a kanji rune, with logging
func GetKanjiReadings(r rune) []string {
	if kanjiReadingMap == nil {
		logger.Logf("kanjiReadingMap is nil when looking up %c", r)
		return nil
	}
	readings := kanjiReadingMap[r]
	if readings == nil {
		logger.Logf("No readings found for kanji %c", r)
	} else {
		logger.Logf("Readings for kanji %c: %v", r, readings)
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
	readingHira := katakanaToHiragana(reading)
	readingRunes := []rune(readingHira)

	// Precompute the position of the last kanji and any trailing okurigana (kana suffix)
	lastKanjiIdx := -1
	for idx := len(surfaceRunes) - 1; idx >= 0; idx-- {
		if isKanji(surfaceRunes[idx]) {
			lastKanjiIdx = idx
			break
		}
	}
	trailingKanaSuffix := ""
	if lastKanjiIdx != -1 && lastKanjiIdx+1 < len(surfaceRunes) {
		// Collect trailing kana after the last kanji
		allKana := true
		for _, r := range surfaceRunes[lastKanjiIdx+1:] {
			if !isKana(r) {
				allKana = false
				break
			}
		}
		if allKana {
			trailingKanaSuffix = string(surfaceRunes[lastKanjiIdx+1:])
		}
	}
	// If the reading ends with the trailing kana suffix (normalized), we'll reserve it for the kana
	okuriLen := 0
	if trailingKanaSuffix != "" {
		if strings.HasSuffix(readingHira, trailingKanaSuffix) {
			okuriLen = len([]rune(trailingKanaSuffix))
		}
	}

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
					// normal match; if this is the last kanji, do not consume beyond reserved okurigana
					limit := len(readingRunes)
					if j == lastKanjiIdx && okuriLen > 0 {
						limit = len(readingRunes) - okuriLen
					}
					if k+len(vRunes) <= limit && string(readingRunes[k:k+len(vRunes)]) == string(vRunes) {
						if len(vRunes) > bestLen {
							bestMatch = string(readingRunes[k : k+len(vRunes)])
							bestLen = len(vRunes)
						}
					}
					// rendaku match for non-first kanji
					if j > 0 {
						rForm := kanji.RendakuForm(v)
						rRunes := []rune(rForm)
						limit := len(readingRunes)
						if j == lastKanjiIdx && okuriLen > 0 {
							limit = len(readingRunes) - okuriLen
						}
						if k+len(rRunes) <= limit && string(readingRunes[k:k+len(rRunes)]) == rForm {
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
					// Do not consume trailing okurigana from reading
					end := len(readingRunes)
					if okuriLen > 0 {
						end = end - okuriLen
						if end < k {
							end = k
						}
					}
					result = append(result, [2]string{string(s), string(readingRunes[k:end])})
					k = end
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

// translatePOSToEnglish converts MeCab Japanese POS tags to English equivalents
// Format: 品詞,品詞細分類1,品詞細分類2,品詞細分類3
func translatePOSToEnglish(posJapanese string) string {
	parts := strings.Split(posJapanese, ",")
	if len(parts) == 0 {
		return ""
	}

	// Map the main POS (first element)
	mainPOS := parts[0]
	var result []string

	switch mainPOS {
	case "名詞":
		result = append(result, "noun")
		if len(parts) > 1 {
			switch parts[1] {
			case "一般":
				result = append(result, "common")
			case "代名詞":
				result = append(result, "pronoun")
			case "固有名詞":
				result = append(result, "proper noun")
			case "サ変接続":
				result = append(result, "verbal noun")
			case "形容動詞語幹":
				result = append(result, "adjectival noun stem")
			case "副詞可能":
				result = append(result, "adverbial")
			case "接尾":
				result = append(result, "suffix")
			case "接続詞的":
				result = append(result, "conjunctive")
			case "数":
				result = append(result, "number")
			case "非自立":
				result = append(result, "dependent")
			case "特殊":
				result = append(result, "special")
			default:
				if parts[1] != "*" {
					result = append(result, parts[1])
				}
			}
		}
	case "動詞":
		result = append(result, "verb")
		if len(parts) > 1 {
			switch parts[1] {
			case "自立":
				result = append(result, "independent")
			case "非自立":
				result = append(result, "dependent")
			case "接尾":
				result = append(result, "suffix")
			default:
				if parts[1] != "*" {
					result = append(result, parts[1])
				}
			}
		}
	case "形容詞":
		result = append(result, "adjective")
		if len(parts) > 1 && parts[1] != "*" {
			result = append(result, parts[1])
		}
	case "副詞":
		result = append(result, "adverb")
		if len(parts) > 1 {
			switch parts[1] {
			case "一般":
				result = append(result, "general")
			case "助詞類接続":
				result = append(result, "particle-conjunctive")
			default:
				if parts[1] != "*" {
					result = append(result, parts[1])
				}
			}
		}
	case "助詞":
		result = append(result, "particle")
		if len(parts) > 1 {
			switch parts[1] {
			case "格助詞":
				result = append(result, "case marker")
			case "係助詞":
				result = append(result, "binding particle")
			case "接続助詞":
				result = append(result, "conjunctive particle")
			case "副助詞":
				result = append(result, "adverbial particle")
			case "終助詞":
				result = append(result, "sentence-ending particle")
			case "連体化":
				result = append(result, "nominalizer")
			case "副詞化":
				result = append(result, "adverbializer")
			case "並立助詞":
				result = append(result, "parallel marker")
			case "特殊":
				result = append(result, "special")
			default:
				if parts[1] != "*" {
					result = append(result, parts[1])
				}
			}
		}
	case "助動詞":
		result = append(result, "auxiliary verb")
	case "連体詞":
		result = append(result, "adnominal")
	case "接続詞":
		result = append(result, "conjunction")
	case "感動詞":
		result = append(result, "interjection")
	case "接頭詞":
		result = append(result, "prefix")
	case "記号":
		result = append(result, "symbol")
		if len(parts) > 1 {
			switch parts[1] {
			case "句点":
				result = append(result, "period")
			case "読点":
				result = append(result, "comma")
			case "括弧開":
				result = append(result, "open bracket")
			case "括弧閉":
				result = append(result, "close bracket")
			case "空白":
				result = append(result, "whitespace")
			case "一般":
				result = append(result, "general")
			default:
				if parts[1] != "*" {
					result = append(result, parts[1])
				}
			}
		}
	case "フィラー":
		result = append(result, "filler")
	case "その他":
		result = append(result, "other")
		if len(parts) > 1 {
			switch parts[1] {
			case "間投":
				result = append(result, "interjection")
			default:
				if parts[1] != "*" {
					result = append(result, parts[1])
				}
			}
		}
	default:
		result = append(result, mainPOS)
	}

	// Add specific details from remaining parts if meaningful
	if len(parts) > 2 && parts[2] != "*" {
		switch parts[2] {
		case "一般":
			result = append(result, "general")
		case "サ変":
			result = append(result, "suru-verb")
		default:
			// Only add if not redundant
			add := true
			for _, existing := range result {
				if existing == parts[2] {
					add = false
					break
				}
			}
			if add {
				result = append(result, parts[2])
			}
		}
	}

	return strings.Join(result, ", ")
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
// rebalanceFuriganaPairs tries to redistribute reading across consecutive kanji
// when the entire reading was attached to the last kanji (e.g., 学校 => [] [がっこう]).
// It works right-to-left, matching each kanji's known readings against the suffix
// of the remaining reading and assigns the longest match; any leftover is assigned
// to the first kanji in the cluster. This helps with sokuon/assimilation cases
// like がっこう where small っ isn't in the surface form.
func rebalanceFuriganaPairs(pairs [][2]string) [][2]string {
	i := 0
	for i < len(pairs) {
		// Skip non-kanji entries
		if !(len(pairs[i][0]) > 0 && isKanji([]rune(pairs[i][0])[0])) {
			i++
			continue
		}
		// Identify a consecutive-kanji cluster [start..end]
		start := i
		end := i
		for end+1 < len(pairs) {
			if len(pairs[end+1][0]) > 0 && isKanji([]rune(pairs[end+1][0])[0]) {
				end++
			} else {
				break
			}
		}
		if start < end { // multi-kanji cluster
			// If ANY kanji in the cluster have empty readings, redistribute
			anyEmpty := false
			for j := start; j <= end; j++ {
				if pairs[j][1] == "" {
					anyEmpty = true
					break
				}
			}
			if anyEmpty {
				// Collect reading ONLY from kanji with empty readings + their neighbors
				// Find the first empty kanji position
				firstEmpty := -1
				for j := start; j <= end; j++ {
					if pairs[j][1] == "" {
						firstEmpty = j
						break
					}
				}
				if firstEmpty == -1 {
					// No empty, skip
					i = end + 1
					continue
				}
				// Redistribute from firstEmpty to end using right-to-left matching
				// Collect all reading from firstEmpty onwards
				allReading := ""
				for j := firstEmpty; j <= end; j++ {
					allReading += pairs[j][1]
				}
				remaining := []rune(allReading)
				// Redistribute ONLY from firstEmpty to end, preserving earlier matches
				for j := end; j >= firstEmpty; j-- {
					sRunes := []rune(pairs[j][0])
					if len(sRunes) == 0 {
						continue
					}
					s := sRunes[0]
					// Build normalized reading variants for this kanji
					variants := []string{}
					for _, kr := range kanji.GetKanjiReadings(s) {
						full := kanji.NormalizeReading(kr)
						if full != "" {
							variants = append(variants, full)
						}
						if idx := strings.IndexRune(kr, '.'); idx >= 0 {
							pre := kanji.NormalizeReading(kr[:idx])
							if pre != "" {
								found := false
								for _, v := range variants {
									if v == pre {
										found = true
										break
									}
								}
								if !found {
									variants = append(variants, pre)
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
					}
					// Find the longest variant that matches the suffix of remaining
					best := ""
					remStr := string(remaining)
					for _, v := range variants {
						if strings.HasSuffix(remStr, v) {
							if len([]rune(v)) > len([]rune(best)) {
								best = v
							}
						}
					}
					if best == "" {
						if j == firstEmpty {
							// Assign any leftover to the first empty kanji in the sub-cluster
							pairs[j][1] = remStr
							remaining = []rune{}
						} else {
							pairs[j][1] = ""
						}
					} else {
						pairs[j][1] = best
						// Trim matched suffix
						rl := len([]rune(best))
						if rl <= len(remaining) {
							remaining = remaining[:len(remaining)-rl]
						} else {
							remaining = []rune{}
						}
					}
				}
			}
		}
		i = end + 1
	}
	return pairs
}

func formatFuriganaBracketsOnly(pairs [][2]string) string {
	// Attempt to rebalance readings across consecutive kanji if needed
	pairs = rebalanceFuriganaPairs(pairs)
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
func getFuriganaFromDictionary(surface string, entry model.DictionaryEntry) string {
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
			POSEnglish:     translatePOSToEnglish(pos),
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
		// Assign roles based on POS or other attributes
		if strings.Contains(pos, "名詞") && strings.Contains(pos, "代名詞") {
			t.Role = "subject"
		} else if strings.Contains(pos, "名詞") && strings.Contains(pos, "一般") {
			t.Role = "object"
		} else if strings.Contains(pos, "動詞") {
			t.Role = "verb"
		}
		// Debugging: Log token details and assigned roles
		logger.Logf("DEBUG: Token details: %+v", t)
		logger.Logf("DEBUG: Assigned role: %s", t.Role)
		// Debugging: Log decision-making for role assignment
		logger.Logf("DEBUG: Evaluating token for role assignment: Text=%s, POS=%s", t.Text, t.POS)
		if strings.Contains(pos, "名詞") && strings.Contains(pos, "代名詞") {
			t.Role = "subject"
			logger.Logf("DEBUG: Assigned role 'subject' to token: %s", t.Text)
		} else if strings.Contains(pos, "名詞") && strings.Contains(pos, "一般") {
			t.Role = "object"
			logger.Logf("DEBUG: Assigned role 'object' to token: %s", t.Text)
		} else if strings.Contains(pos, "動詞") {
			t.Role = "verb"
			logger.Logf("DEBUG: Assigned role 'verb' to token: %s", t.Text)
		} else {
			logger.Logf("DEBUG: No role assigned to token: %s", t.Text)
		}
		// Debugging: Log tokens that fail role assignment
		if t.Role == "" {
			logger.Logf("DEBUG: Token did not receive a role: Text=%s, POS=%s, Lemma=%s", t.Text, t.POS, t.Lemma)
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
		// Use tokenizer reading for surface text by default
		if containsKanjiText {
			tokens[i].FuriganaText = formatFuriganaBracketsOnly(getFuriganaString(tokens[i].Text, tokens[i].Reading))
		} else {
			tokens[i].FuriganaText = formatFuriganaBracketsOnly(getFuriganaString(tokens[i].Text, tokens[i].Reading))
		}
		// For lemma, prefer dictionary reading if available (e.g., 行く -> いく)
		lemmaReading := tokens[i].Reading
		if len(tokens[i].DictionaryEntry.Readings) > 0 {
			// pick the first dictionary reading that best matches lemma's okurigana suffix
			lemmaOkuri := ""
			lemmaRunes := []rune(tokens[i].Lemma)
			lastKanji := -1
			for idx := len(lemmaRunes) - 1; idx >= 0; idx-- {
				if isKanji(lemmaRunes[idx]) {
					lastKanji = idx
					break
				}
			}
			if lastKanji != -1 && lastKanji+1 < len(lemmaRunes) {
				allKana := true
				for _, r := range lemmaRunes[lastKanji+1:] {
					if !isKana(r) {
						allKana = false
						break
					}
				}
				if allKana {
					lemmaOkuri = string(lemmaRunes[lastKanji+1:])
				}
			}
			chosen := ""
			for _, rd := range tokens[i].DictionaryEntry.Readings {
				rh := katakanaToHiragana(rd)
				if lemmaOkuri == "" || strings.HasSuffix(rh, lemmaOkuri) {
					chosen = rh
					break
				}
			}
			if chosen == "" {
				chosen = katakanaToHiragana(tokens[i].DictionaryEntry.Readings[0])
			}
			lemmaReading = chosen
		}
		if containsKanjiLemma {
			tokens[i].FuriganaLemma = formatFuriganaBracketsOnly(getFuriganaString(tokens[i].Lemma, lemmaReading))
		} else {
			tokens[i].FuriganaLemma = formatFuriganaBracketsOnly(getFuriganaString(tokens[i].Lemma, lemmaReading))
		}

		// Post-process lemma furigana for lemmas with okurigana: if the lemma has
		// trailing kana (okurigana) after the last kanji, prefer to reuse the
		// bracketed kanji-reading groups from the token's FuriganaText and append
		// the lemma's okurigana. This avoids incorrect consumption of kana from
		// merged readings (e.g., 行きます -> Text: [い]きます, Lemma: [い]く).
		lemmaRunes := []rune(tokens[i].Lemma)
		lastKanji := -1
		for idx := len(lemmaRunes) - 1; idx >= 0; idx-- {
			if isKanji(lemmaRunes[idx]) {
				lastKanji = idx
				break
			}
		}
		if lastKanji != -1 && lastKanji+1 < len(lemmaRunes) {
			// collect okurigana
			allKana := true
			for _, r := range lemmaRunes[lastKanji+1:] {
				if !isKana(r) {
					allKana = false
					break
				}
			}
			if allKana {
				okuri := string(lemmaRunes[lastKanji+1:])
				// extract bracket groups from FuriganaText
				bt := tokens[i].FuriganaText
				groups := []string{}
				pos := 0
				for {
					l := strings.Index(bt[pos:], "[")
					if l == -1 {
						break
					}
					r := strings.Index(bt[pos+l:], "]")
					if r == -1 {
						break
					}
					groups = append(groups, bt[pos+l:pos+l+r+1])
					pos = pos + l + r + 1
				}
				// count kanji in lemma
				kanjiCount := 0
				for _, rr := range lemmaRunes {
					if isKanji(rr) {
						kanjiCount++
					}
				}
				if kanjiCount > 0 && len(groups) >= kanjiCount {
					newLemmaF := ""
					for gi := 0; gi < kanjiCount; gi++ {
						newLemmaF += groups[gi]
					}
					newLemmaF += okuri
					tokens[i].FuriganaLemma = newLemmaF
				}
			}
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
		logger.Logf("[StartTokenizer] Goroutine started, waiting for sentences...")
		for {
			select {
			case <-ctx.Done():
				logger.Logf("[StartTokenizer] Context done, exiting goroutine.")
				return
			case s := <-ingest.IngestChan:
				logger.Logf("[StartTokenizer] Received sentence: ID=%s, Text=%s", s.ID, s.Text)
				toks, err := Tokenize(ctx, s.Text)
				if err != nil {
					logger.Logf("[StartTokenizer] Tokenize error: %v", err)
					continue
				}
				logger.Logf("[StartTokenizer] Tokenized %d tokens for sentence ID=%s", len(toks), s.ID)
				select {
				case <-ctx.Done():
					logger.Logf("[StartTokenizer] Context done after tokenization, exiting goroutine.")
					return
				case TokenizedChan <- Tokenized{Sentence: s, Tokens: toks}:
					logger.Logf("[StartTokenizer] Published tokenized result for sentence ID=%s", s.ID)
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
		logger.Logf("[FURIGANA] Failed to marshal alignment log for %s: %v", tokenText, err)
		return
	}
	err = os.WriteFile(filename, data, 0644)
	if err != nil {
		logger.Logf("[FURIGANA] Failed to write alignment log for %s: %v", tokenText, err)
	} else {
		logger.Logf("[FURIGANA] Alignment log written: %s", filename)
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
