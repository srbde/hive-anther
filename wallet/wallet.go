// Package wallet provides a simple in-memory key wallet to manage private keys for accounts and sign blockchain transactions locally.
package wallet

import (
	"fmt"
	"sync"

	"github.com/btcsuite/btcd/btcutil"
	"github.com/thecrazygm/anther/exceptions"
	"github.com/thecrazygm/anther/transaction"
)

// Wallet is a simple in-memory wallet for managing Hive private keys.
type Wallet struct {
	mutex sync.RWMutex
	Keys  map[string]map[string]string
}

// NewWallet creates a new Wallet.
func NewWallet() *Wallet {
	return &Wallet{
		Keys: make(map[string]map[string]string),
	}
}

// AddKey adds a private key for an account role.
func (w *Wallet) AddKey(account, role, wif string) error {
	if role != "posting" && role != "active" && role != "memo" && role != "owner" {
		return fmt.Errorf("role must be 'posting', 'active', 'memo', or 'owner'")
	}
	if account == "" {
		return fmt.Errorf("account must be a non-empty string")
	}
	if len(wif) == 0 || wif[0] != '5' {
		return exceptions.NewInvalidKeyFormatError("private WIF keys start with '5'")
	}

	// Validate WIF format
	if _, err := btcutil.DecodeWIF(wif); err != nil {
		return exceptions.NewInvalidKeyFormatError(fmt.Sprintf("invalid WIF format: %v", err))
	}

	w.mutex.Lock()
	defer w.mutex.Unlock()

	if _, ok := w.Keys[account]; !ok {
		w.Keys[account] = make(map[string]string)
	}
	w.Keys[account][role] = wif
	return nil
}

// HasKey checks if a key is loaded for the account/role.
func (w *Wallet) HasKey(account, role string) bool {
	w.mutex.RLock()
	defer w.mutex.RUnlock()

	if _, ok := w.Keys[account]; !ok {
		return false
	}
	_, ok := w.Keys[account][role]
	return ok
}

// GetKey gets WIF key if available.
func (w *Wallet) GetKey(account, role string) (string, error) {
	w.mutex.RLock()
	defer w.mutex.RUnlock()

	if _, ok := w.Keys[account]; !ok {
		return "", exceptions.NewMissingKeyError(account, role)
	}
	wif, ok := w.Keys[account][role]
	if !ok {
		return "", exceptions.NewMissingKeyError(account, role)
	}
	return wif, nil
}

// Sign the transaction using the specified account's role key.
func (w *Wallet) Sign(tx *transaction.Transaction, account, role string) error {
	wif, err := w.GetKey(account, role)
	if err != nil {
		return err
	}
	return tx.Sign(wif)
}
