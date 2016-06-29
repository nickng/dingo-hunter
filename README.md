# dingo-hunter

## Static analyser for finding Deadlocks in Go

[![Build Status](https://travis-ci.org/nickng/dingo-hunter.svg?branch=master)](https://travis-ci.org/nickng/dingo-hunter)

This is a static analyser to model concurrency and find deadlocks in Go code.
We use a system known in the literature as
[**Session Types**](http://mrg.doc.ic.ac.uk/publications/multiparty-asynchronous-session-types/)
to look for potential communication mismatches to preempt potential deadlocks.

## Install

    $ go get -u github.com/nickng/dingo-hunter
    $ cd $GOPATH/nickng/dingo-hunter
    $ git submodule init; git submodule update
    $ cd third_party/gmc-synthesis

Follow `README` to install and build `gmc-synthesis`, i.e.

    $ cabal install MissingH split Graphalyze
    $ ./getpetrify # and install to /usr/local/ or in $PATH
    $ ghc -threaded GMC.hs --make && ghc --make BuildGlobal

## Example usage

To run dingo-hunter CFSMs generation on `example/deadlock/main.go`:

    $ dingo-hunter cfsms --prefix deadlock example/deadlock/main.go

Output should say 2 channels, then run synthesis tool on extracted CFSMs

    $ cd third_party/gmc-synthesis
    $ ./runsmc inputs/deadlock_cfsms 2 # where 2 is the number of channels

`SMC check` line indicates if the global graph satisfies SMC (i.e. safe) or not.

## Publications

  * [Static Deadlock Detection for Concurrent Go by Global Session Graph Synthesis](http://dl.acm.org/citation.cfm?doid=2892208.2892232), Nicholas Ng and Nobuko Yoshida, Int'l Conference on Compiler Construction (CC 2016), ACM

## Contributors

  * [nickng](http://github.com/nickng)

## Limitations

  * Our tool currently support synchronous (unbuffered channel) communication only
  * Goroutines spawned after any communication operations must not depend on
    those communication. Our model assumes goroutines are spawned independenly.

## License

  dingo-hunter is licensed under the [Apache License](http://www.apache.org/licenses/LICENSE-2.0)
