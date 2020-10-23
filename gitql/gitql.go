package gitql

import (
	"go.riyazali.net/gitql/gitql/internal"
	_ "go.riyazali.net/gitql/gitql/internal"
	"go.riyazali.net/sqlite"
)

// a slice of all registered modules
var modules = map[string]sqlite.Module{
	"git_log": &internal.CommitsModule{},
	"git_ref": &internal.RefsModule{},
}

func init() {
	sqlite.Register(func(api *sqlite.ExtensionApi) (sqlite.ErrorCode, error) {
		for name, impl := range modules {
			if err := api.CreateModule(name, impl); err != nil {
				return sqlite.SQLITE_ERROR, err
			}
		}
		return sqlite.SQLITE_OK, nil
	})
}
