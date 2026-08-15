package models

import "context"

// GenericTransaction interface for transactions
type GenericTransaction interface {
	// Rollback rolls back the transaction
	Rollback(ctx context.Context) error

	// Commit commits the transaction
	Commit(ctx context.Context) error
}

// DbTransactionKey key value to store DB transaction in context
const DbTransactionKey ContextKey = "dbTransaction"

// GetTransactionFromContext takes transaction from context
func GetTransactionFromContext(ctx context.Context) GenericTransaction {
	if tx, ok := ctx.Value(DbTransactionKey).(GenericTransaction); ok {
		return tx
	}
	return nil
}
