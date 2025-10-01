package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/williambechard/japaneseparse/analyze"
	"github.com/williambechard/japaneseparse/dictionary"
	"github.com/williambechard/japaneseparse/enamdict"
	"github.com/williambechard/japaneseparse/ingest"
	"github.com/williambechard/japaneseparse/kanji"
	"github.com/williambechard/japaneseparse/logger"
	"github.com/williambechard/japaneseparse/lookup"
	"github.com/williambechard/japaneseparse/model"
	"github.com/williambechard/japaneseparse/tokenize"
)

// Helper: Convert katakana to hiragana
func katakanaToHiragana(s string) string {
	runes := []rune(s)
	for i, r := range runes {
		if r >= 0x30A1 && r <= 0x30F6 {
			runes[i] = r - 0x60
		}
	}
	return string(runes)
}

func main() {
	// Load dictionaries once at startup
	logger.Logf("DEBUG: Starting dictionary.InitDictionaries...")
	if err := dictionary.InitDictionaries("dict/JMdict_e", "dict/enamdict"); err != nil {
		fmt.Println("Failed to load dictionaries:", err)
		return
	}
	logger.Logf("DEBUG: Finished dictionary.InitDictionaries.")

	logger.Logf("DEBUG: Starting enamdict.LoadEnamdict...")
	enamdictMap, err := enamdict.LoadEnamdict("dict/enamdict")
	if err != nil {
		fmt.Println("Failed to load ENAMDICT:", err)
		return
	}
	logger.Logf("DEBUG: Finished enamdict.LoadEnamdict. Entries: %d", len(enamdictMap))

	logger.Logf("DEBUG: Starting kanji.InitKanjidic2...")
	if err := kanji.InitKanjidic2("dict/kanjidic2.xml"); err != nil {
		fmt.Println("Failed to load Kanjidic2:", err)
		return
	}
	logger.Logf("DEBUG: Finished kanji.InitKanjidic2.")

	dictionary.DebugGlossaryFields()

	// replace CLI flag with a const text to make running `go run main.go` simple
	const text = "秋田県仙北市は市内を流れる入見内川の水位が高まっているため、午前8時40分、角館町西長野の283世帯649人に高齢者等避難の情報を出しました。5段階の警戒レベルのうちレベル3に当たる情報で高齢者や体の不自由な人などに避難を始めるよう呼びかけています。"

	// initialize logs directory (clear existing .json files)
	logger.Logf("DEBUG: Initializing logs directory...")
	if err := logger.InitLogs("logs"); err != nil {
		fmt.Println("failed to init logs:", err)
		return
	}
	logger.Logf("DEBUG: Logs directory initialized.")

	// ingest
	logger.Logf("DEBUG: Ingesting sentence...")
	s, err := ingest.IngestSentence(text)
	if err != nil {
		fmt.Println("ingest error:", err)
		return
	}
	logger.Logf("DEBUG: Sentence ingested. ID: %s", s.ID)

	// start pipeline for this sentence (simple asynchronous tokenizer)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// start the background tokenizer which consumes IngestChan -> TokenizedChan
	logger.Logf("DEBUG: Starting tokenizer goroutine...")
	tokenize.StartTokenizer(context.Background()) // Remove timeout, use background context
	logger.Logf("DEBUG: Sending sentence to tokenizer pipeline...")
	ingest.IngestChan <- s
	logger.Logf("DEBUG: Waiting for tokenization result...")
	var tokenized tokenize.Tokenized
	for {
		t := <-tokenize.TokenizedChan
		if t.Sentence.ID == s.ID {
			tokenized = t
			break
		}
	}
	logger.Logf("DEBUG: Tokenization complete. Token count: %d", len(tokenized.Tokens))

	// merge verb+auxiliary tokens
	logger.Logf("DEBUG: Merging verb auxiliaries...")
	mergedTokens := tokenize.MergeVerbAuxiliaries(tokenized.Tokens)
	logger.Logf("DEBUG: Merge complete. Merged token count: %d", len(mergedTokens))

	// output both original and merged tokens
	tokensOut := map[string]interface{}{
		"original_tokens": tokenized.Tokens,
		"merged_tokens":   mergedTokens,
	}

	// print tokens as JSON so you can inspect the tokenizer output
	// tokOut, _ := json.MarshalIndent(tokensOut, "", "  ")
	// fmt.Println(string(tokOut))

	// write tokens to logs/<id>_tokens.json
	if err := logger.LogJSON("logs", s.ID+"_tokens", tokensOut); err != nil {
		fmt.Println("failed to write token log:", err)
	}

	// dictionary lookup (new step)
	logger.Logf("DEBUG: Starting dictionary.LookupDictionary...")
	dictEntries, err := dictionary.LookupDictionary(context.Background(), mergedTokens)
	if err != nil {
		fmt.Println("dictionary lookup error:", err)
		return
	}
	logger.Logf("DEBUG: Dictionary lookup complete.")
	// enrich mergedTokens with dictionary entries
	for i := range mergedTokens {
		mergedTokens[i].DictionaryEntry = dictEntries[i]
	}

	// DEBUG: Print all token surfaces after merging and before furigana update
	// fmt.Println("Merged token surfaces:")
	// for _, t := range mergedTokens {
	// 	fmt.Println(t.Text)
	// }

	// DEBUG: Save all token surfaces after merging and before furigana update
	f, err := os.Create("logs/merged_token_surfaces.log")
	if err == nil {
		for _, t := range mergedTokens {
			f.WriteString(t.Text + "\n")
		}
		f.Close()
	} else {
		fmt.Println("Failed to write merged_token_surfaces.log:", err)
	}

	// update furigana using dictionary data for best accuracy
	logger.Logf("DEBUG: Updating furigana from dictionary...")
	mergedTokens = tokenize.UpdateFuriganaFromDictionary(mergedTokens)
	logger.Logf("DEBUG: Furigana update complete.")

	// log enriched tokens
	if err := logger.LogJSON("logs", s.ID+"_enriched_tokens", mergedTokens); err != nil {
		fmt.Println("failed to write enriched token log:", err)
	}

	// log dictionary results
	if err := logger.LogJSON("logs", s.ID+"_dict", dictEntries); err != nil {
		fmt.Println("failed to write dictionary log:", err)
	}

	// --- DICTIONARY LOOKUP & ANALYSIS ---
	// Lookup: enrich tokens with dictionary entries
	logger.Logf("DEBUG: Starting lookup.Lookup...")
	lexEntries, err := lookup.Lookup(ctx, mergedTokens)
	if err != nil {
		fmt.Println("lookup error:", err)
		return
	}
	logger.Logf("DEBUG: lookup.Lookup complete.")
	// Attach dictionary entries to tokens
	for i := range mergedTokens {
		if i < len(lexEntries) {
			// Only overwrite dictionary entry if the lookup produced definitions
			if len(lexEntries[i].Definitions) > 0 {
				// Preserve any existing source (e.g. JMdict) if present; otherwise mark as lookup.go
				src := mergedTokens[i].DictionaryEntry.Source
				if src == "" {
					src = "lookup.go"
				}
				mergedTokens[i].DictionaryEntry = model.DictionaryEntry{
					Kanji:    []string{lexEntries[i].Token.Text},
					Readings: lexEntries[i].Readings,
					Glosses:  lexEntries[i].Definitions,
					Source:   src,
				}
			}
		}
	}
	// update furigana again after lookup enrichment
	logger.Logf("DEBUG: Updating furigana from dictionary (post-lookup)...")
	mergedTokens = tokenize.UpdateFuriganaFromDictionary(mergedTokens)
	logger.Logf("DEBUG: Furigana update complete (post-lookup).")

	logger.Logf("DEBUG: Starting analyze.Analyze...")
	analysis, err := analyze.Analyze(context.Background(), s, lexEntries)
	if err != nil {
		fmt.Println("analyze error:", err)
		return
	}
	logger.Logf("DEBUG: analyze.Analyze complete.")

	// ENAMDICT fallback for tokens with no glosses
	logger.Logf("DEBUG: Starting ENAMDICT fallback for missing glosses...")
	for i := range mergedTokens {
		entry := mergedTokens[i].DictionaryEntry
		if len(entry.Glosses) == 0 || (len(entry.Glosses) == 1 && entry.Glosses[0] == "<no definition found>") {
			lemma := mergedTokens[i].Text
			reading := mergedTokens[i].Reading
			// Convert katakana reading to hiragana
			hiraganaReading := katakanaToHiragana(reading)
			key := lemma + "|" + hiraganaReading
			if enamEntry, ok := enamdictMap[key]; ok {
				mergedTokens[i].DictionaryEntry = model.DictionaryEntry{
					Kanji:    []string{enamEntry.Lemma},
					Readings: []string{enamEntry.Reading},
					Glosses:  []string{enamEntry.Meaning},
					Source:   "ENAMDICT",
				}
			}
		}
	}
	logger.Logf("DEBUG: ENAMDICT fallback complete.")

	// --- MERGED OUTPUT ---
	mergedOutput := map[string]interface{}{
		"sentence_id": s.ID,
		"token_count": len(mergedTokens),
		"tokens":      mergedTokens,
		"analysis":    analysis,
	}
	logger.Logf("DEBUG: Writing merged output log...")
	if err := logger.LogJSON("logs", s.ID+"_merged", mergedOutput); err != nil {
		fmt.Println("failed to write merged output log:", err)
	}
	logger.Logf("DEBUG: Merged output log written.")
	// out, _ := json.MarshalIndent(mergedOutput, "", "  ")
	// fmt.Println(string(out))

	// write analysis to logs/<id>_analysis.json
	logger.Logf("DEBUG: Writing analysis log...")
	if err := logger.LogJSON("logs", s.ID+"_analysis", analysis); err != nil {
		fmt.Println("failed to write analysis log:", err)
	}
	logger.Logf("DEBUG: Analysis log written.")
}
