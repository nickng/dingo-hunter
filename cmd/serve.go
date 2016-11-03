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
	"go/build"
	"log"
	"path"

	"github.com/nickng/dingo-hunter/webservice"
	"github.com/spf13/cobra"
)

// serveCmd represents the serve command
var serveCmd = &cobra.Command{
	Use:     "serve",
	Aliases: []string{"server"},
	Short:   "Run an HTTP webservice for demo",
	Long: `Run an HTTP webservice for analysis demo.

The analysis will be presented side-by-side with its source code.
Each example is a Go command (i.e. package main) in a directory, under the examples directory.`,
	Run: func(cmd *cobra.Command, args []string) {
		Serve()
	},
}

const basePkg = "github.com/nickng/dingo-hunter"

var (
	addr string // Listen interface.
	port string // Listen port.
)

func init() {
	RootCmd.AddCommand(serveCmd)

	p, err := build.Default.Import(basePkg, "", build.FindOnly)
	if err != nil {
		log.Fatal("Could not find base path")
	}
	basePath := p.Dir

	serveCmd.Flags().StringVar(&addr, "bind", "127.0.0.1", "Bind address. Defaults to 127.0.0.1.")
	serveCmd.Flags().StringVar(&port, "port", "6060", "Listen port. Defaults to 6060.")
	serveCmd.Flags().StringVar(&webservice.ExamplesDir, "examples", path.Join(basePath, "examples", "popl17"), "Path to examples directory")
	serveCmd.Flags().StringVar(&webservice.TemplateDir, "templates", path.Join(basePath, "templates"), "Path to templates directory")
	serveCmd.Flags().StringVar(&webservice.StaticDir, "static", path.Join(basePath, "static"), "Path to static files directory")
}

// Serve starts the HTTP server.
func Serve() {
	server := webservice.NewServer(addr, port)
	server.Start()
	server.Close()
}
