package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/williambechard/JapaneseParse/dictionary"
	"github.com/williambechard/JapaneseParse/tokenize"
)

func main() {
	// Load dictionaries once at startup
	if err := dictionary.InitDictionaries("dict/JMdict_e", "dict/enamdict"); err != nil {
		fmt.Println("Failed to load dictionaries:", err)
		return
	}

	// Load Kanjidic2 at startup for furigana alignment
	if err := tokenize.InitKanjidic2("dict/kanjidic2.xml"); err != nil {
		fmt.Println("Failed to load Kanjidic2:", err)
		return
	}
	// --- DEBUG: Print kanjiReadingMap status ---
	fmt.Printf("Kanjidic2 loaded: %d kanji entries\n", len(tokenize.KanjiReadingMap))
	fmt.Printf("秋 readings: %v\n", tokenize.GetKanjiReadings('秋'))
	fmt.Printf("田 readings: %v\n", tokenize.GetKanjiReadings('田'))

	dictionary.DebugGlossaryFields()

	// replace CLI flag with a const text to make running `go run main.go` simple
	const text = "秋田県仙北市は市内を流れる入見内川の水位が高まっているため、午前8時40分、角館町西長野の283世帯649人に高齢者等避難の情報を出しました。5段階の警戒レベルのうちレベル3に当たる情報で高齢者や体の不自由な人などに避難を始めるよう呼びかけています。"

	// initialize logs directory (clear existing .json files)
	if err := InitLogs("logs"); err != nil {
		fmt.Println("failed to init logs:", err)
		return
	}

	// ingest
	s, err := IngestSentence(text)
	if err != nil {
		fmt.Println("ingest error:", err)
		return
	}

	// start pipeline for this sentence (simple asynchronous tokenizer)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// start the background tokenizer which consumes IngestChan -> TokenizedChan
	StartTokenizer(ctx)

	// wait for tokenization result for this sentence
	var tokenized Tokenized
	select {
	case t := <-TokenizedChan:
		// ensure we got the matching sentence (simple check)
		if t.Sentence.ID != s.ID {
			// if not matching, keep waiting for a short time for the expected one
			// (in a real system you'd correlate by ID more robustly)
			select {
			case t2 := <-TokenizedChan:
				tokenized = t2
			case <-time.After(2 * time.Second):
				fmt.Println("timed out waiting for matching tokenized sentence")
				return
			}
		} else {
			tokenized = t
		}
	case <-ctx.Done():
		fmt.Println("timeout waiting for tokenization:", ctx.Err())
		return
	}

	// merge verb+auxiliary tokens
	mergedTokens := MergeVerbAuxiliaries(tokenized.Tokens)

	// output both original and merged tokens
	tokensOut := map[string]interface{}{
		"original_tokens": tokenized.Tokens,
		"merged_tokens":   mergedTokens,
	}

	// print tokens as JSON so you can inspect the tokenizer output
	tokOut, _ := json.MarshalIndent(tokensOut, "", "  ")
	fmt.Println(string(tokOut))

	// write tokens to logs/<id>_tokens.json
	if err := LogJSON("logs", s.ID+"_tokens", tokensOut); err != nil {
		fmt.Println("failed to write token log:", err)
	}

	// dictionary lookup (new step)
	dictEntries, err := LookupDictionary(ctx, mergedTokens)
	if err != nil {
		fmt.Println("dictionary lookup error:", err)
		return
	}
	// enrich mergedTokens with dictionary entries
	for i := range mergedTokens {
		mergedTokens[i].DictionaryEntry = dictEntries[i]
	}

	// update furigana using dictionary data for best accuracy
	mergedTokens = UpdateFuriganaFromDictionary(mergedTokens)

	// log enriched tokens
	if err := LogJSON("logs", s.ID+"_enriched_tokens", mergedTokens); err != nil {
		fmt.Println("failed to write enriched token log:", err)
	}

	// log dictionary results
	if err := LogJSON("logs", s.ID+"_dict", dictEntries); err != nil {
		fmt.Println("failed to write dictionary log:", err)
	}

	// lookup (legacy, for analysis)
	entries, err := Lookup(ctx, mergedTokens)
	if err != nil {
		fmt.Println("lookup error:", err)
		return
	}

	// analyze
	analysis, err := Analyze(ctx, tokenized.Sentence, entries)
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
	if err := LogJSON("logs", s.ID+"_merged", mergedOutput); err != nil {
		fmt.Println("failed to write merged output log:", err)
	}
	out, _ := json.MarshalIndent(mergedOutput, "", "  ")
	fmt.Println(string(out))

	// write analysis to logs/<id>_analysis.json
	if err := LogJSON("logs", s.ID+"_analysis", analysis); err != nil {
		fmt.Println("failed to write analysis log:", err)
	}
}
