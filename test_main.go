package main

import (
	"fmt"
	"japaneseparse/pkg/parser"
	"log"
)

func testMain() {
	// Create parser with default settings
	parser, err := parser.New()
	if err != nil {
		log.Fatalf("Failed to create parser: %v", err)
	}

	// Parse text and get complete analysis
	text := "私は学校に行きます。"
	result, err := parser.Parse(text)
	if err != nil {
		log.Fatalf("Failed to parse text: %v", err)
	}

	// Print the analysis result
	fmt.Printf("Sentence Analysis for: %s\n", text)
	for _, token := range result.Tokens {
		fmt.Printf("Token: %s, Role: %s, Relation: %v\n", token.Text, token.Role, token.Relation)
	}
}
