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

	"github.com/nickng/dingo-hunter/fairness"
	"github.com/nickng/dingo-hunter/logwriter"
	"github.com/nickng/dingo-hunter/ssabuilder"
	"github.com/spf13/cobra"
)

// checkfairCmd represents the check-fair command
var checkfairCmd = &cobra.Command{
	Use:   "checkfair",
	Short: "Runs loop fairness checks",
	Long: `Runs loop fairness checks

The checks will find potential problematic loops, such as those which are
unbalanced - loop conditions favour infinite iterations deterministically`,
	Run: func(cmd *cobra.Command, args []string) {
		check(args)
	},
}

func init() {
	RootCmd.AddCommand(checkfairCmd)
}

func check(files []string) {
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
	fairness.Check(ssainfo)
}
