package dictionary

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"sort"
	"strings"
	"unicode"

	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/transform"

	"japaneseparse/model"
	"japaneseparse/tokenize"
)

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
		if lemma == "仙北" {
			log.Printf("DEBUG: Scanning line for '仙北': %s", line)
		}
		if strings.Contains(line, lemma) {
			line = strings.TrimFunc(line, func(r rune) bool { return !unicode.IsPrint(r) })
			if lemma == "仙北" {
				log.Printf("DEBUG: Found hit for '仙北': %s", line)
			}
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

	log.Printf("DEBUG: Starting EUC-JP scan for lemma '%s' in ENAMDICT.", lemma)
	tr := transform.NewReader(f, japanese.EUCJP.NewDecoder())
	sc := bufio.NewScanner(tr)
	hits := scanLinesForLemma(sc, lemma, max)
	log.Printf("DEBUG: All hits for lemma '%s': %v", lemma, hits)
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
		log.Printf("DEBUG: Extracted key '%s' and reading '%s' from line '%s'", key, reading, hit)
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
						log.Printf("DEBUG: Reading mismatch for '%s': token '%s' vs dict '%s'", lemma, normLemmaReading, normHitReading)
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
					log.Printf("DEBUG: Extracted English gloss for '%s': %s", lemma, english)
					return english, nil
				}
			}
			log.Printf("DEBUG: Could not extract English gloss for '%s' from line: %s", lemma, hit)
			return "", nil
		}
	}
	log.Printf("DEBUG: No exact matches found for lemma '%s'.", lemma)
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
					Readings: rs,
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
	for i, t := range tokens {
		lemma := t.Lemma
		if lemma == "" {
			lemma = t.Text
		}

		// Try JMdict first
		jmResults, err := lookupJMdict(jmPath, lemma, 3)
		if err == nil && len(jmResults) > 0 {
			// If token provides a reading, prefer the JMdict entry whose reading
			// best matches the token reading (normalize katakana->hiragana).
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

			// common place-related keywords used by heuristics
			placeKeywords := []string{"city", "municipal", "town", "village", "ward", "prefecture", "municipality", "capital", "county", "district"}

			// If no clear reading match, prefer entries whose gloss suggests a place/city
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

			// Context and suffix heuristics: prefer place sense when current token
			// looks like a place suffix (e.g., 市, 町) or when previous token is a
			// proper name (固有名詞). This helps disambiguate single-character
			// suffix tokens like 市 where JMdict has multiple senses.
			prevIsProperNoun := false
			if i > 0 {
				prevPOS := tokens[i-1].POS
				if strings.Contains(prevPOS, "固有名詞") || strings.HasPrefix(prevPOS, "名詞,固有名詞") {
					prevIsProperNoun = true
				}
				// If the previous token was already resolved to a dictionary entry,
				// use that as an additional hint: ENAMDICT hits or IsName flags are
				// strong signals that the previous token is a proper name and the
				// current token might be a place-suffix (e.g. 市, 町).
				prevEntry := out[i-1]
				if prevEntry.Source == "ENAMDICT" || prevEntry.IsName {
					prevIsProperNoun = true
				}
				// Also check any POS markers from the previous dictionary entry for
				// common proper-name markers (some JMdict-derived tags or debug
				// markers may contain hints). This is conservative: we only set
				// the flag when an explicit-looking marker appears.
				for _, ptag := range prevEntry.POS {
					if strings.Contains(ptag, "&pn;") || strings.Contains(strings.ToLower(ptag), "proper") {
						prevIsProperNoun = true
						break
					}
				}
			}

			placeSuffixes := map[string]bool{"市": true, "町": true, "村": true, "県": true, "区": true, "島": true}
			if prevIsProperNoun || strings.Contains(t.POS, "接尾") || placeSuffixes[t.Text] {
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

			// Particle preference: for very common particles, prefer grammatical
			// senses (entries with no kanji or glosses that look like grammatical
			// explanations) to avoid selecting content-word senses like 蛾 for "が".
			// Data-driven particle prioritization: score candidates and reorder
			// so grammatical senses surface first. Keep focused to the common
			// particles to avoid unintended reordering for general lemmas.
			if lemma == "の" || lemma == "が" || lemma == "は" {
				jmResults = prioritizeParticleResults(lemma, jmResults)
			}

			// If multiple candidates still exist, and token POS is available,
			// prefer candidates whose JMdict <pos> contains a matching marker.
			// Expand mappings to cover more token POS types.
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
					// no direct JMdict marker for suffixes; prefer nouns
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
			out[i] = model.DictionaryEntry{
				Source:   first.Source,
				Kanji:    first.Kanji,
				Readings: first.Readings,
				Glosses:  first.Glosses,
			}
			continue
		}

		// Fallback to ENAMDICT (raw lines)
		mostLikely, err := LookupEnamdictMostLikely(enamPath, lemma, 1)
		if err == nil && mostLikely != "" {
			out[i] = model.DictionaryEntry{
				Source:   "ENAMDICT",
				Kanji:    []string{t.Text},
				Readings: []string{t.Reading},
				Glosses:  []string{mostLikely},
			}
			continue
		}

		// No hits
		out[i] = model.DictionaryEntry{
			Source:   "none",
			Kanji:    []string{t.Text},
			Readings: []string{t.Reading},
			Glosses:  []string{"<no definition found>"},
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
		"の": {"indicat", "possess", "nominaliz", "particle", "function word"},
		"が": {"indicat", "subject", "but", "however", "marks", "particle", "case"},
		"は": {"topic", "contrast", "marks the topic", "wa", "particle", "marks", "topic marker", "postposition", "case"},
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
