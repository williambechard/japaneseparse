package dictionary

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"sync"
	"unicode"

	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/transform"

	"japaneseparse/logger"
	"japaneseparse/model"
	"japaneseparse/tokenize"
)

var logEnabled = os.Getenv("JAPARSE_LOG") != "0" && os.Getenv("JAPARSE_LOG") != "false"

func logf(format string, v ...interface{}) {
	if logEnabled {
		logger.Logf(format, v...)
	}
}

// Global in-memory maps for fast dictionary lookup
var jmDictMap map[string][]model.DictionaryEntry
var enamDictMap map[string]string
var internedStrings sync.Map

// intern returns a single instance of each unique string
func intern(s string) string {
	if s == "" {
		return s
	}
	if v, ok := internedStrings.Load(s); ok {
		return v.(string)
	}
	internedStrings.Store(s, s)
	return s
}

func internSlice(ss []string) []string {
	out := make([]string, len(ss))
	for i, s := range ss {
		out[i] = intern(s)
	}
	return out
}

// isKatakana returns true if the string is all katakana (ignoring non-letters)
func isKatakana(s string) bool {
	for _, r := range s {
		if r >= 0x30A1 && r <= 0x30F6 {
			continue
		}
		if unicode.IsSpace(r) || unicode.IsPunct(r) || unicode.IsSymbol(r) {
			continue
		}
		return false
	}
	return len(s) > 0
}

// normalizeReadings returns readings in hiragana unless the original is all katakana
func normalizeReadings(readings []string) []string {
	out := make([]string, len(readings))
	for i, r := range readings {
		if isKatakana(r) {
			out[i] = r
		} else {
			out[i] = katakanaToHiragana(r)
		}
	}
	return out
}

// katakanaToHiragana converts katakana to hiragana for reading normalization
func katakanaToHiragana(s string) string {
	runes := []rune(s)
	for i, r := range runes {
		if r >= 0x30A1 && r <= 0x30F6 {
			runes[i] = r - 0x60
		}
	}
	return string(runes)
}

var (
	jmPath   = "dict/JMdict_e"
	enamPath = "dict/enamdict"
)

func InitDictionaries(jmdictPath, enamdictPath string) error {
	// Load JMdict into memory map
	jmDictMap = make(map[string][]model.DictionaryEntry)
	f, err := os.Open(jmPath)
	if err != nil {
		return fmt.Errorf("JMdict preload failed: %w", err)
	}
	defer f.Close()
	r := bufio.NewReader(f)
	// var buf strings.Builder // removed unused variable
	for {
		line, err := r.ReadString('\n')
		if err != nil && err != io.EOF {
			return fmt.Errorf("JMdict read error: %w", err)
		}
		if strings.Contains(line, "<entry") {
			// Very simple parse: extract keb/reb and glosses
			var ks, rs, glosses []string
			for {
				i := strings.Index(line, "<keb>")
				if i < 0 {
					break
				}
				i += len("<keb>")
				j := strings.Index(line[i:], "</keb>")
				if j < 0 {
					break
				}
				ks = append(ks, intern(strings.TrimSpace(line[i:i+j])))
				line = line[i+j+len("</keb>"):]
			}
			for {
				i := strings.Index(line, "<reb>")
				if i < 0 {
					break
				}
				i += len("<reb>")
				j := strings.Index(line[i:], "</reb>")
				if j < 0 {
					break
				}
				rs = append(rs, intern(strings.TrimSpace(line[i:i+j])))
				line = line[i+j+len("</reb>"):]
			}
			for {
				i := strings.Index(line, "<gloss>")
				if i < 0 {
					break
				}
				i += len("<gloss>")
				j := strings.Index(line[i:], "</gloss>")
				if j < 0 {
					break
				}
				glosses = append(glosses, intern(strings.TrimSpace(line[i:i+j])))
				line = line[i+j+len("</gloss>"):]
			}
			entry := model.DictionaryEntry{
				Source:   "JMdict",
				Kanji:    internSlice(ks),
				Readings: normalizeReadings(internSlice(rs)),
				Glosses:  internSlice(glosses),
			}
			for _, k := range ks {
				jmDictMap[k] = append(jmDictMap[k], entry)
			}
			for _, r := range rs {
				jmDictMap[r] = append(jmDictMap[r], entry)
			}
		}
		if err == io.EOF {
			break
		}
	}

	// Load ENAMDICT into memory map (key: kanji, value: reading/gloss line)
	enamDictMap = make(map[string]string)
	f2, err := os.Open(enamPath)
	if err == nil {
		defer f2.Close()
		r2 := bufio.NewReader(f2)
		for {
			line, err := r2.ReadString('\n')
			if err != nil && err != io.EOF {
				break
			}
			fields := strings.Fields(line)
			if len(fields) > 0 {
				enamDictMap[fields[0]] = line
			}
			if err == io.EOF {
				break
			}
		}
	}
	if jmdictPath != "" {
		jmPath = jmdictPath
	}
	if enamdictPath != "" {
		enamPath = enamdictPath
	}
	// No heavy preloading here; we read on-demand to keep memory usage low.
	// Validate paths exist
	if _, err := os.Stat(jmPath); err != nil {
		return fmt.Errorf("JMdict path not accessible: %w", err)
	}
	if _, err := os.Stat(enamPath); err != nil {
		// ENAMDICT may be optional; just warn by returning nil and let lookups fail gracefully
		// but still return nil so caller can continue
		return nil
	}
	return nil
}

