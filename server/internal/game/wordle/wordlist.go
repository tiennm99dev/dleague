package wordle

import (
	"bufio"
	"bytes"
	"context"
	_ "embed"
	"log"
	"strings"

	"github.com/tiennm99/dleague/server/internal/store"
)

//go:embed data/answers.txt
var embeddedAnswers []byte

//go:embed data/dictionary.txt
var embeddedDictionary []byte

const (
	wordlistIDAnswers    = "wordle_answers"
	wordlistIDDictionary = "wordle_dictionary"
)

// LoadAnswers returns the answer word list. It queries Mongo first;
// if the collection returns an empty list it falls back to the embedded file.
// Returned words are upper-case and exactly WordLen characters.
func LoadAnswers(ctx context.Context, repo *store.WordlistRepo) ([]string, error) {
	words, err := repo.GetByID(ctx, wordlistIDAnswers)
	if err != nil {
		return nil, err
	}
	if len(words) > 0 {
		return words, nil
	}
	return parseWordList(embeddedAnswers), nil
}

// LoadDictionary returns the valid-guess word list (superset of answers).
// Same Mongo-first, embedded-fallback strategy as LoadAnswers.
func LoadDictionary(ctx context.Context, repo *store.WordlistRepo) ([]string, error) {
	words, err := repo.GetByID(ctx, wordlistIDDictionary)
	if err != nil {
		return nil, err
	}
	if len(words) > 0 {
		return words, nil
	}
	return parseWordList(embeddedDictionary), nil
}

// EmbeddedAnswers returns the answer list parsed from the embedded file.
// Useful as an error-path fallback in main.go without a context.
func EmbeddedAnswers() []string { return parseWordList(embeddedAnswers) }

// EmbeddedDictionary returns the dictionary list parsed from the embedded file.
func EmbeddedDictionary() []string { return parseWordList(embeddedDictionary) }

// parseWordList reads one upper-case word per line, skipping blank lines
// and lines that are not exactly WordLen ASCII letters.
func parseWordList(data []byte) []string {
	return parseWordListNamed(data, "embedded")
}

// parseWordListNamed is the implementation used by parseWordList and tests.
// path is used only for logging; it need not be a real file path.
func parseWordListNamed(data []byte, path string) []string {
	var out []string
	dropped := 0
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		w := strings.TrimSpace(strings.ToUpper(scanner.Text()))
		if len(w) == WordLen {
			out = append(out, w)
		} else if len(w) > 0 {
			dropped++
		}
	}
	if dropped > 0 {
		log.Printf("wordle: parseWordList dropped %d malformed lines from %s", dropped, path)
	}
	return out
}
