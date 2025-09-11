package main

import (
	"fmt"
	"japaneseparse/kanji"
	"strings"
)

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

// alignFuriganaDemo: robust furigana alignment for demo
func alignFuriganaDemo(surface, reading string) string {
	fmt.Printf("[DEBUG] alignFuriganaDemo called with surface='%s', reading='%s'\n", surface, reading)
	surfaceRunes := []rune(surface)
	readingRunes := []rune(katakanaToHiragana(reading))
	fmt.Printf("[DEBUG] surfaceRunes: %v\n", surfaceRunes)
	fmt.Printf("[DEBUG] readingRunes: %v\n", readingRunes)
	j, k := 0, 0
	out := ""
	for j < len(surfaceRunes) {
		s := surfaceRunes[j]
		fmt.Printf("[DEBUG] surface[%d]='%c'\n", j, s)
		if isKanji(s) {
			readings := kanji.GetKanjiReadings(s)
			fmt.Printf("[DEBUG] Kanji '%c' readings: %v\n", s, readings)
			bestMatch := ""
			bestLen := 0
			// Try to match the longest possible reading for this kanji
			for _, kr := range readings {
				krBase := katakanaToHiragana(kr)
				if j > 0 && strings.Contains(kr, ".") {
					krBase = katakanaToHiragana(strings.SplitN(kr, ".", 2)[0])
					fmt.Printf("[DEBUG] Dot found in reading '%s', using base '%s'\n", kr, krBase)
				}
				krRunes := []rune(krBase)
				krLen := len(krRunes)
				// Greedily match the longest substring
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
				fmt.Printf("[DEBUG] Furigana for '%c': '%s'\n", s, bestMatch)
			} else {
				// If this is the last kanji and there are remaining reading runes, assign them as furigana
				isLastKanji := true
				for jj := j + 1; jj < len(surfaceRunes); jj++ {
					if isKanji(surfaceRunes[jj]) {
						isLastKanji = false
						break
					}
				}
				if isLastKanji && k < len(readingRunes) {
					out += "[" + string(readingRunes[k:]) + "]"
					fmt.Printf("[DEBUG] Furigana for last kanji '%c': '%s'\n", s, string(readingRunes[k:]))
					k = len(readingRunes)
				} else {
					out += "[]"
					fmt.Printf("[DEBUG] No furigana found for '%c'\n", s)
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
		fmt.Printf("[DEBUG] Remaining readingRunes: '%s'\n", string(readingRunes[k:]))
	}
	fmt.Printf("[DEBUG] Final output: %s\n", out)
	return out
}

func main() {
	kanji.InitKanjidic2("dict/kanjidic2.xml")
	fmt.Println("--- Furigana Demo: 入見内川 ---")

	surface := "入見内川"
	reading := "イリミナイカワ"
	expected := "[いり][み][ない][かわ]"
	output := alignFuriganaDemo(surface, reading)
	fmt.Printf("Surface: %s\nReading: %s\nOutput: %s\nExpected: %s\n", surface, reading, output, expected)
	if output != expected {
		fmt.Println("❌ Furigana alignment does NOT match expected output!")
	} else {
		fmt.Println("✅ Furigana alignment matches expected output.")
	}
}
