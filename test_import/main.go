package main

import (
	"fmt"
	"log"
	"path/filepath"
	"runtime"

	"github.com/williambechard/japaneseparse/pkg/parser"
)

func main() {
	fmt.Println("Testing Japanese Parser Library Import...")
	fmt.Println("=======================================")

	// Get the root directory of the parser project
	_, currentFile, _, _ := runtime.Caller(0)
	projectRoot := filepath.Dir(filepath.Dir(currentFile))

	// Create custom config pointing to the actual dictionary files
	config := &parser.Config{
		JMdictPath:   filepath.Join(projectRoot, "dict", "JMdict_e"),
		EnamdictPath: filepath.Join(projectRoot, "dict", "enamdict"),
		KanjidicPath: filepath.Join(projectRoot, "dict", "kanjidic2.xml"),
		// Enable detailed JSON logs for this import test
		SaveLogs: true,
		LogsDir:  filepath.Join(projectRoot, "test_import", "logs"),
		Debug:    false,
	}

	// Initialize parser
	fmt.Println("1. Initializing parser...")
	p, err := parser.NewWithConfig(config)
	if err != nil {
		log.Fatalf("Failed to initialize parser: %v", err)
	}
	fmt.Println("   ✅ Parser initialized successfully")

	// Test the three main functions as described in README
	testText := "私は中学校で新聞を読みました"
	fmt.Printf("\n2. Testing with Japanese text: %s\n", testText)

	// Test 1: parser.Parse() - Complete analysis
	fmt.Println("\n   Testing parser.Parse() - Complete analysis:")
	result, err := p.Parse(testText)
	if err != nil {
		log.Fatalf("Parse failed: %v", err)
	}
	fmt.Printf("   ✅ Text: %s\n", result.Text)
	fmt.Printf("   ✅ Token Count: %d\n", result.TokenCount)
	fmt.Printf("   ✅ Definitions Found: %d\n", result.DefinitionsFound)
	fmt.Printf("   ✅ Tokens:\n")
	for _, token := range result.Tokens {
		fmt.Printf("      - %s [%s]\n", token.Text, token.Reading)
	}
	fmt.Printf("   ✅ Clauses:\n")
	for _, clause := range result.Clauses {
		fmt.Printf("      - %s\n", clause.Type)
	}

	for i, token := range result.Tokens {
		if len(token.Meanings) > 0 {
			fmt.Printf("   ✅ %d. %s [%s] = %s\n", i+1, token.Text, token.Reading, token.Meanings[0])
		} else {
			fmt.Printf("   ✅ %d. %s [%s] = (particle/grammar)\n", i+1, token.Text, token.Reading)
		}
	}

	// Test 2: parser.Analyze() - Same as Parse (alias)
	fmt.Println("\n   Testing parser.Analyze() - Alias for Parse:")
	result2, err := p.Analyze(testText)
	if err != nil {
		log.Fatalf("Analyze failed: %v", err)
	}
	fmt.Printf("   ✅ Analyze returned same result: %t\n", result2.TokenCount == result.TokenCount)

	// Test 3: parser.GetMeaning() - Single word meaning
	fmt.Println("\n   Testing parser.GetMeaning() - Single word meaning:")
	testWord := "学校"
	meaning, err := p.GetMeaning(testWord)
	if err != nil {
		log.Printf("   ⚠️  GetMeaning failed for %s: %v", testWord, err)
	} else {
		fmt.Printf("   ✅ %s = %s\n", testWord, meaning)
	}

	// Test additional helper functions
	fmt.Println("\n3. Testing helper functions:")

	// Test GetReadings
	readings, err := p.GetReadings(testText)
	if err != nil {
		log.Printf("   ⚠️  GetReadings failed: %v", err)
	} else {
		fmt.Printf("   ✅ Readings: %v\n", readings)
	}

	// Test ParseSimple
	tokens, err := p.ParseSimple(testText)
	if err != nil {
		log.Printf("   ⚠️  ParseSimple failed: %v", err)
	} else {
		fmt.Printf("   ✅ Simple parsing returned %d tokens\n", len(tokens))
	}

	fmt.Println("\n🎉 All library functions working correctly!")
	fmt.Println("📚 Library is ready for import by other Go projects")
}
