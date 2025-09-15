package main

import (
	"fmt"
	"japaneseparse/enamdict"
	"log"
)

func main() {
	path := "dict/enamdict"
	entries, err := enamdict.LoadEnamdict(path)
	if err != nil {
		log.Fatalf("Failed to load ENAMDICT: %v", err)
	}
	fmt.Printf("ENAMDICT loaded: %d entries\n", len(entries))

	key := "仙北|せんほく"
	entry, ok := entries[key]
	if ok {
		fmt.Printf("Found entry for %s: Lemma=%s, Reading=%s, POS=%s, Meaning=%s\n", key, entry.Lemma, entry.Reading, entry.POS, entry.Meaning)
	} else {
		fmt.Printf("No entry found for %s\n", key)
	}
}
