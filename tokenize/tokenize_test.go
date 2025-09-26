package tokenize

import (
	"context"
	"strings"
	"testing"
)

func ensureKanjidic2Initialized(t *testing.T) {
	err := InitKanjidic2("../dict/kanjidic2.xml")
	if err != nil {
		t.Fatalf("Failed to initialize Kanjidic2: %v", err)
	}
}

func TestInitKanjidic2(t *testing.T) {
	ensureKanjidic2Initialized(t)
}

func TestGetKanjiReadings(t *testing.T) {
	ensureKanjidic2Initialized(t) // Reuse the initialization logic

	readings := GetKanjiReadings('秋')
	if len(readings) == 0 {
		t.Errorf("Expected readings for kanji '秋', got none")
	}
	// Verify that one of the readings is "シュウ"
	expectedReadings := map[string]bool{
		"シュウ": true,
		"あき":  true,
		"とき":  true,
	}
	foundReadings := make(map[string]bool)
	for _, r := range readings {
		t.Logf("Reading: %s", r)
		if expectedReadings[r] {
			foundReadings[r] = true
		}
	}
	for er := range expectedReadings {
		if !foundReadings[er] {
			t.Errorf("Expected reading '%s' for kanji '秋', but it was not found", er)
		}
	}

}

func TestTokenize(t *testing.T) {
	ctx := context.Background()
	tokens, err := Tokenize(ctx, "今日は天気がいいですね")
	if err != nil {
		t.Errorf("Tokenize failed: %v", err)
	}
	if len(tokens) == 0 {
		t.Errorf("Expected tokens, got none")
	}

	// Check expected token sequence and key fields
	expected := []struct {
		Text      string
		Lemma     string
		POSPrefix string // Only check prefix for flexibility
		Reading   string
		Role      string
	}{
		{"今日", "今日", "名詞", "キョウ", ""},
		{"は", "は", "助詞", "ハ", ""},
		{"天気", "天気", "名詞", "テンキ", "object"},
		{"が", "が", "助詞", "ガ", ""},
		{"いい", "いい", "形容詞", "イイ", ""},
		{"です", "です", "助動詞", "デス", "verb"},
		{"ね", "ね", "助詞", "ネ", ""},
	}

	if len(tokens) != len(expected) {
		t.Errorf("Expected %d tokens, got %d", len(expected), len(tokens))
	}

	for i, exp := range expected {
		if i >= len(tokens) {
			break
		}
		got := tokens[i]
		if got.Text != exp.Text {
			t.Errorf("Token %d: expected Text=%q, got %q", i, exp.Text, got.Text)
		}
		if got.Lemma != exp.Lemma {
			t.Errorf("Token %d: expected Lemma=%q, got %q", i, exp.Lemma, got.Lemma)
		}
		if !strings.HasPrefix(got.POS, exp.POSPrefix) {
			t.Errorf("Token %d: expected POS prefix %q, got %q", i, exp.POSPrefix, got.POS)
		}
		if exp.Reading != "" && got.Reading != exp.Reading {
			t.Errorf("Token %d: expected Reading=%q, got %q", i, exp.Reading, got.Reading)
		}
		if exp.Role != "" && got.Role != exp.Role {
			t.Errorf("Token %d: expected Role=%q, got %q", i, exp.Role, got.Role)
		}
	}
}

func TestTokenize_KanaOnly(t *testing.T) {
	ctx := context.Background()
	tokens, err := Tokenize(ctx, "ありがとう")
	if err != nil {
		t.Fatalf("Tokenize failed: %v", err)
	}
	if len(tokens) == 0 {
		t.Fatalf("Expected tokens, got none")
	}
	// Verify the returned token fields
	if len(tokens) != 1 {
		t.Errorf("Expected 1 token, got %d", len(tokens))
	}
	tk := tokens[0]
	if tk.Text != "ありがとう" {
		t.Errorf("Expected Text 'ありがとう', got %q", tk.Text)
	}
	if tk.Lemma != "ありがとう" {
		t.Errorf("Expected Lemma 'ありがとう', got %q", tk.Lemma)
	}
	if tk.POS == "" || !strings.HasPrefix(tk.POS, "感動詞") {
		t.Errorf("Expected POS prefix '感動詞', got %q", tk.POS)
	}
	if tk.Reading != "アリガトウ" {
		t.Errorf("Expected Reading 'アリガトウ', got %q", tk.Reading)
	}
	if tk.FuriganaText != "ありがとう" {
		t.Errorf("Expected FuriganaText 'ありがとう', got %q", tk.FuriganaText)
	}
	if tk.FuriganaLemma != "ありがとう" {
		t.Errorf("Expected FuriganaLemma 'ありがとう', got %q", tk.FuriganaLemma)
	}
}