func DebugGlossaryFields() {
	// placeholder for future debug output
}

// scanLinesForLemma reads scanner lines and returns up to max that contain lemma
func scanLinesForLemma(sc *bufio.Scanner, lemma string, max int) []string {
	var hits []string
	for sc.Scan() {
		line := sc.Text()

		if strings.Contains(line, lemma) {
			line = strings.TrimFunc(line, func(r rune) bool { return !unicode.IsPrint(r) })
			hits = append(hits, line)
			if len(hits) >= max {
				break
			}
		}
	}
	return hits
}

// Refine the filtering logic to ensure exact matches are prioritized
func LookupEnamdictMostLikely(path, lemma string, max int) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	logf("DEBUG: Starting EUC-JP scan for lemma '%s' in ENAMDICT.", lemma)
	tr := transform.NewReader(f, japanese.EUCJP.NewDecoder())
	sc := bufio.NewScanner(tr)
	hits := scanLinesForLemma(sc, lemma, max)
	logf("DEBUG: All hits for lemma '%s': %v", lemma, len(hits))
	for _, hit := range hits {
		// Extract key (kanji) from line: up to first '['
		key := hit
		reading := ""
		if idx := strings.Index(key, "["); idx > 0 {
			key = strings.TrimSpace(key[:idx])
			// Extract reading between '[' and ']'
			rstart := strings.Index(hit, "[")
			rend := strings.Index(hit, "]")
			if rstart > 0 && rend > rstart {
				reading = strings.TrimSpace(hit[rstart+1 : rend])
			}
		} else {
			key = strings.Fields(key)[0]
		}
		logf("DEBUG: Extracted key '%s' and reading '%s' from line '%s'", key, reading, hit)
		if key == lemma {
			// If caller provides a reading, require it to match
			// (normalize katakana to hiragana for both)
			if reading != "" && lemma != "" {
				// Try to get the reading from the caller (token)
				// This function doesn't have token, but can accept reading as part of lemma (e.g. "仙北|せんぼく")
				// For now, try to extract reading from lemma if formatted as "kanji|reading"
				var lemmaReading string
				if strings.Contains(lemma, "|") {
					parts := strings.SplitN(lemma, "|", 2)
					lemma = parts[0]
					lemmaReading = parts[1]
				}
				// If no reading in lemma, skip reading check
				if lemmaReading != "" {
					normLemmaReading := katakanaToHiragana(lemmaReading)
					normHitReading := katakanaToHiragana(reading)
					if normLemmaReading != normHitReading {
						logf("DEBUG: Reading mismatch for '%s': token '%s' vs dict '%s'", lemma, normLemmaReading, normHitReading)
						continue
					}
				}
			}
			// Extract gloss: after last '/' and before it, get the word after last space
			lastSlash := strings.LastIndex(hit, "/")
			if lastSlash > 0 {
				beforeSlash := hit[:lastSlash]
				lastSpace := strings.LastIndex(beforeSlash, " ")
				if lastSpace > 0 && lastSpace < len(beforeSlash)-1 {
					english := beforeSlash[lastSpace+1:]
					logf("DEBUG: Extracted English gloss for '%s': %s", lemma, english)
					return english, nil
				}
			}
			logf("DEBUG: Could not extract English gloss for '%s' from line: %s", lemma, hit)
			return "", nil
		}
	}
	logf("DEBUG: No exact matches found for lemma '%s'.", lemma)
	return "", nil
}

