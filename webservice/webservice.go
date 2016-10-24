// Package webservice runs a webservice to display source/SSA/analysis results.
package webservice

import (
	"bytes"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"path"
	"path/filepath"
	"strings"

	"github.com/nickng/dingo-hunter/logwriter"
	"github.com/nickng/dingo-hunter/migoextract"
	"github.com/nickng/dingo-hunter/ssabuilder"
)

var (
	PortNumber  int
	ExampleDir  string
	TemplateDir string
	StaticDir   string
)

type wsError struct {
	Error   error
	Message string
	Code    int
}

type wsHandler func(http.ResponseWriter, *http.Request) *wsError

func (fn wsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if e := fn(w, r); e != nil {
		http.Error(w, fmt.Sprintf("%s: %s", e.Message, e.Error), e.Code)
	}
}

func IndexHandler(w http.ResponseWriter, r *http.Request) *wsError {
	examples := make([]string, 0)
	t, err := template.ParseFiles(path.Join(TemplateDir, "index.tmpl"))
	if err != nil {
		return &wsError{Error: err, Message: "Cannot load template", Code: 500}
	}
	d, err := ioutil.ReadDir(ExampleDir)
	if err != nil {
		return &wsError{Error: err, Message: "Cannot read examples dir", Code: 500}
	}
	for _, f := range d {
		if f.IsDir() {
			examples = append(examples, f.Name())
		}
	}

	data := struct {
		Title    string
		Examples []string
	}{
		Title:    "Examples",
		Examples: examples,
	}
	err = t.Execute(w, data)
	if err != nil {
		return &wsError{Error: err, Message: "Template error", Code: 500}
	}
	return nil
}

func ExampleAnalysisHandler(w http.ResponseWriter, r *http.Request) *wsError {
	log.Println("Loading example", r.URL.Path)
	_, exampleName := path.Split(r.URL.Path)
	buf := new(bytes.Buffer)
	l := logwriter.New(buf, true, false)
	if err := l.Create(); err != nil {
		log.Fatal(err)
	}
	defer l.Cleanup()

	t, err := template.ParseFiles(path.Join(TemplateDir, "analysis.tmpl"))
	if err != nil {
		return &wsError{Error: err, Message: "Cannot load template", Code: 500}
	}

	goFiles, err := filepath.Glob(path.Join(ExampleDir, exampleName, "*.go"))
	if err != nil {
		return &wsError{Error: err, Message: "Bad pattern", Code: 500}
	}
	conf, err := ssabuilder.NewConfig(goFiles)
	if err != nil {
		return &wsError{Error: err, Message: "Create build config failed", Code: 500}
	}
	conf.LogFlags = 0
	conf.BuildLog = l.Writer
	conf.BadPkgs["flag"] = "Shorten analysis for demo"
	ssainfo, err := conf.Build()

	bufSSA := new(bytes.Buffer)
	lSSA := logwriter.New(bufSSA, true, false)
	if err := lSSA.Create(); err != nil {
		log.Fatal(err)
	}
	defer lSSA.Cleanup()
	ssainfo.WriteTo(lSSA.Writer)

	if err != nil {
		return &wsError{Error: err, Message: "SSA build failed", Code: 500}
	}
	extract, err := migoextract.New(ssainfo, l.Writer)
	if err != nil {
		return &wsError{Error: err, Message: "Type inference init failed", Code: 500}
	}
	go extract.Run()

	goSrc := make([]string, len(goFiles))
	for i, goFile := range goFiles {
		content, err := ioutil.ReadFile(goFile)
		if err != nil {
			return &wsError{Error: err, Message: "Cannot read source files", Code: 500}
		}
		goSrc[i] = string(content)
	}

	select {
	case err := <-extract.Error:
		return &wsError{Error: err, Message: "Analysis error", Code: 500}
	case <-extract.Done:
		// Block until infer is done
	}

	data := struct {
		Title string
		Time  string
		Code  string
		Log   string
		SSA   string
	}{
		Title: exampleName,
		Time:  extract.Time.String(),
		Code:  strings.Join(goSrc, "\n// ----- source file separator ----- \n"),
		Log:   buf.String(),
		SSA:   bufSSA.String(),
	}
	err = t.Execute(w, data)
	if err != nil {
		return &wsError{Error: err, Message: "Template error", Code: 500}
	}
	return nil
}

// Serve starts listening and serve HTTP.
func Serve() {
	fs := http.FileServer(http.Dir(StaticDir))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	http.Handle("/", wsHandler(IndexHandler))
	http.Handle("/example/", wsHandler(ExampleAnalysisHandler))
	addr := fmt.Sprintf(":%d", PortNumber)
	log.Println("Listening on", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}
