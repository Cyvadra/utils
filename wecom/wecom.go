package wecom

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/Cyvadra/utils/internal/ratelimit"
	"github.com/Cyvadra/utils/internal/webhook"
)

const (
	// TextLimit is WeCom's maximum UTF-8 byte length for a text message.
	TextLimit = 2048
	// MarkdownLimit is WeCom's maximum UTF-8 byte length for a markdown message.
	MarkdownLimit = 4096
	// DefaultRateLimit is WeCom's documented maximum number of messages per minute per bot.
	DefaultRateLimit = 20
)

var (
	ErrInvalidName       = errors.New("wecom: invalid bot name")
	ErrInvalidWebhookURL = errors.New("wecom: invalid webhook URL")
	ErrBotRegistered     = errors.New("wecom: bot already registered")
	ErrInvalidResponse   = errors.New("wecom: invalid webhook response")
	ErrEmptyContent      = errors.New("wecom: message content cannot be empty")
)

// Service sends messages through one registered WeCom bot.
type Service struct {
	name       string
	webhookURL string
	client     *http.Client
	limiter    *ratelimit.Limiter
}

// Name returns the unique name used to register the bot.
func (service *Service) Name() string {
	return service.name
}

// Option configures a Service during registration.
type Option func(*config) error

type config struct {
	client       *http.Client
	rateLimit    int
	rateInterval time.Duration
}

func defaultConfig() config {
	return config{
		client:       webhook.DefaultClient,
		rateLimit:    DefaultRateLimit,
		rateInterval: time.Minute,
	}
}

// WithHTTPClient sets the client used to call the webhook.
func WithHTTPClient(client *http.Client) Option {
	return func(config *config) error {
		if client == nil {
			return errors.New("wecom: HTTP client cannot be nil")
		}
		config.client = client
		return nil
	}
}

// WithRateLimit sets the maximum number of webhook requests permitted during interval.
// The limit is applied independently to each registered Service.
func WithRateLimit(limit int, interval time.Duration) Option {
	return func(config *config) error {
		if limit < 1 {
			return errors.New("wecom: rate limit must be positive")
		}
		if interval <= 0 {
			return errors.New("wecom: rate limit interval must be positive")
		}
		config.rateLimit = limit
		config.rateInterval = interval
		return nil
	}
}

// Mention identifies recipients to mention in a text message.
// All adds @all to the user ID mention list.
type Mention struct {
	UserIDs       []string
	MobileNumbers []string
	All           bool
}

// APIError is returned when WeCom rejects a webhook request.
type APIError struct {
	Code    int
	Message string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("wecom: API error %d: %s", e.Code, e.Message)
}

// HTTPStatusError is returned when the webhook responds with a non-success status.
type HTTPStatusError struct {
	StatusCode int
}

func (e *HTTPStatusError) Error() string {
	return fmt.Sprintf("wecom: unexpected HTTP status %d", e.StatusCode)
}