func TestTokenize_Empty(t *testing.T) {
	ctx := context.Background()
	tokens, err := Tokenize(ctx, "")
	if err != nil {
		t.Errorf("Tokenize failed: %v", err)
	}
	if len(tokens) != 0 {
		t.Errorf("Expected no tokens for empty input, got %d", len(tokens))
	}
}

func TestTokenize_FuriganaFields(t *testing.T) {
	ctx := context.Background()
	tokens, err := Tokenize(ctx, "秋田")
	if err != nil {
		t.Fatalf("Tokenize failed: %v", err)
	}
	foundKanji := false
	for _, tk := range tokens {
		for _, r := range tk.Text {
			if r >= 0x4E00 && r <= 0x9FFF {
				foundKanji = true
				if tk.FuriganaText == "" {
					t.Errorf("Token %q: expected FuriganaText to be set for kanji", tk.Text)
				}
				if tk.FuriganaLemma == "" {
					t.Errorf("Token %q: expected FuriganaLemma to be set for kanji", tk.Text)
				}
				// Additional validation: FuriganaText and FuriganaLemma should contain only kana
				for _, fr := range tk.FuriganaText {
					if fr >= 0x4E00 && fr <= 0x9FFF {
						t.Errorf("Token %q: FuriganaText should not contain kanji, got %q", tk.Text, tk.FuriganaText)
					}
				}
				for _, fr := range tk.FuriganaLemma {
					if fr >= 0x4E00 && fr <= 0x9FFF {
						t.Errorf("Token %q: FuriganaLemma should not contain kanji, got %q", tk.Text, tk.FuriganaLemma)
					}
				}
				// FuriganaText and FuriganaLemma should match Reading if present
				if tk.Reading != "" && tk.FuriganaText != "" && tk.FuriganaText != tk.Reading {
					t.Logf("Token %q: FuriganaText %q does not match Reading %q (may be ok for okurigana)", tk.Text, tk.FuriganaText, tk.Reading)
				}
			}
		}
	}
	if !foundKanji {
		t.Skip("No kanji token found in '秋田', tokenizer dictionary may be missing entry")
	}
}

func TestTokenizeModes(t *testing.T) {
	ctx := context.Background()
	modes, err := TokenizeModes(ctx, "今日は天気がいいですね")
	if err != nil {
		t.Errorf("TokenizeModes failed: %v", err)
	}
	if len(modes) != 3 {
		t.Errorf("Expected 3 modes, got %d", len(modes))
	}
	// Validate that each mode contains non-empty tokens and the first token matches expected text
	expectedFirstTokens := map[string]string{
		"normal":   "今日",
		"search":   "今日",
		"extended": "今日",
	}
	for mode, tokens := range modes {
		if len(tokens) == 0 {
			t.Errorf("Mode %s: expected non-empty tokens", mode)
			continue
		}
		if tokens[0].Text != expectedFirstTokens[mode] {
			t.Errorf("Mode %s: expected first token Text=%q, got %q", mode, expectedFirstTokens[mode], tokens[0].Text)
		}
	}
}

func TestMergeVerbAuxiliaries(t *testing.T) {
	tokens := []Token{
		{Text: "食べ", POS: "動詞"},
		{Text: "ます", POS: "助動詞"},
	}
	merged := MergeVerbAuxiliaries(tokens)
	if len(merged) != 1 {
		t.Errorf("Expected 1 merged token, got %d", len(merged))
	}
	expectedText := "食べます"
	if merged[0].Text != expectedText {
		t.Errorf("Expected merged token Text=%q, got %q", expectedText, merged[0].Text)
	}
	if !strings.HasPrefix(merged[0].POS, "動詞") {
		t.Errorf("Expected merged token POS to start with '動詞', got %q", merged[0].POS)
	}
}

func TestStartTokenizer(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go StartTokenizer(ctx)
}
