package telegram

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/Cyvadra/utils/internal/ratelimit"
	"github.com/Cyvadra/utils/internal/registry"
)

var tokenPattern = regexp.MustCompile(`^[0-9]+:[A-Za-z0-9_-]+$`)

var services = struct {
	byName   *registry.Registry[*Service]
	byTarget *registry.Registry[*Service]
	byToken  *registry.Registry[*ratelimit.Limiter]
}{
	byName:   registry.New[*Service](),
	byTarget: registry.New[*Service](),
	byToken:  registry.New[*ratelimit.Limiter](),
}

// Register validates and stores a named Telegram bot and default destination.
// Registration is local and does not call Telegram's getMe method.
func Register(name, token, chatID string, options ...Option) (*Service, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, ErrInvalidName
	}
	if !tokenPattern.MatchString(token) {
		return nil, ErrInvalidToken
	}
	chatID = strings.TrimSpace(chatID)
	if !validChatID(chatID) {
		return nil, ErrInvalidChatID
	}

	config := defaultConfig()
	for _, option := range options {
		if option == nil {
			continue
		}
		if err := option(&config); err != nil {
			return nil, err
		}
	}
	baseURL, err := canonicalBaseURL(config.baseURL)
	if err != nil {
		return nil, err
	}

	// Resolve the bot-wide limiter shared by every service using this token.
	// The first registration fixes its limits; conflicting limits are rejected.
	botLimiter := services.byToken.GetOrCreate(token, func() *ratelimit.Limiter {
		return ratelimit.New(config.botRateLimit, config.botRateInterval)
	})
	if botLimiter.Limit() != config.botRateLimit || botLimiter.Interval() != config.botRateInterval {
		return nil, fmt.Errorf("%w (existing: %d per %s)", ErrBotRateLimitConflict, botLimiter.Limit(), botLimiter.Interval())
	}

	service := &Service{
		name:        name,
		chatID:      chatID,
		token:       token,
		baseURL:     baseURL,
		client:      config.client,
		botLimiter:  botLimiter,
		chatLimiter: ratelimit.New(config.chatRateLimit, config.chatRateInterval),
	}

	targetKey := token + "\x00" + chatID
	if err := services.byName.Register(name, service); err != nil {
		return nil, fmt.Errorf("%w: name %q", ErrRegistered, name)
	}
	if err := services.byTarget.Register(targetKey, service); err != nil {
		services.byName.Delete(name)
		return nil, fmt.Errorf("%w: bot and chat are already registered", ErrRegistered)
	}
	return service, nil
}

// Get returns the service registered with name.
func Get(name string) (*Service, bool) {
	return services.byName.Get(strings.TrimSpace(name))
}

func validChatID(chatID string) bool {
	if strings.HasPrefix(chatID, "@") {
		return len(chatID) > 1
	}
	chatID = strings.TrimPrefix(chatID, "-")
	if chatID == "" {
		return false
	}
	for _, character := range chatID {
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
}

func canonicalBaseURL(rawURL string) (string, error) {
	baseURL, err := url.Parse(rawURL)
	if err != nil || baseURL.Scheme == "" || baseURL.Host == "" {
		return "", errors.New("telegram: invalid API base URL")
	}
	if baseURL.Path != "" && baseURL.Path != "/" {
		return "", errors.New("telegram: API base URL must not include a path")
	}
	baseURL.Path = ""
	return strings.TrimRight(baseURL.String(), "/"), nil
}
