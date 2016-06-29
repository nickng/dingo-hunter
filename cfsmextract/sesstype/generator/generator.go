// Package generator
package generator

import (
	"github.com/nickng/dingo-hunter/cfsmextract/sesstype"
	"io"
)

func GenCFSMs(s *sesstype.Session, w io.Writer) (n int, err error) {
	str := getCFSMs(s)
	return w.Write([]byte(str))
}

func GenDot(s *sesstype.Session, w io.Writer) (n int, err error) {
	str := getDot(s)
	return w.Write([]byte(str))
}
