package webservice

import (
	"html/template"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
)

var (
	ExamplesDir string
	TemplateDir string
	StaticDir   string
)

func indexHandler(w http.ResponseWriter, req *http.Request) {
	var examples []string
	t, err := template.ParseFiles(path.Join(TemplateDir, "index.tmpl"))
	if err != nil {
		NewErrInternal(err, "Cannot load template").Report(w)
	}
	d, err := ioutil.ReadDir(ExamplesDir)
	if err != nil {
		NewErrInternal(err, "Cannot read examples").Report(w)
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
		Title:    "GoInfer/Gong demo",
		Examples: examples,
	}
	err = t.Execute(w, data)
	if err != nil {
		NewErrInternal(err, "Template execute failed").Report(w)
	}
}

func loadHandler(w http.ResponseWriter, req *http.Request) {
	b, err := ioutil.ReadAll(req.Body)
	if err != nil {
		NewErrInternal(err, "Cannot read input").Report(w)
	}
	if err := req.Body.Close(); err != nil {
		NewErrInternal(err, "Cannot close request").Report(w)
	}
	log.Println("Load example:", string(b))
	file, err := os.Open(path.Join(ExamplesDir, string(b), "main.go"))
	if err != nil {
		NewErrInternal(err, "Cannot open file").Report(w)
	}
	io.Copy(w, file)
}