// lookupJMdict does a tolerant text-scan of JMdict for entries matching lemma
func lookupJMdict(path, lemma string, max int) ([]model.DictionaryEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	const chunkSize = 64 * 1024
	r := bufio.NewReaderSize(f, chunkSize)
	var buf strings.Builder
	var results []model.DictionaryEntry

	for {
		chunk := make([]byte, chunkSize)
		n, err := r.Read(chunk)
		if n > 0 {
			buf.Write(chunk[:n])
		}
		data := buf.String()

		for {
			start := strings.Index(data, "<entry")
			if start < 0 {
				break
			}
			end := strings.Index(data[start:], "</entry>")
			if end < 0 {
				break
			}
			end += start + len("</entry>")
			entry := data[start:end]

			if strings.Contains(entry, "<keb>"+lemma+"</keb>") || strings.Contains(entry, "<reb>"+lemma+"</reb>") {
				// extract kebs, rebs, glosses
				var ks, rs, glosses []string
				var poss []string
				for pos := 0; ; {
					i := strings.Index(entry[pos:], "<keb>")
					if i < 0 {
						break
					}
					i += pos + len("<keb>")
					j := strings.Index(entry[i:], "</keb>")
					if j < 0 {
						break
					}
					ks = append(ks, strings.TrimSpace(entry[i:i+j]))
					pos = i + j + len("</keb>")
				}
				for pos := 0; ; {
					i := strings.Index(entry[pos:], "<reb>")
					if i < 0 {
						break
					}
					i += pos + len("<reb>")
					j := strings.Index(entry[i:], "</reb>")
					if j < 0 {
						break
					}
					rs = append(rs, strings.TrimSpace(entry[i:i+j]))
					pos = i + j + len("</reb>")
				}
				for pos := 0; ; {
					i := strings.Index(entry[pos:], "<gloss")
					if i < 0 {
						break
					}
					i += pos
					gt := strings.Index(entry[i:], ">")
					if gt < 0 {
						break
					}
					gt += i + 1
					j := strings.Index(entry[gt:], "</gloss>")
					if j < 0 {
						break
					}
					glosses = append(glosses, strings.TrimSpace(entry[gt:gt+j]))
					pos = gt + j + len("</gloss>")
				}

				// extract pos elements if present (e.g., <pos>&n;</pos>)
				for pos := 0; ; {
					i := strings.Index(entry[pos:], "<pos>")
					if i < 0 {
						break
					}
					i += pos + len("<pos>")
					j := strings.Index(entry[i:], "</pos>")
					if j < 0 {
						break
					}
					poss = append(poss, strings.TrimSpace(entry[i:i+j]))
					pos = i + j + len("</pos>")
				}
				results = append(results, model.DictionaryEntry{
					Source:   "JMdict",
					Kanji:    ks,
					Readings: normalizeReadings(rs),
					Glosses:  glosses,
					POS:      poss,
				})
				if len(results) >= max {
					return results, nil
				}
			}

			data = data[end:]
			buf.Reset()
			buf.WriteString(data)
		}

		if err != nil {
			if err == io.ErrUnexpectedEOF || err == io.EOF {
				break
			}
			return results, err
		}
	}
	return results, nil
}

