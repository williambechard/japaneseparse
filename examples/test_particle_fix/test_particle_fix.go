package main

import (
	"fmt"
	"log"

	"github.com/williambechard/japaneseparse/pkg/parser"
)

func main() {
	testParticleFixMain()
}

func testParticleFixMain() {
	fmt.Println("Testing particle detection fix...")

	// Initialize parser
	p, err := parser.New()
	if err != nil {
		log.Fatal("Failed to initialize parser:", err)
	}

	// Test the specific case mentioned: "は" should be identified as a particle
	testText := "これは本です" // "This is a book" - は should be a topic particle

	result, err := p.Parse(testText)
	if err != nil {
		log.Fatal("Failed to parse:", err)
	}

	fmt.Printf("Parsing: %s\n", testText)
	fmt.Printf("Tokens found: %d\n\n", len(result.Tokens))

	// Look for the は token specifically
	for i, token := range result.Tokens {
		fmt.Printf("Token %d: %s\n", i+1, token.Text)
		fmt.Printf("  Reading: %s\n", token.Reading)
		fmt.Printf("  POS: %s\n", token.POS)

		if len(token.Meanings) > 0 {
			fmt.Printf("  Meanings: %v\n", token.Meanings)
		} else {
			fmt.Printf("  Meanings: (none found)\n")
		}

		// Check if this is the は token and what it's interpreted as
		if token.Text == "は" {
			fmt.Printf("  >>> FOUND は TOKEN <<<\n")
			if len(token.Meanings) > 0 {
				fmt.Printf("  First meaning: %s\n", token.Meanings[0])
			}
		}
		fmt.Println()
	}
}
