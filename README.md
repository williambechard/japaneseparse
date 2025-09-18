# Japanese Text Parser

A comprehensive Japanese text analysis tool that performs morphological analysis, dictionary lookups, and grammatical structure detection. This tool is designed to help understand Japanese text by breaking it down into tokens, providing readings, definitions, and grammatical information.

## Features

- **Morphological Analysis**: Uses MeCab for tokenization and part-of-speech tagging
- **Dictionary Integration**: JMdict and ENAMDICT support for comprehensive definitions
- **Furigana Generation**: Automatic reading aids for kanji characters
- **Grammar Analysis**: Clause detection and verb auxiliary merging
- **Flexible Output**: Both human-readable and JSON output formats
- **Configurable**: YAML configuration with environment variable overrides
- **Logging**: Optional detailed logging of intermediate processing steps

## Quick Start

### Prerequisites

1. **Go 1.21 or higher**
2. **MeCab and dictionaries** (see [Setup Guide](SETUP.md))

### Installation

```bash
# Clone the repository
git clone https://github.com/williambechard/japaneseparse.git
cd japaneseparse

# Download dependencies
go mod download

# Build the parser
go build -o bin/japanese-parser cmd/parser/main.go
```

### Basic Usage

```bash
# Analyze a simple sentence
./bin/japanese-parser -text "こんにちは世界"

# Analyze text from a file
./bin/japanese-parser -file input.txt

# Get JSON output
./bin/japanese-parser -text "こんにちは世界" -json

# Verbose mode with detailed information
./bin/japanese-parser -text "こんにちは世界" -verbose
```

### Configuration

Create a `config.yaml` file:

```yaml
dictionary:
  jmdict_path: "dict/JMdict_e"
  enamdict_path: "dict/enamdict"
  kanjidic_path: "dict/kanjidic2.xml"
output:
  logs_dir: "logs"
  save_logs: true
  verbose: false
debug: false
```

Use custom configuration:

```bash
./bin/japanese-parser -config config.yaml -text "文章を解析します"
```

## Output Format

### Human-Readable Output

```
=== Japanese Text Analysis ===
Sentence ID: abc123def456
Tokens: 5
Definitions found: 4
Processed at: 2024-01-15 10:30:45

=== Token Analysis ===
1. こんにちは [コンニチハ] <感動詞,*,*,*,*,*,こんにちは,コンニチハ,コンニチハ>
   Meanings: hello; good day; good afternoon
   Source: JMdict

2. 世界 [セカイ] <名詞,一般,*,*,*,*,世界,セカイ,セカイ>
   Furigana: [せ][かい]
   Meanings: the world; society; the universe
   Source: JMdict
```

### JSON Output

The JSON output follows the same structure as the `_merged.json` files, containing:

- **sentence_id**: Unique identifier for the analysis
- **token_count**: Number of tokens in the sentence
- **tokens**: Array of detailed token information including:
  - Morphological analysis (POS, readings, etc.)
  - Dictionary entries with definitions
  - Furigana for kanji
  - Conjugation information
  - Auxiliary verb information
- **analysis**: Grammatical structure analysis including clause boundaries

See the [API Documentation](API.md) for the complete JSON schema.

## Configuration Options

### Dictionary Paths

Configure paths to your dictionary files:

```yaml
dictionary:
  jmdict_path: "dict/JMdict_e"        # JMdict dictionary file
  enamdict_path: "dict/enamdict"      # ENAMDICT proper names dictionary
  kanjidic_path: "dict/kanjidic2.xml" # Kanjidic2 kanji dictionary
```

### Output Settings

```yaml
output:
  logs_dir: "logs"      # Directory for detailed log files
  save_logs: true       # Save intermediate processing steps
  verbose: false        # Enable verbose console output
```

### Environment Variables

Override configuration with environment variables:

```bash
export JMDICT_PATH="/usr/share/dict/JMdict_e"
export ENAMDICT_PATH="/usr/share/dict/enamdict"
export KANJIDIC_PATH="/usr/share/dict/kanjidic2.xml"
export LOGS_DIR="./analysis_logs"
export SAVE_LOGS=true
export VERBOSE=true
export DEBUG=false
```

## Development

### Project Structure

```
japaneseparse/
├── cmd/parser/          # Main executable
├── internal/            # Private packages
│   ├── analyzer/        # Core analysis orchestrator
│   └── config/          # Configuration management
├── pkg/types/           # Public API types
├── configs/             # Configuration examples
├── docs/                # Documentation
├── analyze/             # Grammar analysis
├── dictionary/          # Dictionary lookup
├── tokenize/            # Tokenization logic
└── ... (other packages)
```

### Building

```bash
# Build for current platform
make build

# Development build with race detection
make dev-build

# Run tests
make test

# Format code
make fmt
```

### Testing

```bash
# Run all tests
go test ./...

# Test with coverage
make test-coverage

# Test specific package
go test ./internal/analyzer
```

## API Usage

You can also use this as a library in your Go projects:

```go
package main

import (
    "fmt"
    "japaneseparse/internal/analyzer"
    "japaneseparse/internal/config"
)

func main() {
    // Load configuration
    cfg, err := config.Load("config.yaml")
    if err != nil {
        panic(err)
    }

    // Create analyzer
    analyzer := analyzer.New(cfg)
    if err := analyzer.Initialize(); err != nil {
        panic(err)
    }

    // Analyze text
    result, err := analyzer.AnalyzeText("こんにちは世界")
    if err != nil {
        panic(err)
    }

    fmt.Printf("Found %d tokens\n", result.TokenCount)
    for _, token := range result.Tokens {
        fmt.Printf("Token: %s, Reading: %s\n", token.Text, token.Reading)
    }
}
```

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
