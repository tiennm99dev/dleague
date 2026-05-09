package game

// TerminalState is a convenience embed for game states that track their own
// terminal flag. Concrete states embed this and call SetTerminal(true) when
// the game ends. They still implement the State interface via IsTerminal().
type TerminalState struct {
	terminal bool
}

// IsTerminal implements State.
func (t *TerminalState) IsTerminal() bool { return t.terminal }

// SetTerminal marks the state as terminal.
func (t *TerminalState) SetTerminal(v bool) { t.terminal = v }
