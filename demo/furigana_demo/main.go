package main

import (
	"fmt"
	"japaneseparse/kanji"
	"strings"
)

// use shared helpers in kanji package

func main() {
	// Load kanjidic2
	if err := kanji.InitKanjidic2("dict/kanjidic2.xml"); err != nil {
		fmt.Printf("Warning: failed to init kanjidic2: %v\n", err)
	}

	text := "入見内川"
	reading := "イリミナイカワ"
	readingH := katakanaToHiragana(reading)
	fmt.Printf("Surface: %s\nReading (katakana): %s\nReading (hiragana): %s\n", text, reading, readingH)

	// Greedy, longest-match per kanji with rendaku support
	surfaceRunes := []rune(text)
	readingRunes := []rune(readingH)
	rPos := 0
	out := ""
	// track positions of empty brackets for kanji so we can fill leftover reading
	bracketPos := make([]int, len(surfaceRunes))
	for i := range bracketPos {
		bracketPos[i] = -1
	}
	fmt.Println("\nStep-by-step matching:")
	for i, s := range surfaceRunes {
		if isKanji(s) {
			candidates := kanji.GetKanjiReadings(s)
			fmt.Printf("kanji[%d]=%c candidates=%v\n", i, s, candidates)
			bestMatch := ""
			bestLen := 0
			// Try each candidate reading (converted to hiragana)
			for _, cand := range candidates {
				// produce variants: full normalized reading and prefix before '.' if present
				full := kanji.NormalizeReading(cand)
				variants := []string{}
				if full != "" {
					variants = append(variants, full)
				}
				if idx := strings.IndexRune(cand, '.'); idx >= 0 {
					pre := cand[:idx]
					preNorm := kanji.NormalizeReading(pre)
					if preNorm != "" && preNorm != full {
						variants = append(variants, preNorm)
					}
				}
				// also try removing leading '-' markers
				if strings.HasPrefix(cand, "-") {
					noLead := kanji.NormalizeReading(strings.TrimPrefix(cand, "-"))
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
					// normal match at current rPos
					if rPos+len(vRunes) <= len(readingRunes) && string(readingRunes[rPos:rPos+len(vRunes)]) == string(vRunes) {
						if len(vRunes) > bestLen {
							bestMatch = string(readingRunes[rPos : rPos+len(vRunes)])
							bestLen = len(vRunes)
						}
					}
					// rendaku (only for non-first kanji)
					if i > 0 {
						rForm := kanji.RendakuForm(v)
						rRunes := []rune(rForm)
						if rPos+len(rRunes) <= len(readingRunes) && string(readingRunes[rPos:rPos+len(rRunes)]) == rForm {
							if len(rRunes) > bestLen {
								bestMatch = string(readingRunes[rPos : rPos+len(rRunes)])
								bestLen = len(rRunes)
							}
						}
					}
				}
			}
			if bestLen > 0 {
				fmt.Printf("  chosen: %s (len=%d) at rPos=%d\n", bestMatch, bestLen, rPos)
				// output only the furigana in brackets (kanji is not printed)
				out += "[" + bestMatch + "]"
				rPos += bestLen
			} else {
				fmt.Printf("  no candidate matched at rPos=%d -> leaving empty\n", rPos)
				// append empty bracket and record its position to fill later
				bracketPos[i] = len(out)
				out += "[]"
			}
		} else {
			// kana or other: copy directly if matches reading
			ch := string(s)
			if rPos < len(readingRunes) && readingRunes[rPos] == s {
				rPos++
			}
			out += ch
		}
	}

	// assign any remaining reading to last kanji if empty
	if rPos < len(readingRunes) {
		fmt.Printf("remaining reading after loop: %s\n", string(readingRunes[rPos:]))
		// find last kanji index
		lastKanji := -1
		for i := len([]rune(text)) - 1; i >= 0; i-- {
			if isKanji([]rune(text)[i]) {
				lastKanji = i
				break
			}
		}
		if lastKanji >= 0 {
			if bracketPos[lastKanji] != -1 {
				// replace the empty [] at recorded position with [remaining]
				pos := bracketPos[lastKanji]
				out = out[:pos] + "[" + string(readingRunes[rPos:]) + "]" + out[pos+2:]
			} else {
				// last kanji already had furigana; append remaining reading after it
				out += string(readingRunes[rPos:])
			}
		} else {
			out += string(readingRunes[rPos:])
		}
	}

	fmt.Printf("\nFinal alignment: %s\n", out)
	fmt.Println("Expected visual grouping (hiragana): [いり][み][ない][かわ]")
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
