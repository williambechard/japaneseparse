package kanji

import (
	"strings"
	"testing"
)

func TestFuriganaAlignmentForIriminaiKawa(t *testing.T) {
	err := InitKanjidic2("../dict/kanjidic2.xml")
	if err != nil {
		t.Fatalf("Failed to initialize Kanjidic2: %v", err)
	}
	surface := "入見内川"
	reading := "イリミナイカワ"
	// Convert katakana to hiragana for alignment
	hiraganaReading := katakanaToHiragana(reading)
	aligned := alignFuriganaDemo(surface, reading)
	t.Logf("Furigana alignment for %s (%s): %s", surface, hiraganaReading, aligned)
	expected := "[いり][み][ない][かわ]"
	if aligned != expected {
		t.Errorf("Expected %s, got %s", expected, aligned)
	}
}

func isKanji(r rune) bool {
	return r >= 0x4E00 && r <= 0x9FFF
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

// alignFuriganaDemo: robust furigana alignment for test
func alignFuriganaDemo(surface, reading string) string {
	surfaceRunes := []rune(surface)
	readingRunes := []rune(katakanaToHiragana(reading))
	j, k := 0, 0
	out := ""
	for j < len(surfaceRunes) {
		s := surfaceRunes[j]
		if isKanji(s) {
			readings := GetKanjiReadings(s)
			bestMatch := ""
			bestLen := 0
			for _, kr := range readings {
				krBase := katakanaToHiragana(kr)
				if j > 0 && strings.Contains(kr, ".") {
					krBase = katakanaToHiragana(strings.SplitN(kr, ".", 2)[0])
				}
				krRunes := []rune(krBase)
				krLen := len(krRunes)
				for l := krLen; l > 0; l-- {
					if k+l <= len(readingRunes) && string(readingRunes[k:k+l]) == string(krRunes[:l]) {
						if l > bestLen {
							bestMatch = string(krRunes[:l])
							bestLen = l
						}
						break
					}
				}
			}
			if bestMatch != "" {
				out += "[" + bestMatch + "]"
				k += bestLen
			} else {
				isLastKanji := true
				for jj := j + 1; jj < len(surfaceRunes); jj++ {
					if isKanji(surfaceRunes[jj]) {
						isLastKanji = false
						break
					}
				}
				if isLastKanji && k < len(readingRunes) {
					out += "[" + string(readingRunes[k:]) + "]"
					k = len(readingRunes)
				} else {
					out += "[]"
				}
			}
			j++
		} else {
			out += string(s)
			if k < len(readingRunes) && readingRunes[k] == s {
				k++
			}
			j++
		}
	}
	if k < len(readingRunes) {
		out += string(readingRunes[k:])
	}
	return out
}
