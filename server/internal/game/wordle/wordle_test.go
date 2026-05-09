package wordle

import (
	"errors"
	"testing"
)

var testDict = []string{
	"CRANE", "TRACE", "ALERT", "ADIEU", "GUESS", "WRONG",
	"SPINE", "GLOOM", "EERIE", "ALLEE", "SHARP", "SHRED",
	"TASER", "RATES", "ENTER", "ETHER", "SPELL", "LLAMA",
	"AAAAA", "AROMA",
}

func TestNew(t *testing.T) {
	w := New("crane")
	if w.solution != "CRANE" {
		t.Errorf("New normalises to upper-case: got %q want %q", w.solution, "CRANE")
	}
	if w.attemptsRemaining != MaxAttempts {
		t.Errorf("attemptsRemaining = %d, want %d", w.attemptsRemaining, MaxAttempts)
	}
	if w.IsTerminal() {
		t.Error("new game should not be terminal")
	}
}

func TestValidate_Length(t *testing.T) {
	w := New("CRANE")
	// Too short
	if err := w.Validate("HI", testDict); !errors.Is(err, ErrWrongLength) {
		t.Errorf("expected ErrWrongLength for short guess, got %v", err)
	}
	// Too long
	if err := w.Validate("TOOLONG", testDict); !errors.Is(err, ErrWrongLength) {
		t.Errorf("expected ErrWrongLength for long guess, got %v", err)
	}
}

func TestValidate_Dictionary(t *testing.T) {
	w := New("CRANE")
	if err := w.Validate("ZZZZZ", testDict); !errors.Is(err, ErrNotInDictionary) {
		t.Errorf("expected ErrNotInDictionary for unknown word, got %v", err)
	}
}

func TestValidate_OK(t *testing.T) {
	w := New("CRANE")
	if err := w.Validate("ALERT", testDict); err != nil {
		t.Errorf("unexpected error for valid guess: %v", err)
	}
}

func TestHappyPath_WinOnFourth(t *testing.T) {
	w := New("CRANE")
	guesses := []string{"ADIEU", "ALERT", "TRACE", "CRANE"}
	for i, g := range guesses {
		if err := w.Validate(g, testDict); err != nil {
			t.Fatalf("guess %d validate: %v", i, err)
		}
		if err := w.Apply(g); err != nil {
			t.Fatalf("guess %d apply: %v", i, err)
		}
	}
	if !w.IsTerminal() {
		t.Error("game should be terminal after winning")
	}
	if !w.won {
		t.Error("won flag should be true")
	}
	r := w.Result()
	if !r.Won {
		t.Error("Result.Won should be true")
	}
	if r.AttemptsUsed != 4 {
		t.Errorf("AttemptsUsed = %d, want 4", r.AttemptsUsed)
	}
}

func TestHappyPath_WinFirstGuess(t *testing.T) {
	w := New("CRANE")
	if err := w.Apply("crane"); err != nil { // lower-case normalised
		t.Fatalf("apply: %v", err)
	}
	if !w.IsTerminal() {
		t.Error("game should be terminal after immediate win")
	}
	if !w.won {
		t.Error("won should be true")
	}
}

func TestHappyPath_LossAfterSixAttempts(t *testing.T) {
	w := New("CRANE")
	for i := 0; i < MaxAttempts; i++ {
		if err := w.Apply("WRONG"); err != nil {
			t.Fatalf("attempt %d: %v", i, err)
		}
		if i < MaxAttempts-1 && w.IsTerminal() {
			t.Errorf("should not be terminal after attempt %d", i+1)
		}
	}
	if !w.IsTerminal() {
		t.Error("game should be terminal after 6 wrong guesses")
	}
	if !w.lost {
		t.Error("lost flag should be true")
	}
	if w.won {
		t.Error("won flag should be false")
	}
	if w.attemptsRemaining != 0 {
		t.Errorf("attemptsRemaining = %d, want 0", w.attemptsRemaining)
	}
}

func TestApply_GameOver(t *testing.T) {
	w := New("CRANE")
	_ = w.Apply("CRANE") // win immediately
	if err := w.Apply("ALERT"); !errors.Is(err, ErrGameOver) {
		t.Errorf("expected ErrGameOver after terminal, got %v", err)
	}
}

func TestToProto_SolutionHiddenPreTerminal(t *testing.T) {
	w := New("CRANE")
	_ = w.Apply("WRONG")
	proto := w.ToProto()
	if proto.Solution != "" {
		t.Errorf("solution should be empty pre-terminal, got %q", proto.Solution)
	}
}

func TestToProto_SolutionRevealedOnWin(t *testing.T) {
	w := New("CRANE")
	_ = w.Apply("CRANE")
	proto := w.ToProto()
	if proto.Solution != "CRANE" {
		t.Errorf("solution should be revealed on win, got %q", proto.Solution)
	}
}

func TestToProto_SolutionRevealedOnLoss(t *testing.T) {
	w := New("CRANE")
	for i := 0; i < MaxAttempts; i++ {
		_ = w.Apply("WRONG")
	}
	proto := w.ToProto()
	if proto.Solution != "CRANE" {
		t.Errorf("solution should be revealed on loss, got %q", proto.Solution)
	}
}

func TestToProto_AttemptsRemainingDecrement(t *testing.T) {
	w := New("CRANE")
	_ = w.Apply("WRONG")
	proto := w.ToProto()
	if proto.AttemptsRemaining != int32(MaxAttempts-1) {
		t.Errorf("AttemptsRemaining = %d, want %d", proto.AttemptsRemaining, MaxAttempts-1)
	}
}

func TestToProto_HintsLength(t *testing.T) {
	w := New("CRANE")
	_ = w.Apply("WRONG")
	_ = w.Apply("SPINE")
	proto := w.ToProto()
	if len(proto.Hints) != 2 {
		t.Errorf("hints length = %d, want 2", len(proto.Hints))
	}
	for i, h := range proto.Hints {
		if len(h.Colors) != WordLen {
			t.Errorf("hint[%d] has %d colors, want %d", i, len(h.Colors), WordLen)
		}
	}
}

func TestWonFlagSetCorrectly(t *testing.T) {
	w := New("GLOOM")
	_ = w.Apply("WRONG") // wrong
	if w.won || w.IsTerminal() {
		t.Error("should not be terminal/won after wrong guess")
	}
	_ = w.Apply("GLOOM")
	if !w.won {
		t.Error("should be won after correct guess")
	}
}
