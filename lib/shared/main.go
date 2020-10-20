// Package lib/shared/main provides a dummy implementation of main() to satisfy
// -buildmode=c-shared requirements. It imports gitql package from root folder of this repository.
package main

import (
	_ "go.riyazali.net/gitql/gitql"
)

func main() {}
