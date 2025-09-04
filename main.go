package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

func main() {
	// replace CLI flag with a const text to make running `go run main.go` simple
	const text = "気象庁によりますと、4日午前3時、熱帯低気圧が奄美大島の東の海上で台風15号に変わりました。"

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

	// print tokens as JSON so you can inspect the tokenizer output
	tokOut, _ := json.MarshalIndent(tokenized.Tokens, "", "  ")
	fmt.Println(string(tokOut))

	// write tokens to logs/<id>_tokens.json
	if err := LogJSON("logs", s.ID+"_tokens", tokenized.Tokens); err != nil {
		fmt.Println("failed to write token log:", err)
	}

	// lookup
	entries, err := Lookup(ctx, tokenized.Tokens)
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

	out, _ := json.MarshalIndent(analysis, "", "  ")
	fmt.Println(string(out))

	// write analysis to logs/<id>_analysis.json
	if err := LogJSON("logs", s.ID+"_analysis", analysis); err != nil {
		fmt.Println("failed to write analysis log:", err)
	}
}
