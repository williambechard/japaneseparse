package main

import (
	"bufio"
	"io"
	"os"
	"sort"
	"strings"
	"unicode"

	"japaneseparse/logger"

	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/transform"
)

type Result struct {
	Kanji   []string
	Reading []string
	Glosses []string
}

// lookupJMdict streams the JMdict XML file and returns up to max matching entries
// where the lemma matches either a kanji form (keb) or a reading (reb).
func lookupJMdict(path, lemma string, max int) ([]Result, error) {
	// Use a tolerant text scanner to extract <entry>...</entry> chunks and
	// perform substring matches for <keb> and <reb> to avoid XML entity
	// parsing failures in the provided JMdict file.
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	const chunkSize = 64 * 1024
	r := bufio.NewReaderSize(f, chunkSize)
	var buf strings.Builder
	var results []Result

	for {
		chunk := make([]byte, chunkSize)
		n, err := r.Read(chunk)
		if n > 0 {
			buf.Write(chunk[:n])
		}
		data := buf.String()

		// Find complete <entry> ... </entry> blocks
		for {
			start := strings.Index(data, "<entry")
			if start < 0 {
				break
			}
			end := strings.Index(data[start:], "</entry>")
			if end < 0 {
				break // wait for more data
			}
			end += start + len("</entry>")
			entry := data[start:end]

			// simple lemma match (exact keb or reb)
			matched := false
			if strings.Contains(entry, "<keb>"+lemma+"</keb>") || strings.Contains(entry, "<reb>"+lemma+"</reb>") {
				matched = true
			}

			if matched {
				// extract kebs
				var ks, rs, glosses []string
				// naive keb extraction
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
					keb := entry[i : i+j]
					ks = append(ks, strings.TrimSpace(keb))
					pos = i + j + len("</keb>")
				}
				// naive reb extraction
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
					reb := entry[i : i+j]
					rs = append(rs, strings.TrimSpace(reb))
					pos = i + j + len("</reb>")
				}
				// gloss extraction
				for pos := 0; ; {
					// <gloss ...>content</gloss>
					i := strings.Index(entry[pos:], "<gloss")
					if i < 0 {
						break
					}
					i += pos
					// find closing '>' of opening tag
					gt := strings.Index(entry[i:], ">")
					if gt < 0 {
						break
					}
					gt += i + 1
					j := strings.Index(entry[gt:], "</gloss>")
					if j < 0 {
						break
					}
					gloss := entry[gt : gt+j]
					glosses = append(glosses, strings.TrimSpace(gloss))
					pos = gt + j + len("</gloss>")
				}

				results = append(results, Result{Kanji: ks, Reading: rs, Glosses: glosses})
				if len(results) >= max {
					return results, nil
				}
			}

			// remove processed prefix from data
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

// lookupEnamdict does a cheap line scan over the ENAMDICT file and returns lines
// that contain the lemma. ENAMDICT encoding can vary; this function assumes UTF-8
// and will return any matching raw lines.
func lookupEnamdict(path, lemma string, max int) ([]string, error) {
	// Try as UTF-8 first
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	hits := scanLinesForLemma(bufio.NewScanner(f), lemma, max)
	if len(hits) > 0 {
		return hits, nil
	}

	// If no hits, try Shift_JIS decoding (ENAMDICT often in Shift_JIS/EUC-JP)
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return hits, nil
	}
	tr := transform.NewReader(f, japanese.ShiftJIS.NewDecoder())
	sc := bufio.NewScanner(tr)
	hits = scanLinesForLemma(sc, lemma, max)
	return hits, nil
}

func scanLinesForLemma(sc *bufio.Scanner, lemma string, max int) []string {
	var hits []string
	for sc.Scan() {
		line := sc.Text()
		if strings.Contains(line, lemma) {
			// trim non-printable runes
			line = strings.TrimFunc(line, func(r rune) bool { return !unicode.IsPrint(r) })
			hits = append(hits, line)
			if len(hits) >= max {
				break
			}
		}
	}
	return hits
}

