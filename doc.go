// Command dingo-hunter is a tool for analysing Go code to extract the
// communication patterns for deadlock analysis.
//
// The deadlock analysis approach is based on multiparty session types and
// global graph synthesis POPL'15 (Lange, Tuosto, Yoshida) and CC'16 (Ng,
// Yoshida).
package main
