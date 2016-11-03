package webservice

import (
	"bytes"
	"fmt"
	"go/build"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"path/filepath"
	"time"

	"golang.org/x/tools/godoc/static"
	"golang.org/x/tools/playground/socket"
)

const basePkg = "golang.org/x/tools/cmd/present"

var scripts = []string{"jquery.js", "jquery-ui.js", "playground.js", "play.js"}

func initPlayground(origin *url.URL) {
	p, err := build.Default.Import(basePkg, "", build.FindOnly)
	if err != nil {
		log.Fatalf("Could not find gopresent files: %v", err)
	}
	basePath := p.Dir

	playScript(basePath, "SocketTransport")
	http.Handle("/socket", socket.NewHandler(origin))
}

func playScript(root, transport string) {
	modTime := time.Now()
	var buf bytes.Buffer
	for _, p := range scripts {
		if s, ok := static.Files[p]; ok {
			buf.WriteString(s)
			continue
		}
		b, err := ioutil.ReadFile(filepath.Join(root, "static", p))
		if err != nil {
			panic(err)
		}
		buf.Write(b)
	}
	fmt.Fprintf(&buf, "\ninitPlayground(new %v());\n", transport)
	b := buf.Bytes()
	http.HandleFunc("/play.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-type", "application/javascript")
		http.ServeContent(w, r, "", modTime, bytes.NewReader(b))
	})
}
