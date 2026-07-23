package idempotency

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresRepository struct{ db queryer }
type queryer interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{db: pool}
}
func NewPostgresRepositoryTx(tx pgx.Tx) *PostgresRepository { return &PostgresRepository{db: tx} }
func scan(row pgx.Row) (Record, error) {
	var value Record
	var responseBody []byte
	err := row.Scan(&value.ID, &value.Scope, &value.Key, &value.RequestHash, &value.ResponseStatus, &responseBody, &value.CreatedAt, &value.ExpiresAt)
	if err != nil {
		return Record{}, err
	}
	value.ResponseBody = json.RawMessage(responseBody)
	if !json.Valid(value.ResponseBody) {
		return Record{}, fmt.Errorf("invalid idempotency response: %w", ErrConflict)
	}
	return value, nil
}
func (r *PostgresRepository) Get(ctx context.Context, scope, key string) (Record, error) {
	value, err := scan(r.db.QueryRow(ctx, "SELECT id,scope,idempotency_key,request_hash,response_status,response_body,created_at,expires_at FROM idempotency_records WHERE scope=$1 AND idempotency_key=$2", scope, key))
	if errors.Is(err, pgx.ErrNoRows) {
		return Record{}, ErrNotFound
	}
	if err != nil {
		return Record{}, fmt.Errorf("get idempotency record: %w", err)
	}
	return value, nil
}
func (r *PostgresRepository) Create(ctx context.Context, value Record) (Record, error) {
	if !json.Valid(value.ResponseBody) {
		return Record{}, fmt.Errorf("invalid idempotency response: %w", ErrConflict)
	}
	created, err := scan(r.db.QueryRow(ctx, "INSERT INTO idempotency_records (id,scope,idempotency_key,request_hash,response_status,response_body,expires_at) VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING id,scope,idempotency_key,request_hash,response_status,response_body,created_at,expires_at", value.ID, value.Scope, value.Key, value.RequestHash, value.ResponseStatus, value.ResponseBody, value.ExpiresAt))
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return Record{}, ErrConflict
		}
		return Record{}, fmt.Errorf("create idempotency record: %w", err)
	}
	return created, nil
}
