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
	"github.com/nickng/dingo-hunter/webservice"
	"github.com/spf13/cobra"
)

// serveCmd represents the serve command
var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Serve analysis examples as a webservice",
	Long: `Serve analysis examples as a webservice

The analysis will be presented side-by-side with its source code.
Each example is a Go command (i.e. package main) in a directory, under the examples directory.`,
	Run: func(cmd *cobra.Command, args []string) {
		webservice.Serve()
	},
}

func init() {
	RootCmd.AddCommand(serveCmd)

	serveCmd.Flags().IntVarP(&webservice.PortNumber, "port", "p", 6060, "Port number to serve on")
	serveCmd.Flags().StringVar(&webservice.ExampleDir, "example-dir", "examples/", "Path to examples directory")
	serveCmd.Flags().StringVar(&webservice.TemplateDir, "template-dir", "webservice/templates/", "Path to HTML templates directory")
	serveCmd.Flags().StringVar(&webservice.StaticDir, "static-dir", "webservice/static/", "Path to static files directory")
}
