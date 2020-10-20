package gitql

import (
	"go.riyazali.net/sqlite"
)

func init() {
	sqlite.Register(func(api *sqlite.ExtensionApi) (sqlite.ErrorCode, error) {
		return sqlite.SQLITE_OK, nil
	})
}
