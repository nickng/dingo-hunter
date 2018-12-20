package webservice

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/nickng/dingo-hunter/migoextract"
	"github.com/nickng/dingo-hunter/ssabuilder"
	"github.com/nickng/migo/v3/migoutil"
)

func migoHandler(w http.ResponseWriter, req *http.Request) {
	b, err := ioutil.ReadAll(req.Body)
	if err != nil {
		NewErrInternal(err, "Cannot read input Go source code").Report(w)
	}
	req.Body.Close()
	conf, err := ssabuilder.NewConfigFromString(string(b))
	if err != nil {
		NewErrInternal(err, "Cannot initialise SSA").Report(w)
	}
	info, err := conf.Build()
	if err != nil {
		NewErrInternal(err, "Cannot build SSA").Report(w)
	}
	extract, err := migoextract.New(info, ioutil.Discard)
	go extract.Run()

	select {
	case <-extract.Error:
		NewErrInternal(err, "MiGo type inference failed").Report(w)
	case <-extract.Done:
		log.Println("MiGo: analysis completed in", extract.Time)
		migoutil.SimplifyProgram(extract.Env.MigoProg)
	}

	reply := struct {
		MiGo string `json:"MiGo"`
		Time string `json:"time"`
	}{
		MiGo: extract.Env.MigoProg.String(),
		Time: extract.Time.String(),
	}
	json.NewEncoder(w).Encode(&reply)
}
