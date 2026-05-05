// Package scene holds Ebitengine scenes (title, game, results).
package scene

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
)

var bg = color.RGBA{R: 0x18, G: 0x1c, B: 0x24, A: 0xff}

// Title is the placeholder landing scene for Phase 1.
type Title struct{}

func NewTitle() *Title { return &Title{} }

func (t *Title) Update() error { return nil }

func (t *Title) Draw(screen *ebiten.Image) {
	screen.Fill(bg)
	ebitenutil.DebugPrintAt(screen, "Dleague", 200, 280)
	ebitenutil.DebugPrintAt(screen, "Phase 1 — foundation", 170, 320)
}
