package kvlite

import (
	"path/filepath"
	"testing"
)

// Changing the padlock keeps every value and moves which padlock opens the
// database; a wrong old padlock changes nothing.
func TestChangePadlockKeepsTheValues(t *testing.T) {
	file := filepath.Join(t.TempDir(), "test.db")
	db, err := Open(file, []byte("old-padlock")...)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.CryptSet("secrets", "api_key", "s3cr3t"); err != nil {
		t.Fatal(err)
	}
	if err := db.Set("plain", "name", "kept"); err != nil {
		t.Fatal(err)
	}
	db.Close()

	if err := ChangePadlock(file, []byte("wrong"), []byte("new-padlock")); err != ErrBadPadlock {
		t.Fatalf("a wrong old padlock: %v", err)
	}
	if err := ChangePadlock(file, []byte("old-padlock"), []byte("new-padlock")); err != nil {
		t.Fatal(err)
	}

	if _, err := Open(file, []byte("old-padlock")...); err != ErrBadPadlock {
		t.Fatalf("the old padlock still opens it: %v", err)
	}
	db, err = Open(file, []byte("new-padlock")...)
	if err != nil {
		t.Fatalf("the new padlock does not open it: %v", err)
	}
	defer db.Close()
	var secret, name string
	if _, err := db.Get("secrets", "api_key", &secret); err != nil || secret != "s3cr3t" {
		t.Fatalf("encrypted value = %q, %v", secret, err)
	}
	if _, err := db.Get("plain", "name", &name); err != nil || name != "kept" {
		t.Fatalf("plain value = %q, %v", name, err)
	}
}

// A database never opened has no padlock to change.
func TestChangePadlockOnAFreshFile(t *testing.T) {
	file := filepath.Join(t.TempDir(), "fresh.db")
	db, err := open(file)
	if err != nil {
		t.Fatal(err)
	}
	db.Close()
	if err := ChangePadlock(file, []byte("a"), []byte("b")); err != ErrNoPadlock {
		t.Fatalf("fresh file: %v", err)
	}
}

// OpenRekeying opens under the target, else finds the fallback that opens it,
// copies aside, moves it to the target, and opens it there; a later open needs
// only the target.
func TestOpenRekeyingMovesAnOldDatabase(t *testing.T) {
	file := filepath.Join(t.TempDir(), "test.db")
	db, err := Open(file, []byte("mac")...)
	if err != nil {
		t.Fatal(err)
	}
	db.CryptSet("s", "k", "v")
	db.Close()

	backedUp := false
	db, note, err := OpenRekeying(file, []byte("machine"), [][]byte{[]byte("ini"), []byte("mac")}, func() error { backedUp = true; return nil })
	if err != nil || !backedUp || note == "" {
		t.Fatalf("err %v, backed up %v, note %q", err, backedUp, note)
	}
	var v string
	db.Get("s", "k", &v)
	db.Close()
	if v != "v" {
		t.Fatalf("value after the move = %q", v)
	}
	db, note, err = OpenRekeying(file, []byte("machine"), nil, nil)
	if err != nil || note != "" {
		t.Fatalf("second open: %v %q", err, note)
	}
	db.Close()

	if _, _, err := OpenRekeying(file, []byte("other"), [][]byte{[]byte("mac")}, nil); err != ErrBadPadlock {
		t.Fatalf("no padlock that opens it: %v", err)
	}
}

// A failed copy-aside moves nothing: the database stays on the padlock that
// opened it.
func TestOpenRekeyingStaysPutWhenTheCopyFails(t *testing.T) {
	file := filepath.Join(t.TempDir(), "test.db")
	db, _ := Open(file, []byte("mac")...)
	db.Close()
	db, note, err := OpenRekeying(file, []byte("machine"), [][]byte{[]byte("mac")}, func() error { return ErrLocked })
	if err != nil || note == "" {
		t.Fatalf("err %v note %q", err, note)
	}
	db.Close()
	if db, err := Open(file, []byte("mac")...); err != nil {
		t.Fatalf("it moved anyway: %v", err)
	} else {
		db.Close()
	}
}
