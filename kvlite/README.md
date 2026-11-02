# kvlite
--
    import "github.com/cmcoffee/snugforge/kvlite"


## Usage

```go
var ErrBadPadlock = errors.New("Invalid padlock provided, unable to open database.")
```
ErrBadPadlock indicates an invalid padlock was provided.

```go
var ErrLocked = errors.New("Database is currently in use by an exisiting instance, please close it and try again.")
```
ErrLocked indicates that the database is currently in use by another instance.

```go
var ErrDecodeAfterReturn = errors.New("kvlite: ForEach decode called after fn returned")
```
ErrDecodeAfterReturn is a ForEach decode called after its fn returned, when the
value it would read is no longer there.

#### type Batch

```go
type Batch interface {
	// Set sets the key/value pair in table.
	Set(table, key string, value interface{}) (err error)
	// CryptSet encrypts the value within the key/value pair in table.
	CryptSet(table, key string, value interface{}) (err error)
	// Unset deletes the key/value pair in table.
	Unset(table, key string) (err error)
}
```
Batch gathers writes for Store.Batch, which makes them all in one transaction.
Each value is encoded when it is given, so an encoding error comes back from
that call.

#### func  CryptReset

```go
func CryptReset(filename string) (err error)
```
CryptReset resets the database by deleting all encrypted keys and the KVLite
bucket.

#### type Store

```go
type Store interface {
	// Tables provides a list of all tables.
	Tables() (tables []string, err error)
	// Table creats a key/val direct to a specified Table.
	Table(table string) Table
	// SubStore Creates a new bucket with a different namespace, tied to
	Sub(name string) Store
	// SyncStore Creates a new bucket for shared tenants.
	Bucket(name string) Store
	// Drop drops the specified table.
	Drop(table string) (err error)
	// CountKeys provides a total of keys in table.
	CountKeys(table string) (count int, err error)
	// Keys provides a listing of all keys in table.
	Keys(table string) (keys []string, err error)
	// CryptSet encrypts the value within the key/value pair in table.
	CryptSet(table, key string, value interface{}) (err error)
	// Set sets the key/value pair in table.
	Set(table, key string, value interface{}) (err error)
	// Unset deletes the key/value pair in table.
	Unset(table, key string) (err error)
	// Get retrieves value at key in table.
	Get(table, key string, output interface{}) (found bool, err error)
	// ForEach reads every key in table in one read transaction.
	ForEach(table string, fn func(key string, decode func(out interface{}) error) error) (err error)
	// Batch makes all the writes fn asks of b in one write transaction.
	Batch(fn func(b Batch) error) (err error)
	// Close closes the kvliter.Store.
	Close() (err error)
	// contains filtered or unexported methods
}
```

Store provides a list of all tables. Table creats a key/val direct to a
specified Table. Sub Creates a new bucket with a different namespace. Bucket
Creates a new bucket for shared tenants. Drop drops the specified table.
CountKeys provides a total of keys in table. Keys provides a listing of all keys
in table. CryptSet encrypts the value within the key/value pair. Set sets the
key/value pair in table. Unset deletes the key/value pair in table. Get
retrieves value at key in table. Close closes the kvliter.Store. Buckets lists
all bucket namespaces.

**ForEach** reads a whole table in one read transaction, in key order for a
file store. `decode` decodes that key's value and may be called zero or one
times, so keys whose values are not needed skip the gob cost. A missing table
calls `fn` zero times and returns nil; an error from `fn` stops the read and
is returned unchanged. `fn` must not write to the store: a write from inside
the read can deadlock (the in-memory store holds its read lock while `fn`
runs). Keys and decoded values never point into the file, so they stay good
after `fn` returns; `decode` itself works only while `fn` is running.

```go
err := db.ForEach("chunks", func(key string, decode func(out interface{}) error) error {
	if !strings.HasPrefix(key, "doc1/") {
		return nil // value never decoded
	}
	var c Chunk
	if err := decode(&c); err != nil {
		return err
	}
	chunks = append(chunks, c)
	return nil
})
```

**Batch** makes many writes in one write transaction: one commit, one sync.
Values are encoded as `fn` gives them, before the write lock is taken. If `fn`
returns an error nothing is written. The writes are made after `fn` returns, so
reads inside `fn` do not see them. Through a `Sub()` store, the batch's tables
are in that namespace.

```go
err := db.Batch(func(b kvlite.Batch) error {
	for path, f := range files {
		if err := b.Set("repo_files", path, f); err != nil {
			return err
		}
	}
	return b.Unset("repo_files", "stale/path")
})
```

#### func  MemStore

```go
func MemStore() Store
```
MemStore returns a new in-memory store.

#### func  Open

```go
func Open(filename string, padlock ...byte) (Store, error)
```
Open opens or creates a database file and performs necessary setup. It handles
reset, decryption, and sets up the encoder.

#### type Table

```go
type Table interface {
	Keys() (keys []string, err error)
	CountKeys() (count int, err error)
	Set(key string, value interface{}) (err error)
	CryptSet(key string, value interface{}) (err error)
	Get(key string, value interface{}) (found bool, err error)
	Unset(key string) (err error)
	Drop() (err error)
	ForEach(fn func(key string, decode func(out interface{}) error) error) (err error)
}
```

Table represents an interface for key-value storage. It provides methods for
managing keys, values, and the table itself.