func main() {
	// Default paths (adjust if your files are elsewhere)
	jmdictPath := "dict/JMdict_e"
	enamPath := "dict/enamdict"

	tests := []struct {
		Token string
		Lemma string
		Note  string
	}{
		{Token: "人", Lemma: "人", Note: "1-kanji example"},
		{Token: "学校", Lemma: "学校", Note: "2-kanji example"},
		{Token: "行った", Lemma: "行く", Note: "conjugated verb (lookup lemma)"},
		// particle tests added for grammatical-sense prioritization
		{Token: "の", Lemma: "の", Note: "particle (possessive)"},
		{Token: "が", Lemma: "が", Note: "particle (subject)"},
		{Token: "は", Lemma: "は", Note: "particle (topic)"},
	}

	for _, t := range tests {
		// fmt.Println("========================================")
		// fmt.Printf("Token: %s   Lemma: %s   (%s)\n", t.Token, t.Lemma, t.Note)

		jmResults, err := lookupJMdict(jmdictPath, t.Lemma, 5)
		if err != nil {
			logger.Logf("JMdict lookup error: %v", err)
		} else if len(jmResults) == 0 {
			// fmt.Println("JMdict: no entries found for lemma")
		} else {
			// For particles, reorder JMdict results using a data-driven prioritizer
			// instead of injecting hardcoded canonical entries. This scores each
			// candidate using features: no-kanji (likely grammatical), gloss
			// keyword matches, and reading match, and sorts by score.
			p := t.Lemma
			if p == "の" || p == "が" || p == "は" {
				jmResults = prioritizeParticleResults(p, jmResults)
			}
			// fmt.Println("JMdict results:")
			// for i, r := range jmResults {
			// 	fmt.Printf("  Result %d:\n", i+1)
			// 	fmt.Printf("    Kanji: %v\n", r.Kanji)
			// 	fmt.Printf("    Readings: %s\n", strings.Join(r.Reading, ", "))
			// 	if len(r.Glosses) > 0 {
			// 		limit := 5
			// 		if len(r.Glosses) < limit {
			// 			limit = len(r.Glosses)
			// 		}
			// 		fmt.Println("    Glosses:")
			// 		for gi := 0; gi < limit; gi++ {
			// 			fmt.Printf("      %d. %s\n", gi+1, r.Glosses[gi])
			// 		}
			// 		if len(r.Glosses) > limit {
			// 			fmt.Printf("      ...and %d more\n", len(r.Glosses)-limit)
			// 		}
			// 	}
			// }
		}

		enamResults, err := lookupEnamdict(enamPath, t.Lemma, 5)
		if err != nil {
			logger.Logf("ENAMDICT lookup error: %v", err)
		} else if len(enamResults) == 0 {
			// fmt.Println("ENAMDICT: no matches found for lemma (encoding/format may differ)")
		} else {
			// fmt.Println("ENAMDICT hits:")
			// for i, l := range enamResults {
			// 	fmt.Printf("  %d: %s\n", i+1, l)
			// }
		}
	}
}

// prioritizeParticleResults scores each JMdict Result for a particle and
// returns a re-ordered slice with higher-scoring candidates first.
func prioritizeParticleResults(p string, results []Result) []Result {
	// scoring weights — tuned to prefer grammatical senses
	const (
		weightNoKanji    = 80
		weightGlossHit   = 50
		weightReadingHit = 40
	)
	// gloss keywords per particle (expanded)
	keywords := map[string][]string{
		"の": {"indicat", "possess", "nominaliz", "particle", "function word"},
		"が": {"indicat", "subject", "but", "however", "marks", "particle", "case"},
		"は": {"topic", "contrast", "marks the topic", "wa", "particle", "marks", "topic marker", "postposition", "case"},
	}

	scored := make([]struct {
		idx   int
		score int
	}, 0, len(results))

	for i, r := range results {
		score := 0
		if len(r.Kanji) == 0 {
			score += weightNoKanji
		}
		// gloss keyword matches
		for _, g := range r.Glosses {
			lg := strings.ToLower(g)
			for _, kw := range keywords[p] {
				if strings.Contains(lg, kw) {
					score += weightGlossHit
					break
				}
			}
		}
		// reading match
		for _, rd := range r.Reading {
			if rd == p || rd == strings.ToUpper(p) {
				score += weightReadingHit
				break
			}
		}
		scored = append(scored, struct {
			idx   int
			score int
		}{idx: i, score: score})
	}

	sort.SliceStable(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	out := make([]Result, 0, len(results))
	for _, s := range scored {
		out = append(out, results[s.idx])
	}
	return out
}
