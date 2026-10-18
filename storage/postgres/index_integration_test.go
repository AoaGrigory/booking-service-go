package postgres

import (
	"booking-service/app/models"
	"context"
	"database/sql"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/require"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
)

func TestIndexes(t *testing.T) {

	db, _ := setupTestDB(t)
	ctx := context.Background()

	cases := []struct {
		table  string
		name   string
		column string
	}{
		{"bookings", "idx_status_and_cancellation_sent_at", "(status, cancellation_sent_at)"},
		{"bookings", "idx_bookings_created_at", "(created_at)"},
		{"bookings", "idx_status_and_created_at", "(status, created_at)"},
		{"outbox_messages", "idx_outbox_messages_status_created_at", "(status, created_at)"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var indexDef string
			err := db.QueryRowContext(ctx, `
		SELECT indexdef
		FROM pg_indexes
		WHERE schemaname = $1
			AND tablename = $2
			AND indexname = $3`,
				"public", tc.table, tc.name).Scan(&indexDef)
			require.NoError(t, err)
			require.Contains(t, indexDef, tc.column)
		})
	}

}

func setupTestDB(t *testing.T) (*sql.DB, string) {
	t.Helper()

	ctx := context.Background()

	container, err := tcpostgres.Run(
		ctx,
		"postgres:16-alpine",
		tcpostgres.WithDatabase("booking_test"),
		tcpostgres.WithUsername("test"),
		tcpostgres.WithPassword("test"),
		tcpostgres.BasicWaitStrategies(),
	)
	require.NoError(t, err)

	t.Cleanup(func() {
		err := container.Terminate(context.Background())
		require.NoError(t, err)
	})

	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	db, err := goose.OpenDBWithDriver("pgx", dsn)
	require.NoError(t, err)

	t.Cleanup(func() {
		require.NoError(t, db.Close())
	})

	err = goose.Up(db, "../../migrations")
	require.NoError(t, err)
	return db, dsn
}

func setupRepositoryTest(t *testing.T) (*sql.DB, *pgxpool.Pool) {
	t.Helper()

	db, dsn := setupTestDB(t)

	pool, err := pgxpool.New(context.Background(), dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	return db, pool
}

func TestBookingsRepository_GetBookingsWithStatusCancellationPending(t *testing.T) {
	db, dsn := setupTestDB(t)
	ctx := context.Background()

	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	repo := NewBookingsRepository(pool)
	threshold := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)

	_, err = db.ExecContext(ctx, `
	INSERT INTO bookings
		(id, status, user_id, resource_id, start_date, end_date, cancellation_sent_at)
	VALUES
		(101, 'cancellation_pending', 1, 1, '2026-11-01', '2026-11-02', $1),
		(102, 'cancellation_pending', 1, 1, '2026-11-01', '2026-11-02', $2),
		(103, 'confirmed',            1, 1, '2026-11-01', '2026-11-02', $1)
`, threshold.Add(-time.Hour), threshold.Add(time.Hour))
	require.NoError(t, err)

	bookings, err := repo.GetBookingsWithStatusCancellationPending(ctx, threshold, 10)
	require.NoError(t, err)
	require.Len(t, bookings, 1)
	require.Equal(t, int64(101), bookings[0].ID())

	_, err = db.ExecContext(ctx, `
	INSERT INTO bookings
		(id, status, user_id, resource_id, start_date, end_date,
		 cancellation_sent_at, created_at)
	VALUES
		(104, 'cancellation_pending', 1, 1, '2026-11-01', '2026-11-02', $1, $1),
		(105, 'cancellation_pending', 1, 1, '2026-11-01', '2026-11-02', NULL, $1),
		(106, 'cancellation_pending', 1, 1, '2026-11-01', '2026-11-02', $2, $2)
`, threshold, threshold.Add(-time.Hour))
	require.NoError(t, err)

	_, err = db.ExecContext(ctx, `
	UPDATE bookings SET created_at = $1 WHERE id = 101
`, threshold)
	require.NoError(t, err)

	bookings, err = repo.GetBookingsWithStatusCancellationPending(ctx, threshold, 1)
	require.NoError(t, err)
	require.Len(t, bookings, 1)
	require.Equal(t, int64(106), bookings[0].ID())
}