// LookupDictionary returns dictionary entries for tokens. It uses the token.Lemma
// when present (preferred) or token.Text otherwise.
func LookupDictionary(ctx context.Context, tokens []tokenize.Token) ([]model.DictionaryEntry, error) {
	out := make([]model.DictionaryEntry, len(tokens))
	lemmaToIdxs := make(map[string][]int)
	lemmaToToken := make(map[string]tokenize.Token)
	for i, t := range tokens {
		lemma := t.Lemma
		if lemma == "" {
			lemma = t.Text
		}
		lemmaToIdxs[lemma] = append(lemmaToIdxs[lemma], i)
		if _, ok := lemmaToToken[lemma]; !ok {
			lemmaToToken[lemma] = t
		}
	}

	lemmaResults := make(map[string]model.DictionaryEntry)
	for lemma, t := range lemmaToToken {
		lemma = intern(lemma)
		// Try JMdict first
		jmResults, err := lookupJMdict(jmPath, lemma, 3)
		var entry model.DictionaryEntry
		if err == nil && len(jmResults) > 0 {
			for i := range jmResults {
				jmResults[i].Kanji = internSlice(jmResults[i].Kanji)
				jmResults[i].Readings = internSlice(jmResults[i].Readings)
				jmResults[i].Glosses = internSlice(jmResults[i].Glosses)
			}
			// (reuse the original per-token logic for picking best entry)
			pick := 0
			tokenReading := t.Reading
			if tokenReading == "" {
				tokenReading = t.Pronunciation
			}
			normTokenReading := katakanaToHiragana(tokenReading)
			if normTokenReading != "" {
				for idx, candidate := range jmResults {
					for _, r := range candidate.Readings {
						if katakanaToHiragana(r) == normTokenReading {
							pick = idx
							break
						}
					}
					if pick == idx {
						break
					}
				}
			}
			placeKeywords := []string{"city", "municipal", "town", "village", "ward", "prefecture", "municipality", "capital", "county", "district"}
			if normTokenReading == "" || (len(jmResults) > 0 && pick == 0) {
				for idx, candidate := range jmResults {
					for _, g := range candidate.Glosses {
						lg := strings.ToLower(g)
						for _, kw := range placeKeywords {
							if strings.Contains(lg, kw) {
								pick = idx
								break
							}
						}
						if pick == idx {
							break
						}
					}
					if pick == idx {
						break
					}
				}
			}
			// No context/suffix heuristics in batch mode (could be added if needed)
			if lemma == "の" || lemma == "が" || lemma == "は" {
				jmResults = prioritizeParticleResults(lemma, jmResults)
			}
			tokenPOS := strings.SplitN(t.POS, ",", 2)[0]
			if tokenPOS != "" && len(jmResults) > 1 {
				var target string
				switch tokenPOS {
				case "名詞", "固有名詞":
					target = "&n;"
				case "動詞":
					target = "&v;"
				case "形容詞":
					target = "&adj;"
				case "副詞":
					target = "&adv;"
				case "助詞":
					target = "&prt;"
				case "助動詞":
					target = "&aux;"
				case "接尾":
					target = "&n;"
				case "連体詞":
					target = "&adj;"
				}
				if target != "" {
					for idx, c := range jmResults {
						for _, p := range c.POS {
							if strings.Contains(p, target) {
								pick = idx
								break
							}
						}
						if pick == idx {
							break
						}
					}
				}
			}
			first := jmResults[pick]
			entry = model.DictionaryEntry{
				Source:   first.Source,
				Kanji:    internSlice(first.Kanji),
				Readings: normalizeReadings(internSlice(first.Readings)),
				Glosses:  internSlice(first.Glosses),
			}
		} else {
			// Fallback to ENAMDICT (raw lines)
			mostLikely, err := LookupEnamdictMostLikely(enamPath, lemma, 1)
			if err == nil && mostLikely != "" {
				entry = model.DictionaryEntry{
					Source:   "ENAMDICT",
					Kanji:    internSlice([]string{t.Text}),
					Readings: normalizeReadings(internSlice([]string{t.Reading})),
					Glosses:  internSlice([]string{mostLikely}),
				}
			} else {
				entry = model.DictionaryEntry{
					Source:   "none",
					Kanji:    internSlice([]string{t.Text}),
					Readings: normalizeReadings(internSlice([]string{t.Reading})),
					Glosses:  internSlice([]string{"<no definition found>"}),
				}
			}
		}
		lemmaResults[lemma] = entry
	}
	for lemma, idxs := range lemmaToIdxs {
		entry := lemmaResults[lemma]
		for _, i := range idxs {
			out[i] = entry
		}
	}
	return out, nil
}

