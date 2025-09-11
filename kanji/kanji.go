package kanji

import (
	"encoding/xml"
	"io"
	"log"
	"os"
	"strings"
	"sync"
	"unicode/utf8"
)

var (
	kanjiReadingMap     map[rune][]string
	kanjiReadingMapOnce sync.Once
)

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
						loadedKanji = append(loadedKanji, k.Literal+": "+strings.Join(readings, ", "))
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
		// Log each reading and its runes for debugging dot/character issues
		for _, reading := range readings {
			log.Printf("Reading for %c: '%s' (runes: %v)", r, reading, []rune(reading))
			for i, rr := range reading {
				log.Printf("  rune[%d]: '%c' (U+%04X)", i, rr, rr)
			}
		}
	}
	// Extra: log all readings for all kanji for debugging
	//for k, v := range kanjiReadingMap {
	//	log.Printf("KANJI MAP: %c => %v", k, v)
	//}
	return readings
}

// Count returns the number of kanji entries loaded
func Count() int {
	if kanjiReadingMap == nil {
		return 0
	}
	return len(kanjiReadingMap)
}
