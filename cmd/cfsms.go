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

	"github.com/nickng/dingo-hunter/cfsmextract"
	"github.com/nickng/dingo-hunter/logwriter"
	"github.com/nickng/dingo-hunter/ssabuilder"
	"github.com/spf13/cobra"
)

var (
	prefix string // Output files prefix
	outdir string // CFMSs output directory
)

// cfsmsCmd represents the analyse command
var cfsmsCmd = &cobra.Command{
	Use:   "cfsms",
	Short: "Extract CFSMs from source code",
	Long: `Extract CFSMs from source code

The inputs should be a list of .go files in the same directory (of package main)
One of the .go file should contain the main function.`,
	Run: func(cmd *cobra.Command, args []string) {
		extractCFSMs(args)
	},
}

func init() {
	cfsmsCmd.Flags().StringVar(&prefix, "prefix", "output", "Output files prefix")
	cfsmsCmd.Flags().StringVar(&outdir, "outdir", "third_party/gmc-synthesis/inputs", "Output directory for CFSMs")

	RootCmd.AddCommand(cfsmsCmd)
}

func extractCFSMs(files []string) {
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
	conf.BuildLog = l.Writer
	if err != nil {
		log.Fatal(err)
	}
	ssainfo, err := conf.Build()
	if err != nil {
		log.Fatal(err)
	}
	extract := cfsmextract.New(ssainfo, prefix, outdir)
	go extract.Run()

	select {
	case <-extract.Error:
		log.Fatal(err)
	case <-extract.Done:
		log.Println("Analysis finished in", extract.Time)
		extract.WriteOutput()
	}
}
