# Setup Guide

This guide will help you set up the Japanese Text Parser on your system.

## Prerequisites

### 1. Go Programming Language

Install Go 1.21 or higher:

**Windows:**
- Download from [golang.org](https://golang.org/dl/)
- Run the installer and follow the instructions

**macOS:**
```bash
# Using Homebrew
brew install go

# Or download from golang.org
```

**Linux (Ubuntu/Debian):**
```bash
# Using package manager
sudo apt update
sudo apt install golang-go

# Or download from golang.org for latest version
```

### 2. MeCab (Morphological Analyzer)

MeCab is required for Japanese text tokenization.

**Windows:**
1. Download MeCab from the [official site](https://taku910.github.io/mecab/)
2. Install the MeCab binary and dictionaries
3. Add MeCab to your system PATH

**macOS:**
```bash
# Using Homebrew
brew install mecab
brew install mecab-ipadic
```

**Linux (Ubuntu/Debian):**
```bash
sudo apt update
sudo apt install mecab mecab-ipadic-utf8
```

### 3. Dictionary Files

You need three dictionary files:

#### JMdict (Japanese-English Dictionary)
```bash
# Download JMdict_e (English version)
wget http://ftp.edrdg.org/pub/Nihongo/JMdict_e.gz
gunzip JMdict_e.gz
mkdir -p dict
mv JMdict_e dict/
```

#### ENAMDICT (Proper Names Dictionary)
```bash
# Download ENAMDICT
wget http://ftp.edrdg.org/pub/Nihongo/enamdict.gz
gunzip enamdict.gz
mv enamdict dict/
```

#### Kanjidic2 (Kanji Dictionary)
```bash
# Download Kanjidic2
wget http://ftp.edrdg.org/pub/Nihongo/kanjidic2.xml.gz
gunzip kanjidic2.xml.gz
mv kanjidic2.xml dict/
```

## Installation

### Method 1: Build from Source

```bash
# Clone the repository
git clone https://github.com/williambechard/japaneseparse.git
cd japaneseparse

# Download Go dependencies
go mod download

# Build the parser
go build -o bin/japanese-parser cmd/parser/main.go

# Make executable (Linux/macOS)
chmod +x bin/japanese-parser
```

### Method 2: Using Makefile

```bash
# Clone and navigate to project
git clone https://github.com/williambechard/japaneseparse.git
cd japaneseparse

# Install dependencies and build
make deps
make build
```

## Configuration

### Default Configuration

The parser works with default paths, but you may need to adjust them for your system.

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

### Environment Variables

Alternatively, set environment variables:

**Windows (PowerShell):**
```powershell
$env:JMDICT_PATH = "C:\path\to\dict\JMdict_e"
$env:ENAMDICT_PATH = "C:\path\to\dict\enamdict"
$env:KANJIDIC_PATH = "C:\path\to\dict\kanjidic2.xml"
```

**Linux/macOS:**
```bash
export JMDICT_PATH="/usr/share/dict/JMdict_e"
export ENAMDICT_PATH="/usr/share/dict/enamdict"
export KANJIDIC_PATH="/usr/share/dict/kanjidic2.xml"
```

## Verification

Test your installation:

```bash
# Simple test
./bin/japanese-parser -text "こんにちは"

# Expected output should include token analysis with readings and definitions
```

### Troubleshooting

**Error: "Failed to load dictionaries"**
- Check that dictionary files exist at the specified paths
- Verify file permissions (readable)
- Ensure files are not corrupted (try re-downloading)

**Error: "MeCab initialization failed"**
- Verify MeCab is installed and in your PATH
- Check MeCab dictionary installation
- Try running `mecab --version` to test MeCab

**Error: "No such file or directory"**
- Ensure the binary was built successfully
- Check file permissions
- Verify you're running from the correct directory

## Advanced Setup

### Custom MeCab Dictionary

If you want to use a different MeCab dictionary:

```bash
# Install NEologd (recommended for better accuracy)
git clone --depth 1 https://github.com/neologd/mecab-ipadic-neologd.git
cd mecab-ipadic-neologd
./bin/install-mecab-ipadic-neologd -n
```

Then update your MeCab configuration to use the new dictionary.

### Docker Setup

For a containerized setup:

```dockerfile
FROM golang:1.21-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git make

# Copy source code
WORKDIR /app
COPY . .

# Build the application
RUN make build

FROM alpine:latest

# Install runtime dependencies
RUN apk add --no-cache mecab mecab-ipadic

# Copy binary and dictionaries
COPY --from=builder /app/bin/japanese-parser /usr/local/bin/
COPY --from=builder /app/dict /usr/share/dict

# Set default configuration
ENV JMDICT_PATH=/usr/share/dict/JMdict_e
ENV ENAMDICT_PATH=/usr/share/dict/enamdict
ENV KANJIDIC_PATH=/usr/share/dict/kanjidic2.xml

ENTRYPOINT ["japanese-parser"]
```

### Performance Optimization

For better performance with large texts:

1. **Increase memory allocation:**
   ```bash
   export GOGC=200  # Reduce GC frequency
   ```

2. **Use SSD storage** for dictionary files

3. **Pre-load dictionaries** by keeping the process running

## Next Steps

Once setup is complete:

1. Read the [API Documentation](API.md) for detailed usage
2. Check out [examples](examples/) for common use cases
3. Explore configuration options in the main [README](README.md)

## Getting Help

If you encounter issues during setup:

1. Check the [troubleshooting section](#troubleshooting) above
2. Search [existing issues](https://github.com/williambechard/japaneseparse/issues)
3. Create a new issue with:
   - Your operating system and version
   - Go version (`go version`)
   - MeCab version (`mecab --version`)
   - Complete error messages
   - Steps to reproduce the problem
