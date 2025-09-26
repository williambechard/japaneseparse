package analyze

import (
	"context"
	"japaneseparse/ingest"
	"japaneseparse/model"
	"testing"
)

func TestAnalyze(t *testing.T) {
	// Define a sample sentence and lexicon entries for testing
	sampleSentence := ingest.Sentence{
		ID:   "test_sentence_1",
		Text: "私はりんごを食べます。",
	}

	sampleEntries := []model.LexEntry{
		{Token: model.Token{Text: "私", Role: "subject"}, Definitions: []string{"I"}},
		{Token: model.Token{Text: "は"}},
		{Token: model.Token{Text: "りんご", Role: "object"}, Definitions: []string{"apple"}},
		{Token: model.Token{Text: "を"}},
		{Token: model.Token{Text: "食べます", Role: "verb"}, Definitions: []string{"eat"}},
		{Token: model.Token{Text: "。"}},
	}

	// Call the Analyze function
	ctx := context.Background()
	analysis, err := Analyze(ctx, sampleSentence, sampleEntries)
	if err != nil {
		t.Fatalf("Analyze function returned an error: %v", err)
	}

	// Validate the analysis result
	if analysis.SentenceID != sampleSentence.ID {
		t.Errorf("Expected SentenceID %s, got %s", sampleSentence.ID, analysis.SentenceID)
	}

	if analysis.TokenCount != len(sampleEntries) {
		t.Errorf("Expected TokenCount %d, got %d", len(sampleEntries), analysis.TokenCount)
	}

	if analysis.Definitions != 2 {
		t.Errorf("Expected Definitions 2, got %d", analysis.Definitions)
	}

	// Validate clause structure
	clauses := analysis.Structure.(map[string]interface{})["clauses"].([]Clause)
	if len(clauses) != 1 {
		t.Errorf("Expected 1 clause, got %d", len(clauses))
	}

	clause := clauses[0]
	if clause.Start != 0 || clause.End != 6 {
		t.Errorf("Expected clause range 0-6, got %d-%d", clause.Start, clause.End)
	}

	if clause.Roles.Subject == nil || len(*clause.Roles.Subject) != 1 || (*clause.Roles.Subject)[0] != 0 {
		t.Errorf("Expected Subject role at index 0, got %v", clause.Roles.Subject)
	}

	if clause.Roles.Object == nil || len(*clause.Roles.Object) != 1 || (*clause.Roles.Object)[0] != 2 {
		t.Errorf("Expected Object role at index 2, got %v", clause.Roles.Object)
	}

	if clause.Roles.Verb == nil || *clause.Roles.Verb != 4 {
		t.Errorf("Expected Verb role at index 4, got %v", clause.Roles.Verb)
	}
}

func TestAnalyzeMinimal(t *testing.T) {
	ctx := context.Background()
	sampleSentence := ingest.Sentence{
		ID:   "test_sentence_1",
		Text: "私はりんごを食べます。",
	}
	sampleEntries := []model.LexEntry{}
	_, err := Analyze(ctx, sampleSentence, sampleEntries)
	if err != nil {
		t.Fatalf("Analyze function failed: %v", err)
	}
}
