// Package main provides a cli tool that exposes a cui over the gitql extension.
package main

// #cgo CFLAGS: -I ${SRCDIR}/.build/sqlite
// #cgo LDFLAGS: -L${SRCDIR}/.build -lsqlite3
//
// #include "sqlite3.h"
//
// // extension function defined in the archive from go.riyazali.net/sqlite
// // the symbol is only available during the final linkage when compiling the binary
// // and so, the linker would complain for missing symbol during an intermediate step.
// // Refer o Makefile to see how it's suppressed.
// int sqlite3_extension_init(sqlite3*, char**, const sqlite3_api_routines*);
//
// // Use constructor to register extension function with sqlite.
// // The extension is automatically registered for all sqlite databases opened during the lifetime of this app.
// // See https://stackoverflow.com/q/2053029/6611700 for details about constructor function.
// void __attribute__((constructor)) init(void) {
//   sqlite3_auto_extension((void*) sqlite3_extension_init);
// }
import "C"
import (
	_ "github.com/mattn/go-sqlite3"
	_ "go.riyazali.net/gitql/gitql"
)

func main() {

}
