package model

// Token represents a token / morpheme produced by the tokenizer.
type Token struct {
	Text             string          `json:"text"`
	Lemma            string          `json:"lemma,omitempty"`
	POS              string          `json:"pos,omitempty"`
	Start            int             `json:"start"`
	End              int             `json:"end"`
	Reading          string          `json:"reading,omitempty"`
	Pronunciation    string          `json:"pronunciation,omitempty"`
	TokenID          int             `json:"token_id,omitempty"`
	Conjugation      []string        `json:"conjugation,omitempty"`
	Auxiliaries      []Token         `json:"auxiliaries,omitempty"`
	MergedIndices    []int           `json:"merged_indices,omitempty"`
	ConjugationLabel string          `json:"conjugation_label,omitempty"`
	InflectionType   string          `json:"inflection_type,omitempty"`
	InflectionForm   string          `json:"inflection_form,omitempty"`
	DictionaryEntry  DictionaryEntry `json:"dictionary_entry,omitempty"`
	FuriganaText     string          `json:"furigana_text,omitempty"`
	FuriganaLemma    string          `json:"furigana_lemma,omitempty"`
}

type DictionaryEntry struct {
	Source      string                 `json:"source,omitempty"`
	Kanji       []string               `json:"kanji,omitempty"`
	Readings    []string               `json:"readings,omitempty"`
	Glosses     []string               `json:"glosses,omitempty"`
	POS         []string               `json:"pos,omitempty"`
	Frequency   int                    `json:"frequency,omitempty"`
	IsName      bool                   `json:"is_name,omitempty"`
	IsCommon    bool                   `json:"is_common,omitempty"`
	OtherFields map[string]interface{} `json:"other_fields,omitempty"`
}

type LexEntry struct {
	Token       Token    `json:"token"`
	Readings    []string `json:"readings,omitempty"`
	Definitions []string `json:"definitions,omitempty"`
}
