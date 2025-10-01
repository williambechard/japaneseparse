package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/williambechard/japaneseparse/internal/analyzer"
	"github.com/williambechard/japaneseparse/internal/config"
	"github.com/williambechard/japaneseparse/pkg/types"
)

func main() {
	// Command line flags
	var (
		text       = flag.String("text", "", "Japanese text to analyze")
		file       = flag.String("file", "", "File containing Japanese text to analyze")
		configPath = flag.String("config", "", "Path to configuration file")
		outputJSON = flag.Bool("json", false, "Output result as JSON")
		verbose    = flag.Bool("verbose", false, "Enable verbose output")
	)
	flag.Parse()

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading configuration: %v\n", err)
		os.Exit(1)
	}

	// Override config with command line flags
	if *verbose {
		cfg.Output.Verbose = true
	}

	// Determine input text
	var inputText string
	if *text != "" {
		inputText = *text
	} else if *file != "" {
		data, err := os.ReadFile(*file)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading file: %v\n", err)
			os.Exit(1)
		}
		inputText = string(data)
	} else {
		// Default text for quick testing
		inputText = "こんにちは世界"
		if cfg.Output.Verbose {
			fmt.Println("No input provided, using default text:", inputText)
		}
	}

	if inputText == "" {
		fmt.Fprintf(os.Stderr, "No text to analyze. Use -text or -file flag.\n")
		flag.Usage()
		os.Exit(1)
	}

	// Create and initialize analyzer
	analyzer := analyzer.New(cfg)
	if err := analyzer.Initialize(); err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing analyzer: %v\n", err)
		os.Exit(1)
	}

	if cfg.Output.Verbose {
		fmt.Printf("Analyzing text: %s\n", inputText)
	}

	// Analyze the text
	result, err := analyzer.AnalyzeText(inputText)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error analyzing text: %v\n", err)
		os.Exit(1)
	}

	// Output results
	if *outputJSON {
		// Output as JSON
		output, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error marshaling JSON: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(string(output))
	} else {
		// Human-readable output
		printHumanReadable(result, cfg.Output.Verbose)
	}

	if cfg.Output.Verbose {
		fmt.Printf("\nAnalysis complete. Sentence ID: %s\n", result.SentenceID)
		if cfg.Output.SaveLogs {
			fmt.Printf("Logs saved to: %s\n", cfg.Output.LogsDir)
		}
	}
}

func printHumanReadable(result *types.SentenceAnalysis, verbose bool) {
	fmt.Printf("=== Japanese Text Analysis ===\n")
	fmt.Printf("Sentence ID: %s\n", result.SentenceID)
	fmt.Printf("Tokens: %d\n", result.TokenCount)
	fmt.Printf("Definitions found: %d\n", result.DefinitionsFound)
	fmt.Printf("Processed at: %s\n\n", result.ProcessedAt.Format("2006-01-02 15:04:05"))

	fmt.Printf("=== Token Analysis ===\n")
	for i, token := range result.Tokens {
		fmt.Printf("%d. %s", i+1, token.Text)

		if token.Lemma != "" && token.Lemma != token.Text {
			fmt.Printf(" (%s)", token.Lemma)
		}

		if token.Reading != "" {
			fmt.Printf(" [%s]", token.Reading)
		}

		if token.POS != "" {
			fmt.Printf(" <%s>", token.POS)
		}

		fmt.Println()

		// Show furigana if available
		if token.FuriganaText != "" {
			fmt.Printf("   Furigana: %s\n", token.FuriganaText)
		}

		// Show dictionary information
		if len(token.DictionaryEntry.Glosses) > 0 {
			fmt.Printf("   Meanings: %s\n", joinStrings(token.DictionaryEntry.Glosses, "; "))
			if token.DictionaryEntry.Source != "" {
				fmt.Printf("   Source: %s\n", token.DictionaryEntry.Source)
			}
		}

		// Show conjugation information
		if token.ConjugationLabel != "" {
			fmt.Printf("   Conjugation: %s\n", token.ConjugationLabel)
		}

		// Show auxiliaries if present
		if len(token.Auxiliaries) > 0 {
			fmt.Printf("   Auxiliaries: ")
			for j, aux := range token.Auxiliaries {
				if j > 0 {
					fmt.Printf(" + ")
				}
				fmt.Printf("%s", aux.Text)
			}
			fmt.Println()
		}

		if verbose {
			// Additional verbose information
			if token.InflectionType != "" {
				fmt.Printf("   Inflection Type: %s\n", token.InflectionType)
			}
			if token.InflectionForm != "" {
				fmt.Printf("   Inflection Form: %s\n", token.InflectionForm)
			}
			if len(token.MergedIndices) > 0 {
				fmt.Printf("   Merged Indices: %v\n", token.MergedIndices)
			}
		}

		fmt.Println()
	}

	// Show clause structure
	if len(result.Analysis.Structure.Clauses) > 0 {
		fmt.Printf("=== Clause Structure ===\n")
		for i, clause := range result.Analysis.Structure.Clauses {
			fmt.Printf("Clause %d: tokens %d-%d", i+1, clause.Start, clause.End)
			if clause.Type != "" {
				fmt.Printf(" (%s)", clause.Type)
			}
			fmt.Println()
		}
		fmt.Println()
	}
}

func joinStrings(strings []string, separator string) string {
	if len(strings) == 0 {
		return ""
	}
	if len(strings) == 1 {
		return strings[0]
	}

	result := strings[0]
	for i := 1; i < len(strings); i++ {
		result += separator + strings[i]
	}
	return result
}
