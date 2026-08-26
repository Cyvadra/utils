package telegram

import (
	"errors"
	"fmt"
	"net/http"
	"time"
)

const (
	// TextLimit is Telegram's maximum number of characters in a text message.
	TextLimit = 4096

	// DefaultBotRateLimit is Telegram's free broadcast limit per bot.
	DefaultBotRateLimit = 30
	// DefaultBotRateInterval is the time window for DefaultBotRateLimit.
	DefaultBotRateInterval = time.Second
	// DefaultChatRateLimit conservatively supports Telegram group destinations.
	DefaultChatRateLimit = 20
	// DefaultChatRateInterval is the time window for DefaultChatRateLimit.
	DefaultChatRateInterval = time.Minute
)

var (
	ErrInvalidName     = errors.New("telegram: invalid service name")
	ErrInvalidToken    = errors.New("telegram: invalid bot token")
	ErrInvalidChatID   = errors.New("telegram: invalid chat ID")
	ErrRegistered      = errors.New("telegram: service already registered")
	ErrEmptyText       = errors.New("telegram: message text cannot be empty")
	ErrInvalidMarkup   = errors.New("telegram: invalid markup")
	ErrCannotSplit     = errors.New("telegram: formatted message cannot be split safely")
	ErrInvalidResponse = errors.New("telegram: invalid API response")
)

// Service sends notifications to one default Telegram chat.
type Service struct {
	name        string
	chatID      string
	token       string
	baseURL     string
	client      *http.Client
	botLimiter  *limiter
	chatLimiter *limiter
}

// Name returns the unique name used to register the service.
func (service *Service) Name() string {
	return service.name
}

// ChatID returns the default destination configured for the service.
func (service *Service) ChatID() string {
	return service.chatID
}

// Message is the minimal result returned for every successfully delivered chunk.
type Message struct {
	ID     int64
	Date   time.Time
	ChatID string
	Text   string
}

// Option configures a Service during registration.
type Option func(*config) error

type config struct {
	client           *http.Client
	baseURL          string
	botRateLimit     int
	botRateInterval  time.Duration
	chatRateLimit    int
	chatRateInterval time.Duration
}

func defaultConfig() config {
	return config{
		client:           http.DefaultClient,
		baseURL:          "https://api.telegram.org",
		botRateLimit:     DefaultBotRateLimit,
		botRateInterval:  DefaultBotRateInterval,
		chatRateLimit:    DefaultChatRateLimit,
		chatRateInterval: DefaultChatRateInterval,
	}
}

// WithHTTPClient sets the client used to call the Telegram Bot API.
func WithHTTPClient(client *http.Client) Option {
	return func(config *config) error {
		if client == nil {
			return errors.New("telegram: HTTP client cannot be nil")
		}
		config.client = client
		return nil
	}
}

// WithBaseURL sets the Bot API base URL. It is primarily useful with a local Bot API server.
func WithBaseURL(baseURL string) Option {
	return func(config *config) error {
		if baseURL == "" {
			return errors.New("telegram: API base URL cannot be empty")
		}
		config.baseURL = baseURL
		return nil
	}
}

// WithBotRateLimit sets the shared request limit for all services using one token.
func WithBotRateLimit(limit int, interval time.Duration) Option {
	return rateLimitOption("bot", limit, interval, func(config *config, limit int, interval time.Duration) {
		config.botRateLimit = limit
		config.botRateInterval = interval
	})
}

// WithChatRateLimit sets the request limit for this service's default chat.
func WithChatRateLimit(limit int, interval time.Duration) Option {
	return rateLimitOption("chat", limit, interval, func(config *config, limit int, interval time.Duration) {
		config.chatRateLimit = limit
		config.chatRateInterval = interval
	})
}

func rateLimitOption(scope string, limit int, interval time.Duration, apply func(*config, int, time.Duration)) Option {
	return func(config *config) error {
		if limit < 1 {
			return fmt.Errorf("telegram: %s rate limit must be positive", scope)
		}
		if interval <= 0 {
			return fmt.Errorf("telegram: %s rate limit interval must be positive", scope)
		}
		apply(config, limit, interval)
		return nil
	}
}

// APIError is returned when Telegram rejects a request.
type APIError struct {
	Code            int
	Description     string
	RetryAfter      time.Duration
	MigrateToChatID string
}

func (error *APIError) Error() string {
	return fmt.Sprintf("telegram: API error %d: %s", error.Code, error.Description)
}

// HTTPStatusError is returned when the Bot API responds with a non-success status.
type HTTPStatusError struct {
	StatusCode int
}

func (error *HTTPStatusError) Error() string {
	return fmt.Sprintf("telegram: unexpected HTTP status %d", error.StatusCode)
}
