package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"

	_ "github.com/mattn/go-sqlite3"
)

func main() {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		log.Fatalf("cannot open database: %s", err)
	}
	defer db.Close()

	store, err := NewStore(db)
	if err != nil {
		log.Fatalf("cannot create store: %s", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	session := store.Session(ctx)

}

type Store interface {
	Session(context.Context) StoreSession
}

type StoreSession interface {
	Commit() error
	Rollback() error
	CreateAccount(ctx context.Context, address string, balance uint) error
	MoveFunds(ctx context.Context, fromAccount, toAccount string, amount uint) error
}

func NewStore(db *sql.DB) (Store, error) {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS accounts (
			address TEXT NOT NULL,
			balance INT NOT NULL CHECK balance >= 0
		)`,
	)
	if err != nil {
		return nil, fmt.Errorf("ensure schema: %w", err)
	}
	return &sqliteStore{db: db}, nil
}

type sqliteStore struct {
	db *sql.DB
}

func (s sqliteStore) Session(ctx context.Context) StoreSession {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return errSession{err: err}
	}
	return sqliteStoreSession{tx: tx}
}

// errSession always returns given error.
type errSession struct{ err error }

func (e errSession) Commit() error                                         { return e.err }
func (e errSession) Rollback() error                                       { return e.err }
func (e errSession) CreateAccount(context.Context, string, uint) error     { return e.err }
func (e errSession) MoveFunds(context.Context, string, string, uint) error { return e.err }

type sqliteStoreSession struct {
	tx *sql.Tx
}

func (s sqliteStoreSession) Commit() error {
	return s.tx.Commit()
}

func (s sqliteStoreSession) Rollback() error {
	return s.tx.Rollback()
}

func (s sqliteStoreSession) CreateAccount(ctx context.Context, address string, balance uint) error {
	if _, err := s.tx.ExecContext(ctx, `INSERT INTO accounts (address, balance) VALUES (?, ?)`, address, balance); err != nil {
		return fmt.Errorf("insert account: %w", err)
	}
	return nil
}

func (s sqliteStoreSession) MoveFunds(ctx context.Context, fromAccount, toAccount string, amount uint) error {
	var fromBalance uint
	err := s.tx.QueryRowContext(ctx, `
		UPDATE accounts SET balance = balance - ?
		WHERE address = ?
		RETURNING balance
		`, amount, fromAccount).Scan(&fromBalance)
	switch {
	case err == nil && fromBalance < 0:
		return ErrInsufficientFunds
	case errors.Is(err, sql.ErrNoRows):
		return fmt.Errorf("invalid from account: %w", ErrAccountNotFound)
	case err == nil:
		// All good.
	default:
		return fmt.Errorf("transfer from account: %w", err)
	}

	res, err := s.tx.ExecContext(ctx, `
		UPDATE accounts SET balance = balance + ?
		WHERE address = ?
	`, amount, toAccount)
	if err != nil {
		return fmt.Errorf("transfer to account: %w", err)
	}
	if n, err := res.RowsAffected(); err != nil {
		return fmt.Errorf("transfer to rows affected: %w", err)
	} else if n != 1 {
		return fmt.Errorf("transfer to accounts affected: %d", n)
	}

	return nil
}

var (
	ErrInsufficientFunds = errors.New("insufficient funds")
	ErrAccountNotFound   = errors.New("account not found")
)