// prioritizeParticleResults orders JMdict candidates for common particles by
// scoring features: no-kanji, gloss keyword matches, and reading match.
func prioritizeParticleResults(p string, results []model.DictionaryEntry) []model.DictionaryEntry {
	const (
		weightNoKanji    = 80
		weightGlossHit   = 50
		weightReadingHit = 40
	)
	keywords := map[string][]string{
		"の":      {"indicat", "possess", "nominaliz", "particle", "function word"},
		"が":      {"indicat", "subject", "but", "however", "marks", "particle", "case"},
		"は":      {"topic", "contrast", "marks the topic", "wa", "particle", "marks", "topic marker", "postposition", "case"},
		"を":      {"object", "direct object", "accusative", "particle", "marks the object"},
		"に":      {"direction", "location", "time", "indirect object", "particle", "goal", "destination", "target"},
		"で":      {"means", "location", "by", "with", "at", "particle", "place of action", "method"},
		"へ":      {"direction", "to", "toward", "particle", "destination", "goal"},
		"と":      {"and", "with", "quotative", "together", "particle", "accompaniment", "quotation"},
		"も":      {"also", "too", "as well", "even", "particle", "in addition"},
		"から":     {"from", "since", "because", "particle", "origin", "starting point"},
		"まで":     {"until", "to", "as far as", "particle", "limit", "end point"},
		"より":     {"than", "from", "particle", "comparison", "starting point"},
		"や":      {"and", "or", "among", "particle", "listing", "examples"},
		"やら":     {"and", "or", "particle", "uncertainty", "listing"},
		"か":      {"or", "question", "particle", "interrogative", "alternative"},
		"ね":      {"seeking agreement", "isn't it", "particle", "confirmation", "tag question"},
		"よ":      {"assertion", "emphasis", "particle", "exclamation", "informing"},
		"ぞ":      {"assertion", "emphasis", "particle", "masculine", "informal"},
		"ぜ":      {"assertion", "emphasis", "particle", "masculine", "informal"},
		"さ":      {"assertion", "particle", "casual", "filler", "masculine"},
		"な":      {"prohibition", "command", "particle", "don't", "imperative"},
		"でも":     {"even", "but", "however", "particle", "contrast", "alternative"},
		"しか":     {"only", "nothing but", "particle", "limitation", "exclusivity"},
		"ばかり":    {"only", "just", "nothing but", "particle", "amount", "limitation"},
		"ほど":     {"extent", "degree", "as much as", "particle", "comparison"},
		"くらい":    {"about", "approximately", "as much as", "particle", "degree", "extent"},
		"ぐらい":    {"about", "approximately", "as much as", "particle", "degree", "extent"},
		"だって":    {"even", "but", "because", "particle", "emphasis", "reason"},
		"って":     {"quoting", "as for", "topic", "particle", "quotation", "emphasis"},
		"とか":     {"among other things", "such as", "or something", "particle", "listing", "examples"},
		"ながら":    {"while", "during", "although", "particle", "simultaneous action"},
		"つつ":     {"while", "although", "particle", "simultaneous action", "ongoing"},
		"つもり":    {"intention", "plan", "particle", "purpose"},
		"きり":     {"only", "just", "since", "particle", "limitation", "completion"},
		"きりで":    {"just with", "only with", "particle", "limitation"},
		"きりに":    {"just to", "only to", "particle", "limitation"},
		"ずつ":     {"each", "apiece", "particle", "distribution"},
		"までに":    {"by (time)", "before", "particle", "deadline"},
		"にて":     {"at", "in", "by means of", "particle", "location", "method"},
		"にして":    {"at", "in", "with", "as", "particle", "state", "means"},
		"にとって":   {"for", "to", "from the standpoint of", "particle", "perspective"},
		"により":    {"by", "due to", "because of", "particle", "means", "reason"},
		"において":   {"at", "in", "on", "particle", "location", "situation"},
		"に関して":   {"regarding", "concerning", "about", "particle", "relation"},
		"について":   {"about", "concerning", "regarding", "particle", "topic"},
		"として":    {"as", "in the role of", "particle", "capacity", "function"},
		"としても":   {"even as", "even if", "particle", "hypothetical"},
		"としては":   {"as for", "in terms of", "particle", "topic"},
		"としての":   {"as", "of", "particle", "attributive"},
		"にしては":   {"for", "considering", "particle", "unexpected"},
		"にしても":   {"even for", "even if", "particle", "hypothetical"},
		"にしてからが": {"even after", "even since", "particle", "emphasis"},
	}

	type scored struct {
		idx   int
		score int
	}
	var s []scored
	for i, r := range results {
		sc := 0
		if len(r.Kanji) == 0 {
			sc += weightNoKanji
		}
		for _, g := range r.Glosses {
			lg := strings.ToLower(g)
			for _, kw := range keywords[p] {
				if strings.Contains(lg, kw) {
					sc += weightGlossHit
					break
				}
			}
		}
		for _, rd := range r.Readings {
			if rd == p || rd == strings.ToUpper(p) {
				sc += weightReadingHit
				break
			}
		}
		s = append(s, scored{idx: i, score: sc})
	}
	sort.SliceStable(s, func(i, j int) bool { return s[i].score > s[j].score })
	out := make([]model.DictionaryEntry, 0, len(results))
	for _, v := range s {
		out = append(out, results[v.idx])
	}
	return out
}
