package postgres

import (
	"context"
	"fmt"
	"iter"
	"slices"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/scouser-122/meeting-analyzer/internal/config"
)

// QueryExecutor interface for DB queries
type QueryExecutor interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// GenericRepository generic Postgres repository to work with any entity
type GenericRepository[T any] struct {
	db          QueryExecutor
	retryConfig config.RetryConfig
	table       string
	keyName     string
	mapper      func(row pgx.Row) (*T, error)
}

// NewGenericRepository creates generic repository
func NewGenericRepository[T any](
	db *PostgresDatabase,
	table string,
	keyName string,
	mapper func(row pgx.Row) (*T, error),
) *GenericRepository[T] {
	return &GenericRepository[T]{
		db:          db,
		retryConfig: db.Config.RetryConfig,
		table:       table,
		keyName:     keyName,
		mapper:      mapper,
	}
}

// NewGenericRepositoryFromExecutor creates generic repository from executor (for testing with pgxmock)
func NewGenericRepositoryFromExecutor[T any](
	executor QueryExecutor,
	table string,
	keyName string,
	mapper func(row pgx.Row) (*T, error),
) *GenericRepository[T] {
	return &GenericRepository[T]{
		db:          executor,
		retryConfig: config.DefaultRetryConfig(),
		table:       table,
		keyName:     keyName,
		mapper:      mapper,
	}
}

// Create creates entity, fields should be string of entity field names separated by comma
func (r *GenericRepository[T]) Create(ctx context.Context, fields string, args ...any) (*T, error) {
	fieldNames := strings.Split(strings.ReplaceAll(fields, " ", ""), ",")
	var valuesArgs []string
	for i := 1; i <= len(fieldNames); i++ {
		valuesArgs = append(valuesArgs, fmt.Sprintf("$%d", i))
	}
	query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", r.table, fields, strings.Join(valuesArgs, ","))
	_, err := r.db.Exec(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	keyIndex := slices.Index(fieldNames, r.keyName)
	keyValue := args[keyIndex].(string)
	return r.GetByID(ctx, keyValue)
}

// Update updates entity
func (r *GenericRepository[T]) Update(ctx context.Context, query string, args ...any) (pgconn.CommandTag, error) {
	return r.db.Exec(ctx, query, args...)
}

// GetByID find entity by ID
func (r *GenericRepository[T]) GetByID(ctx context.Context, id string) (*T, error) {
	query := fmt.Sprintf("SELECT * FROM %s WHERE %s = $1", r.table, r.keyName)
	row := r.db.QueryRow(ctx, query, id)
	var entity *T
	err := DataBaseRequestRetry(
		ctx,
		r.retryConfig,
		func() error {
			var err error
			entity, err = r.mapper(row)
			return err
		},
	)
	return entity, err
}

// GetByParameter find entity parameter value
func (r *GenericRepository[T]) GetByParameter(ctx context.Context, key string, value string) (*T, error) {
	query := fmt.Sprintf("SELECT * FROM %s WHERE %s = $1", r.table, key)
	row := r.db.QueryRow(ctx, query, value)
	var entity *T
	err := DataBaseRequestRetry(
		ctx,
		r.retryConfig,
		func() error {
			var err error
			entity, err = r.mapper(row)
			return err
		},
	)
	return entity, err
}

// GetAllConditional returns all entities which met condition expression with specified conditionFields,
// entities will be ordered by field specified in orderBy expression, and taken by pages of specified pageSize
func (r *GenericRepository[T]) GetAllConditional(
	ctx context.Context,
	condition string,
	conditionFields []any,
	orderBy string,
	pageSize int,
) iter.Seq2[[]*T, error] {
	return func(yield func([]*T, error) bool) {
		offset := 0
		for {
			var args []any
			argCounter := 0
			for i := 0; i < len(conditionFields); i++ {
				argCounter++
				args = append(args, conditionFields[i])
			}
			args = append(args, pageSize)
			argCounter++
			limitArg := fmt.Sprintf("$%d", argCounter)

			args = append(args, offset)
			argCounter++
			offsetArg := fmt.Sprintf("$%d", argCounter)

			query := fmt.Sprintf("SELECT * FROM %s %s ORDER BY %s LIMIT %s OFFSET %s", r.table, condition, orderBy, limitArg, offsetArg)
			rows, err := r.db.Query(ctx, query, args...)
			if err != nil {
				yield(nil, fmt.Errorf("query failed at offset %d: %w", offset, err))
				return
			}

			entities := make([]*T, 0, pageSize)

			for rows.Next() {
				row := &pgxRowAdapter{rows: rows}
				entity, err := r.mapper(row)
				if err != nil {
					rows.Close()
					yield(nil, fmt.Errorf("scan failed at offset %d: %w", offset, err))
					return
				}
				entities = append(entities, entity)
			}

			if err := rows.Err(); err != nil {
				rows.Close()
				yield(nil, fmt.Errorf("rows iteration error at offset %d: %w", offset, err))
				return
			}

			rows.Close()

			if len(entities) == 0 {
				return
			}

			if !yield(entities, nil) {
				return
			}

			offset += pageSize
		}
	}
}

// CustomQuery makes custom query request and returns result
func (r *GenericRepository[T]) CustomQuery(
	ctx context.Context,
	scanner func(rows pgx.Rows) error,
	query string,
	args ...any,
) error {
	return DataBaseRequestRetry(
		ctx,
		r.retryConfig,
		func() error {
			rows, err := r.db.Query(ctx, query, args...)
			if err != nil {
				return err
			}
			for rows.Next() {
				err = scanner(rows)
				if err != nil {
					return err
				}
			}
			return nil
		},
	)
}

// CustomQueryRow makes custom query request and returns result
func (r *GenericRepository[T]) CustomQueryRow(
	ctx context.Context,
	mapper func(row pgx.Row) error,
	query string,
	args ...any,
) error {
	row := r.db.QueryRow(ctx, query, args...)
	return DataBaseRequestRetry(
		ctx,
		r.retryConfig,
		func() error {
			return mapper(row)
		},
	)
}

// WithTx returns new generic repository using transaction
func (r *GenericRepository[T]) WithTx(tx pgx.Tx) *GenericRepository[T] {
	return &GenericRepository[T]{
		db:          tx,
		retryConfig: r.retryConfig,
		table:       r.table,
		keyName:     r.keyName,
		mapper:      r.mapper,
	}
}

// pgxRowAdapter - adapter for pgx.Rows
type pgxRowAdapter struct {
	rows pgx.Rows
}

func (r *pgxRowAdapter) Scan(dest ...any) error {
	return r.rows.Scan(dest...)
}
