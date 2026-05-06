// Phaser scene for the Wordle variant. Owns: 6×N tile grid, on-screen
// keyboard, tile flip animation, color reveal, EventBus emits on
// attempt-complete. Pure scoring lives in scoring.ts.

import { Scene } from 'phaser';
import { EventBus } from '../../game/EventBus';
import {
  evaluateGuess,
  isWin,
  MAX_GUESSES,
  type LetterEval,
} from './scoring';
import { emitAttemptComplete, emitGuessSubmitted } from '../runner/eventbus-helpers';

const COLORS = {
  hit: 0x6aaa64,
  present: 0xc9b458,
  miss: 0x787c7e,
  empty: 0xffffff,
  border: 0xd3d6da,
  text: 0x1a1a1b,
  textInverse: 0xffffff,
};

const HEX = (n: number) => '#' + n.toString(16).padStart(6, '0');

interface WordleInitData {
  solution: string;
  length: number;
  resumeGuesses?: string[];
}

const KEYBOARD_ROWS = ['QWERTYUIOP', 'ASDFGHJKL', '↵ZXCVBNM⌫'];

export class WordleScene extends Scene {
  private solution = '';
  private wordLength = 5;
  private guesses: string[] = [];
  private evaluations: LetterEval[][] = [];
  private currentInput = '';
  private finished = false;

  private tiles: Phaser.GameObjects.Rectangle[][] = [];
  private tileLabels: Phaser.GameObjects.Text[][] = [];
  private keyButtons = new Map<string, { rect: Phaser.GameObjects.Rectangle; label: Phaser.GameObjects.Text }>();
  private statusText?: Phaser.GameObjects.Text;

  constructor() {
    super('Wordle');
  }

  init(data: WordleInitData) {
    this.solution = (data.solution || '').toUpperCase();
    this.wordLength = data.length || this.solution.length || 5;
    this.guesses = [];
    this.evaluations = [];
    this.currentInput = '';
    this.finished = false;

    if (data.resumeGuesses && data.resumeGuesses.length > 0) {
      for (const g of data.resumeGuesses) {
        const guess = g.toUpperCase();
        this.guesses.push(guess);
        this.evaluations.push(evaluateGuess(guess, this.solution));
      }
      if (this.evaluations.some(isWin)) this.finished = true;
      else if (this.guesses.length >= MAX_GUESSES) this.finished = true;
    }
  }

  create() {
    this.cameras.main.setBackgroundColor(0xffffff);
    this.buildGrid();
    this.buildKeyboard();
    this.statusText = this.add
      .text(this.scale.width / 2, this.scale.height - 30, '', {
        fontFamily: 'system-ui, sans-serif',
        fontSize: '20px',
        color: HEX(COLORS.text),
      })
      .setOrigin(0.5);

    this.input.keyboard?.on('keydown', this.handleKeyDown, this);

    // Re-render any resumed state.
    for (let row = 0; row < this.guesses.length; row++) {
      for (let col = 0; col < this.wordLength; col++) {
        this.tileLabels[row][col].setText(this.guesses[row][col]);
        this.applyTileColor(row, col, this.evaluations[row][col]);
      }
    }
    this.refreshKeyboardColors();

    if (this.finished) {
      this.showStatus(this.evaluations.some(isWin) ? 'You won — already solved.' : 'Already attempted.');
    }

    EventBus.emit('current-scene-ready', this);
  }

  private buildGrid() {
    const tileSize = 56;
    const gap = 6;
    const rows = MAX_GUESSES;
    const cols = this.wordLength;
    const totalW = cols * tileSize + (cols - 1) * gap;
    const startX = (this.scale.width - totalW) / 2 + tileSize / 2;
    const startY = 60;

    for (let row = 0; row < rows; row++) {
      this.tiles[row] = [];
      this.tileLabels[row] = [];
      for (let col = 0; col < cols; col++) {
        const x = startX + col * (tileSize + gap);
        const y = startY + row * (tileSize + gap);
        const rect = this.add.rectangle(x, y, tileSize, tileSize, COLORS.empty)
          .setStrokeStyle(2, COLORS.border);
        const label = this.add.text(x, y, '', {
          fontFamily: 'system-ui, sans-serif',
          fontSize: '32px',
          fontStyle: 'bold',
          color: HEX(COLORS.text),
        }).setOrigin(0.5);
        this.tiles[row].push(rect);
        this.tileLabels[row].push(label);
      }
    }
  }

