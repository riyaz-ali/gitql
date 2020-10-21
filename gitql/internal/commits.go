package internal

import (
	"errors"
	"fmt"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/storer"
	"go.riyazali.net/sqlite"
	"io"
	"strconv"
	"time"
)

var ErrNoRowid = errors.New("cannot generate rowid for commits")

const (
	colCommitHash = iota
	colCommitMessage
	colCommitAuthorName
	colCommitAuthorEmail
	colCommitAuthorWhen
	colCommitCommitterName
	colCommitCommitterEmail
	colCommitCommitterWhen
	colCommitParentId
	colCommitParentCount
	colCommitTreeId
	colCommitAddition
	colCommitDeletion
)

// CommitsModule implement sqlite.Module interface and provides a virtual table module that
// exposes commit / log information
type CommitsModule struct{}

func (c *CommitsModule) Connect(args []string, declare func(string) error) (_ sqlite.VirtualTable, err error) {
	// TODO: allow specifying alternative branch / head / committish
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
			hash TEXT PRIMARY KEY,
			message TEXT,
			author_name TEXT,
			author_email TEXT,
			author_when DATETIME,
			committer_name TEXT,
			committer_email TEXT,
			committer_when DATETIME,
			parent_id TEXT,
			parent_count INTEGER,
			tree_id TEXT,
			addition INTEGER,
			deletion INTEGER
		) WITHOUT ROWID`, args[0])); err != nil {
		return nil, err
	}

	return &CommitsTable{repo}, nil
}

// CommitsTable implement sqlite.VirtualTable interface and provides virtual table for CommitsModule
type CommitsTable struct {
	repo *git.Repository // handle to git repository
}

func (c *CommitsTable) BestIndex(input *sqlite.IndexInfoInput) (*sqlite.IndexInfoOutput, error) {
	var idx = 0                 // query plan bitmask
	var args = 0                // number of arguments that Filter accepts
	var cn = [3]int{-1, -1, -1} // constraints on hash and committer_when (GT[E] & LT[E])
	var output = &sqlite.IndexInfoOutput{
		ConstraintUsage: make([]*sqlite.ConstraintUsage, len(input.Constraints)),
	}

	for i, c := range input.Constraints {
		if c.ColumnIndex == colCommitHash {
			if (idx & 1) != 0 { // we've already seen another hash = '...' .. can't have multiple of them
				return nil, sqlite.SQLITE_CONSTRAINT
			}
			if c.Op == sqlite.INDEX_CONSTRAINT_EQ && c.Usable {
				idx |= 1
				cn[0] = i
			}
		}

		if c.ColumnIndex == colCommitCommitterWhen { // filter by committer_when timestamps
			if (c.Op == sqlite.INDEX_CONSTRAINT_GT || c.Op == sqlite.INDEX_CONSTRAINT_GE) && c.Usable {
				if (idx & 2) != 0 { // we've already seen one of > or >= .. can only use one
					return nil, sqlite.SQLITE_CONSTRAINT
				}
				idx |= 2
				cn[1] = i
			}

			if (c.Op == sqlite.INDEX_CONSTRAINT_LT || c.Op == sqlite.INDEX_CONSTRAINT_LE) && c.Usable {
				if (idx & 4) != 0 { // we've already seen one of < or <= .. can only use one
					return nil, sqlite.SQLITE_CONSTRAINT
				}
				idx |= 4
				cn[2] = i
			}
		}
	}

	for i := 0; i < 3; i++ {
		if j := cn[i]; j >= 0 {
			args++
			output.ConstraintUsage[j] = &sqlite.ConstraintUsage{ArgvIndex: args}
		}
	}

	if (idx & 1) != 0 { // only a single row will be fetched if hash = '...' is defined
		output.EstimatedCost = 1
		output.EstimatedRows = 1
		output.IdxFlags = sqlite.INDEX_SCAN_UNIQUE
	}

	if len(input.OrderBy) == 1 {
		if input.OrderBy[0].Desc {
			idx |= 8
			output.OrderByConsumed = true
		}
	}
	output.IndexNumber = idx
	return output, nil
}

func (c *CommitsTable) Open() (sqlite.VirtualCursor, error) { return &CommitsCursor{repo: c.repo}, nil }
func (c *CommitsTable) Disconnect() error                   { return nil }
func (c *CommitsTable) Destroy() error                      { return c.Disconnect() }

// CommitsCursor implement sqlite.VirtualCursor interface and provides a cursor that indexes into CommitsTable
type CommitsCursor struct {
	repo *git.Repository   // handle to git repository
	iter object.CommitIter // handle to commit iterator .. setup during Filter
	curr *object.Commit    // commit that is currently pointed to
}

func (c *CommitsCursor) Filter(idx int, _ string, values ...sqlite.Value) (err error) {
	var args = 0
	if (idx & 1) != 0 {
		var committish = values[args].Text()
		var h = plumbing.NewHash(committish)
		var commit *object.Commit
		if commit, err = c.repo.CommitObject(h); err != nil {
			if err == plumbing.ErrObjectNotFound {
				return nil
			} else {
				return err
			}
		} else {
			c.iter = &singleCommitIter{commit: commit}
		}
		return c.Next()
	}

	var lo = &git.LogOptions{All: true}

	if (idx & 2) != 0 {
		if since, err := parseTimestamp(values[args].Text()); err != nil {
			return err
		} else {
			lo.Since = &since
		}
		args++
	}

	if (idx & 4) != 0 {
		if until, err := parseTimestamp(values[args].Text()); err != nil {
			return err
		} else {
			lo.Until = &until
		}
		args++
	}

	if (idx & 8) != 0 {
		lo.Order = git.LogOrderCommitterTime
	}

	if c.iter, err = c.repo.Log(lo); err != nil {
		return err
	}

	return c.Next() // reset to first position
}

func (c *CommitsCursor) Next() (err error) {
	if c.curr, err = c.iter.Next(); err == io.EOF {
		return nil
	}
	return err
}

func (c *CommitsCursor) Column(context *sqlite.Context, i int) error {
	switch i {
	case colCommitHash:
		context.ResultText(c.curr.Hash.String())
	case colCommitMessage:
		context.ResultText(c.curr.Message)
	case colCommitAuthorName:
		context.ResultText(c.curr.Author.Name)
	case colCommitAuthorEmail:
		context.ResultText(c.curr.Author.Email)
	case colCommitAuthorWhen:
		context.ResultText(c.curr.Author.When.Format(time.RFC3339))
	case colCommitCommitterName:
		context.ResultText(c.curr.Committer.Name)
	case colCommitCommitterEmail:
		context.ResultText(c.curr.Committer.Email)
	case colCommitCommitterWhen:
		context.ResultText(c.curr.Committer.When.Format(time.RFC3339))
	case colCommitParentId:
		if c.curr.NumParents() > 0 {
			context.ResultText(c.curr.ParentHashes[0].String())
		}
	case colCommitParentCount:
		context.ResultInt(c.curr.NumParents())
	case colCommitTreeId:
		context.ResultText(c.curr.TreeHash.String())
	case colCommitAddition:
		if a, _, err := calcCommitStats(c.curr); err != nil {
			return err
		} else {
			context.ResultInt(a)
		}
	case colCommitDeletion:
		if _, d, err := calcCommitStats(c.curr); err != nil {
			return err
		} else {
			context.ResultInt(d)
		}
	}
	return nil
}

func (c *CommitsCursor) Close() error {
	if c.iter != nil {
		c.iter.Close()
	}
	return nil
}

func (c *CommitsCursor) Rowid() (int64, error) { return -1, ErrNoRowid }
func (c *CommitsCursor) Eof() bool             { return c.curr == nil }

// calcCommitStats computes total addition and deletion for a given commit
func calcCommitStats(commit *object.Commit) (add int, del int, err error) {
	var stats object.FileStats
	if stats, err = commit.Stats(); err != nil {
		return 0, 0, err
	}
	for i := range stats {
		add += stats[i].Addition
		del += stats[i].Deletion
	}
	return add, del, nil
}

type singleCommitIter struct {
	commit *object.Commit
	called bool
}

func (s *singleCommitIter) Next() (*object.Commit, error) {
	if s.called {
		return nil, io.EOF
	}
	s.called = true
	return s.commit, nil
}

func (s *singleCommitIter) ForEach(f func(*object.Commit) error) (err error) {
	if !s.called {
		s.called = true
		if err = f(s.commit); err == storer.ErrStop {
			return nil
		}
	}
	return err
}

func (s *singleCommitIter) Close() {}
