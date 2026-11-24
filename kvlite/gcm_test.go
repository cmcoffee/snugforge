package kvlite

import (
	"errors"
	"path/filepath"
	"reflect"
	"testing"

	bolt "go.etcd.io/bbolt"
)

// put_gcm stores value at table/key the way a release that writes GCM will:
// gob, sealed for that table and key, the GCM flag in front.
func put_gcm(t *testing.T, K *boltDB, table, key string, value interface{}) {
	t.Helper()
	put_raw(t, K, table, key, gcm_bytes(t, K.encoder, table, key, value))
}

func gcm_bytes(t *testing.T, e encoder, table, key string, value interface{}) []byte {
	t.Helper()
	v, err := e.encode(value)
	if err != nil {
		t.Fatal(err)
	}
	sealed, err := e.sealGCM(table, key, v)
	if err != nil {
		t.Fatal(err)
	}
	return append([]byte{flagGCM}, sealed...)
}

func put_raw(t *testing.T, K *boltDB, table, key string, v []byte) {
	t.Helper()
	err := K.db.Update(func(tx *bolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists([]byte(table))
		if err != nil {
			return err
		}
		return b.Put([]byte(key), v)
	})
	if err != nil {
		t.Fatal(err)
	}
}

func open_test_db(t *testing.T, padlock string) (Store, *boltDB, string) {
	t.Helper()
	file := filepath.Join(t.TempDir(), "gcm.db")
	db, err := Open(file, []byte(padlock)...)
	if err != nil {
		t.Fatal(err)
	}
	return db, db.(*boltDB), file
}

// A GCM value reads back through Get, ForEach and a sub-store, beside CFB and
// plain ones in the same table.
func TestGCMValuesRead(t *testing.T) {
	db, K, _ := open_test_db(t, "padlock")
	defer db.Close()
	want := record{"secret", []byte{9, 8, 7}}
	put_gcm(t, K, "t", "gcm", want)
	db.CryptSet("t", "cfb", record{"older", []byte{1}})
	db.Set("t", "plain", record{"plain", nil})
	put_gcm(t, K, "tenant\x1ft", "sub", 42)

	var got record
	if found, err := db.Get("t", "gcm", &got); !found || err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("Get = %v %v %v", got, found, err)
	}
	seen := map[string]bool{}
	err := db.ForEach("t", func(key string, decode func(interface{}) error) error {
		var r record
		if err := decode(&r); err != nil {
			return err
		}
		seen[key] = true
		return nil
	})
	if err != nil || len(seen) != 3 {
		t.Fatalf("ForEach saw %v, %v", seen, err)
	}
	var n int
	if found, err := db.Sub("tenant").Get("t", "sub", &n); !found || err != nil || n != 42 {
		t.Fatalf("sub-store GCM value = %d %v %v", n, found, err)
	}
}

// A GCM value that was altered, moved to another key or table, or written
// under another database key does not open: the read fails rather than
// returning something else.
func TestGCMValuesRefuseTampering(t *testing.T) {
	db, K, _ := open_test_db(t, "padlock")
	defer db.Close()
	v := gcm_bytes(t, K.encoder, "t", "k", "the secret")

	altered := append([]byte(nil), v...)
	altered[len(altered)-1] ^= 1
	put_raw(t, K, "t", "altered", altered)
	put_raw(t, K, "t", "moved", v) // sealed for t/k, stored at t/moved
	put_raw(t, K, "other", "k", v) // sealed for t/k, stored at other/k
	put_raw(t, K, "t", "short", []byte{flagGCM, 1, 2, 3})
	other_key := encoder(hashBytes([]byte("a different database key")))
	put_raw(t, K, "t", "foreign", gcm_bytes(t, other_key, "t", "foreign", "x"))

	for _, at := range [][2]string{{"t", "altered"}, {"t", "moved"}, {"other", "k"}, {"t", "short"}, {"t", "foreign"}} {
		var s string
		if _, err := db.Get(at[0], at[1], &s); !errors.Is(err, ErrTampered) {
			t.Errorf("%s/%s: got %q, %v; want ErrTampered", at[0], at[1], s, err)
		}
	}
	put_raw(t, K, "t", "k", v)
	var s string
	if _, err := db.Get("t", "k", &s); err != nil || s != "the secret" {
		t.Fatalf("the value in its own place = %q, %v", s, err)
	}
}

// This release still writes CFB: nothing it stores is unreadable to the
// release before it.
func TestCryptSetStillWritesCFB(t *testing.T) {
	db, _, _ := open_test_db(t, "padlock")
	defer db.Close()
	db.CryptSet("t", "a", "x")
	db.Batch(func(b Batch) error { return b.CryptSet("t", "b", "y") })
	for _, k := range []string{"a", "b"} {
		if v := raw(t, db, "t", k); len(v) == 0 || v[0] != flagCFB {
			t.Errorf("%s was written with flag %v, want CFB until GCM writes are switched on", k, v[:1])
		}
	}
	m := MemStore()
	m.CryptSet("t", "a", "x")
	var s string
	if _, err := m.Get("t", "a", &s); err != nil || s != "x" {
		t.Fatalf("memstore round trip = %q, %v", s, err)
	}
}

// A reset after a padlock mismatch clears GCM secrets as it clears CFB ones,
// and keeps the plain values.
func TestCryptResetClearsGCMValues(t *testing.T) {
	db, K, file := open_test_db(t, "padlock")
	put_gcm(t, K, "t", "gcm", "gone")
	db.CryptSet("t", "cfb", "gone")
	db.Set("t", "plain", "kept")
	db.Close()
	if err := CryptReset(file); err != nil {
		t.Fatal(err)
	}
	db, err := Open(file, []byte("another")...)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	keys, _ := db.Keys("t")
	if !reflect.DeepEqual(keys, []string{"plain"}) {
		t.Fatalf("after reset, keys = %v; want only the plain value", keys)
	}
}
