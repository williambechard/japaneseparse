# Japanese Text Parser

A comprehensive Japanese text analysis library designed for integration into larger applications. This library provides morphological analysis, dictionary lookups, and grammatical structure detection for Japanese text.

**Note**: This is primarily a library component, not a standalone application. It's designed to be embedded in Japanese language interpreters, translation tools, and other language processing applications.

## Features

- **Morphological Analysis**: MeCab-based tokenization and part-of-speech tagging
- **Dictionary Integration**: JMdict and ENAMDICT support for comprehensive definitions  
- **Furigana Generation**: Automatic reading aids for kanji characters
- **Grammar Analysis**: Clause detection and verb auxiliary merging
- **Clean API**: Simple integration with minimal dependencies
- **Flexible Output**: Both detailed analysis and simplified token extraction
- **No External Dependencies**: Self-contained once dictionaries are provided

## Quick Start

### Installation

```bash
# Add to your Go project
go get github.com/yourusername/japaneseparse

# In your Go code
import "japaneseparse/pkg/parser"
```

### Basic Usage

```go
package main

import (
    "fmt"
    "log"
    "japaneseparse/pkg/parser"
)

func main() {
    // Initialize parser
    p, err := parser.New()
    if err != nil {
        log.Fatal(err)
    }

    // Parse Japanese text
    result, err := p.Parse("私が昨日買った本は面白いです")
    if err != nil {
        log.Fatal(err)
    }

    // Access analysis results
    fmt.Printf("Text: %s\n", result.Text)
    fmt.Printf("Tokens: %d\n", result.TokenCount)
    
    for _, token := range result.Tokens {
        fmt.Printf("- %s [%s] = %v\n", token.Text, token.Reading, token.Meanings)
    }
}
```

## API Reference

### Core Methods

```go
// Create parser with default settings
parser, err := parser.New()

// Create parser with custom configuration  
parser, err := parser.NewWithConfig(&parser.Config{
    JMdictPath: "/path/to/JMdict_e",
    SaveLogs:   false, // Disable logging for library use
})

// Parse text and get complete analysis
result, err := parser.Parse("日本語のテキスト")

// Get simplified token list
tokens, err := parser.ParseSimple("日本語のテキスト") 

// Extract just readings (for pronunciation)
readings, err := parser.GetReadings("日本語のテキスト")

// Extract just meanings (for translation)
meanings, err := parser.GetMeanings("日本語のテキスト")
```

### Data Structures

```go
type ParseResult struct {
    Text             string   `json:"text"`              // Original input
    SentenceID       string   `json:"sentence_id"`       // Unique identifier  
    TokenCount       int      `json:"token_count"`       // Number of tokens
    DefinitionsFound int      `json:"definitions_found"` // Tokens with definitions
    Tokens           []Token  `json:"tokens"`            // Detailed analysis
    Clauses          []Clause `json:"clauses"`           // Sentence structure
    ProcessedAt      string   `json:"processed_at"`      // Processing timestamp
}

type Token struct {
    Text           string   `json:"text"`            // Original text
    Lemma          string   `json:"lemma"`           // Dictionary form
    Reading        string   `json:"reading"`         // Pronunciation
    POS            string   `json:"pos"`             // Part of speech
    Meanings       []string `json:"meanings"`        // English definitions
    Furigana       string   `json:"furigana"`        // Reading aids
    IsConjugated   bool     `json:"is_conjugated"`   // Is this conjugated?
    Conjugation    string   `json:"conjugation"`     // Conjugation type
    HasAuxiliaries bool     `json:"has_auxiliaries"` // Has helper verbs?
    // ... additional fields
}
```

## Integration Examples

### Language Interpreter Integration

```go
// In your interpreter's initialization
parser, err := parser.New()
if err != nil {
    return err
}

// In your interpreter's main loop
func processUserInput(input string) {
    result, err := parser.Parse(input)
    if err != nil {
        handleParseError(err)
        return
    }
    
    // Analyze the parsed result for your interpreter logic
    for _, token := range result.Tokens {
        if token.POS == "名詞,固有名詞,人名,*" {
            // User mentioned a person's name
            handlePersonName(token.Text, token.Meanings)
        } else if token.IsConjugated {
            // Handle conjugated verbs
            handleVerb(token.Lemma, token.Conjugation)
        }
    }
}
```

### Batch Processing

```go
sentences := []string{
    "おはよう",
    "今日は何をしますか？", 
    "映画を見たいです",
}

results := make([]*parser.ParseResult, len(sentences))
for i, sentence := range sentences {
    result, err := parser.Parse(sentence)
    if err != nil {
        log.Printf("Failed to parse: %v", err)
        continue
    }
    results[i] = result
    
    // Process each result in your application
    processResult(result)
}
```

### Extract Specific Information

```go
// Get just pronunciation info
readings, err := parser.GetReadings("こんにちは世界")
// ["コンニチハ", "セカイ"]

// Get just translation info  
meanings, err := parser.GetMeanings("こんにちは世界")
// [["hello", "good day"], ["world", "society"]]

// Lightweight token processing
tokens, err := parser.ParseSimple("こんにちは世界")
for _, token := range tokens {
    fmt.Printf("%s = %s\n", token.Text, token.Meanings[0])
}
```

## Configuration

### Default Configuration (Recommended)

```go
// Uses embedded dictionary paths, no logging
parser, err := parser.New()
```

### Custom Configuration

```go
config := &parser.Config{
    JMdictPath:   "/custom/path/to/JMdict_e",
    EnamdictPath: "/custom/path/to/enamdict", 
    KanjidicPath: "/custom/path/to/kanjidic2.xml",
    SaveLogs:     false, // Recommended: false for library use
    Debug:        false,
}

parser, err := parser.NewWithConfig(config)
```

## Prerequisites

You need these dictionary files in your `dict/` directory:
- **JMdict_e**: Japanese-English dictionary
- **enamdict**: Proper names dictionary  
- **kanjidic2.xml**: Kanji information

See [Setup Guide](docs/SETUP.md) for download instructions.

## Examples

See the `examples/` directory for complete integration examples:
- `minimal_integration.go`: Basic usage patterns
- `library_usage.go`: Advanced integration techniques

## Performance Notes

- **Initialization**: Create the parser once and reuse it
- **Dictionary Loading**: Happens once during initialization
- **Thread Safety**: Create separate parser instances for concurrent use
- **Memory Usage**: Parser keeps dictionaries in memory for fast access

## Error Handling

```go
result, err := parser.Parse(text)
if err != nil {
    // Handle parsing errors
    log.Printf("Parse failed: %v", err)
    return
}

if result.DefinitionsFound == 0 {
    // Handle case where no definitions were found
    log.Printf("No dictionary entries found for: %s", text)
}
```

## License

MIT License - see [LICENSE](LICENSE) file for details.

## Support

This library is designed for programmatic use. For standalone command-line usage, see the `cmd/parser/` directory.

## Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

See [CONTRIBUTING.md](CONTRIBUTING.md) for detailed guidelines.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Acknowledgments

- [MeCab](https://taku910.github.io/mecab/) for morphological analysis
- [JMdict](http://www.edrdg.org/jmdict/j_jmdict.html) for Japanese-English dictionary data
- [ENAMDICT](http://www.edrdg.org/enamdict/enamdict_doc.html) for proper names
- [Kanjidic2](http://www.edrdg.org/wiki/index.php/KANJIDIC_Project) for kanji information

## Support

If you encounter any issues or have questions:

1. Check the [documentation](docs/)
2. Search existing [issues](https://github.com/williambechard/japaneseparse/issues)
3. Create a new issue with detailed information about your problem
