package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"japaneseparse/analyze"
	"japaneseparse/dictionary"
	"japaneseparse/ingest"
	"japaneseparse/kanji"
	"japaneseparse/logger"
	"japaneseparse/lookup"
	"japaneseparse/model"
	"japaneseparse/tokenize"
)

func main() {
	// Load dictionaries once at startup
	if err := dictionary.InitDictionaries("dict/JMdict_e", "dict/enamdict"); err != nil {
		fmt.Println("Failed to load dictionaries:", err)
		return
	}

	// Load Kanjidic2 at startup for furigana alignment
	if err := kanji.InitKanjidic2("dict/kanjidic2.xml"); err != nil {
		fmt.Println("Failed to load Kanjidic2:", err)
		return
	}
	// --- DEBUG: Print kanjiReadingMap status ---
	fmt.Printf("Kanjidic2 loaded: %d kanji entries\n", kanji.Count())
	fmt.Printf("秋 readings: %v\n", kanji.GetKanjiReadings('秋'))
	fmt.Printf("田 readings: %v\n", kanji.GetKanjiReadings('田'))

	dictionary.DebugGlossaryFields()

	// replace CLI flag with a const text to make running `go run main.go` simple
	const text = "秋田県仙北市は市内を流れる入見内川の水位が高まっているため、午前8時40分、角館町西長野の283世帯649人に高齢者等避難の情報を出しました。5段階の警戒レベルのうちレベル3に当たる情報で高齢者や体の不自由な人などに避難を始めるよう呼びかけています。"

	// initialize logs directory (clear existing .json files)
	if err := logger.InitLogs("logs"); err != nil {
		fmt.Println("failed to init logs:", err)
		return
	}

	// ingest
	s, err := ingest.IngestSentence(text)
	if err != nil {
		fmt.Println("ingest error:", err)
		return
	}

	// start pipeline for this sentence (simple asynchronous tokenizer)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// start the background tokenizer which consumes IngestChan -> TokenizedChan
	tokenize.StartTokenizer(context.Background()) // Remove timeout, use background context

	// send sentence to tokenizer pipeline
	ingest.IngestChan <- s

	// wait for tokenization result for this sentence
	var tokenized tokenize.Tokenized
	for {
		t := <-tokenize.TokenizedChan
		if t.Sentence.ID == s.ID {
			tokenized = t
			break
		}
	}

	// merge verb+auxiliary tokens
	mergedTokens := tokenize.MergeVerbAuxiliaries(tokenized.Tokens)

	// output both original and merged tokens
	tokensOut := map[string]interface{}{
		"original_tokens": tokenized.Tokens,
		"merged_tokens":   mergedTokens,
	}

	// print tokens as JSON so you can inspect the tokenizer output
	tokOut, _ := json.MarshalIndent(tokensOut, "", "  ")
	fmt.Println(string(tokOut))

	// write tokens to logs/<id>_tokens.json
	if err := logger.LogJSON("logs", s.ID+"_tokens", tokensOut); err != nil {
		fmt.Println("failed to write token log:", err)
	}

	// dictionary lookup (new step)
	dictEntries, err := dictionary.LookupDictionary(context.Background(), mergedTokens)
	if err != nil {
		fmt.Println("dictionary lookup error:", err)
		return
	}
	// enrich mergedTokens with dictionary entries
	for i := range mergedTokens {
		mergedTokens[i].DictionaryEntry = dictEntries[i]
	}

	// DEBUG: Print all token surfaces after merging and before furigana update
	fmt.Println("Merged token surfaces:")
	for _, t := range mergedTokens {
		fmt.Println(t.Text)
	}

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
	mergedTokens = tokenize.UpdateFuriganaFromDictionary(mergedTokens)

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
	lexEntries, err := lookup.Lookup(ctx, mergedTokens)
	if err != nil {
		fmt.Println("lookup error:", err)
		return
	}
	// Attach dictionary entries to tokens
	for i := range mergedTokens {
		if i < len(lexEntries) {
			mergedTokens[i].DictionaryEntry = model.DictionaryEntry{
				Kanji:    []string{lexEntries[i].Token.Text},
				Readings: lexEntries[i].Readings,
				Glosses:  lexEntries[i].Definitions,
				Source:   "lookup.go",
			}
		}
	}
	analysis, err := analyze.Analyze(context.Background(), s, lexEntries)
	if err != nil {
		fmt.Println("analyze error:", err)
		return
	}

	// --- MERGED OUTPUT ---
	mergedOutput := map[string]interface{}{
		"sentence_id": s.ID,
		"token_count": len(mergedTokens),
		"tokens":      mergedTokens,
		"analysis":    analysis,
	}
	if err := logger.LogJSON("logs", s.ID+"_merged", mergedOutput); err != nil {
		fmt.Println("failed to write merged output log:", err)
	}
	out, _ := json.MarshalIndent(mergedOutput, "", "  ")
	fmt.Println(string(out))

	// write analysis to logs/<id>_analysis.json
	if err := logger.LogJSON("logs", s.ID+"_analysis", analysis); err != nil {
		fmt.Println("failed to write analysis log:", err)
	}
}
