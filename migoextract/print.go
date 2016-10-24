package migoextract

// Printing and formatting utilities.

import (
	"fmt"
	"strings"

	"github.com/fatih/color"
)

const (
	BlockSymbol     = "┝ "
	CallSymbol      = " ↘ "
	ExitSymbol      = " ↙ "
	ChanSymbol      = "ν "
	FuncEnterSymbol = "┌"
	FuncExitSymbol  = "└"
	RecvSymbol      = "❓ "
	SendSymbol      = "❗ "
	SkipSymbol      = "┆ "
	SpawnSymbol     = "┿ "
	JumpSymbol      = " ⇾ "
	ParamSymbol     = "├param "
	MoreSymbol      = "┊ "
	ReturnSymbol    = "⏎ "
	LoopSymbol      = "↻ "
	PhiSymbol       = "φ "
	IfSymbol        = "⨁ "
	SelectSymbol    = "Sel:"
	SplitSymbol     = "分"
	ErrorSymbol     = " ◹ "
	FieldSymbol     = " ↦ "
	NewSymbol       = "新"
	SubSymbol       = "    ▸ "
	ValSymbol       = "├ "
	AssignSymbol    = "≔"
)

var (
	fmtBlock  = color.New(color.Italic).SprintFunc()
	fmtChan   = color.New(color.FgRed, color.Bold).SprintFunc()
	fmtClose  = color.New(color.FgGreen, color.Bold).SprintFunc()
	fmtLoopHL = color.New(color.FgHiRed, color.Italic).SprintFunc()
	fmtPos    = color.New(color.FgYellow, color.Italic).SprintFunc()
	fmtRecv   = color.New(color.FgHiBlue).SprintFunc()
	fmtSend   = color.New(color.FgCyan).SprintFunc()
	fmtSpawn  = color.New(color.FgMagenta, color.Bold).SprintFunc()
	fmtType   = color.New(color.BgBlue).SprintFunc()
)

// Sprintf in current function context.
func (ctx *Function) Sprintf(format string, a ...interface{}) string {
	return fmt.Sprintf(strings.Repeat(" ", ctx.Level*2)+format, a...)
}
