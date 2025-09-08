package main

import (
	"encoding/xml"
	"fmt"
	"os"
	"unicode/utf8"
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

func katakanaToHiragana(s string) string {
	runes := []rune(s)
	for i, r := range runes {
		if r >= 0x30A1 && r <= 0x30F6 {
			runes[i] = r - 0x60
		}
	}
	return string(runes)
}

func main() {
	kanjiReadingMap := make(map[rune][]string)
	f, err := os.Open("dict/kanjidic2.xml")
	if err != nil {
		panic(err)
	}
	defer f.Close()
	d := xml.NewDecoder(f)
	for {
		tok, tokenErr := d.Token()
		if tokenErr != nil {
			break
		}
		switch se := tok.(type) {
		case xml.StartElement:
			if se.Name.Local == "character" {
				var k Kanjidic2Kanji
				if decodeErr := d.DecodeElement(&k, &se); decodeErr != nil {
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
			}
		}
	}

	surface := "秋田"
	reading := "アキタ"
	surfaceRunes := []rune(surface)
	readingRunes := []rune(katakanaToHiragana(reading))
	j, k := 0, 0
	fmt.Printf("Furigana for '%s' (reading '%s'):\n", surface, reading)
	for j < len(surfaceRunes) {
		s := surfaceRunes[j]
		if rds, ok := kanjiReadingMap[s]; ok {
			found := ""
			for _, kr := range rds {
				krH := katakanaToHiragana(kr)
				krRunes := []rune(krH)
				if k+len(krRunes) <= len(readingRunes) && string(readingRunes[k:k+len(krRunes)]) == krH {
					found = krH
					k += len(krRunes)
					break
				}
			}
			fmt.Printf("[%c|%s]", s, found)
		} else {
			fmt.Printf("[%c|]", s)
		}
		j++
	}
	fmt.Println()
}
