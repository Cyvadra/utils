package telegram

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"sync"
)

var tokenPattern = regexp.MustCompile(`^[0-9]+:[A-Za-z0-9_-]+$`)

var services = struct {
	sync.RWMutex
	byName   map[string]*Service
	byTarget map[string]*Service
	byToken  map[string]*limiter
}{
	byName:   make(map[string]*Service),
	byTarget: make(map[string]*Service),
	byToken:  make(map[string]*limiter),
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

	targetKey := token + "\x00" + chatID
	services.Lock()
	defer services.Unlock()
	if _, exists := services.byName[name]; exists {
		return nil, fmt.Errorf("%w: name %q", ErrRegistered, name)
	}
	if _, exists := services.byTarget[targetKey]; exists {
		return nil, fmt.Errorf("%w: bot and chat are already registered", ErrRegistered)
	}

	botLimiter, exists := services.byToken[token]
	if !exists {
		botLimiter = newLimiter(config.botRateLimit, config.botRateInterval)
		services.byToken[token] = botLimiter
	}
	service := &Service{
		name:        name,
		chatID:      chatID,
		token:       token,
		baseURL:     baseURL,
		client:      config.client,
		botLimiter:  botLimiter,
		chatLimiter: newLimiter(config.chatRateLimit, config.chatRateInterval),
	}
	services.byName[name] = service
	services.byTarget[targetKey] = service
	return service, nil
}

// Get returns the service registered with name.
func Get(name string) (*Service, bool) {
	services.RLock()
	defer services.RUnlock()
	service, exists := services.byName[name]
	return service, exists
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
	return strings.TrimRight(baseURL.String(), "/"), nil
}
