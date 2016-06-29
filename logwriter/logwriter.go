// Package logwriter wraps a io.Writer for dingo-hunter logging.
//
package logwriter // "github.com/nickng/dingo-hunter/logwriter"

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"

	"github.com/fatih/color"
)

// Writer is a log writer and its configurations.
type Writer struct {
	io.Writer

	LogFile       string
	EnableLogging bool
	EnableColour  bool
	Cleanup       func()
}

// New creates a new file writer.
func NewFile(logfile string, enableLogging, enableColour bool) *Writer {
	return &Writer{
		LogFile:       logfile,
		EnableLogging: enableLogging,
		EnableColour:  enableColour,
	}
}

// New creates a new log writer.
func New(w io.Writer, enableLogging, enableColour bool) *Writer {
	return &Writer{
		Writer:        w,
		EnableLogging: enableLogging,
		EnableColour:  enableColour,
	}
}

// Create initialises a new writer.
func (w *Writer) Create() error {
	color.NoColor = !w.EnableColour
	if !w.EnableLogging {
		w.Writer = ioutil.Discard
		w.Cleanup = func() {}
		return nil
	}
	if w.Writer != nil {
		w.Cleanup = func() {}
		return nil
	}
	if w.LogFile != "" {
		if f, err := os.Create(w.LogFile); err != nil {
			return fmt.Errorf("Failed to create log file: %s", err)
		} else {
			bufWriter := bufio.NewWriter(f)
			w.Writer = bufWriter
			w.Cleanup = func() {
				if err := bufWriter.Flush(); err != nil {
					log.Printf("flush: %s", err)
				}
				if err := f.Close(); err != nil {
					log.Printf("close: %s", err)
				}
			}
		}
	} else { // Logfile non-empty
		w.Writer = os.Stdout
		w.Cleanup = func() {}
	}
	return nil
}
