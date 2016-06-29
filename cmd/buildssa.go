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
	"github.com/spf13/cobra"
)

// buildssaCmd represents the buildssa command
var buildssaCmd = &cobra.Command{
	Use:   "buildssa",
	Short: "Build SSA IR of the input source files",
	Long:  `Build SSA IR of the input source files`,
	Run: func(cmd *cobra.Command, args []string) {
		build(args)
	},
}

var (
	dumpSSA bool
	dumpAll bool
)

func init() {
	RootCmd.AddCommand(buildssaCmd)

	buildssaCmd.Flags().BoolVar(&dumpSSA, "dump", false, "dump SSA IR of input files (based on CFG)")
	buildssaCmd.Flags().BoolVar(&dumpAll, "dump-all", false, "dump all SSA IR of input files (including unused)")
	if dumpSSA && dumpAll {
		dumpSSA = false // dumpAll override dumpSSA
	}
}

func build(files []string) {
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
	if dumpSSA {
		if _, err := ssainfo.WriteTo(os.Stdout); err != nil {
			log.Fatal(err)
		}
	}
	if dumpAll {
		if _, err := ssainfo.WriteAll(os.Stdout); err != nil {
			log.Fatal(err)
		}
	}
}
