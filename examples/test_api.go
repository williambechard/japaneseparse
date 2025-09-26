package main

import (
	"fmt"
	"log"

	"japaneseparse/pkg/parser"
)

// Quick test to verify the new library API works
func testApiMain() {
	fmt.Println("Testing Japanese Text Parser Library API...")

	// Test 1: Basic initialization
	fmt.Println("\n1. Testing parser initialization...")
	p, err := parser.New()
	if err != nil {
		log.Fatal("Failed to initialize parser:", err)
	}
	fmt.Println("✓ Parser initialized successfully")

	// Test 2: Basic parsing
	fmt.Println("\n2. Testing basic parsing...")
	result, err := p.Parse("こんにちは")
	if err != nil {
		log.Fatal("Failed to parse:", err)
	}

	fmt.Printf("✓ Parsed: %s\n", result.Text)
	fmt.Printf("  - Tokens: %d\n", len(result.Tokens))
	fmt.Printf("  - Definitions: %d\n", result.DefinitionsFound)
	fmt.Printf("  - Sentence ID: %s\n", result.SentenceID)

	// Test 3: Token details
	fmt.Println("\n3. Testing token access...")
	for i, token := range result.Tokens {
		fmt.Printf("  Token %d: %s [%s] = %v\n",
			i+1, token.Text, token.Reading, token.Meanings)
	}

	// Test 4: Simple parsing
	fmt.Println("\n4. Testing simple parsing...")
	tokens, err := p.ParseSimple("世界")
	if err != nil {
		log.Fatal("Failed simple parse:", err)
	}
	fmt.Printf("✓ Simple parse returned %d tokens\n", len(tokens))

	// Test 5: Reading extraction
	fmt.Println("\n5. Testing reading extraction...")
	readings, err := p.GetReadings("こんにちは世界")
	if err != nil {
		log.Fatal("Failed to get readings:", err)
	}
	fmt.Printf("✓ Readings: %v\n", readings)

	// Test 6: Meaning extraction
	fmt.Println("\n6. Testing meaning extraction...")
	meanings, err := p.GetMeanings("こんにちは世界")
	if err != nil {
		log.Fatal("Failed to get meanings:", err)
	}
	fmt.Printf("✓ Meanings: %v\n", meanings)

	fmt.Println("\n✓ All tests passed! Library API is working correctly.")
	fmt.Println("\nThis parser is now ready for integration into your Japanese language interpreter!")
}
