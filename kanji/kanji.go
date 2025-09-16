package kanji

import (
	"encoding/xml"
	"io"
	"os"
	"strings"
	"sync"
	"unicode/utf8"

	"japaneseparse/logger"
)

// rendaku map for shared use
var rendakuMap = map[rune]rune{
	'か': 'が', 'き': 'ぎ', 'く': 'ぐ', 'け': 'げ', 'こ': 'ご',
	'さ': 'ざ', 'し': 'じ', 'す': 'ず', 'せ': 'ぜ', 'そ': 'ぞ',
	'た': 'だ', 'ち': 'ぢ', 'つ': 'づ', 'て': 'で', 'と': 'ど',
	'は': 'ば', 'ひ': 'び', 'ふ': 'ぶ', 'へ': 'べ', 'ほ': 'ぼ',
}

// RendakuForm returns the voiced (rendaku) form of a hiragana string
func RendakuForm(s string) string {
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

// NormalizeReading removes non-kana characters (dots, hyphens) and
// converts katakana to hiragana so kanjidic readings like "い.り" match "いり".
func NormalizeReading(s string) string {
	var out []rune
	for _, r := range []rune(s) {
		// katakana -> hiragana
		if r >= 0x30A1 && r <= 0x30F6 {
			out = append(out, r-0x60)
			continue
		}
		// hiragana: keep
		if r >= 0x3041 && r <= 0x3096 {
			out = append(out, r)
			continue
		}
		// drop others
	}
	return string(out)
}

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
						loadedKanji = append(loadedKanji, k.Literal+": "+strings.Join(readings, ", "))
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
		logger.Logf("Total readings for kanji %c: %d", r, len(readings))
		// Log each reading and its runes for debugging dot/character issues
		// (per-user request) do not log each reading individually
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
