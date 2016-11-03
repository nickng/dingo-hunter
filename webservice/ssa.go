package webservice

import (
	"io/ioutil"
	"net/http"

	"github.com/nickng/dingo-hunter/ssabuilder"
)

func ssaHandler(w http.ResponseWriter, req *http.Request) {
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
	info.WriteTo(w)
}
