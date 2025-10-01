# Japanese Text Parser

A comprehensive Japanese text analysis library designed for integration into larger applications. This library provides morphological analysis, dictionary lookups, and grammatical structure detection for Japanese text.

**This is a Go library** - designed to be imported and used by any Go program that needs Japanese text processing capabilities.

## Features

- **Morphological Analysis**: MeCab-based tokenization and part-of-speech tagging
- **Dictionary Integration**: JMdict and ENAMDICT support for comprehensive definitions  
- **Furigana Generation**: Automatic reading aids for kanji characters
- **Grammar Analysis**: Clause detection and verb auxiliary merging
- **Clean API**: Simple integration with minimal dependencies
- **Flexible Output**: Both detailed analysis and simplified token extraction
- **No External Dependencies**: Self-contained once dictionaries are provided

## Installation

### Option 1: Direct Import (if published to GitHub)
```bash
go get github.com/williambechard/japaneseparse
```

### Option 2: Local Development (current setup)
Since this library may not be published yet, you can use it locally:

```bash
# In your go.mod file, add a replace directive:
go mod edit -replace github.com/williambechard/japaneseparse=/path/to/this/project
go mod tidy
```

## Quick Start

### Basic Usage

```go
package main

import (
    "fmt"
    "log"
    "github.com/williambechard/japaneseparse/pkg/parser"
)

func main() {
    // Initialize parser
    p, err := parser.New()
    if err != nil {
        log.Fatal(err)
    }

    // Parse Japanese text (comprehensive analysis)
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

## Main Library Functions

The library provides three main functions as requested:

### 1. `parser.Parse(text)` - Get Complete Analysis
Returns comprehensive analysis including tokens, meanings, readings, and grammar structure.

```go
result, err := parser.Parse("私は学校に行きます")
// Returns: *ParseResult with full linguistic analysis
```

### 2. `parser.Analyze(text)` - Get Everything (Alias for Parse)
Identical to Parse - provides complete analysis of Japanese text.

```go
result, err := parser.Analyze("私は学校に行きます")  
// Returns: Same as Parse - comprehensive analysis
```

### 3. `parser.GetMeaning(word)` - Get Meaning of Single Word
Returns the first/primary meaning of a single word.

```go
meaning, err := parser.GetMeaning("学校")
// Returns: "school"
```

## Additional Helper Functions

```go
// Get simplified token list
tokens, err := parser.ParseSimple("日本語のテキスト") 

// Extract just readings (for pronunciation)
readings, err := parser.GetReadings("日本語のテキスト")

// Extract meanings for all words (for translation)
meanings, err := parser.GetMeanings("日本語のテキスト")
```

## Data Structures

```go
type ParseResult struct {
    Text             string         `json:"text"`              // Original input
    SentenceID       string         `json:"sentence_id"`       // Unique identifier  
    TokenCount       int            `json:"token_count"`       // Number of tokens
    DefinitionsFound int            `json:"definitions_found"` // Tokens with definitions
    Tokens           []Token        `json:"tokens"`            // Detailed analysis
    Clauses          []types.Clause `json:"clauses"`           // Sentence structure
    ProcessedAt      string         `json:"processed_at"`      // Processing timestamp
}

type Token struct {
    Text           string   `json:"text"`            // Original text
    Lemma          string   `json:"lemma"`           // Dictionary form
    Reading        string   `json:"reading"`         // Pronunciation
    POS            string   `json:"pos"`             // Part of speech
    Meanings       []string `json:"meanings"`        // English definitions
    Furigana       string   `json:"furigana"`        // Reading aids
    IsConjugated   bool     `json:"is_conjugated"`   // Is this conjugated?
    Conjugation    []string `json:"conjugation"`     // Conjugation details
    HasAuxiliaries bool     `json:"has_auxiliaries"` // Has helper verbs?
    // ... additional fields for advanced use
}
```

## Configuration

### Default Configuration (Recommended for Library Use)

```go
// Uses default dictionary paths, no logging
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

## Integration Examples

### Language Interpreter Integration

```go
package main

import (
    "log"
    "github.com/williambechard/japaneseparse/pkg/parser"
)

// In your interpreter's initialization
func initializeInterpreter() (*parser.Parser, error) {
    p, err := parser.New()
    if err != nil {
        return nil, err
    }
    return p, nil
}

// In your interpreter's main loop
func processUserInput(p *parser.Parser, input string) {
    // Method 1: Get complete analysis
    result, err := p.Parse(input)
    if err != nil {
        log.Printf("Parse error: %v", err)
        return
    }
    
    // Method 2: Alternative - use Analyze (same functionality)
    result, err = p.Analyze(input)
    
    // Method 3: Get meaning of specific words
    for _, token := range result.Tokens {
        meaning, err := p.GetMeaning(token.Text)
        if err == nil {
            log.Printf("Word: %s, Meaning: %s", token.Text, meaning)
        }
    }
    
    // Your interpreter logic here...
    processTokensForInterpreter(result.Tokens)
}
```

### Batch Processing

```go
func batchProcessing() {
    p, err := parser.New()
    if err != nil {
        log.Fatal(err)
    }

    sentences := []string{
        "おはよう",
        "今日は何をしますか？", 
        "映画を見たいです",
    }

    for _, sentence := range sentences {
        result, err := p.Parse(sentence)
        if err != nil {
            log.Printf("Failed to parse '%s': %v", sentence, err)
            continue
        }
        
        // Process each sentence result
        fmt.Printf("Processed: %s (%d tokens)\n", result.Text, result.TokenCount)
    }
}
```

### Extract Specific Information

```go
// Get just pronunciation info
readings, err := parser.GetReadings("こんにちは世界")
// Returns: ["コンニチハ", "セカイ"]

// Get just translation info  
meanings, err := parser.GetMeanings("こんにちは世界")
// Returns: [["hello", "good day"], ["world", "society"]]

// Lightweight token processing
tokens, err := parser.ParseSimple("こんにちは世界")
for _, token := range tokens {
    if len(token.Meanings) > 0 {
        fmt.Printf("%s = %s\n", token.Text, token.Meanings[0])
    }
}
```

## Prerequisites

You need these dictionary files in your `dict/` directory:
- **JMdict_e**: Japanese-English dictionary
- **enamdict**: Proper names dictionary  
- **kanjidic2.xml**: Kanji information

See [Setup Guide](docs/SETUP.md) for download instructions.

## Examples

See the `examples/` directory for complete integration examples:
- `examples/minimal_integration/`: Basic usage patterns
- `examples/library_usage/`: Advanced integration techniques
- `examples/test_api/`: API verification examples
- `examples/test_three_functions/`: Test the three main functions

## Library Structure

**This project is now a proper Go library** with the following structure:

```
github.com/williambechard/japaneseparse/
├── pkg/parser/          # 🎯 Main library API - import this!
├── examples/            # Usage examples  
├── cmd/                 # Command-line tools (optional)
├── internal/           # Internal implementation
├── model/              # Data structures
└── docs/               # Documentation
```

**Key Library Functions:**
- `parser.Parse(text)` - Complete analysis
- `parser.Analyze(text)` - Same as Parse (alias)  
- `parser.GetMeaning(word)` - Single word meaning
- `parser.ParseSimple(text)` - Just tokens
- `parser.GetReadings(text)` - Just readings
- `parser.GetMeanings(text)` - All meanings

## Performance Notes

- **Initialization**: Create the parser once and reuse it
- **Dictionary Loading**: Happens once during initialization (takes a few seconds)
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

## Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

MIT License - see [LICENSE](LICENSE) file for details.

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