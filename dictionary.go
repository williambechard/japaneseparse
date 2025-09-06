package main

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"strings"
	"sync"
	"unicode"

	jmdict "github.com/yomidevs/jmdict-go"
)

// DictionaryEntry represents enriched dictionary info for a token.
type DictionaryEntry struct {
	Kanji      []string `json:"kanji,omitempty"`
	Readings   []string `json:"readings,omitempty"`
	Glosses    []string `json:"glosses,omitempty"`
	POS        []string `json:"pos,omitempty"`
	UsageNotes []string `json:"usage_notes,omitempty"`
	Examples   []string `json:"examples,omitempty"`
	Frequency  string   `json:"frequency,omitempty"`
	Source     string   `json:"source"`
}

var (
	jmDict       *jmdict.Jmdict
	jmDictOnce   sync.Once
	jmIndex      map[string][]*jmdict.JmdictEntry
	enamDict     *jmdict.Jmdict
	enamDictOnce sync.Once
	enamIndex    map[string][]*jmdict.JmdictEntry
)

// InitDictionaries initializes the dictionaries by loading JMdict and ENAMDICT files.
func InitDictionaries(jmdictPath, enamdictPath string) error {
	return LoadJMdict(jmdictPath, enamdictPath)
}

// LoadJMdict loads JMdict and ENAMDICT files and builds lookup maps, with error logging.
func LoadJMdict(jmdictPath, enamdictPath string) error {
	var err error
	jmDictOnce.Do(func() {
		jmdictFile, fileErr := os.Open(jmdictPath)
		if fileErr != nil {
			LogError("Failed to open JMdict file: " + fileErr.Error())
			return
		}
		defer jmdictFile.Close()
		jmDictVal, _, loadErr := jmdict.LoadJmdict(jmdictFile)
		if loadErr != nil {
			LogError("Failed to load JMdict: " + loadErr.Error())
			return
		}
		jmDict = &jmDictVal
		jmIndex = make(map[string][]*jmdict.JmdictEntry)
		for _, entry := range jmDict.Entries {
			entryPtr := &entry
			for _, k := range entry.Kanji {
				jmIndex[k.Expression] = append(jmIndex[k.Expression], entryPtr)
			}
			for _, r := range entry.Readings {
				jmIndex[r.Reading] = append(jmIndex[r.Reading], entryPtr)
			}
		}
	})
	enamDictOnce.Do(func() {
		enamFile, fileErr := os.Open(enamdictPath)
		if fileErr != nil {
			LogError("Failed to open ENAMDICT file: " + fileErr.Error())
			return
		}
		defer enamFile.Close()
		enamDictVal, _, loadErr := jmdict.LoadJmdict(enamFile)
		if loadErr != nil {
			LogError("Failed to load ENAMDICT: " + loadErr.Error())
			return
		}
		enamDict = &enamDictVal
		enamIndex = make(map[string][]*jmdict.JmdictEntry)
		for _, entry := range enamDict.Entries {
			entryPtr := &entry
			for _, k := range entry.Kanji {
				enamIndex[k.Expression] = append(enamIndex[k.Expression], entryPtr)
			}
			for _, r := range entry.Readings {
				enamIndex[r.Reading] = append(enamIndex[r.Reading], entryPtr)
			}
		}
	})
	return err
}

// LookupJMdictEntry looks up a normalized key in JMdict.
func LookupJMdictEntry(key string) (*jmdict.JmdictEntry, bool) {
	keyNorm := normalizeJapanese(key)
	for dictKey, entries := range jmIndex {
		dictKeyNorm := normalizeJapanese(dictKey)
		if keyNorm == dictKeyNorm || strings.Contains(dictKeyNorm, keyNorm) {
			return entries[0], true
		}
	}
	return nil, false
}

// LookupENAMDICTEntry looks up a normalized key in ENAMDICT.
func LookupENAMDICTEntry(key string) (*jmdict.JmdictEntry, bool) {
	keyNorm := normalizeJapanese(key)
	for dictKey, entries := range enamIndex {
		dictKeyNorm := normalizeJapanese(dictKey)
		if keyNorm == dictKeyNorm || strings.Contains(dictKeyNorm, keyNorm) {
			return entries[0], true
		}
	}
	return nil, false
}

