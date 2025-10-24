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

### Step 1: Install the Library

```bash
# Option 1: From GitHub (main branch)
go get github.com/williambechard/japaneseparse

# Option 2: From a specific branch (e.g., clean-up)
go get github.com/williambechard/japaneseparse@clean-up

# Option 3: Local development with replace directive
go mod edit -replace github.com/williambechard/japaneseparse=/path/to/this/project
go mod tidy
```

### Step 2: Set Up Dictionary Files

**Required**: Download dictionary files (not included in the library due to size):

1. **Create a `dict/` folder** in your project root:
   ```bash
   mkdir dict
   ```

2. **Download the required dictionaries**:
   - **JMdict_e**: Download from [JMdict](http://www.edrdg.org/jmdict/j_jmdict.html)
   - **enamdict**: Download from [ENAMDICT](http://www.edrdg.org/enamdict/enamdict_doc.html)
   - **kanjidic2.xml**: Download from [Kanjidic2](http://www.edrdg.org/wiki/index.php/KANJIDIC_Project)

3. **Place files in your `dict/` folder**:
   ```
   your-project/
   ├── dict/
   │   ├── JMdict_e
   │   ├── enamdict
   │   └── kanjidic2.xml
   ├── main.go
   └── go.mod
   ```

4. **Add to `.gitignore`** (dictionaries are large):
   ```bash
   echo "dict/" >> .gitignore
   echo "logs/" >> .gitignore
   ```

See [docs/SETUP.md](docs/SETUP.md) for detailed download instructions.

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
    // Initialize parser with dictionary paths
    config := &parser.Config{
        JMdictPath:   "dict/JMdict_e",
        EnamdictPath: "dict/enamdict",
        KanjidicPath: "dict/kanjidic2.xml",
        SaveLogs:     false,  // Set true for debugging
        LogsDir:      "logs", // Where to save logs (if SaveLogs=true)
    }
    
    p, err := parser.NewWithConfig(config)
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
        fmt.Printf("- %s [%s] (%s)\n", 
            token.Text, 
            token.FuriganaText,
            token.POSEnglish,
        )
        if len(token.DictionaryEntry.Glosses) > 0 {
            fmt.Printf("  Meaning: %s\n", token.DictionaryEntry.Glosses[0])
        }
    }
}
```

**Note**: If `SaveLogs=true`, the parser will automatically create the `logs/` directory (if missing) and write detailed analysis files there for debugging. Add `logs/` to your `.gitignore`.

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

### Option 1: Custom Configuration (Recommended)

```go
config := &parser.Config{
    JMdictPath:   "dict/JMdict_e",
    EnamdictPath: "dict/enamdict",
    KanjidicPath: "dict/kanjidic2.xml",
    SaveLogs:     false, // Set to true for debugging
    LogsDir:      "logs", // Directory for log output
    Debug:        false,
}

parser, err := parser.NewWithConfig(config)
```

### Option 2: Environment-Based Configuration

```go
import "os"

dictPath := os.Getenv("DICT_PATH")
if dictPath == "" {
    dictPath = "dict" // default
}

config := &parser.Config{
    JMdictPath:   dictPath + "/JMdict_e",
    EnamdictPath: dictPath + "/enamdict",
    KanjidicPath: dictPath + "/kanjidic2.xml",
    SaveLogs:     false,
}
```

### Configuration Fields

- **JMdictPath**: Path to JMdict_e dictionary file (required)
- **EnamdictPath**: Path to enamdict dictionary file (required)
- **KanjidicPath**: Path to kanjidic2.xml file (required)
- **SaveLogs**: Set `true` to generate detailed analysis logs (creates files in LogsDir)
- **LogsDir**: Directory where logs are saved (default: "logs")
- **Debug**: Enable debug mode for additional console output

**Important**: 
- Dictionary files must exist before initializing the parser
- If `SaveLogs=true`, ensure the `LogsDir` directory exists or will be created
- Add `logs/` and `dict/` to your `.gitignore`

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

### Dictionary Files

You need these dictionary files in your `dict/` directory:

| File | Description | Download Link |
|------|-------------|---------------|
| **JMdict_e** | Japanese-English dictionary | [JMdict Project](http://www.edrdg.org/jmdict/j_jmdict.html) |
| **enamdict** | Proper names dictionary | [ENAMDICT Project](http://www.edrdg.org/enamdict/enamdict_doc.html) |
| **kanjidic2.xml** | Kanji information database | [Kanjidic2 Project](http://www.edrdg.org/wiki/index.php/KANJIDIC_Project) |

**Setup Steps**:

```bash
# 1. Create dict folder in your project
mkdir dict

# 2. Download files (see links above) and place in dict/

# 3. Verify files exist
ls dict/
# Should show: JMdict_e  enamdict  kanjidic2.xml

# 4. Add to .gitignore
echo "dict/" >> .gitignore
echo "logs/" >> .gitignore
```

See [docs/SETUP.md](docs/SETUP.md) for detailed download and setup instructions.

### Project Structure

Your project should look like this:

```
your-project/
├── dict/                    # Dictionary files (gitignored)
│   ├── JMdict_e
│   ├── enamdict
│   └── kanjidic2.xml
├── logs/                    # Generated logs (if SaveLogs=true, gitignored)
├── main.go                  # Your code
├── go.mod
├── go.sum
└── .gitignore
```

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