package kvlite

import (
	"bytes"
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

// CryptSet, Batch and a sub-store write GCM, a fresh nonce each time, and
// read back what was written.
func TestCryptSetWritesGCM(t *testing.T) {
	db, _, _ := open_test_db(t, "padlock")
	defer db.Close()
	db.CryptSet("t", "a", "x")
	db.CryptSet("t", "a2", "x")
	db.Batch(func(b Batch) error { return b.CryptSet("t", "b", "y") })
	db.Sub("s").CryptSet("t", "c", "z")
	for _, at := range [][2]string{{"t", "a"}, {"t", "b"}, {"s\x1ft", "c"}} {
		if v := raw(t, db, at[0], at[1]); len(v) == 0 || v[0] != flagGCM {
			t.Errorf("%s/%s was written with flag %v, want GCM", at[0], at[1], v[:1])
		}
	}
	if bytes.Equal(raw(t, db, "t", "a")[1:], raw(t, db, "t", "a2")[1:]) {
		t.Error("the same value written twice came out the same: no fresh nonce")
	}
	var s string
	if _, err := db.Sub("s").Get("t", "c", &s); err != nil || s != "z" {
		t.Fatalf("sub-store GCM round trip = %q, %v", s, err)
	}
	m := MemStore()
	m.CryptSet("t", "a", "x")
	s = ""
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

// UpgradeCipher re-encrypts the CFB values a file holds, in the root and in
// sub-stores, as GCM bound to where each is, and leaves plain and GCM values
// alone. Everything reads as before, and a second run finds nothing.
func TestUpgradeCipher(t *testing.T) {
	db, K, file := open_test_db(t, "padlock")
	for i := 0; i < upgradeChunk+3; i++ {
		old_set(t, K, "many", string(rune('a'+i%26))+string(rune('0'+i/26)), i, true)
	}
	old_set(t, K, "t", "secret", record{"secret", []byte{4, 5}}, true)
	old_set(t, K, "tenant\x1ft", "sub", "sub secret", true)
	old_set(t, K, "t", "plain", "plain", false)
	db.CryptSet("t", "already", "gcm")
	plain := raw(t, db, "t", "plain")
	already := raw(t, db, "t", "already")

	var seen []UpgradeProgress
	n, err := UpgradeCipher(db, func(p UpgradeProgress) { seen = append(seen, p) })
	if err != nil {
		t.Fatal(err)
	}
	if want := upgradeChunk + 3 + 2; n != want {
		t.Fatalf("re-encrypted %d, want %d", n, want)
	}
	// Told after the first chunk of the big table and after every table, and
	// the last word is all tables done with the full count.
	if len(seen) < 4 {
		t.Fatalf("progress was told %d time(s): %+v; want each table and the mid-table chunk", len(seen), seen)
	}
	last := seen[len(seen)-1]
	if last.TablesDone != last.Tables || last.Upgraded != n || last.Tables < 3 {
		t.Errorf("last progress %+v; want every table done and %d upgraded", last, n)
	}
	for i := 1; i < len(seen); i++ {
		if seen[i].TablesDone < seen[i-1].TablesDone || seen[i].Upgraded < seen[i-1].Upgraded {
			t.Errorf("progress went backwards: %+v then %+v", seen[i-1], seen[i])
		}
	}
	if !bytes.Equal(raw(t, db, "t", "plain"), plain) || !bytes.Equal(raw(t, db, "t", "already"), already) {
		t.Error("a plain or GCM value was rewritten")
	}
	db.Close()

	db, err = Open(file, []byte("padlock")...)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, at := range [][2]string{{"t", "secret"}, {"tenant\x1ft", "sub"}} {
		if v := raw(t, db, at[0], at[1]); len(v) == 0 || v[0] != flagGCM {
			t.Errorf("%s/%s is still flag %v", at[0], at[1], v[:1])
		}
	}
	var r record
	var s string
	var i int
	if _, err := db.Get("t", "secret", &r); err != nil || r.Name != "secret" {
		t.Errorf("secret reads %v, %v", r, err)
	}
	if _, err := db.Sub("tenant").Get("t", "sub", &s); err != nil || s != "sub secret" {
		t.Errorf("sub-store secret reads %q, %v", s, err)
	}
	if _, err := db.Get("many", "c0", &i); err != nil || i != 2 {
		t.Errorf("many/c0 reads %d, %v", i, err)
	}
	// A second run finds nothing, and still says how far its scan has got.
	var again []UpgradeProgress
	if n, err := UpgradeCipher(db, func(p UpgradeProgress) { again = append(again, p) }); n != 0 || err != nil {
		t.Errorf("a second run re-encrypted %d (%v)", n, err)
	}
	if len(again) == 0 || again[len(again)-1].TablesDone != again[len(again)-1].Tables {
		t.Errorf("a run with nothing to upgrade reported %+v; want each table scanned", again)
	}
	if n, _ := UpgradeCipher(MemStore(), nil); n != 0 {
		t.Errorf("a memory store reported %d upgraded", n)
	}
}
