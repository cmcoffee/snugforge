package kvlite

import "errors"

// Batch gathers writes for Store.Batch, which makes them all in one
// transaction. Each value is encoded when it is given, so an encoding
// error comes back from that call.
type Batch interface {
	// Set sets the key/value pair in table.
	Set(table, key string, value interface{}) (err error)
	// CryptSet encrypts the value within the key/value pair in table.
	CryptSet(table, key string, value interface{}) (err error)
	// Unset deletes the key/value pair in table.
	Unset(table, key string) (err error)
}

// ErrDecodeAfterReturn is a ForEach decode called after its fn returned,
// when the value it would read is no longer there.
var ErrDecodeAfterReturn = errors.New("kvlite: ForEach decode called after fn returned")

// batch_op is one write a Batch asks for: value already holds the stored
// bytes, flag byte and all.
type batch_op struct {
	table string
	key   string
	value []byte
	unset bool
}

// staged_batch encodes the writes a Batch asks for and keeps them, in
// order, for the store to make in one transaction once fn is done.
type staged_batch struct {
	encoder encoder
	ops     []batch_op
}

// Set stages the key/value pair in table.
func (b *staged_batch) Set(table, key string, value interface{}) (err error) {
	return b.set(table, key, value, false)
}

// CryptSet stages the key/value pair in table, encrypting the value.
func (b *staged_batch) CryptSet(table, key string, value interface{}) (err error) {
	return b.set(table, key, value, true)
}

// Unset stages the delete of key in table.
func (b *staged_batch) Unset(table, key string) (err error) {
	b.ops = append(b.ops, batch_op{table: table, key: key, unset: true})
	return nil
}

// set encodes value as Set and CryptSet store it and stages the write.
func (b *staged_batch) set(table, key string, value interface{}, encrypt_value bool) (err error) {
	v, err := b.encoder.pack(value, encrypt_value)
	if err != nil {
		return err
	}
	b.ops = append(b.ops, batch_op{table: table, key: key, value: v})
	return nil
}

// sub_batch is a substore's view of a Batch, with its prefix on the tables.
type sub_batch struct {
	store substore
	batch Batch
}

// Set sets the key/value pair in table.
func (b sub_batch) Set(table, key string, value interface{}) (err error) {
	return b.batch.Set(b.store.apply_prefix(table), key, value)
}

// CryptSet encrypts the value within the key/value pair in table.
func (b sub_batch) CryptSet(table, key string, value interface{}) (err error) {
	return b.batch.CryptSet(b.store.apply_prefix(table), key, value)
}

// Unset deletes the key/value pair in table.
func (b sub_batch) Unset(table, key string) (err error) {
	return b.batch.Unset(b.store.apply_prefix(table), key)
}

// each_value calls a ForEach fn for one key and its stored value. decode
// works only until fn returns, as value may point into the database file.
// It never hands that memory on: gob copies what it decodes out of its own
// buffer, and decrypt works on a copy.
func each_value(e encoder, key string, value []byte, fn func(key string, decode func(out interface{}) error) error) error {
	live := true
	decode := func(out interface{}) error {
		if !live {
			return ErrDecodeAfterReturn
		}
		return e.decode(value, out)
	}
	err := fn(key, decode)
	live = false
	return err
}
