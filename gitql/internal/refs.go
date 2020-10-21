package internal

import (
	"fmt"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/storer"
	"go.riyazali.net/sqlite"
	"io"
	"strconv"
	"strings"
)

const (
	colRefsName = iota
	colRefsHash
	colRefsType
	colRefsRemote
)

// RefsModule implement sqlite.Module interface and provides a virtual table module that
// exposes git ref / branch information.
type RefsModule struct{}

func (r *RefsModule) Connect(args []string, declare func(string) error) (_ sqlite.VirtualTable, err error) {
	if len(args) != 4 {
		var err error
		if len(args) > 4 {
			err = fmt.Errorf("supplied more than required number of arguments")
		} else {
			err = fmt.Errorf("missing required argument: path to git repository")
		}
		return nil, err
	}

	var path string
	if path, err = strconv.Unquote(args[3]); err != nil {
		return nil, err
	}

	var repo *git.Repository
	if repo, err = git.PlainOpen(path); err != nil {
		return nil, err
	}

	if err = declare(fmt.Sprintf(`
		CREATE TABLE %q(
			name TEXT PRIMARY KEY,
			hash TEXT,
			type TEXT,
			remote BOOL
		) WITHOUT ROWID`, args[0])); err != nil {
		return nil, err
	}

	return &RefsTable{repo}, nil
}

// RefsTable implement sqlite.VirtualTable interface and provides virtual table for RefsModule
type RefsTable struct {
	repo *git.Repository // handle to git repository
}

func (r *RefsTable) BestIndex(_ *sqlite.IndexInfoInput) (*sqlite.IndexInfoOutput, error) {
	// there is no advantage in having an index here
	// or handling any filters by type, since, most of the
	// methods defined on git.Repository that filters by reference type
	// such as, Branches() or Notes() or Tags(), iterates through the whole range of
	// references anyways, filtering them out if they aren't of the given type.
	// And so, having an index doesn't really help for the refs table.
	return &sqlite.IndexInfoOutput{}, nil
}

func (r *RefsTable) Open() (sqlite.VirtualCursor, error) { return &RefsCursor{repo: r.repo}, nil }
func (r *RefsTable) Disconnect() error                   { return nil }
func (r *RefsTable) Destroy() error                      { return r.Disconnect() }

// RefsCursor implement sqlite.VirtualCursor interface and provides a cursor that indexes into RefsTable
type RefsCursor struct {
	repo *git.Repository      // handle to git repository
	iter storer.ReferenceIter // handle to reference iterator .. setup during Filter
	curr *plumbing.Reference  // pointer to current reference
}

func (r *RefsCursor) Filter(i int, s string, value ...sqlite.Value) (err error) {
	var iter storer.ReferenceIter
	if iter, err = r.repo.References(); err != nil {
		return err
	}

	// filter the references and don't include any _special_ references
	r.iter = storer.NewReferenceFilteredIter(func(r *plumbing.Reference) bool {
		return strings.HasPrefix(r.Name().String(), "refs/")
	}, iter)

	return r.Next() // reset to first position
}

func (r *RefsCursor) Next() (err error) {
	if r.curr, err = r.iter.Next(); err == io.EOF {
		return nil
	}
	return err
}

func (r *RefsCursor) Column(context *sqlite.Context, i int) error {
	switch i {
	case colRefsName:
		context.ResultText(r.curr.Name().String())
	case colRefsHash:
		var h = r.curr.Hash()
		if r.curr.Type() == plumbing.SymbolicReference {
			if resolved, err := r.repo.Reference(r.curr.Name(), true); err != nil {
				return err
			} else {
				h = resolved.Hash()
			}
		}
		context.ResultText(h.String())
	case colRefsType:
		context.ResultText(referenceNameToType(r.curr.Name()))
	case colRefsRemote:
		if r.curr.Name().IsRemote() {
			context.ResultInt(1)
		} else {
			context.ResultInt(0)
		}
	}
	return nil
}

func (r *RefsCursor) Close() error {
	if r.iter != nil {
		r.iter.Close()
	}
	return nil
}

func (r *RefsCursor) Rowid() (int64, error) { return 0, ErrNoRowid }
func (r *RefsCursor) Eof() bool             { return r.curr == nil }

func referenceNameToType(ref plumbing.ReferenceName) string {
	switch {
	case ref.IsBranch(), ref.IsRemote():
		return "branch"
	case ref.IsNote():
		return "note"
	case ref.IsTag():
		return "tag"
	}
	return "unknown"
}
