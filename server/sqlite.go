package main

/*
#cgo LDFLAGS: -lsqlite3
#include <sqlite3.h>
#include <stdlib.h>

static int gb_bind_text(sqlite3_stmt *stmt, int idx, const char *s, int n) {
    return sqlite3_bind_text(stmt, idx, s, n, SQLITE_TRANSIENT);
}
*/
import "C"
import (
	"errors"
	"fmt"
	"sync"
	"unsafe"
)

// SQLiteDB is a minimal, zero-external-dependency, mutex-serialized CGO wrapper
// around libsqlite3. Single-writer serialization matches SQLite's concurrency model
// cleanly and eliminates SQLITE_BUSY errors.
type SQLiteDB struct {
	mu sync.Mutex
	db *C.sqlite3
}

var ErrNotFound = errors.New("not found")

func OpenSQLite(path string) (*SQLiteDB, error) {
	cPath := C.CString(path)
	defer C.free(unsafe.Pointer(cPath))

	var db *C.sqlite3
	flags := C.int(C.SQLITE_OPEN_READWRITE | C.SQLITE_OPEN_CREATE | C.SQLITE_OPEN_FULLMUTEX)
	rc := C.sqlite3_open_v2(cPath, &db, flags, nil)
	if rc != C.SQLITE_OK {
		msg := "failed to open sqlite"
		if db != nil {
			msg = C.GoString(C.sqlite3_errmsg(db))
			C.sqlite3_close(db)
		}
		return nil, errors.New(msg)
	}
	C.sqlite3_busy_timeout(db, 5000)
	return &SQLiteDB{db: db}, nil
}

func (s *SQLiteDB) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db != nil {
		C.sqlite3_close(s.db)
		s.db = nil
	}
	return nil
}

func (s *SQLiteDB) lastErrLocked() error {
	return errors.New(C.GoString(C.sqlite3_errmsg(s.db)))
}

func (s *SQLiteDB) ExecRaw(sqlStr string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.execRawLocked(sqlStr)
}

func (s *SQLiteDB) execRawLocked(sqlStr string) error {
	cSQL := C.CString(sqlStr)
	defer C.free(unsafe.Pointer(cSQL))
	var errMsg *C.char
	rc := C.sqlite3_exec(s.db, cSQL, nil, nil, &errMsg)
	if rc != C.SQLITE_OK {
		msg := "sqlite exec error"
		if errMsg != nil {
			msg = C.GoString(errMsg)
			C.sqlite3_free(unsafe.Pointer(errMsg))
		}
		return errors.New(msg)
	}
	return nil
}

func (s *SQLiteDB) prepareAndBindLocked(query string, args ...any) (*C.sqlite3_stmt, error) {
	cQuery := C.CString(query)
	defer C.free(unsafe.Pointer(cQuery))
	var stmt *C.sqlite3_stmt
	if rc := C.sqlite3_prepare_v2(s.db, cQuery, -1, &stmt, nil); rc != C.SQLITE_OK {
		return nil, fmt.Errorf("prepare failed (%s): %w", query, s.lastErrLocked())
	}
	for i, arg := range args {
		idx := C.int(i + 1)
		var rc C.int
		switch v := arg.(type) {
		case nil:
			rc = C.sqlite3_bind_null(stmt, idx)
		case int:
			rc = C.sqlite3_bind_int64(stmt, idx, C.sqlite3_int64(v))
		case int64:
			rc = C.sqlite3_bind_int64(stmt, idx, C.sqlite3_int64(v))
		case *int:
			if v == nil {
				rc = C.sqlite3_bind_null(stmt, idx)
			} else {
				rc = C.sqlite3_bind_int64(stmt, idx, C.sqlite3_int64(*v))
			}
		case *int64:
			if v == nil {
				rc = C.sqlite3_bind_null(stmt, idx)
			} else {
				rc = C.sqlite3_bind_int64(stmt, idx, C.sqlite3_int64(*v))
			}
		case bool:
			if v {
				rc = C.sqlite3_bind_int(stmt, idx, 1)
			} else {
				rc = C.sqlite3_bind_int(stmt, idx, 0)
			}
		case string:
			cs := C.CString(v)
			rc = C.gb_bind_text(stmt, idx, cs, C.int(len(v)))
			C.free(unsafe.Pointer(cs))
		case *string:
			if v == nil {
				rc = C.sqlite3_bind_null(stmt, idx)
			} else {
				cs := C.CString(*v)
				rc = C.gb_bind_text(stmt, idx, cs, C.int(len(*v)))
				C.free(unsafe.Pointer(cs))
			}
		default:
			C.sqlite3_finalize(stmt)
			return nil, fmt.Errorf("unsupported bind arg type %T at index %d", arg, idx)
		}
		if rc != C.SQLITE_OK {
			C.sqlite3_finalize(stmt)
			return nil, s.lastErrLocked()
		}
	}
	return stmt, nil
}

