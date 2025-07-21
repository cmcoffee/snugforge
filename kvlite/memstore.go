package kvlite

import (
	"fmt"
	"strings"
	"sync"
)

// memStore is an in-memory store for key-value pairs.
// It uses a map to store the data and a mutex for concurrency control.
type memStore struct {
	mutex   sync.RWMutex
	kv      map[string]map[string][]byte
	encoder encoder
}

// Table returns a focused view on a specific table within the store.
// It allows for key-value operations scoped to that table.
func (K *memStore) Table(table string) Table {
	return focused{table: table, store: K}
}

// Bucket returns a new bucket with a different namespace.
func (K *memStore) Bucket(name string) Store {
	return K.Sub(name)
}

// Sub creates a new bucket with a different namespace.
func (K *memStore) Sub(table string) Store {
	return &substore{fmt.Sprintf("%s%c", table, sepr), K}
}

// buckets returns a list of unique bucket names.
// If limit_depth is true, it returns the base names
// without the full path.
func (K *memStore) buckets(limit_depth bool) (buckets []string, err error) {
	K.mutex.RLock()
	defer K.mutex.RUnlock()

	bmap := make(map[string]struct{})

	for k := range K.kv {
		if !limit_depth {
			buckets = append(buckets, k)
		} else {
			k = strings.Split(k, string(sepr))[0]
			if _, ok := bmap[k]; !ok {
				bmap[k] = struct{}{}
				buckets = append(buckets, k)
			}
		}
	}
	return
}

// Keys returns a list of keys for the given table.
// It returns an error if the table does not exist.
func (K *memStore) Keys(table string) (keys []string, err error) {
	K.mutex.RLock()
	defer K.mutex.RUnlock()
	if t, ok := K.kv[table]; ok {
		for k := range t {
			keys = append(keys, k)
		}
	}
	return keys, nil
}

// Tables returns a list of table names present in the store.
// It retrieves the buckets and filters for those that do not
// contain the separator character, indicating they are top-level
// tables.
func (K *memStore) Tables() (tables []string, err error) {
	tmp, err := K.buckets(true)
	if err != nil {
		return tables, err
	}
	for _, v := range tmp {
		if !strings.ContainsRune(v, sepr) {
			tables = append(tables, v)
		}
	}
	return tables, err
}

// Drop removes the table and all its associated data from the store.
// It deletes all keys that have the table name as a prefix.
func (K *memStore) Drop(table string) (err error) {
	K.mutex.Lock()
	defer K.mutex.Unlock()

	for k := range K.kv {
		if strings.HasPrefix(k, fmt.Sprintf("%s%c", table, sepr)) || k == table {
			delete(K.kv, k)
		}
	}
	return nil
}

// Unset removes the key-value pair from the specified table.
// It locks the mutex to ensure concurrent access safety.
func (K *memStore) Unset(table, key string) (err error) {
	K.mutex.Lock()
	defer K.mutex.Unlock()
	if t, ok := K.kv[table]; ok {
		delete(t, key)
	}
	return nil
}

// Get retrieves a value from the store given a table and key.
// Returns true if the value was found, and an error if any.
func (K *memStore) Get(table, key string, output interface{}) (found bool, err error) {
	K.mutex.RLock()
	defer K.mutex.RUnlock()
	if t, ok := K.kv[table]; ok {
		if v, ok := t[key]; ok {
			return true, K.encoder.decode(v, output)
		}
	}
	return false, nil
}

// CountKeys returns the number of keys in the specified table.
// It returns an error if the table does not exist.
func (K *memStore) CountKeys(table string) (count int, err error) {
	K.mutex.RLock()
	defer K.mutex.RUnlock()
	if t, ok := K.kv[table]; ok {
		count = len(t)
	}
	return count, nil
}

// Sets the value of a key in a given table.
// It encrypts the value if encrypt_value is true.
func (K *memStore) Set(table, key string, value interface{}) (err error) {
	return K.set(table, key, value, false)
}

// CryptSet sets a key-value pair in the specified table, encrypting the value.
// It returns an error if encoding fails.
func (K *memStore) CryptSet(table, key string, value interface{}) (err error) {
	return K.set(table, key, value, true)
}

// Sets the value for the given key in the specified table.
// If encrypt_value is true, the value will be encrypted before storing.
func (K *memStore) set(table, key string, value interface{}, encrypt_value bool) (err error) {
	K.mutex.Lock()
	defer K.mutex.Unlock()

	if _, ok := K.kv[table]; !ok {
		K.kv[table] = make(map[string][]byte)
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

	K.kv[table][key] = v

	return nil

}

// Closes the store, deleting all key-value pairs.
func (K *memStore) Close() (err error) {
	K.mutex.Lock()
	defer K.mutex.Unlock()
	for k := range K.kv {
		delete(K.kv, k)
	}
	return nil
}

// MemStore returns a new in-memory store.
func MemStore() Store {
	return &memStore{kv: make(map[string]map[string][]byte), encoder: hashBytes(randBytes(256))}
}
