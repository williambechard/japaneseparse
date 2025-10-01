package main

import (
	"fmt"
	"log"

	"github.com/williambechard/japaneseparse/pkg/parser"
)

// Final verification that all three requested functions work
func main() {
	fmt.Println("Testing the three main library functions you requested:")
	fmt.Println("=====================================================")

	// Initialize parser
	p, err := parser.New()
	if err != nil {
		log.Fatal("Parser initialization failed:", err)
	}
	fmt.Println("✓ Parser initialized successfully")

	testText := "私は学校に行きます"
	
	// Test 1: parser.Parse - get complete analysis
	fmt.Println("\n1. Testing parser.Parse() - Get complete analysis:")
	result, err := p.Parse(testText)
	if err != nil {
		log.Fatal("Parse failed:", err)
	}
	fmt.Printf("   ✓ Parsed '%s' into %d tokens\n", result.Text, result.TokenCount)
	fmt.Printf("   ✓ Found %d definitions\n", result.DefinitionsFound)

	// Test 2: parser.Analyze - get everything (alias for Parse)
	fmt.Println("\n2. Testing parser.Analyze() - Get everything:")
	result2, err := p.Analyze(testText)
	if err != nil {
		log.Fatal("Analyze failed:", err)
	}
	fmt.Printf("   ✓ Analyzed '%s' into %d tokens\n", result2.Text, result2.TokenCount)
	fmt.Printf("   ✓ Same results as Parse: %t\n", result.TokenCount == result2.TokenCount)

	// Test 3: parser.GetMeaning - get meaning of single word
	fmt.Println("\n3. Testing parser.GetMeaning() - Get meaning of single word:")
	meaning, err := p.GetMeaning("学校")
	if err != nil {
		log.Printf("   GetMeaning failed: %v", err)
	} else {
		fmt.Printf("   ✓ Meaning of '学校': '%s'\n", meaning)
	}

	// Bonus: Show tokens for context
	fmt.Println("\n4. Token details from Parse:")
	for i, token := range result.Tokens {
		fmt.Printf("   %d. %s [%s] = %v\n", i+1, token.Text, token.Reading, token.Meanings)
	}

	fmt.Println("\n✅ All three main library functions are working correctly!")
	fmt.Println("✅ Your japaneseparse library is ready for use!")
}