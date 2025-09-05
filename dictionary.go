package main

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"sync"

	jmdict "github.com/yomidevs/jmdict-go"
)

// DictionaryEntry represents a dictionary definition for a token.
type DictionaryEntry struct {
	Text          string   `json:"text"`
	Lemma         string   `json:"lemma"`
	POS           string   `json:"pos"`
	Reading       string   `json:"reading,omitempty"`
	Pronunciation string   `json:"pronunciation,omitempty"`
	Definitions   []string `json:"definitions"`
	Source        string   `json:"source"` // e.g. "JMdict", "EDICT", "placeholder"
}

var (
	jmDict       *jmdict.Jmdict
	jmDictOnce   sync.Once
	jmIndex      map[string][]*jmdict.JmdictEntry
	enamDict     *jmdict.Jmdict
	enamDictOnce sync.Once
	enamIndex    map[string][]*jmdict.JmdictEntry
)

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

// LookupJMdictEntry looks up a token in JMdict and ENAMDICT, aggregating all matching definitions.
func LookupJMdictEntry(token Token) (DictionaryEntry, bool) {

	debugGlossaryFields() // Print JmdictGlossary fields for debugging

	// Try lemma, reading, then surface form
	keys := []string{token.Lemma, token.Reading, token.Text}
	defs := []string{}
	var source string
	found := false
	for _, key := range keys {
		if key == "" {
			continue
		}
		if entries, ok := jmIndex[key]; ok && len(entries) > 0 {
			// for _, entry := range entries {
			// 	for _, sense := range entry.Sense {
			// 		for _, gloss := range sense.Glossary {
			// 			defs = append(defs, gloss.Gloss) // commented for debug
			// 		}
			// 	}
			// }
			source = "JMdict"
			found = true
		}
		if entries, ok := enamIndex[key]; ok && len(entries) > 0 {
			// for _, entry := range entries {
			// 	for _, sense := range entry.Sense {
			// 		for _, gloss := range sense.Glossary {
			// 			defs = append(defs, gloss.Gloss) // commented for debug
			// 		}
			// 	}
			// }
			if !found { // Prefer JMdict if found
				source = "ENAMDICT"
				found = true
			}
		}
	}
	if found && len(defs) > 0 {
		return DictionaryEntry{
			Text:          token.Text,
			Lemma:         token.Lemma,
			POS:           token.POS,
			Reading:       token.Reading,
			Pronunciation: token.Pronunciation,
			Definitions:   defs,
			Source:        source,
		}, true
	}
	// Fallback stub for EDICT/Jisho
	return DictionaryEntry{
		Text:          token.Text,
		Lemma:         token.Lemma,
		POS:           token.POS,
		Reading:       token.Reading,
		Pronunciation: token.Pronunciation,
		Definitions:   []string{"<no definition found>", "(EDICT/Jisho fallback stub)"},
		Source:        "none",
	}, false
}

// convertJMdictEntry converts a jmdict.Entry to DictionaryEntry.
func convertJMdictEntry(token Token, entry *jmdict.JmdictEntry, source string) DictionaryEntry {
	defs := []string{}
	// for _, sense := range entry.Sense {
	// 	for _, gloss := range sense.Glossary {
	// 		defs = append(defs, gloss.Gloss) // commented for debug
	// 	}
	// }
	return DictionaryEntry{
		Text:          token.Text,
		Lemma:         token.Lemma,
		POS:           token.POS,
		Reading:       token.Reading,
		Pronunciation: token.Pronunciation,
		Definitions:   defs,
		Source:        source,
	}
}

// LookupDictionary takes a slice of tokens and returns dictionary entries for each.
func LookupDictionary(ctx context.Context, tokens []Token) ([]DictionaryEntry, error) {
	// Ensure dictionaries are loaded
	LoadJMdict("dict/JMdict_e", "dict/enamdict")
	entries := make([]DictionaryEntry, len(tokens))
	for i, t := range tokens {
		if def, ok := LookupJMdictEntry(t); ok {
			entries[i] = def
		} else {
			entries[i] = DictionaryEntry{
				Text:          t.Text,
				Lemma:         t.Lemma,
				POS:           t.POS,
				Reading:       t.Reading,
				Pronunciation: t.Pronunciation,
				Definitions:   []string{"<no definition found>"},
				Source:        "none",
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
