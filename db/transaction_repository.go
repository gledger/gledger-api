package db

import (
	"database/sql"
	"fmt"

	"github.com/lib/pq"
	"github.com/pkg/errors"

	"github.com/gledger/api"
)

func SaveTransaction(db *sql.DB) func(gledger.Transaction) error {
	return func(t gledger.Transaction) error {
		_, err := db.Exec(
			`INSERT INTO transactions VALUES ($1, $2, $3, $4, $5, $6, $7, now(), now(), $8)`,
			t.UUID, t.AccountUUID, t.OccurredAt, t.Payee, t.Amount, t.Cleared, t.Reconciled, t.EnvelopeUUID,
		)

		if pqErr, ok := err.(*pq.Error); ok {
			if pqErr.Code == ForeignKeyViolation { // TODO: Figure out which FK failed
				return notFoundError(fmt.Sprintf("Account %s not found", t.AccountUUID))
			}
		}

		return errors.Wrap(err, "error writing transaction")
	}
}

func TransactionsForAccount(db *sql.DB) func(string) ([]gledger.Transaction, error) {
	return func(u string) ([]gledger.Transaction, error) {
		var transactions []gledger.Transaction

		var uuid string
		err := db.QueryRow(`SELECT account_uuid FROM accounts WHERE account_uuid = $1`, u).Scan(&uuid)
		if err == sql.ErrNoRows {
			return transactions, notFoundError(fmt.Sprintf("Account %s not found", u))
		}

		rows, err := db.Query(
			`SELECT
				transaction_uuid,
				account_uuid,
				occurred_at,
				payee,
				amount,
				sum(amount) OVER (PARTITION BY account_uuid ORDER BY occurred_at, created_at),
				cleared,
				reconciled,
				envelope_uuid
			FROM transactions where account_uuid = $1`,
			u,
		)
		if err != nil {
			return transactions, errors.Wrapf(err, "error getting transactions for %s", u)
		}
		defer rows.Close()

		for rows.Next() {
			var t gledger.Transaction
			err := rows.Scan(&t.UUID, &t.AccountUUID, &t.OccurredAt, &t.Payee, &t.Amount, &t.RollingTotal, &t.Cleared, &t.Reconciled, &t.EnvelopeUUID)
			if err != nil {
				return transactions, errors.Wrapf(err, "error scanning getting transactions for %s", u)
			}

			transactions = append(transactions, t)
		}
		err = rows.Err()
		return transactions, errors.Wrapf(err, "error getting transactions for %s", u)
	}
}
