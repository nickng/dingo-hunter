// Copyright © 2016 Nicholas Ng <nickng@projectfate.org>
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"log"
	"os"

	"github.com/nickng/dingo-hunter/logwriter"
	"github.com/nickng/dingo-hunter/ssabuilder"
	"github.com/nickng/dingo-hunter/typeinfer"
	"github.com/spf13/cobra"
)

var (
	outfile string // Path to output file
)

// inferCmd represents the analyse command
var inferCmd = &cobra.Command{
	Use:   "infer",
	Short: "Run analysis on the inputs",
	Long: `Run analysis and type inference on the inputs

The inputs should be a list of .go files in the same directory (of package main)
One of the .go file should contain the main function.`,
	Run: func(cmd *cobra.Command, args []string) {
		infer(args)
	},
}

func init() {
	inferCmd.Flags().StringVar(&outfile, "output", "", "output migo file")

	RootCmd.AddCommand(inferCmd)
}

func infer(files []string) {
	logFile, err := RootCmd.PersistentFlags().GetString("log")
	if err != nil {
		log.Fatal(err)
	}
	noLogging, err := RootCmd.PersistentFlags().GetBool("no-logging")
	if err != nil {
		log.Fatal(err)
	}
	noColour, err := RootCmd.PersistentFlags().GetBool("no-colour")
	if err != nil {
		log.Fatal(err)
	}
	l := logwriter.NewFile(logFile, !noLogging, !noColour)
	if err := l.Create(); err != nil {
		log.Fatal(err)
	}
	defer l.Cleanup()

	conf, err := ssabuilder.NewConfig(files)
	if err != nil {
		log.Fatal(err)
	}
	conf.BuildLog = l.Writer
	ssainfo, err := conf.Build()
	if err != nil {
		log.Fatal(err)
	}
	infer, err := typeinfer.New(ssainfo, l.Writer)
	if err != nil {
		log.Fatal(err)
	}
	go infer.Run()

	select {
	case err := <-infer.Error:
		log.Fatal(err)
	case <-infer.Done:
		infer.Logger.Println("Analysis finished in", infer.Time)
	}

	if outfile != "" {
		f, err := os.Create(outfile)
		if err != nil {
			log.Fatal(err)
		}
		f.WriteString(infer.Env.MigoProg.String())
		defer f.Close()
	} else {
		os.Stdout.WriteString(infer.Env.MigoProg.String())
	}
}
