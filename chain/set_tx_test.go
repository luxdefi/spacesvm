// Copyright (C) 2019-2021, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain

import (
	"bytes"
	"errors"
	"testing"

	"github.com/ava-labs/avalanchego/database/memdb"
	"github.com/ava-labs/avalanchego/ids"
	"golang.org/x/crypto/sha3"

	"github.com/ava-labs/quarkvm/parser"
)

func TestSetTx(t *testing.T) {
	t.Parallel()

	priv, err := f.NewPrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	sender, err := FormatPK(priv.PublicKey())
	if err != nil {
		t.Fatal(err)
	}

	priv2, err := f.NewPrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	sender2, err := FormatPK(priv2.PublicKey())
	if err != nil {
		t.Fatal(err)
	}

	db := memdb.New()
	defer db.Close()

	g := DefaultGenesis()
	tt := []struct {
		utx       UnsignedTransaction
		blockTime int64
		err       error
	}{
		{ // write with no previous claim should fail
			utx: &SetTx{
				BaseTx: &BaseTx{
					Sender:  sender,
					Prefix:  []byte("foo"),
					BlockID: ids.GenerateTestID(),
				},
				Key:   []byte("bar"),
				Value: []byte("value"),
			},
			blockTime: 1,
			err:       ErrPrefixMissing,
		},
		{ // successful claim
			utx: &ClaimTx{
				BaseTx: &BaseTx{
					Sender: sender,
					Prefix: []byte("foo"),
				},
			},
			blockTime: 1,
			err:       nil,
		},
		{ // write
			utx: &SetTx{
				BaseTx: &BaseTx{
					Sender:  sender,
					Prefix:  []byte("foo"),
					BlockID: ids.GenerateTestID(),
				},
				Key:   []byte("bar"),
				Value: []byte("value"),
			},
			blockTime: 1,
			err:       nil,
		},
		{ // empty value to delete by prefix
			utx: &SetTx{
				BaseTx: &BaseTx{
					Sender:  sender,
					Prefix:  []byte("foo"),
					BlockID: ids.GenerateTestID(),
				},
				Key: []byte("bar"),
			},
			blockTime: 1,
			err:       nil,
		},
		{ // write hashed value
			utx: &SetTx{
				BaseTx: &BaseTx{
					Sender:  sender,
					Prefix:  []byte("foo"),
					BlockID: ids.GenerateTestID(),
				},
				Key: func() []byte {
					h := sha3.Sum256([]byte("value"))
					return h[:]
				}(),
				Value: []byte("value"),
			},
			blockTime: 1,
			err:       nil,
		},
		{ // write incorrect hashed value
			utx: &SetTx{
				BaseTx: &BaseTx{
					Sender:  sender,
					Prefix:  []byte("foo"),
					BlockID: ids.GenerateTestID(),
				},
				Key: func() []byte {
					h := sha3.Sum256([]byte("not value"))
					return h[:]
				}(),
				Value: []byte("value"),
			},
			blockTime: 1,
			err:       ErrInvalidKey,
		},
		{ // delete hashed value
			utx: &SetTx{
				BaseTx: &BaseTx{
					Sender:  sender,
					Prefix:  []byte("foo"),
					BlockID: ids.GenerateTestID(),
				},
				Key: func() []byte {
					h := sha3.Sum256([]byte("value"))
					return h[:]
				}(),
			},
			blockTime: 1,
			err:       nil,
		},
		{
			utx: &SetTx{
				BaseTx: &BaseTx{
					Sender:  sender2,
					Prefix:  []byte("foo"),
					BlockID: ids.GenerateTestID(),
				},
				Key: []byte("bar"),
			},
			blockTime: 1,
			err:       ErrUnauthorized,
		},
		{
			utx: &SetTx{
				BaseTx: &BaseTx{
					Prefix:  []byte("foo"),
					BlockID: ids.GenerateTestID(),
				},
			},
			blockTime: 1,
			err:       parser.ErrKeyEmpty,
		},
		{
			utx: &SetTx{
				BaseTx: &BaseTx{
					Sender: sender,
					Prefix: []byte("foo"),
				},
			},
			blockTime: 1,
			err:       parser.ErrKeyEmpty,
		},
		{
			utx: &SetTx{
				BaseTx: &BaseTx{
					Prefix:  []byte("foo"),
					BlockID: ids.GenerateTestID(),
				},
				Key: bytes.Repeat([]byte{'a'}, parser.MaxKeySize+1),
			},
			blockTime: 1,
			err:       parser.ErrKeyTooBig,
		},
		{
			utx: &SetTx{
				BaseTx: &BaseTx{
					Prefix:  []byte("foo"),
					BlockID: ids.GenerateTestID(),
				},
				Key:   []byte("bar"),
				Value: bytes.Repeat([]byte{'b'}, int(g.MaxValueSize)+1),
			},
			blockTime: 1,
			err:       ErrValueTooBig,
		},
		{
			utx: &SetTx{
				BaseTx: &BaseTx{
					Sender:  sender,
					Prefix:  []byte("foo"),
					BlockID: ids.GenerateTestID(),
				},
				Key: []byte("bar///"),
			},
			blockTime: 1,
			err:       parser.ErrInvalidDelimiter,
		},
		{
			utx: &SetTx{
				BaseTx: &BaseTx{
					Sender:  sender,
					Prefix:  []byte("foo"),
					BlockID: ids.GenerateTestID(),
				},
				Key: []byte("bar"),
			},
			blockTime: 100,
			err:       ErrKeyMissing,
		},
		{
			utx: &SetTx{
				BaseTx: &BaseTx{
					Sender:  sender,
					Prefix:  []byte("foo"),
					BlockID: ids.GenerateTestID(),
				},
				Key: []byte("bar"),
			},
			blockTime: int64(g.ClaimReward) * 2,
			err:       ErrPrefixMissing,
		},
	}
	for i, tv := range tt {
		if i > 0 {
			// Expire old prefixes between txs
			if err := ExpireNext(db, tt[i-1].blockTime, tv.blockTime, true); err != nil {
				t.Fatalf("#%d: ExpireNext errored %v", i, err)
			}
		}
		// Set linked value (normally done in block processing)
		id := ids.GenerateTestID()
		if tp, ok := tv.utx.(*SetTx); ok {
			if len(tp.Value) > 0 {
				if err := db.Put(PrefixTxValueKey(id), tp.Value); err != nil {
					t.Fatal(err)
				}
			}
		}
		err := tv.utx.Execute(g, db, uint64(tv.blockTime), id)
		if !errors.Is(err, tv.err) {
			t.Fatalf("#%d: tx.Execute err expected %v, got %v", i, tv.err, err)
		}
		if tv.err != nil {
			continue
		}

		// check committed states from db
		switch tp := tv.utx.(type) {
		case *ClaimTx: // "ClaimTx.Execute" must persist "PrefixInfo"
			info, exists, err := GetPrefixInfo(db, tp.Prefix)
			if err != nil {
				t.Fatalf("#%d: failed to get prefix info %v", i, err)
			}
			if !exists {
				t.Fatalf("#%d: failed to find prefix info", i)
			}
			if !bytes.Equal(info.Owner[:], tp.Sender[:]) {
				t.Fatalf("#%d: unexpected owner found (expected pub key %q)", i, string(sender[:]))
			}
			// each claim must delete all existing keys with the value key
			kvs, err := Range(db, tp.Prefix, nil, WithPrefix())
			if err != nil {
				t.Fatalf("#%d: unexpected error when fetching range %v", i, err)
			}
			if len(kvs) > 0 {
				t.Fatalf("#%d: unexpected key-values for the prefix after claim", i)
			}

		case *SetTx:
			emptyValue := len(tp.Value) == 0
			val, exists, err := GetValue(db, tp.Prefix, tp.Key)
			if err != nil {
				t.Fatalf("#%d: failed to get key info %v", i, err)
			}
			switch {
			case emptyValue && exists:
				t.Fatalf("#%d: empty value should have deleted keys", i)
			case !emptyValue && !exists:
				t.Fatalf("#%d: non-empty value should have been persisted but not found", i)
			case !emptyValue && exists:
				if !emptyValue && exists && !bytes.Equal(tp.Value, val) {
					t.Fatalf("#%d: unexpected value %q, expected %q", i, val, tp.Value)
				}
			}
		}
	}
}
