package enamdict

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/transform"
)

type EnamdictEntry struct {
	Lemma   string
	Reading string
	POS     string
	Meaning string
}

// Parse a line like: 仙北 [せんほく] /(s) Senhoku/
func parseEnamdictLine(line string) (EnamdictEntry, bool) {
	entry := EnamdictEntry{}
	line = strings.TrimSpace(line)
	if line == "" || !strings.Contains(line, "[") || !strings.Contains(line, "]") {
		return entry, false
	}
	bracket := strings.Index(line, "[")
	entry.Lemma = strings.TrimSpace(line[:bracket])
	rest := line[bracket+1:]
	close := strings.Index(rest, "]")
	if close < 0 {
		return entry, false
	}
	entry.Reading = rest[:close]
	after := rest[close+1:]
	// POS and Meaning
	posStart := strings.Index(after, "/(")
	posEnd := strings.Index(after, ") ")
	if posStart >= 0 && posEnd > posStart {
		entry.POS = after[posStart+2 : posEnd]
		entry.Meaning = strings.TrimSpace(after[posEnd+2:])
	} else {
		// Fallback: just meaning after last /
		slash := strings.LastIndex(after, "/")
		if slash >= 0 {
			entry.Meaning = strings.TrimSpace(after[slash+1:])
		}
	}
	return entry, true
}

// LoadEnamdict parses the ENAMDICT file into a map for fast lookup
func LoadEnamdict(path string) (map[string]EnamdictEntry, error) {
	entries := make(map[string]EnamdictEntry)
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	r := bufio.NewReader(transform.NewReader(f, japanese.EUCJP.NewDecoder()))
	logged := false
	for {
		line, err := r.ReadString('\n')
		line = strings.TrimRight(line, "\r\n")
		entry, ok := parseEnamdictLine(line)
		if ok {
			if !logged {
				fmt.Printf("First parsed entry: Lemma=%s, Reading=%s, POS=%s, Meaning=%s\n", entry.Lemma, entry.Reading, entry.POS, entry.Meaning)
				logged = true
			}
			key := entry.Lemma + "|" + entry.Reading
			entries[key] = entry
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
	}
	return entries, nil
}

// readFileLines tries to read file as UTF-8; if decodeSJIS is true, decodes as Shift_JIS
// if decodeEUCJP is true, decodes as EUC-JP
func readFileLines(path string, decodeSJIS, decodeEUCJP bool) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var r io.Reader = f
	if decodeSJIS {
		r = transform.NewReader(f, japanese.ShiftJIS.NewDecoder())
	} else if decodeEUCJP {
		r = transform.NewReader(f, japanese.EUCJP.NewDecoder())
	}
	br := bufio.NewReader(r)
	lines := []string{}
	for {
		line, err := br.ReadString('\n')
		if len(line) > 0 {
			// trim trailing newline but keep the rest
			lines = append(lines, strings.TrimRight(line, "\r\n"))
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			return lines, err
		}
	}
	return lines, nil
}

// lookupEnamdictFlexible searches the enamdict file using several strategies
// depending on mode: exact, partial, fuzzy. It returns matching line indices
// and the lines themselves.
func LookupEnamdictFlexible(path, lemma, mode string, max, ctx int) ([]int, []string, error) {
	// Try UTF-8 first
	lines, err := readFileLines(path, false, false)
	if err != nil {
		return nil, nil, err
	}
	idxs := searchLines(lines, lemma, mode, max)
	if len(idxs) == 0 {
		// Try Shift_JIS decoding
		lines, err = readFileLines(path, true, false)
		if err != nil {
			return nil, nil, err
		}
		idxs = searchLines(lines, lemma, mode, max)
	}
	if len(idxs) == 0 {
		// Try EUC-JP decoding
		lines, err = readFileLines(path, false, true)
		if err != nil {
			return nil, nil, err
		}
		idxs = searchLines(lines, lemma, mode, max)
	}
	// Collect context lines for each hit
	outLines := []string{}
	for _, i := range idxs {
		start := i - ctx
		if start < 0 {
			start = 0
		}
		end := i + ctx
		if end >= len(lines) {
			end = len(lines) - 1
		}
		for j := start; j <= end; j++ {
			outLines = append(outLines, fmt.Sprintf("%5d: %s", j+1, lines[j]))
		}
		outLines = append(outLines, "----")
	}
	return idxs, outLines, nil
}

func searchLines(lines []string, lemma, mode string, max int) []int {
	var idxs []int
	lower := strings.ToLower(lemma)
	for i, l := range lines {
		if mode == "exact" {
			if strings.Contains(l, lemma) {
				idxs = append(idxs, i)
			}
		} else if mode == "partial" {
			// search for any character substrings from lemma, longest-first
			matched := false
			for sz := len([]rune(lemma)); sz >= 1 && !matched; sz-- {
				// take prefix substrings
				r := []rune(lemma)
				sub := string(r[:sz])
				if strings.Contains(l, sub) {
					matched = true
					break
				}
			}
			if matched {
				idxs = append(idxs, i)
			}
		} else if mode == "fuzzy" {
			// compare lemma against whole line (strip after first tab/space for enamdict fields)
			key := l
			if tab := strings.IndexAny(l, "\t "); tab >= 0 {
				key = l[:tab]
			}
			// compute a small edit distance
			d := levenshteinDistance([]rune(lemma), []rune(key))
			// accept if distance relatively small
			if d <= 2 || float64(d) <= float64(len([]rune(key)))/3.0 {
				idxs = append(idxs, i)
			}
		} else {
			// default to case-insensitive contains
			if strings.Contains(strings.ToLower(l), lower) {
				idxs = append(idxs, i)
			}
		}
		if len(idxs) >= max {
			break
		}
	}
	return idxs
}

func levenshteinDistance(a, b []rune) int {
	if len(a) == 0 {
		return len(b)
	}
	if len(b) == 0 {
		return len(a)
	}
	dp := make([][]int, len(a)+1)
	for i := range dp {
		dp[i] = make([]int, len(b)+1)
	}
	for i := 0; i <= len(a); i++ {
		dp[i][0] = i
	}
	for j := 0; j <= len(b); j++ {
		dp[0][j] = j
	}
	for i := 1; i <= len(a); i++ {
		for j := 1; j <= len(b); j++ {
			cost := 0
			if a[i-1] != b[j-1] {
				cost = 1
			}
			dp[i][j] = min(dp[i-1][j]+1, min(dp[i][j-1]+1, dp[i-1][j-1]+cost))
		}
	}
	return dp[len(a)][len(b)]
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
