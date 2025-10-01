package main

import (
	"fmt"

	"github.com/williambechard/japaneseparse/pkg/parser"
)

// Example showing how to use the japaneseparse library in your Go application
func main() {
	fmt.Println("Japanese Text Parser Library - API Demo")
	fmt.Println("========================================")

	// The three main functions you requested:
	fmt.Println("\n1. parser.Parse() - Get complete analysis")
	fmt.Println("2. parser.Analyze() - Alias for Parse, get everything")
	fmt.Println("3. parser.GetMeaning() - Get meaning of a single word")
	fmt.Println("4. Additional helpers: ParseSimple(), GetMeanings(), GetReadings()")

	fmt.Println("\nTo use this library in your Go program:")
	fmt.Println("--------------------------------------")
	fmt.Printf("import \"%s\"\n", "github.com/williambechard/japaneseparse/pkg/parser")

	fmt.Println("\nAPI Usage Examples:")
	fmt.Println("------------------")

	fmt.Println(`
// Initialize the parser once in your application
parser, err := parser.New()
if err != nil {
    log.Fatal(err)
}

// Method 1: Parse Japanese text (complete analysis)
result, err := parser.Parse("私は学校に行きます")
if err != nil {
    log.Fatal(err)  
}

// Method 2: Analyze (same as Parse - comprehensive analysis) 
result, err := parser.Analyze("私は学校に行きます")

// Method 3: Get meaning of a single word
meaning, err := parser.GetMeaning("学校")
// Returns: "school"

// Additional helpers:
tokens, err := parser.ParseSimple("私は学校に行きます")  // Just tokens
readings, err := parser.GetReadings("私は学校に行きます")  // Just readings
meanings, err := parser.GetMeanings("私は学校に行きます") // All meanings
`)

	// Try to demonstrate without dictionary files
	fmt.Println("\nAttempting to initialize parser...")
	fmt.Println("(This will fail without dictionary files, but shows the API)")

	_, err := parser.New()
	if err != nil {
		fmt.Printf("Expected error (dictionary files needed): %v\n", err)
		fmt.Println("\nTo use this library, you need:")
		fmt.Println("- dict/JMdict_e (Japanese-English dictionary)")
		fmt.Println("- dict/enamdict (proper names dictionary)")
		fmt.Println("- dict/kanjidic2.xml (kanji information)")
		fmt.Println("\nSee docs/SETUP.md for download instructions.")
	}

	fmt.Println("\n✓ Library API is ready for integration!")
	fmt.Println("✓ This is now a proper Go library that can be imported by any Go program")
}
