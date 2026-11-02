package kvlite

import (
	"bytes"
	"errors"
	"path/filepath"
	"reflect"
	"testing"

	bolt "go.etcd.io/bbolt"
)

type record struct {
	Name string
	Data []byte
}

// each_store runs test against a bolt store in a temp dir and a MemStore.
func each_store(t *testing.T, test func(t *testing.T, db Store)) {
	t.Run("bolt", func(t *testing.T) {
		db, err := Open(filepath.Join(t.TempDir(), "test.db"), []byte("padlock")...)
		if err != nil {
			t.Fatal(err)
		}
		defer db.Close()
		test(t, db)
	})
	t.Run("mem", func(t *testing.T) {
		db := MemStore()
		defer db.Close()
		test(t, db)
	})
}

func isBolt(db Store) bool {
	_, ok := db.(*boltDB)
	return ok
}

// commits is the number of write transactions a bolt store has committed.
func commits(t *testing.T, db Store) (id int) {
	t.Helper()
	err := db.(*boltDB).db.View(func(tx *bolt.Tx) error {
		id = tx.ID()
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return id
}

// raw is the bytes stored at key in table of a bolt store.
func raw(t *testing.T, db Store, table, key string) (v []byte) {
	t.Helper()
	db.(*boltDB).db.View(func(tx *bolt.Tx) error {
		if b := tx.Bucket([]byte(table)); b != nil {
			v = append([]byte(nil), b.Get([]byte(key))...)
		}
		return nil
	})
	return v
}

// ForEach decodes plain and encrypted values alike, in key order.
func TestForEachDecodes(t *testing.T) {
	each_store(t, func(t *testing.T, db Store) {
		db.Set("t", "a", record{"plain", []byte{1, 2}})
		db.CryptSet("t", "b", record{"secret", []byte{3}})
		db.Set("t", "c", "just a string")

		var keys []string
		got := make(map[string]record)
		err := db.ForEach("t", func(key string, decode func(out interface{}) error) error {
			keys = append(keys, key)
			if key == "c" {
				var s string
				if err := decode(&s); err != nil || s != "just a string" {
					t.Errorf("c = %q, %v", s, err)
				}
				return nil
			}
			var r record
			if err := decode(&r); err != nil {
				return err
			}
			got[key] = r
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(keys, []string{"a", "b", "c"}) {
			t.Fatalf("keys = %v", keys)
		}
		if !reflect.DeepEqual(got["a"], record{"plain", []byte{1, 2}}) || !reflect.DeepEqual(got["b"], record{"secret", []byte{3}}) {
			t.Fatalf("values = %v", got)
		}

		// The Table view reads the same.
		n := 0
		db.Table("t").ForEach(func(string, func(interface{}) error) error { n++; return nil })
		if n != 3 {
			t.Fatalf("Table ForEach saw %d keys", n)
		}
	})
}

// Keys whose values are not needed can skip decode; decode after fn
// returns is refused.
func TestForEachSkipsDecode(t *testing.T) {
	each_store(t, func(t *testing.T, db Store) {
		for _, k := range []string{"a", "b", "c", "d"} {
			db.Set("t", k, k+"-value")
		}
		var later func(interface{}) error
		var decoded []string
		err := db.ForEach("t", func(key string, decode func(out interface{}) error) error {
			if key != "c" {
				return nil
			}
			later = decode
			var s string
			if err := decode(&s); err != nil {
				return err
			}
			decoded = append(decoded, s)
			return nil
		})
		if err != nil || !reflect.DeepEqual(decoded, []string{"c-value"}) {
			t.Fatalf("decoded %v, %v", decoded, err)
		}
		var s string
		if err := later(&s); err != ErrDecodeAfterReturn {
			t.Fatalf("decode after return: %v", err)
		}
	})
}

// An error from fn stops the read and comes back unchanged; a missing
// table calls fn zero times.
func TestForEachStopsAndMissingTable(t *testing.T) {
	each_store(t, func(t *testing.T, db Store) {
		for _, k := range []string{"a", "b", "c"} {
			db.Set("t", k, 1)
		}
		stop := errors.New("stop")
		calls := 0
		err := db.ForEach("t", func(key string, _ func(interface{}) error) error {
			calls++
			if key == "b" {
				return stop
			}
			return nil
		})
		if err != stop || calls != 2 {
			t.Fatalf("err %v after %d calls", err, calls)
		}

		calls = 0
		err = db.ForEach("missing", func(string, func(interface{}) error) error { calls++; return nil })
		if err != nil || calls != 0 {
			t.Fatalf("missing table: err %v, %d calls", err, calls)
		}
	})
}

// Through a Sub store, ForEach sees only that namespace's keys.
func TestForEachSub(t *testing.T) {
	each_store(t, func(t *testing.T, db Store) {
		sub := db.Sub("tenant")
		db.Set("t", "root", 1)
		sub.Set("t", "mine", 2)
		sub.Sub("inner").Set("t", "deeper", 3)
		db.Sub("other").Set("t", "theirs", 4)

		for name, store := range map[string]Store{"sub": sub, "table": nil} {
			var keys []string
			fn := func(key string, decode func(interface{}) error) error {
				var n int
				if err := decode(&n); err != nil || n != 2 {
					t.Errorf("%s: %s = %d, %v", name, key, n, err)
				}
				keys = append(keys, key)
				return nil
			}
			var err error
			if store != nil {
				err = store.ForEach("t", fn)
			} else {
				err = sub.Table("t").ForEach(fn)
			}
			if err != nil || !reflect.DeepEqual(keys, []string{"mine"}) {
				t.Fatalf("%s: keys %v, %v", name, keys, err)
			}
		}
	})
}

// Keys and decoded values handed out by ForEach stay good once the file is
// unmapped: were they pointing into it, reading them would fault.
func TestForEachDoesNotAliasTheFile(t *testing.T) {
	file := filepath.Join(t.TempDir(), "test.db")
	db, err := Open(file, []byte("padlock")...)
	if err != nil {
		t.Fatal(err)
	}
	payload := bytes.Repeat([]byte("abcdefgh"), 512)
	for _, k := range []string{"plain", "crypt"} {
		if k == "plain" {
			err = db.Set("t", k, record{k, payload})
		} else {
			err = db.CryptSet("t", k, record{k, payload})
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	var keys []string
	var values []record
	err = db.ForEach("t", func(key string, decode func(interface{}) error) error {
		var r record
		keys = append(keys, key)
		values = append(values, r)
		return decode(&values[len(values)-1])
	})
	if err != nil {
		t.Fatal(err)
	}
	db.Close()

	if !reflect.DeepEqual(keys, []string{"crypt", "plain"}) {
		t.Fatalf("keys = %v", keys)
	}
	for i, r := range values {
		if r.Name != keys[i] || !bytes.Equal(r.Data, payload) {
			t.Fatalf("value %d = %q, %d bytes", i, r.Name, len(r.Data))
		}
	}
}

// A batch makes its Set, CryptSet and Unset writes across tables, in the
// order asked, in one commit.
func TestBatchCommits(t *testing.T) {
	each_store(t, func(t *testing.T, db Store) {
		db.Set("one", "gone", "x")
		db.Set("one", "kept", "x")

		var before int
		if isBolt(db) {
			before = commits(t, db)
		}
		err := db.Batch(func(b Batch) error {
			b.Set("one", "a", "plain")
			b.CryptSet("two", "b", "secret")
			b.Unset("one", "gone")
			b.Set("two", "c", "first")
			b.Set("two", "c", "second")
			b.Set("two", "d", "unset after")
			b.Unset("two", "d")
			b.Unset("never", "made")
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		if isBolt(db) {
			if n := commits(t, db) - before; n != 1 {
				t.Fatalf("batch took %d commits", n)
			}
			if v := raw(t, db, "one", "a"); v[0] != 0 {
				t.Fatalf("Set flag = %d", v[0])
			}
			if v := raw(t, db, "two", "b"); v[0] != 1 {
				t.Fatalf("CryptSet flag = %d", v[0])
			}
		}

		want := map[string]map[string]string{
			"one": {"a": "plain", "kept": "x"},
			"two": {"b": "secret", "c": "second"},
		}
		for table, kv := range want {
			if n, _ := db.CountKeys(table); n != len(kv) {
				t.Fatalf("%s has %d keys", table, n)
			}
			for k, v := range kv {
				var s string
				if found, err := db.Get(table, k, &s); !found || err != nil || s != v {
					t.Fatalf("%s/%s = %q %v %v", table, k, s, found, err)
				}
			}
		}
		if found, _ := db.Get("never", "made", nil); found {
			t.Fatal("Unset made a value")
		}
		if n, _ := db.CountKeys("never"); n != 0 {
			t.Fatal("Unset made a table")
		}
	})
}

// A batch whose fn fails writes nothing.
func TestBatchRollsBack(t *testing.T) {
	each_store(t, func(t *testing.T, db Store) {
		db.Set("one", "a", "before")
		fail := errors.New("fail")
		err := db.Batch(func(b Batch) error {
			b.Set("one", "a", "after")
			b.Unset("one", "a")
			b.CryptSet("new", "b", "secret")
			return fail
		})
		if err != fail {
			t.Fatalf("err = %v", err)
		}
		var s string
		if found, _ := db.Get("one", "a", &s); !found || s != "before" {
			t.Fatalf("one/a = %q %v", s, found)
		}
		tables, _ := db.Tables()
		if !reflect.DeepEqual(tables, []string{"one"}) {
			t.Fatalf("tables = %v", tables)
		}
	})
}

// A batch through a Sub store writes to that namespace.
func TestBatchSub(t *testing.T) {
	each_store(t, func(t *testing.T, db Store) {
		sub := db.Sub("tenant")
		sub.Set("t", "gone", 0)
		err := sub.Batch(func(b Batch) error {
			b.Set("t", "a", 1)
			b.CryptSet("u", "b", 2)
			return b.Unset("t", "gone")
		})
		if err != nil {
			t.Fatal(err)
		}
		var n int
		if found, _ := sub.Get("t", "a", &n); !found || n != 1 {
			t.Fatalf("tenant t/a = %d %v", n, found)
		}
		if found, _ := sub.Get("u", "b", &n); !found || n != 2 {
			t.Fatalf("tenant u/b = %d %v", n, found)
		}
		if found, _ := sub.Get("t", "gone", nil); found {
			t.Fatal("tenant t/gone still there")
		}
		if found, _ := db.Get("t", "a", nil); found {
			t.Fatal("batch wrote outside the sub store")
		}
		all, _ := db.AllTables()
		for _, name := range all {
			if name != "tenant\x1ft" && name != "tenant\x1fu" {
				t.Fatalf("unexpected table %q in %v", name, all)
			}
		}
	})
}

// Drop removes a table and its sub-tables, in one commit, and leaves
// neighbors whose names only start the same.
func TestDrop(t *testing.T) {
	each_store(t, func(t *testing.T, db Store) {
		db.Set("t", "k", 1)
		db.Sub("t").Set("inner", "k", 1)
		db.Sub("t").Sub("deeper").Set("x", "k", 1)
		db.Set("tx", "k", 1)
		db.Sub("other").Set("t", "k", 1)

		var before int
		if isBolt(db) {
			before = commits(t, db)
		}
		if err := db.Drop("t"); err != nil {
			t.Fatal(err)
		}
		if isBolt(db) {
			if n := commits(t, db) - before; n != 1 {
				t.Fatalf("drop took %d commits", n)
			}
		}
		all, _ := db.AllTables()
		want := map[string]bool{"tx": true, "other\x1ft": true}
		if len(all) != len(want) {
			t.Fatalf("tables left = %q", all)
		}
		for _, name := range all {
			if !want[name] {
				t.Fatalf("tables left = %q", all)
			}
		}
		if err := db.Drop("missing"); err != nil {
			t.Fatalf("dropping a missing table: %v", err)
		}
		if err := db.Table("tx").Drop(); err != nil {
			t.Fatal(err)
		}
		if n, _ := db.CountKeys("tx"); n != 0 {
			t.Fatal("Table Drop left the table")
		}
	})
}

// Drop gives back the error that stopped it.
func TestDropReturnsErrors(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	db.Set("t", "k", 1)
	db.Close()
	if err := db.Drop("t"); err != bolt.ErrDatabaseNotOpen {
		t.Fatalf("drop on a closed database: %v", err)
	}
}

// CountKeys counts every key and gives 0 for a missing table.
func TestCountKeys(t *testing.T) {
	each_store(t, func(t *testing.T, db Store) {
		for i := 0; i < 1500; i++ {
			db.Set("t", string(rune('a'+i%26))+string(rune(i)), i)
		}
		db.CryptSet("t", "secret", 1)
		if n, err := db.CountKeys("t"); err != nil || n != 1501 {
			t.Fatalf("count = %d, %v", n, err)
		}
		if n, err := db.Table("t").CountKeys(); err != nil || n != 1501 {
			t.Fatalf("Table count = %d, %v", n, err)
		}
		if n, err := db.CountKeys("missing"); err != nil || n != 0 {
			t.Fatalf("missing count = %d, %v", n, err)
		}
		if b, ok := db.(*boltDB); ok {
			b.db.View(func(tx *bolt.Tx) error {
				if s := tx.Bucket([]byte("t")).Stats().KeyN; s != 1501 {
					t.Fatalf("Stats().KeyN = %d", s)
				}
				return nil
			})
		}
	})
}

// CryptReset removes the encrypted values, in every table and sub-table,
// and the KVLite bucket, and keeps the plain ones; the database then opens
// under any padlock.
func TestCryptResetRemovesOnlyEncrypted(t *testing.T) {
	file := filepath.Join(t.TempDir(), "test.db")
	db, err := Open(file, []byte("padlock")...)
	if err != nil {
		t.Fatal(err)
	}
	db.Set("a", "plain", "kept")
	db.CryptSet("a", "secret", "gone")
	db.Sub("s").Set("b", "plain", "kept")
	db.Sub("s").CryptSet("b", "secret", "gone")
	db.CryptSet("only_secrets", "secret", "gone")
	db.Close()

	if err := CryptReset(file); err != nil {
		t.Fatal(err)
	}

	raw_db, err := open(file)
	if err != nil {
		t.Fatal(err)
	}
	raw_db.db.View(func(tx *bolt.Tx) error {
		if tx.Bucket([]byte("KVLite")) != nil {
			t.Fatal("KVLite bucket left")
		}
		return nil
	})
	raw_db.Close()

	db, err = Open(file, []byte("another")...)
	if err != nil {
		t.Fatalf("open after reset: %v", err)
	}
	defer db.Close()
	for _, store := range []Store{db, db.Sub("s")} {
		for _, table := range []string{"a", "b"} {
			keys, _ := store.Keys(table)
			if len(keys) == 0 {
				continue
			}
			if !reflect.DeepEqual(keys, []string{"plain"}) {
				t.Fatalf("%s keys = %v", table, keys)
			}
			var s string
			if _, err := store.Get(table, "plain", &s); err != nil || s != "kept" {
				t.Fatalf("%s/plain = %q, %v", table, s, err)
			}
		}
	}
	if n, _ := db.CountKeys("only_secrets"); n != 0 {
		t.Fatalf("only_secrets has %d keys", n)
	}
	if n, _ := db.Sub("s").CountKeys("b"); n != 1 {
		t.Fatalf("s/b has %d keys", n)
	}
}

// old_set writes a value the way set did before Batch: one Update, gob,
// then encrypt, then the flag byte in front.
func old_set(t *testing.T, K *boltDB, table, key string, value interface{}, encrypt_value bool) {
	t.Helper()
	err := K.db.Update(func(tx *bolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists([]byte(table))
		if err != nil {
			return err
		}
		v, err := K.encoder.encode(value)
		if err != nil {
			return err
		}
		if encrypt_value {
			v = K.encoder.encrypt(v)
			v = append([]byte{1}, v[0:]...)
		} else {
			v = append([]byte{0}, v[0:]...)
		}
		return bucket.Put([]byte(key), v)
	})
	if err != nil {
		t.Fatal(err)
	}
}

// A file written by the old write path opens and reads the same through the
// new read paths, and the new write paths store the very same bytes.
func TestOldFilesReadTheSame(t *testing.T) {
	file := filepath.Join(t.TempDir(), "old.db")
	db, err := Open(file, []byte("padlock")...)
	if err != nil {
		t.Fatal(err)
	}
	K := db.(*boltDB)
	old_set(t, K, "t", "plain", record{"plain", []byte{1, 2, 3}}, false)
	old_set(t, K, "t", "secret", record{"secret", []byte{4, 5}}, true)
	old_set(t, K, "tenant\x1ft", "sub", 42, false)
	old_plain := raw(t, db, "t", "plain")
	old_secret := raw(t, db, "t", "secret")
	db.Close()

	db, err = Open(file, []byte("padlock")...)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	got := make(map[string]record)
	err = db.ForEach("t", func(key string, decode func(interface{}) error) error {
		var r record
		if err := decode(&r); err != nil {
			return err
		}
		got[key] = r
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, map[string]record{"plain": {"plain", []byte{1, 2, 3}}, "secret": {"secret", []byte{4, 5}}}) {
		t.Fatalf("ForEach = %v", got)
	}
	var n int
	if found, err := db.Sub("tenant").Get("t", "sub", &n); !found || err != nil || n != 42 {
		t.Fatalf("sub value = %d %v %v", n, found, err)
	}
	if c, _ := db.CountKeys("t"); c != 2 {
		t.Fatalf("count = %d", c)
	}

	// The new paths write byte for byte what the old one did.
	db.Set("new", "plain", record{"plain", []byte{1, 2, 3}})
	db.Batch(func(b Batch) error { return b.CryptSet("new", "secret", record{"secret", []byte{4, 5}}) })
	if !bytes.Equal(raw(t, db, "new", "plain"), old_plain) {
		t.Fatal("Set stores different bytes than before")
	}
	if !bytes.Equal(raw(t, db, "new", "secret"), old_secret) {
		t.Fatal("Batch CryptSet stores different bytes than before")
	}
}