  private buildKeyboard() {
    const keyW = 36;
    const keyH = 48;
    const gap = 4;
    const startY = this.scale.height - 200;

    for (let r = 0; r < KEYBOARD_ROWS.length; r++) {
      const row = KEYBOARD_ROWS[r];
      const totalW = row.length * keyW + (row.length - 1) * gap;
      const startX = (this.scale.width - totalW) / 2 + keyW / 2;
      for (let i = 0; i < row.length; i++) {
        const k = row[i];
        const x = startX + i * (keyW + gap);
        const y = startY + r * (keyH + gap);
        const rect = this.add.rectangle(x, y, keyW, keyH, COLORS.empty)
          .setStrokeStyle(1, COLORS.border)
          .setInteractive({ useHandCursor: true });
        const label = this.add.text(x, y, k, {
          fontFamily: 'system-ui, sans-serif',
          fontSize: '18px',
          color: HEX(COLORS.text),
        }).setOrigin(0.5);
        rect.on('pointerdown', () => this.handleVirtualKey(k));
        this.keyButtons.set(k, { rect, label });
      }
    }
  }

  private handleKeyDown(ev: KeyboardEvent) {
    if (this.finished) return;
    if (ev.key === 'Enter') this.submitGuess();
    else if (ev.key === 'Backspace') this.popLetter();
    else if (/^[a-zA-Z]$/.test(ev.key)) this.pushLetter(ev.key.toUpperCase());
  }

  private handleVirtualKey(k: string) {
    if (this.finished) return;
    if (k === '↵') this.submitGuess();
    else if (k === '⌫') this.popLetter();
    else this.pushLetter(k);
  }

  private pushLetter(c: string) {
    if (this.currentInput.length >= this.wordLength) return;
    this.currentInput += c;
    const row = this.guesses.length;
    const col = this.currentInput.length - 1;
    if (this.tileLabels[row]?.[col]) {
      this.tileLabels[row][col].setText(c);
      this.tweens.add({
        targets: this.tiles[row][col],
        scale: 1.05,
        duration: 80,
        yoyo: true,
      });
    }
  }

  private popLetter() {
    if (this.currentInput.length === 0) return;
    const row = this.guesses.length;
    const col = this.currentInput.length - 1;
    this.currentInput = this.currentInput.slice(0, -1);
    if (this.tileLabels[row]?.[col]) this.tileLabels[row][col].setText('');
  }

  private submitGuess() {
    if (this.currentInput.length !== this.wordLength) {
      this.showStatus(`Word must be ${this.wordLength} letters`);
      this.shakeRow(this.guesses.length);
      return;
    }
    const guess = this.currentInput;
    const evaluation = evaluateGuess(guess, this.solution);
    const row = this.guesses.length;
    this.guesses.push(guess);
    this.evaluations.push(evaluation);
    this.currentInput = '';

    emitGuessSubmitted(guess);
    this.flipRow(row, evaluation, () => {
      this.refreshKeyboardColors();
      const won = isWin(evaluation);
      const lost = !won && this.guesses.length >= MAX_GUESSES;
      if (won || lost) {
        this.finished = true;
        this.showStatus(won ? `Solved in ${this.guesses.length}!` : `Out of guesses. Word: ${this.solution}`);
        emitAttemptComplete({
          guesses: [...this.guesses],
          evaluations: this.evaluations.map((e) => [...e]),
          status: won ? 'won' : 'lost',
        });
      }
    });
  }

  private flipRow(row: number, evaluation: LetterEval[], done: () => void) {
    let pending = this.wordLength;
    for (let col = 0; col < this.wordLength; col++) {
      this.tweens.add({
        targets: this.tiles[row][col],
        scaleY: 0,
        duration: 200,
        delay: col * 100,
        onComplete: () => {
          this.applyTileColor(row, col, evaluation[col]);
          this.tileLabels[row][col].setColor(HEX(COLORS.textInverse));
          this.tweens.add({
            targets: this.tiles[row][col],
            scaleY: 1,
            duration: 200,
            onComplete: () => {
              pending--;
              if (pending === 0) done();
            },
          });
        },
      });
    }
  }

  private applyTileColor(row: number, col: number, ev: LetterEval) {
    this.tiles[row][col].setFillStyle(COLORS[ev]);
    this.tiles[row][col].setStrokeStyle(0);
    this.tileLabels[row][col].setColor(HEX(COLORS.textInverse));
  }

  private refreshKeyboardColors() {
    // Best-known eval per letter: hit > present > miss.
    const rank: Record<LetterEval, number> = { hit: 3, present: 2, miss: 1 };
    const best: Record<string, LetterEval> = {};
    for (let i = 0; i < this.guesses.length; i++) {
      for (let j = 0; j < this.wordLength; j++) {
        const c = this.guesses[i][j];
        const e = this.evaluations[i][j];
        if (!best[c] || rank[e] > rank[best[c]]) best[c] = e;
      }
    }
    for (const [k, btn] of this.keyButtons) {
      const e = best[k];
      if (e) {
        btn.rect.setFillStyle(COLORS[e]);
        btn.label.setColor(HEX(COLORS.textInverse));
      }
    }
  }

  private shakeRow(row: number) {
    if (!this.tiles[row]) return;
    for (const t of this.tiles[row]) {
      this.tweens.add({
        targets: t,
        x: t.x - 4,
        duration: 50,
        yoyo: true,
        repeat: 2,
      });
    }
  }

  private showStatus(msg: string) {
    this.statusText?.setText(msg);
  }
}
