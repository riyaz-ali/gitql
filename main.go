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
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	_ "github.com/mattn/go-sqlite3"
	"github.com/pkg/errors"
	_ "go.riyazali.net/gitql/gitql"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
)

func main() {
	var err error
	var repo string

	flag.StringVar(&repo, "repo", ".", "path to git repository")
	flag.Parse()

	if repo, err = filepath.Abs(repo); err != nil {
		log.Fatal(errors.Wrap(err, "failed to resolve repository path"))
	}

	var query string
	if query = flag.Arg(0); query == "" {
		if fi, err := os.Stdin.Stat(); err == nil {
			if fi.Mode()&os.ModeCharDevice == 0 {
				var b []byte
				if b, err = ioutil.ReadAll(bufio.NewReader(os.Stdin)); err == nil {
					query = string(b)
					goto _continue
				}
			}
		}
		log.Fatal("please provide a valid sql query")
	}

_continue:

	var db *sql.DB
	if db, err = sql.Open("sqlite3", fmt.Sprintf("file:%x?mode=memory&cache=shared", repo)); err != nil {
		log.Fatal(errors.Wrap(err, "failed to open database connection"))
	}
	defer db.Close()

	// initialize the virtual tables
	for _, q := range []string{
		"CREATE VIRTUAL TABLE commits USING git_log(%q)",
		"CREATE VIRTUAL TABLE refs    USING git_ref(%q)",
	} {
		if _, err = db.Exec(fmt.Sprintf(q, repo)); err != nil {
			log.Fatal(errors.Wrap(err, "failed to create virtual table"))
		}
	}

	var rows *sql.Rows
	if rows, err = db.QueryContext(context.Background(), query); err != nil {
		log.Fatal(errors.Wrap(err, "failed to execute query"))
	}
	defer rows.Close()

	var columns []*sql.ColumnType
	if columns, err = rows.ColumnTypes(); err != nil {
		log.Fatal(errors.Wrap(err, "failed to fetch column details"))
	}

	var values = make([]interface{}, len(columns))
	for i := range values {
		values[i] = new(interface{})
	}

	var out = json.NewEncoder(os.Stdout)
	out.SetIndent("", "  ")

	for rows.Next() {
		if err = rows.Scan(values...); err != nil {
			log.Fatal(errors.Wrap(err, "failed to scan row"))
		}

		var dest = make(map[string]interface{})
		for i, column := range columns {
			dest[column.Name()] = *(values[i].(*interface{}))
		}

		if err = out.Encode(dest); err != nil {
			log.Fatal(errors.Wrap(err, "failed to write record"))
		}
	}

	if err = rows.Err(); err != nil {
		log.Fatal(errors.Wrap(err, "failed to query git data"))
	}
}

func init() {
	log.SetPrefix("gitql: ")
	log.SetFlags(log.Ldate | log.Ltime | log.Lmsgprefix)
}