func TestBookingsRepository_Statistics(t *testing.T) {
	db, dsn := setupTestDB(t)
	ctx := context.Background()

	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	repo := NewBookingsRepository(pool)

	dateFrom := time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC)
	dateTo := time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC)

	_, err = db.ExecContext(ctx, `
		INSERT INTO bookings
			(status, user_id, resource_id, start_date, end_date, created_at)
		VALUES
			('confirmed',           1, 10, '2026-11-01', '2026-11-02', $1),
			('confirmed',           1, 10, '2026-11-01', '2026-11-02', $2),
			('cancelled',           1, 20, '2026-11-01', '2026-11-02', $3),
			('awaits_confirmation', 1, 20, '2026-11-01', '2026-11-02', $4),
			('confirmed',           1, 30, '2026-11-01', '2026-11-02', $5)
	`,
		dateFrom.Add(-time.Second),
		dateFrom,
		dateFrom.Add(24*time.Hour),
		dateTo.Add(24*time.Hour-time.Microsecond),
		dateTo.Add(24*time.Hour),
	)
	require.NoError(t, err)

	t.Run("count", func(t *testing.T) {
		count, err := repo.GetOrdersPerPeriod(ctx, dateFrom, dateTo)
		require.NoError(t, err)
		require.Equal(t, int64(3), count)
	})

	t.Run("by_status", func(t *testing.T) {
		counts, err := repo.GetOrdersByStatus(ctx, dateFrom, dateTo)
		require.NoError(t, err)

		require.Equal(t, map[string]int64{
			"awaits_confirmation":  1,
			"confirmed":            1,
			"cancelled":            1,
			"cancellation_pending": 0,
		}, counts)
	})
}

func TestBookingsRepository_GetOrdersTopFiveResource(t *testing.T) {
	db, dsn := setupTestDB(t)
	ctx := context.Background()

	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	repo := NewBookingsRepository(pool)

	dateFrom := time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC)
	dateTo := time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC)

	insertBooking := func(resourceID int64, createdAt time.Time) {
		t.Helper()

		_, err := db.ExecContext(ctx, `
			INSERT INTO bookings
				(status, user_id, resource_id, start_date, end_date, created_at)
			VALUES
				('confirmed', 1, $1, '2026-11-01', '2026-11-02', $2)
		`, resourceID, createdAt)
		require.NoError(t, err)
	}

	for resourceID := int64(1); resourceID <= 6; resourceID++ {
		for i := 0; i < 7-int(resourceID); i++ {
			insertBooking(resourceID, dateFrom.Add(time.Hour))
		}
	}

	for i := 0; i < 10; i++ {
		insertBooking(99, dateFrom.Add(-time.Second))
		insertBooking(99, dateTo.Add(24*time.Hour))
	}

	result, err := repo.GetOrdersTopFiveResource(ctx, dateFrom, dateTo)
	require.NoError(t, err)
	require.Len(t, result, 5)

	for i, item := range result {
		require.Equal(t, int64(i+1), item.ResourceID)
		require.Equal(t, int64(6-i), item.BookingCount)
	}
}

func TestBookingsRepository_GetAwaitingConfirmation(t *testing.T) {
	db, pool := setupRepositoryTest(t)
	ctx := context.Background()
	repo := NewBookingsRepository(pool)

	_, err := db.ExecContext(ctx, `
		INSERT INTO bookings
			(id, status, user_id, resource_id, start_date, end_date, created_at)
		VALUES
			(201, 'awaits_confirmation', 1, 1,
			 '2026-11-01', '2026-11-02', '2026-10-12 12:00:00+00'),
			(202, 'awaits_confirmation', 1, 1,
			 '2026-11-01', '2026-11-02', '2026-10-10 12:00:00+00'),
			(203, 'confirmed', 1, 1,
			 '2026-11-01', '2026-11-02', '2026-10-09 12:00:00+00')
	`)
	require.NoError(t, err)

	bookings, err := repo.GetAwaitingConfirmation(ctx, 10)
	require.NoError(t, err)
	require.Len(t, bookings, 2)
	require.Equal(t, int64(202), bookings[0].ID())
	require.Equal(t, int64(201), bookings[1].ID())

	bookings, err = repo.GetAwaitingConfirmation(ctx, 1)
	require.NoError(t, err)
	require.Len(t, bookings, 1)
	require.Equal(t, int64(202), bookings[0].ID())
}

func TestBookingsRepository_GetByFilter(t *testing.T) {
	db, pool := setupRepositoryTest(t)
	ctx := context.Background()
	repo := NewBookingsRepository(pool)

	_, err := db.ExecContext(ctx, `
		INSERT INTO bookings
			(id, status, user_id, resource_id, start_date, end_date)
		VALUES
			(301, 'confirmed', 1, 10, '2026-11-01', '2026-11-02'),
			(302, 'cancelled', 1, 20, '2026-11-01', '2026-11-02'),
			(303, 'confirmed', 2, 10, '2026-11-01', '2026-11-02'),
			(304, 'confirmed', 1, 10, '2026-11-01', '2026-11-02')
	`)
	require.NoError(t, err)

	userID := int64(1)
	resourceID := int64(10)
	missingUserID := int64(999)
	status := models.BookingStatusConfirmed

	cases := []struct {
		name   string
		filter models.BookingFilter
		ids    []int64
		total  int64
	}{
		{
			name:   "without_filters",
			filter: models.BookingFilter{Page: 1, Size: 10},
			ids:    []int64{304, 303, 302, 301},
			total:  4,
		},
		{
			name: "by_user",
			filter: models.BookingFilter{
				UserID: &userID, Page: 1, Size: 10,
			},
			ids:   []int64{304, 302, 301},
			total: 3,
		},
		{
			name: "by_resource",
			filter: models.BookingFilter{
				ResourceID: &resourceID, Page: 1, Size: 10,
			},
			ids:   []int64{304, 303, 301},
			total: 3,
		},
		{
			name: "by_status",
			filter: models.BookingFilter{
				Status: &status, Page: 1, Size: 10,
			},
			ids:   []int64{304, 303, 301},
			total: 3,
		},
		{
			name: "combined",
			filter: models.BookingFilter{
				UserID: &userID, ResourceID: &resourceID,
				Status: &status, Page: 1, Size: 10,
			},
			ids:   []int64{304, 301},
			total: 2,
		},
		{
			name: "second_page",
			filter: models.BookingFilter{
				UserID: &userID, Page: 2, Size: 2,
			},
			ids:   []int64{301},
			total: 3,
		},
		{
			name: "empty_result",
			filter: models.BookingFilter{
				UserID: &missingUserID, Page: 1, Size: 10,
			},
			ids:   []int64{},
			total: 0,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			bookings, total, err := repo.GetByFilter(ctx, tc.filter)
			require.NoError(t, err)
			require.Equal(t, tc.total, total)

			ids := make([]int64, 0, len(bookings))
			for _, booking := range bookings {
				ids = append(ids, booking.ID())
			}
			require.Equal(t, tc.ids, ids)
		})
	}
}

