package webservice

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path"
	"strings"
	"time"
)

func synthesisHandler(w http.ResponseWriter, req *http.Request) {
	log.Println("Running SMC check on snippet")
	b, err := ioutil.ReadAll(req.Body)
	if err != nil {
		NewErrInternal(err, "Cannot read input CFSM").Report(w)
	}
	req.Body.Close()
	chanCFSMs := req.FormValue("chan")

	// ---- Executables ----
	log.Println("Finding required executables")
	gmc, err := exec.LookPath("GMC")
	if err != nil {
		NewErrInternal(err, "Cannot find GMC executable (Check $PATH?)").Report(w)
	}
	bg, err := exec.LookPath("BuildGlobal")
	if err != nil {
		NewErrInternal(err, "Cannot find BuildGobal executable (Check $PATH?)").Report(w)
	}
	petrify, err := exec.LookPath("petrify")
	if err != nil {
		NewErrInternal(err, "Cannot find petrify executable (Check $PATH?)").Report(w)
	}
	dot, err := exec.LookPath("dot")
	if err != nil {
		NewErrInternal(err, "Cannot find dot executable (Check $PATH?)").Report(w)
	}

	// ---- Output dirs/files ----
	baseDir := path.Join(os.TempDir(), "syn")
	err = os.MkdirAll(baseDir, 0777)
	if err != nil {
		NewErrInternal(err, "Cannot create temp dir").Report(w)
	}
	err = os.MkdirAll(path.Join(baseDir, "outputs"), 0777)
	if err != nil {
		NewErrInternal(err, "Cannot create final output dir").Report(w)
	}
	err = os.Chdir(baseDir)
	if err != nil {
		NewErrInternal(err, "Cannot chdir to temp dir").Report(w)
	}
	file, err := ioutil.TempFile(baseDir, "cfsm")
	if err != nil {
		NewErrInternal(err, "Cannot create temp file for CFSM input").Report(w)
	}
	defer os.Remove(file.Name())
	toPetrifyPath := path.Join(baseDir, "outputs", fmt.Sprintf("%s_toPetrify", path.Base(file.Name())))
	petriPath := path.Join(baseDir, "default")
	machinesDotPath := path.Join(baseDir, "outputs", fmt.Sprintf("%s_machines.dot", path.Base(file.Name())))
	globalDotPath := path.Join(baseDir, "outputs", "default_global.dot")

	if _, err := file.Write(b); err != nil {
		NewErrInternal(err, "Cannot write to temp file for CFSM input").Report(w)
	}
	if err := file.Close(); err != nil {
		NewErrInternal(err, "Cannot close temp file for CFSM input").Report(w)
	}

	// Replace symbols
	re := strings.NewReplacer("AAA", "->", "CCC", ",", "COCO", ":")
	outReplacer := strings.NewReplacer("True", "<span style='color: #87ff87; font-weight: bold'>True</span>", "False", "<span style='color: #ff005f; font-weight: bold'>False</span>")

	startTime := time.Now()
	gmcOut, err := exec.Command(gmc, file.Name(), chanCFSMs, "+RTS", "-N").CombinedOutput()
	if err != nil {
		log.Printf("GMC execution failed: %v\n", err)
	}
	petriOut, err := exec.Command(petrify, "-dead", "-ip", toPetrifyPath).CombinedOutput()
	if err != nil {
		log.Printf("petrify execution failed: %v\n", err)
	}
	ioutil.WriteFile(petriPath, []byte(re.Replace(string(petriOut))), 0664)
	bgOut, err := exec.Command(bg, petriPath).CombinedOutput()
	if err != nil {
		log.Printf("BuildGlobal execution failed: %v\n", err)
	}
	log.Println("BuildGlobal:", string(bgOut))

	execTime := time.Now().Sub(startTime)

	machinesSVG, err := exec.Command(dot, "-Tsvg", machinesDotPath).CombinedOutput()
	if err != nil {
		log.Printf("dot execution failed for : %v\n", err)
	}
	globalSVG, err := exec.Command(dot, "-Tsvg", globalDotPath).CombinedOutput()
	if err != nil {
		log.Printf("dot execution failed for : %v\n", err)
	}

	reply := struct {
		SMC      string `json:"SMC"`
		Machines string `json:"Machines"`
		Global   string `json:"Global"`
		Time     string `json:"time"`
	}{
		SMC:      outReplacer.Replace(string(gmcOut)),
		Machines: string(machinesSVG),
		Global:   string(globalSVG),
		Time:     execTime.String(),
	}
	log.Println("Synthesis completed in", execTime.String())
	json.NewEncoder(w).Encode(&reply)
}
