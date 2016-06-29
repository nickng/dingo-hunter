package ssabuilder

import (
	"io"
	"sort"

	"github.com/nickng/dingo-hunter/ssabuilder/callgraph"
	"golang.org/x/tools/go/ssa"
)

type Members []ssa.Member

func (m Members) Len() int           { return len(m) }
func (m Members) Less(i, j int) bool { return m[i].Pos() < m[j].Pos() }
func (m Members) Swap(i, j int)      { m[i], m[j] = m[j], m[i] }

// WriteTo writes SSA IR of info.Prog to w.
func (info *SSAInfo) WriteTo(w io.Writer) (int64, error) {
	visitedFunc := make(map[*ssa.Function]bool)
	var n int64
	nodeQueue := []*callgraph.Node{info.CallGraph()}
	for len(nodeQueue) > 0 {
		head := nodeQueue[0]
		headFunc := head.Func
		nodeQueue = nodeQueue[1:]
		if _, ok := visitedFunc[headFunc]; !ok {
			visitedFunc[headFunc] = true
			written, err := headFunc.WriteTo(w)
			if err != nil {
				return n, err
			}
			n += written
		}
		for _, childNode := range head.Children {
			nodeQueue = append(nodeQueue, childNode)
		}
	}
	return n, nil
}

// WriteAll writes all SSA IR to w.
func (info *SSAInfo) WriteAll(w io.Writer) (int64, error) {
	pkgFuncs := make(map[*ssa.Package]Members)
	var n int64
	for _, pkg := range info.Prog.AllPackages() {
		pkgFuncs[pkg] = make(Members, 0)
		for _, memb := range pkg.Members {
			if f, ok := memb.(*ssa.Function); ok {
				pkgFuncs[pkg] = append(pkgFuncs[pkg], f)
			}
		}
		sort.Sort(pkgFuncs[pkg])
	}
	for pkg, funcs := range pkgFuncs {
		for _, f := range funcs {
			written, err := pkg.Func(f.Name()).WriteTo(w)
			if err != nil {
				return n, err
			}
			n += written
		}
	}
	return n, nil
}
