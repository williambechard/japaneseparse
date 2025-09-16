package main

import (
	"fmt"
	"japaneseparse/enamdict"
	"japaneseparse/logger"
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

func main() {
	path := "dict/enamdict"
	entries, err := enamdict.LoadEnamdict(path)
	if err != nil {
		logger.Logf("Failed to load ENAMDICT: %v", err)
		return
	}

	// Print the first entry in the map
	for k, entry := range entries {
		fmt.Printf("First entry: key=%s, Lemma=%s, Reading=%s, POS=%s, Meaning=%s\n", k, entry.Lemma, entry.Reading, entry.POS, entry.Meaning)
		break
	}

	kanji := "仙北"
	reading := "せんほく"
	hiraganaReading := katakanaToHiragana(reading)
	key := kanji + "|" + hiraganaReading
	entry, ok := entries[key]
	if ok {
		fmt.Printf("Found entry for %s: Lemma=%s, Reading=%s, POS=%s, Meaning=%s\n", key, entry.Lemma, entry.Reading, entry.POS, entry.Meaning)
	} else {
		fmt.Printf("No entry found for %s\n", key)
	}
}
