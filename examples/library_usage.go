package main

import (
	"encoding/json"
	"fmt"
	"log"

	"japaneseparse/pkg/parser"
)

// Example showing how to integrate the parser into a larger Japanese language interpreter
func libraryUsageMain() {
	// Example 1: Basic integration - no logs, just analysis
	fmt.Println("=== Example 1: Basic Usage ===")
	basicExample()

	fmt.Println("\n=== Example 2: Language Interpreter Integration ===")
	interpreterExample()

	fmt.Println("\n=== Example 3: Batch Processing ===")
	batchExample()

	fmt.Println("\n=== Example 4: Extract Specific Information ===")
	extractionExample()
}

func basicExample() {
	// Create parser with default settings (no logging)
	p, err := parser.New()
	if err != nil {
		log.Fatal("Failed to create parser:", err)
	}

	// Parse a sentence
	result, err := p.Parse("私が昨日買った本は面白いです")
	if err != nil {
		log.Fatal("Failed to parse:", err)
	}

	fmt.Printf("Analyzed: %s\n", result.Text)
	fmt.Printf("Found %d tokens with %d definitions\n", len(result.Tokens), result.DefinitionsFound)

	// Show basic token information
	for i, token := range result.Tokens {
		fmt.Printf("  %d. %s [%s] - %v\n", i+1, token.Text, token.Reading, token.Meanings)
	}
}

func interpreterExample() {
	// This simulates how you might use it in your interpreter
	p, err := parser.New()
	if err != nil {
		log.Fatal(err)
	}

	// Simulate processing user input in your interpreter
	userInput := "今日は天気がいいですね"

	// Get detailed analysis
	analysis, err := p.Parse(userInput)
	if err != nil {
		log.Fatal(err)
	}

	// Your interpreter can now access all the linguistic information
	fmt.Printf("User said: %s\n", analysis.Text)

	// Process each token for your interpreter logic
	for _, token := range analysis.Tokens {
		if token.POS == "名詞,一般,*,*" { // Noun
			fmt.Printf("Found noun: %s (%s) - %v\n", token.Text, token.Lemma, token.Meanings)
		} else if token.IsConjugated {
			fmt.Printf("Found conjugated word: %s -> %s (%s)\n", token.Text, token.Lemma, token.Conjugation)
		}
	}

	// Check sentence structure for your interpreter
	if len(analysis.Clauses) > 1 {
		fmt.Printf("Complex sentence with %d clauses\n", len(analysis.Clauses))
	} else {
		fmt.Printf("Simple sentence structure\n")
	}
}

func batchExample() {
	// For processing multiple sentences efficiently
	p, err := parser.New()
	if err != nil {
		log.Fatal(err)
	}

	sentences := []string{
		"おはよう",
		"今日は何をしますか？",
		"映画を見たいです",
		"ありがとうございました",
	}

	results := make([]*parser.ParseResult, len(sentences))

	for i, sentence := range sentences {
		result, err := p.Parse(sentence)
		if err != nil {
			log.Printf("Failed to parse '%s': %v", sentence, err)
			continue
		}
		results[i] = result
		fmt.Printf("Processed: %s (%d tokens)\n", result.Text, result.TokenCount)
	}

	// Now you have all results for further processing in your interpreter
	fmt.Printf("Batch processed %d sentences\n", len(results))
}

func extractionExample() {
	// Extract specific information efficiently
	p, err := parser.New()
	if err != nil {
		log.Fatal(err)
	}

	text := "田中さんは東京で働いています"

	// Get just readings (for pronunciation in your interpreter)
	readings, err := p.GetReadings(text)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Readings: %v\n", readings)

	// Get just meanings (for translation features)
	meanings, err := p.GetMeanings(text)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Meanings per token:\n")
	for i, tokenMeanings := range meanings {
		if len(tokenMeanings) > 0 {
			fmt.Printf("  Token %d: %v\n", i+1, tokenMeanings)
		}
	}

	// Get simple tokens (lightweight processing)
	tokens, err := p.ParseSimple(text)
	if err != nil {
		log.Fatal(err)
	}

	// Your interpreter can efficiently access what it needs
	for _, token := range tokens {
		if len(token.Meanings) > 0 {
			fmt.Printf("%s = %s\n", token.Text, token.Meanings[0])
		}
	}
}

// Example of how you might serialize results for storage/transmission
func serializationExample() {
	p, err := parser.New()
	if err != nil {
		log.Fatal(err)
	}

	result, err := p.Parse("こんにちは世界")
	if err != nil {
		log.Fatal(err)
	}

	// Convert to JSON for storage/API responses
	jsonData, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("JSON output:\n%s\n", string(jsonData))
}
