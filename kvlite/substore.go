package kvlite

import (
	"fmt"
	"strings"
)

// substore represents a substore within a larger store, allowing for namespacing.
type substore struct {
	prefix string
	db     Store
}

// sepr is the separator rune used to delimit table names in hierarchical stores.
const sepr = '\x1f'

// apply_prefix prepends the substore's prefix to the given name.
func (d substore) apply_prefix(name string) string {
	return string(append([]rune(d.prefix), []rune(name)...))
}

// Sub creates a new bucket with a different namespace.
func (d *substore) Sub(name string) Store {
	return &substore{fmt.Sprintf("%s%s%c", d.prefix, name, sepr), d.db}
}

// Bucket returns a new bucket within the store.
func (d *substore) Bucket(name string) Store {
	return d.db.Sub(name)
}

// Close closes the underlying store.
func (d substore) Close() (err error) {
	return d.db.Close()
}

// Drop removes the specified table.
func (d substore) Drop(table string) (err error) {
	return d.db.Drop(d.apply_prefix(table))
}

// CryptSet encrypts the value within the key/value pair.
func (d substore) CryptSet(table, key string, value interface{}) error {
	return d.db.CryptSet(d.apply_prefix(table), key, value)
}

// Set sets the key/value pair in table.
func (d substore) Set(table, key string, value interface{}) error {
	return d.db.Set(d.apply_prefix(table), key, value)
}

// Get retrieves value at key in table.
// Returns found and error if any.
func (d substore) Get(table, key string, output interface{}) (bool, error) {
	return d.db.Get(d.apply_prefix(table), key, output)
}

// Keys returns a listing of all keys in table.
func (d substore) Keys(table string) ([]string, error) {
	return d.db.Keys(d.apply_prefix(table))
}

// CountKeys returns the number of keys in the specified table.
func (d substore) CountKeys(table string) (int, error) {
	return d.db.CountKeys(d.apply_prefix(table))
}

// buckets returns a list of bucket names.
// If limit_depth is true, it returns only the top-level buckets.
func (d substore) buckets(limit_depth bool) (buckets []string, err error) {
	bmap := make(map[string]struct{})

	tmp, e := d.db.buckets(false)
	if e != nil {
		return buckets, e
	}
	for _, t := range tmp {
		if strings.HasPrefix(t, d.prefix) {
			name := strings.TrimPrefix(t, d.prefix)
			if !limit_depth {
				buckets = append(buckets, name)
			} else {
				name = strings.Split(name, string(sepr))[0]
				if _, ok := bmap[name]; !ok {
					bmap[name] = struct{}{}
					buckets = append(buckets, name)
				}
			}
		}
	}
	return buckets, err
}

// Tables returns a list of table names within the substore.
// It filters out any table names that contain the separator rune.
func (d substore) Tables() (buckets []string, err error) {
	tmp, e := d.buckets(true)
	if e != nil {
		return buckets, e
	}
	for _, t := range tmp {
		if name := strings.TrimPrefix(t, d.prefix); !strings.ContainsRune(name, sepr) {
			buckets = append(buckets, name)
		}
	}
	return buckets, err
}

// Unset deletes the key/value pair in the specified table.
func (d substore) Unset(table, key string) error {
	return d.db.Unset(d.apply_prefix(table), key)
}

// Table returns a table from the store.
func (d substore) Table(table string) Table {
	return d.db.Table(d.apply_prefix(table))
}
