// Copyright (C) 2019-2021, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain

import (
	"bytes"

	"github.com/ethereum/go-ethereum/common"
)

var _ UnsignedTransaction = &TransferTx{}

type TransferTx struct {
	*BaseTx `serialize:"true" json:"baseTx"`

	// To is the recipient of the [Value].
	To common.Address `serialize:"true" json:"to"`

	// Value is the number of units to transfer to [To].
	Value uint64 `serialize:"true" json:"value"`
}

func (t *TransferTx) Execute(c *TransactionContext) error {
	// Must transfer to someone
	if bytes.Equal(t.To[:], zeroAddress[:]) {
		return ErrNonActionable
	}

	// This prevents someone from transferring to themselves.
	if bytes.Equal(t.To[:], c.Sender[:]) {
		return ErrNonActionable
	}
	if t.Value == 0 {
		return ErrNonActionable
	}
	if _, err := ModifyBalance(c.Database, c.Sender, false, t.Value); err != nil {
		return err
	}
	if _, err := ModifyBalance(c.Database, t.To, true, t.Value); err != nil {
		return err
	}
	return nil
}

func (t *TransferTx) Copy() UnsignedTransaction {
	to := make([]byte, common.AddressLength)
	copy(to, t.To[:])
	return &TransferTx{
		BaseTx: t.BaseTx.Copy(),
		To:     common.BytesToAddress(to),
		Value:  t.Value,
	}
}