// SQLiteRow provides typed getters for the current row during Query callback.
type SQLiteRow struct {
	stmt *C.sqlite3_stmt
}

func (r SQLiteRow) IsNull(col int) bool {
	return C.sqlite3_column_type(r.stmt, C.int(col)) == C.SQLITE_NULL
}

func (r SQLiteRow) Int(col int) int {
	return int(C.sqlite3_column_int64(r.stmt, C.int(col)))
}

func (r SQLiteRow) Int64(col int) int64 {
	return int64(C.sqlite3_column_int64(r.stmt, C.int(col)))
}

func (r SQLiteRow) NullableInt(col int) *int {
	if r.IsNull(col) {
		return nil
	}
	v := int(C.sqlite3_column_int64(r.stmt, C.int(col)))
	return &v
}

func (r SQLiteRow) String(col int) string {
	if r.IsNull(col) {
		return ""
	}
	c := C.sqlite3_column_text(r.stmt, C.int(col))
	if c == nil {
		return ""
	}
	n := C.sqlite3_column_bytes(r.stmt, C.int(col))
	return C.GoStringN((*C.char)(unsafe.Pointer(c)), n)
}

func (r SQLiteRow) NullableString(col int) *string {
	if r.IsNull(col) {
		return nil
	}
	v := r.String(col)
	return &v
}

// Tx represents a locked execution context (with or without explicit BEGIN/COMMIT).
type Tx struct {
	s *SQLiteDB
}

func (t *Tx) Exec(query string, args ...any) (int64, error) {
	stmt, err := t.s.prepareAndBindLocked(query, args...)
	if err != nil {
		return 0, err
	}
	defer C.sqlite3_finalize(stmt)
	rc := C.sqlite3_step(stmt)
	if rc != C.SQLITE_DONE && rc != C.SQLITE_ROW {
		return 0, t.s.lastErrLocked()
	}
	return int64(C.sqlite3_changes(t.s.db)), nil
}

func (t *Tx) Query(query string, fn func(SQLiteRow) error, args ...any) error {
	stmt, err := t.s.prepareAndBindLocked(query, args...)
	if err != nil {
		return err
	}
	defer C.sqlite3_finalize(stmt)
	for {
		rc := C.sqlite3_step(stmt)
		if rc == C.SQLITE_ROW {
			if err := fn(SQLiteRow{stmt: stmt}); err != nil {
				return err
			}
			continue
		}
		if rc == C.SQLITE_DONE {
			return nil
		}
		return t.s.lastErrLocked()
	}
}

func (t *Tx) QueryRow(query string, fn func(SQLiteRow) error, args ...any) error {
	found := false
	err := t.Query(query, func(r SQLiteRow) error {
		if !found {
			found = true
			return fn(r)
		}
		return nil
	}, args...)
	if err != nil {
		return err
	}
	if !found {
		return ErrNotFound
	}
	return nil
}

func (s *SQLiteDB) WithLock(fn func(*Tx) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return fn(&Tx{s: s})
}

func (s *SQLiteDB) WithTx(fn func(*Tx) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.execRawLocked("BEGIN IMMEDIATE"); err != nil {
		return err
	}
	if err := fn(&Tx{s: s}); err != nil {
		_ = s.execRawLocked("ROLLBACK")
		return err
	}
	return s.execRawLocked("COMMIT")
}
