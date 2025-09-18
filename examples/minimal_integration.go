package main

import (
	"fmt"
	"log"

	"japaneseparse/pkg/parser"
)

// Minimal example for integrating into your Japanese language interpreter
func main() {
	// Initialize parser once (typically in your interpreter's init)
	p, err := parser.New()
	if err != nil {
		log.Fatal("Parser initialization failed:", err)
	}

	// Example: Process user input in your interpreter
	userInput := "私が昨日買った本は面白いです"

	// Get complete analysis
	result, err := p.Parse(userInput)
	if err != nil {
		log.Fatal("Parse failed:", err)
	}

	// Your interpreter can now access:
	fmt.Printf("Original: %s\n", result.Text)
	fmt.Printf("Tokens: %d\n", result.TokenCount)

	// Process each word for your interpreter logic
	for i, token := range result.Tokens {
		fmt.Printf("%d. %s [%s] = %v\n",
			i+1,
			token.Text,     // What the user typed
			token.Reading,  // How to pronounce it
			token.Meanings, // What it means in English
		)

		// Check if it's a conjugated verb (useful for grammar analysis)
		if token.IsConjugated {
			fmt.Printf("   (conjugated form of: %s, type: %s)\n", token.Lemma, token.Conjugation)
		}
	}

	// For debugging: get human-readable format
	if false { // Enable for debugging
		readable := p.FormatHumanReadable(result)
		fmt.Println("\nDetailed analysis:")
		fmt.Println(readable)
	}
}

// Example of how to integrate into your interpreter's main loop
func interpreterMainLoop() {
	// Initialize parser once
	p, err := parser.New()
	if err != nil {
		log.Fatal(err)
	}

	// Your interpreter's main loop
	for {
		// Get user input (however your interpreter does this)
		userInput := getUserInput()
		if userInput == "exit" {
			break
		}

		// Parse the Japanese text
		result, err := p.Parse(userInput)
		if err != nil {
			fmt.Printf("Could not understand: %v\n", err)
			continue
		}

		// Process the result in your interpreter
		processForInterpreter(result)
	}
}

func getUserInput() string {
	// Placeholder - replace with your input method
	return "こんにちは"
}

func processForInterpreter(result *parser.ParseResult) {
	// Example of how your interpreter might use the results

	// Look for specific patterns
	for _, token := range result.Tokens {
		switch {
		case contains(token.Meanings, "hello"):
			fmt.Println("User is greeting")
		case contains(token.Meanings, "goodbye"):
			fmt.Println("User is saying goodbye")
		case token.POS == "名詞,固有名詞,人名,*": // Person's name
			fmt.Printf("User mentioned person: %s\n", token.Text)
		}
	}

	// Analyze sentence complexity
	if len(result.Clauses) > 1 {
		fmt.Println("Complex sentence - might need special handling")
	}

	// Your interpreter's response logic here...
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
