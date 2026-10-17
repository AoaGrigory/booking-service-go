package models

import (
	"context"
	"github.com/jackc/pgx/v5"
)

type ProcessRepository interface {
	// AddProcessTx если записи нет, то добавляем событие в рамках переданной транзакции
	AddProcessTx(ctx context.Context, tx pgx.Tx, eventID string) error

	// CheckProcess проверяет есть ли запись. true - есть
	CheckProcess(ctx context.Context, eventID string) (bool, error)
}
