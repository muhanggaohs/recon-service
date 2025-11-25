package models

import (
	"fmt"
	"time"
)

// Note: Comments in English per instruction

type TransactionType string

const (
	TypeDebit  TransactionType = "DEBIT"
	TypeCredit TransactionType = "CREDIT"
)

// SystemTransaction represents internal system transaction
type SystemTransaction struct {
	TrxID           string
	AmountMinor     int64 // amount in minor unit (e.g., cents)
	Type            TransactionType
	TransactionTime time.Time
}

// BankStatement represents a single bank row
type BankStatement struct {
	UniqueIdentifier string
	AmountMinor      int64 // signed: negative for debit, positive for credit
	Date             time.Time // date only (normalized to midnight)
	BankName         string
}

func (t TransactionType) SignedAmount(minor int64) (int64, error) {
	switch t {
	case TypeDebit:
		if minor > 0 {
			return -minor, nil
		}
		return minor, nil
	case TypeCredit:
		if minor < 0 {
			return -minor, nil
		}
		return minor, nil
	default:
		return 0, fmt.Errorf("unknown transaction type: %s", t)
	}
}


