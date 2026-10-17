package postgres

const (
	queryInsertBooking = `
		INSERT INTO bookings (status, user_id, resource_id, start_date, end_date, created_at, previous_status, cancellation_sent_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id`

	queryGetBookingByID = `
		SELECT id, status, user_id, resource_id, start_date, end_date, created_at, previous_status, cancellation_sent_at
		FROM bookings
		WHERE id = $1`

	queryUpdateBookingStatus = `
		UPDATE bookings
		SET status = $1,
		previous_status = $2,
		cancellation_sent_at = $3
		WHERE id = $4`

	queryGetBookingsByFilter = `
		SELECT id, status, user_id, resource_id, start_date, end_date, created_at, previous_status, cancellation_sent_at
		FROM bookings
		WHERE ($1::BIGINT IS NULL OR user_id = $1)
		  AND ($2::BIGINT IS NULL OR resource_id = $2)
		  AND ($3::VARCHAR IS NULL OR status = $3)
		ORDER BY id DESC
		LIMIT $4 OFFSET $5`

	queryCountBookingsByFilter = `
		SELECT COUNT(*)
		FROM bookings
		WHERE ($1::BIGINT IS NULL OR user_id = $1)
		  AND ($2::BIGINT IS NULL OR resource_id = $2)
		  AND ($3::VARCHAR IS NULL OR status = $3)`

	queryGetAwaitingConfirmation = `
		SELECT id, status, user_id, resource_id, start_date, end_date, created_at, previous_status, cancellation_sent_at
		FROM bookings
		WHERE status = 'awaits_confirmation'
		ORDER BY created_at ASC
		LIMIT $1
		FOR UPDATE SKIP LOCKED`

	queryGetCountOrdersBetweenPeriod = `
		SELECT  COUNT(*) FROM bookings 
		WHERE created_at >= $1 AND created_at < ($2::date + INTERVAL '1 day')`

	queryGetTopFiveResource = `
		SELECT resource_id, COUNT(*) FROM bookings
		WHERE created_at >= $1 AND created_at < ($2::date + INTERVAL '1 day')
		GROUP BY resource_id
		ORDER BY COUNT(*) DESC
		LIMIT 5`

	queryGetStatusInfo = `
		SELECT status, COUNT(*) FROM bookings
		WHERE created_at >= $1 AND created_at < ($2::date + INTERVAL '1 day')
		GROUP BY status
`
	queryGetOrdersWithCancellationPending = `
	SELECT id, status, user_id, resource_id, start_date, end_date, created_at, previous_status, cancellation_sent_at
	FROM bookings
	WHERE status = 'cancellation_pending' AND cancellation_sent_at < $1
	ORDER BY created_at ASC
	LIMIT $2`

	queryInsertBookingHistory = `
	INSERT INTO booking_history(booking_id, previous_status, new_status, changed_at, reason, initiator)
	VALUES ($1, $2, $3, $4, $5, $6)`

	queryGetBookingHistoryByID = `
	SELECT id ,booking_id, previous_status, new_status, changed_at, reason, initiator 
	FROM booking_history
	WHERE booking_id = $1
	ORDER BY changed_at DESC 
	LIMIT $2 OFFSET $3`

	queryGetCountBookingHistory = `
	SELECT COUNT(*) 
	FROM booking_history
	WHERE booking_id = $1`

	queryCheckProcessByEventID = `
	SELECT EXISTS(SELECT event_id FROM processed_events
	WHERE event_id = $1)`

	queryInsertEventID = `
	INSERT INTO processed_events(event_id)
	VALUES ($1)`

	queryInsertOutboxMessage = `
	INSERT INTO outbox_messages(payload)
	VALUES ($1)`

	queryGetOutboxMessage = `
	SELECT id, payload, status, attempts, created_at, sent_at, last_error FROM outbox_messages
	WHERE status = 'pending'
	order by created_at, id`

	queryUpdateOutboxMessage = `
	UPDATE outbox_messages
	SET status = 'sent',
	sent_at = NOW(),
	last_error = NULL
	WHERE id =$1`

	queryRecordFailureOutboxMessage = `
	UPDATE outbox_messages
	SET attempts = attempts + 1,
	    last_error = $2,
	    status = CASE
	        WHEN attempts + 1 >= $3 THEN 'failed'
			ELSE 'pending'
	END
	WHERE id = $1
`
)
