// Package fairness runs a fairness analysis.
//
// Fairness analysis is an estimation of loop and recursive calls to find
// potentially unfair loop and recurse conditions.
//  - If the loop is a range slice/map expression --> usually safe (finite)
//  - If the loop is a range channel expression --> safe if channel is closed
//  - If the loop is an ordinary for-loop --> safe if
//     * loop condition is not constant or constant expression
//     * loop index is modified [in the loop body]
package fairness

import (
	"log"
	"os"

	"github.com/fatih/color"
	"github.com/nickng/dingo-hunter/logwriter"
	"github.com/nickng/dingo-hunter/ssabuilder"
	"golang.org/x/tools/go/ssa"
)

// FairnessAnalysis
type FairnessAnalysis struct {
	unsafe int
	total  int
	info   *ssabuilder.SSAInfo
	logger *log.Logger
}

// NewFairnessAnalysis starts a new analysis.
func NewFairnessAnalysis() *FairnessAnalysis {
	return &FairnessAnalysis{unsafe: 0, total: 0}
}

func (fa *FairnessAnalysis) Visit(fn *ssa.Function) {
	visitedBlk := make(map[*ssa.BasicBlock]bool)
	fa.logger.Printf("Visiting: %s", fn.String())
	for _, blk := range fn.Blocks {
		if _, visited := visitedBlk[blk]; !visited {
			visitedBlk[blk] = true
			fa.logger.Printf(" block %d %s", blk.Index, blk.Comment)
			// First consider blocks with loop initialisation blocks.
			if blk.Comment == "rangeindex.loop" {
				fa.total++
				fa.logger.Println(color.GreenString("✓ range loops are fair"))
			} else if blk.Comment == "rangechan.loop" {
				fa.total++
				hasClose := false
				for _, ch := range fa.info.FindChan(blk.Instrs[0].(*ssa.UnOp).X) {
					if ch.Type == ssabuilder.ChanClose {
						fa.logger.Println(color.GreenString("✓ found corresponding close() - channel range likely fair"))
						hasClose = true
					}
				}
				if !hasClose {
					fa.logger.Println(color.RedString("❌ range over channel w/o close() likely unfair (%s)", fa.info.FSet.Position(blk.Instrs[0].Pos())))
					fa.unsafe++
				}
			} else if blk.Comment == "for.loop" {
				fa.total++
				if fa.isLikelyUnsafe(blk) {
					fa.logger.Println(color.RedString("❌ for.loop maybe bad"))
					fa.unsafe++
				} else {
					fa.logger.Println(color.GreenString("✓ for.loop is ok"))
				}
			} else { // Normal blocks (or loops without initialisation blocks).
				if len(blk.Instrs) > 1 {
					if ifInst, ok := blk.Instrs[len(blk.Instrs)-1].(*ssa.If); ok {
						_, thenVisited := visitedBlk[ifInst.Block().Succs[0]]
						_, elseVisited := visitedBlk[ifInst.Block().Succs[1]]
						if thenVisited || elseVisited { // there is a loop!
							fa.total++
							if !fa.isCondFair(ifInst.Cond) {
								fa.logger.Println(color.YellowString("Warning: recurring block condition probably unfair"))
								fa.unsafe++
							} else {
								fa.logger.Println(color.GreenString("✓ recurring block is ok"))
							}
						}
					} else if jInst, ok := blk.Instrs[len(blk.Instrs)-1].(*ssa.Jump); ok {
						if _, visited := visitedBlk[jInst.Block().Succs[0]]; visited {
							fa.total++
							fa.unsafe++
							fa.logger.Println(color.RedString("❌ infinite loop or recurring block, probably bad (%s)", fa.info.FSet.Position(blk.Instrs[0].Pos())))
						}
					}
				}
			}
		}
	}
}

// isLikelyUnsafe checks if a given "for.loop" block has non-static index and
// non-static loop condition.
func (fa *FairnessAnalysis) isLikelyUnsafe(blk *ssa.BasicBlock) bool {
	for _, instr := range blk.Instrs {
		switch instr := instr.(type) {
		case *ssa.DebugRef:
		case *ssa.If: // Last instruction of block
			if !fa.isCondFair(instr.Cond) {
				fa.logger.Println(color.YellowString("Warning: loop condition probably unfair"))
				return true // Definitely unsafe
			}
		}
	}
	// Reaching here mean the exit cond is not func call or constant
	if fa.isIndexStatic(blk) {
		// If index is static or unchanged (i.e. while loop), that means
		//   1. The exit condition is NOT index
		//   2. The exit condition dependent on 'outside' variable
		// TODO(nickng): check that if-condition is used in body
		fa.logger.Println(color.YellowString("Warning: cannot find loop index"))
		return true // Assume unsafe
	}
	return false
}

// isCondFair returns true if an if condition (bool expression) is constant.
func (fa *FairnessAnalysis) isCondFair(cond ssa.Value) bool {
	switch cond := cond.(type) {
	case *ssa.Const:
		fa.logger.Println(color.YellowString("Warning: loop condition is constant"))
		return false
	case *ssa.BinOp: // <, <=, !=, ==
		if _, xConst := cond.X.(*ssa.Const); xConst {
			if _, yConst := cond.Y.(*ssa.Const); yConst {
				fa.logger.Println(color.YellowString("Warning: loop condition is constant"))
				return false
			} else {
				fa.logger.Println(color.YellowString("Try to trace back on Y"))
			}
		} else {
			fa.logger.Println(color.YellowString("Try to trace back on X"))
		}
	case *ssa.UnOp:
		if _, con := cond.X.(*ssa.Const); con {
			fa.logger.Println(color.YellowString("Warning: loop condition is constant"))
			return false
		}
	case *ssa.Call:
		fa.logger.Println(color.YellowString("Warning:%s: condition is function call --> unsure", fa.info.FSet.Position(cond.Pos()).String()))
		return false
	}
	return true // Assume fair by default
}

// isIndexStatic returns true if a block does not have a modify-index Phi.
func (fa *FairnessAnalysis) isIndexStatic(blk *ssa.BasicBlock) bool {
	for _, instr := range blk.Instrs {
		switch instr := instr.(type) {
		case *ssa.DebugRef:
		case *ssa.Phi:
			if len(instr.Comment) > 0 {
				fa.logger.Println(color.BlueString("  note: Index var %s", instr.Comment))
				return false
			}
		}
	}
	fa.logger.Println(color.BlueString("  note: Index var not found"))
	return true
}

// Check for fairness on a built SSA
func Check(info *ssabuilder.SSAInfo) {
	if cgRoot := info.CallGraph(); cgRoot != nil {
		fa := NewFairnessAnalysis()
		fa.info = info
		fa.logger = log.New(logwriter.New(os.Stdout, true, true), "fairness: ", log.LstdFlags)
		cgRoot.Traverse(fa)
		if fa.unsafe <= 0 {
			fa.logger.Printf(color.GreenString("Result: %d/%d is likely unsafe", fa.unsafe, fa.total))
		} else {
			fa.logger.Printf(color.RedString("Result: %d/%d is likely unsafe", fa.unsafe, fa.total))
		}
	}
}
