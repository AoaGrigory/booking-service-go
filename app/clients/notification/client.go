package notification

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"go.uber.org/zap"
	"io"
	"net/http"
	"time"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
	retryCfg   RetryConfig
	logger     *zap.Logger
}

type HTTPError struct {
	StatusCode int
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("HTTP %d от Notification", e.StatusCode)
}

func NewClient(baseURL string, timeout time.Duration, maxRetries int, retryBaseDelay time.Duration, logger *zap.Logger) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: timeout,
		},
		retryCfg: RetryConfig{
			MaxRetries: maxRetries,
			BaseDelay:  retryBaseDelay,
		},
		logger: logger,
	}
}

func (c *Client) doJSON(ctx context.Context, method, path string, body any) ([]byte, error) {
	url := c.baseURL + path

	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("сериализация тела запроса: %w", err)
		}
		reqBody = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("создание запроса: %w", err)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("выполнение запроса %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 && resp.StatusCode < 500 {
		return nil, &HTTPError{StatusCode: resp.StatusCode}
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("чтение ответа: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d от Notification (%s %s): %s", resp.StatusCode, method, path, string(respBody))
	}

	return respBody, nil
}

func (c *Client) SendNotification(ctx context.Context, req SendNotificationRequest) error {
	err := withRetry(ctx, c.retryCfg, "SendNotification", func() error {
		_, err := c.doJSON(ctx, http.MethodPost, "/api/notifications", req)
		return err
	})

	if err != nil {
		c.logger.Warn("withRetry завершился с ошибкой в SendNotification",
			zap.Int64("BookingID", req.BookingID),
			zap.Error(err))
	}
	return nil
}
