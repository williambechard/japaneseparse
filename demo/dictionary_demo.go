package demotmp

import (
	"bufio"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
)

// Lightweight JMdict entry subset for streaming decode
type jmEntry struct {
	KEle []struct {
		Keb string `xml:"keb"`
	} `xml:"k_ele"`
	REle []struct {
		Reb string `xml:"reb"`
	} `xml:"r_ele"`
	Senses []struct {
		Gloss []string `xml:"gloss"`
	} `xml:"sense"`
}

type Result struct {
	Kanji   []string
	Reading []string
	Glosses []string
}

// lookupJMdict streams the JMdict XML file and returns up to max matching entries
// where the lemma matches either a kanji form (keb) or a reading (reb).
func lookupJMdict(path, lemma string, max int) ([]Result, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	dec := xml.NewDecoder(f)
	var results []Result
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return results, err
		}
		switch se := tok.(type) {
		case xml.StartElement:
			if se.Name.Local == "entry" {
				var e jmEntry
				if err := dec.DecodeElement(&e, &se); err != nil {
					// skip problematic entries but continue
					continue
				}

				matched := false
				for _, k := range e.KEle {
					if k.Keb == lemma {
						matched = true
						break
					}
				}
				if !matched {
					for _, r := range e.REle {
						if r.Reb == lemma {
							matched = true
							break
						}
					}
				}
				if matched {
					var glosses []string
					for _, s := range e.Senses {
						for _, g := range s.Gloss {
							glosses = append(glosses, strings.TrimSpace(g))
						}
					}
					var ks []string
					for _, k := range e.KEle {
						ks = append(ks, k.Keb)
					}
					var rs []string
					for _, r := range e.REle {
						rs = append(rs, r.Reb)
					}
					results = append(results, Result{Kanji: ks, Reading: rs, Glosses: glosses})
					if len(results) >= max {
						return results, nil
					}
				}
			}
		}
	}
	return results, nil
}

// lookupEnamdict does a cheap line scan over the ENAMDICT file and returns lines
// that contain the lemma. ENAMDICT encoding can vary; this function assumes UTF-8
// and will return any matching raw lines.
func lookupEnamdict(path, lemma string, max int) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var hits []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if strings.Contains(line, lemma) {
			hits = append(hits, line)
			if len(hits) >= max {
				break
			}
		}
	}
	if err := sc.Err(); err != nil {
		return hits, err
	}
	return hits, nil
}

func DemoMainDictionary() {
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
	}

	for _, t := range tests {
		fmt.Println("========================================")
		fmt.Printf("Token: %s   Lemma: %s   (%s)\n", t.Token, t.Lemma, t.Note)

		jmResults, err := lookupJMdict(jmdictPath, t.Lemma, 5)
		if err != nil {
			log.Printf("JMdict lookup error: %v\n", err)
		} else if len(jmResults) == 0 {
			fmt.Println("JMdict: no entries found for lemma")
		} else {
			fmt.Println("JMdict results:")
			for i, r := range jmResults {
				fmt.Printf("  Result %d:\n", i+1)
				fmt.Printf("    Kanji: %v\n", r.Kanji)
				fmt.Printf("    Readings: %v\n", r.Reading)
				if len(r.Glosses) > 0 {
					fmt.Printf("    Glosses (first 5): %v\n", r.Glosses)
				}
			}
		}

		enamResults, err := lookupEnamdict(enamPath, t.Lemma, 5)
		if err != nil {
			log.Printf("ENAMDICT lookup error: %v\n", err)
		} else if len(enamResults) == 0 {
			fmt.Println("ENAMDICT: no matches found for lemma (encoding/format may differ)")
		} else {
			fmt.Println("ENAMDICT hits:")
			for i, l := range enamResults {
				fmt.Printf("  %d: %s\n", i+1, l)
			}
		}
	}
}