func TestOutboxRepository_Queries(t *testing.T) {
	db, pool := setupRepositoryTest(t)
	ctx := context.Background()
	repo := NewOutboxRepository(pool)

	_, err := db.ExecContext(ctx, `
		INSERT INTO outbox_messages
			(id, payload, status, created_at, last_error)
		VALUES
			(401, '{"EventId":"event-401"}', 'pending',
			 '2026-10-10 12:00:00+00', 'previous error'),
			(402, '{"EventId":"event-402"}', 'pending',
			 '2026-10-10 12:00:00+00', NULL),
			(403, '{"EventId":"event-403"}', 'sent',
			 '2026-10-09 12:00:00+00', NULL),
			(404, '{"EventId":"event-404"}', 'failed',
			 '2026-10-09 12:00:00+00', 'retry limit'),
			(405, '{"EventId":"event-405"}', 'pending',
			 '2026-10-11 12:00:00+00', NULL)
	`)
	require.NoError(t, err)

	tx1, err := repo.BeginTx(ctx)
	require.NoError(t, err)
	defer func() { _ = tx1.Rollback(context.Background()) }()

	messages, err := repo.GetPending(ctx, tx1, 1)
	require.NoError(t, err)
	require.Len(t, messages, 1)
	require.Equal(t, int64(401), messages[0].ID)

	// Отдельная транзакция должна пропустить заблокированную запись 401.
	tx2, err := repo.BeginTx(ctx)
	require.NoError(t, err)
	defer func() { _ = tx2.Rollback(context.Background()) }()

	lockCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	other, err := repo.GetPending(lockCtx, tx2, 10)
	require.NoError(t, err)
	require.Len(t, other, 2)
	require.Equal(t, int64(402), other[0].ID)
	require.Equal(t, int64(405), other[1].ID)
	require.NoError(t, tx2.Rollback(ctx))

	require.NoError(t, repo.MarkSent(ctx, tx1, 401))
	require.NoError(t, tx1.Commit(ctx))

	var status string
	var sentAt sql.NullTime
	var lastError sql.NullString

	err = db.QueryRowContext(ctx, `
		SELECT status, sent_at, last_error
		FROM outbox_messages
		WHERE id = 401
	`).Scan(&status, &sentAt, &lastError)
	require.NoError(t, err)
	require.Equal(t, "sent", status)
	require.True(t, sentAt.Valid)
	require.False(t, lastError.Valid)

	// При лимите 2 первая ошибка оставляет pending, вторая ставит failed.
	for attempt := 1; attempt <= 2; attempt++ {
		tx, err := repo.BeginTx(ctx)
		require.NoError(t, err)
		defer func() { _ = tx.Rollback(context.Background()) }()

		require.NoError(t,
			repo.RecordFailure(ctx, tx, 402, "broker unavailable", 2))
		require.NoError(t, tx.Commit(ctx))

		var attempts int
		err = db.QueryRowContext(ctx, `
			SELECT status, attempts, last_error
			FROM outbox_messages
			WHERE id = 402
		`).Scan(&status, &attempts, &lastError)
		require.NoError(t, err)

		require.Equal(t, attempt, attempts)
		require.True(t, lastError.Valid)
		require.Equal(t, "broker unavailable", lastError.String)

		expectedStatus := "pending"
		if attempt == 2 {
			expectedStatus = "failed"
		}
		require.Equal(t, expectedStatus, status)
	}

	tx3, err := repo.BeginTx(ctx)
	require.NoError(t, err)
	defer func() { _ = tx3.Rollback(context.Background()) }()

	messages, err = repo.GetPending(ctx, tx3, 10)
	require.NoError(t, err)
	require.Len(t, messages, 1)
	require.Equal(t, int64(405), messages[0].ID)
}
