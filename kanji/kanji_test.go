package kanji

import (
	"testing"
)

func TestInitKanjidic2(t *testing.T) {
	err := InitKanjidic2("../dict/kanjidic2.xml")
	if err != nil {
		t.Fatalf("Failed to initialize Kanjidic2: %v", err)
	}
}

func TestGetKanjiReadings(t *testing.T) {
	err := InitKanjidic2("../dict/kanjidic2.xml")
	if err != nil {
		t.Fatalf("Failed to initialize Kanjidic2: %v", err)
	}

	tests := []struct {
		kanji    rune
		expected []string
	}{
		{'入', []string{"ニュウ", "ジュ", "い.る", "-い.る", "-い.り", "い.れる", "-い.れ", "はい.る"}},
		{'見', []string{"ケン", "み.る", "み.える", "み.せる"}},
		{'内', []string{"ナイ", "ダイ", "うち"}},
		{'川', []string{"セン", "かわ"}},
	}

	for _, test := range tests {
		readings := GetKanjiReadings(test.kanji)
		if len(readings) != len(test.expected) {
			t.Errorf("For kanji '%c', expected %d readings, got %d", test.kanji, len(test.expected), len(readings))
			continue
		}
		for i, reading := range readings {
			if reading != test.expected[i] {
				t.Errorf("For kanji '%c', expected reading '%s', got '%s'", test.kanji, test.expected[i], reading)
			}
		}
	}
}
