package webservice

import (
	"fmt"
	"log"
	"net/http"
)

type ErrInternal struct {
	cause error
	msg   string
}

func NewErrInternal(cause error, message string) *ErrInternal {
	return &ErrInternal{cause: cause, msg: message}
}

func (e *ErrInternal) Error() string {
	return fmt.Sprintf("%s: %v", e.msg, e.cause)
}

// Report sends internal server error to web client also logs to console.
func (e *ErrInternal) Report(w http.ResponseWriter) {
	http.Error(w, e.Error(), http.StatusInternalServerError)
	log.Fatal(e)
}
