# dingo-hunter

## Static analyser for finding Deadlocks in Go

This is a static analyser to model concurrency and find deadlocks in Go code.
We use a system known in the literature as
[**Session Types**](http://mrg.doc.ic.ac.uk/publications/multiparty-asynchronous-session-types/)
to look for potential communication mismatches to preempt potential deadlocks.

## Usage

To run dingo-hunter on a command 'example':

    $ go build
    $ ./dingo-hunter example/main.go

## Contributors

  * [nickng](http://github.com/nickng)

## License

  dingo-hunter is licensed under the [Apache License](http://www.apache.org/licenses/LICENSE-2.0)