// normalizeJapanese normalizes a Japanese string for dictionary lookup.
func normalizeJapanese(s string) string {
	s = strings.ToLower(s)
	// Convert katakana to hiragana
	var out []rune
	for _, r := range s {
		// Katakana range: U+30A0 to U+30FF
		if r >= 0x30A0 && r <= 0x30FF {
			// Hiragana starts at U+3040
			out = append(out, r-0x60)
		} else if unicode.IsPunct(r) || unicode.IsSpace(r) {
			// Skip punctuation and whitespace
			continue
		} else {
			out = append(out, r)
		}
	}
	return string(out)
}

// convertJMdictEntry converts a JMdict entry to DictionaryEntry with enrichment.
func convertJMdictEntry(jm *jmdict.JmdictEntry) DictionaryEntry {
	var kanji, readings, glosses, pos, misc []string
	if jm == nil {
		return DictionaryEntry{Source: "JMdict"}
	}
	for _, k := range jm.Kanji {
		kanji = append(kanji, k.Expression)
	}
	for _, r := range jm.Readings {
		readings = append(readings, r.Reading)
	}
	for _, s := range jm.Sense {
		for _, g := range s.Glossary {
			glosses = append(glosses, g.Content)
		}
		pos = append(pos, s.PartsOfSpeech...)
		misc = append(misc, s.Misc...)
	}
	return DictionaryEntry{
		Kanji:      kanji,
		Readings:   readings,
		Glosses:    glosses,
		POS:        pos,
		UsageNotes: misc,
		Examples:   nil, // Example extraction can be added if needed
		Frequency:  "",  // Not present in struct
		Source:     "JMdict",
	}
}

// convertENAMDICTEntry converts an ENAMDICT entry to DictionaryEntry with enrichment.
func convertENAMDICTEntry(enam *jmdict.JmdictEntry) DictionaryEntry {
	var kanji, readings, glosses, pos []string
	if enam == nil {
		return DictionaryEntry{Source: "ENAMDICT"}
	}
	for _, k := range enam.Kanji {
		kanji = append(kanji, k.Expression)
	}
	for _, r := range enam.Readings {
		readings = append(readings, r.Reading)
	}
	for _, s := range enam.Sense {
		for _, g := range s.Glossary {
			glosses = append(glosses, g.Content)
		}
		pos = append(pos, s.PartsOfSpeech...)
	}
	return DictionaryEntry{
		Kanji:    kanji,
		Readings: readings,
		Glosses:  glosses,
		POS:      pos,
		Source:   "ENAMDICT",
	}
}

// LookupDictionary takes a slice of tokens and returns dictionary entries for each.
func LookupDictionary(ctx context.Context, tokens []Token) ([]DictionaryEntry, error) {
	// Dictionaries are loaded once at startup via InitDictionaries
	entries := make([]DictionaryEntry, len(tokens))
	for i, t := range tokens {
		if def, ok := LookupJMdictEntry(t.Text); ok {
			entries[i] = convertJMdictEntry(def)
		} else if def, ok := LookupENAMDICTEntry(t.Text); ok {
			entries[i] = convertENAMDICTEntry(def)
		} else {
			entries[i] = DictionaryEntry{
				Kanji:    []string{t.Text},
				Readings: []string{t.Reading},
				Glosses:  []string{"<no definition found>"},
				Source:   "none",
			}
		}
	}
	return entries, nil
}

// LogError logs error messages to logs/errors.log
func LogError(msg string) {
	f, err := OpenLogFile("logs/errors.log")
	if err != nil {
		return
	}
	defer f.Close()
	f.WriteString(msg + "\n")
}

// OpenLogFile opens a log file for appending
func OpenLogFile(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
}

func debugGlossaryFields() {
	var dummy jmdict.JmdictGlossary
	t := reflect.TypeOf(dummy)
	fmt.Println("JmdictGlossary fields:")
	for i := 0; i < t.NumField(); i++ {
		fmt.Println("-", t.Field(i).Name, t.Field(i).Type)
	}
}
