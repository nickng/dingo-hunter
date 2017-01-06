# dingo-hunter [![Build Status](https://travis-ci.org/nickng/dingo-hunter.svg?branch=master)](https://travis-ci.org/nickng/dingo-hunter)

## Static analyser for finding Deadlocks in Go

This is a static analyser to model concurrency and find deadlocks in Go code.
The main purpose of this tool is to infer from Go source code its concurrency
model in either of the two formats:

 - Communicating Finite State Machines (CFSMs)
 - MiGo types

The inferred models are then passed to separate tools for formal analysis.
In both approaches, we apply a system known in the literature as
[**Session Types**](http://mrg.doc.ic.ac.uk/publications/multiparty-asynchronous-session-types/)
to look for potential communication mismatches to preempt potential deadlocks.

## Install

`dingo-hunter` can be installed by `go get`, go version `go1.7.1` is required.

    $ go get -u github.com/nickng/dingo-hunter

## Usage

There are two approaches (CFSMs and MiGo types) based on two research work.

### CFSMs approach

This approach generates CFSMs as models for goroutines spawned in the program,
the CFSMs are then passed to a synthesis tool to construct a global choreography
and check for validity (See [paper][cc16]).

First install the synthesis tool `gmc-synthesis` by checking out the submodule:

    $ cd $GOPATH/src/github.com/nickng/dingo-hunter; git submodule init; git submodule update
    $ cd third_party/gmc-synthesis

Follow `README` to install and build `gmc-synthesis`, i.e.

    $ cabal install MissingH split Graphalyze
    $ ./getpetrify # and install to /usr/local/ or in $PATH
    $ ghc -threaded GMC.hs --make && ghc --make BuildGlobal

To run CFSMs generation on `example/local-deadlock/main.go`:

    $ dingo-hunter cfsms --prefix deadlock example/local-deadlock/main.go

Output should say 2 channels, then run synthesis tool on extracted CFSMs

    $ cd third_party/gmc-synthesis
    $ ./runsmc inputs/deadlock_cfsms 2 # where 2 is the number of channels

The `SMC check` line indicates if the global graph satisfies SMC (i.e. safe) or not.

#### Limitations

  * Our tool currently support synchronous (unbuffered channel) communication only
  * Goroutines spawned after any communication operations must not depend on
    those communication. Our model assumes goroutines are spawned independenly.

### MiGo types approach

This approach generates MiGo types, a behavioural type introduced in [this work][popl17],
to check for safety and liveness by a restriction called *fencing* on channel
usage (See [paper][popl17]).

The checker for MiGo types is available at
[nickng/gong](https://github.com/nickng/gong), follow the instructions to build
the tool:

    $ git clone ssh://github.com/nickng/gong.git
    $ cd gong; ghc Gong.hs

To run MiGo types generation on `example/local-deadlock/main.go`:

    $ dingo-hunter infer example/local-deadlock/main.go --no-logging --output deadlock.migo
    $ /path/to/Gong -A deadlock.migo

#### Limitations

  * Channels as return values are not supported right now
  * Channel recv,ok test not possible to represent in MiGo (requires inspecting
    value but abstracted by types)

## Research publications

  * [Static Deadlock Detection for Concurrent Go by Global Session Graph Synthesis][cc16],
    Nicholas Ng and Nobuko Yoshida,
    Int'l Conference on Compiler Construction (CC 2016), ACM
  * [Fencing off Go: Liveness and Safety for Channel-based Programming][popl17],
    Julien Lange, Nicholas Ng, Bernardo Toninho and Nobuko Yoshida,
    ACM SIGPLAN Symposium on Principles of Programming Languages (POPL 2017), ACM

## Notes

This is a research prototype, and may not work for all Go source code. Please
file an issue for problems that look like a bug.

## License

  dingo-hunter is licensed under the [Apache License](http://www.apache.org/licenses/LICENSE-2.0)

[cc16]: http://dl.acm.org/citation.cfm?doid=2892208.2892232 "Static Deadlock Detection for Concurrent Go by Global Graph Synthesis"
[popl17]: http://dl.acm.org/citation.cfm?doiid=3009837.3009847 "Fencing off Go: Liveness and Safety for Channel-based Programming"
