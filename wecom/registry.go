package wecom

import (
	"fmt"
	"net/url"
	"strings"
	"sync"
)

var services = struct {
	sync.RWMutex
	byName    map[string]*Service
	byWebhook map[string]*Service
}{
	byName:    make(map[string]*Service),
	byWebhook: make(map[string]*Service),
}

// Register validates and stores a uniquely named WeCom bot. Registration is local
// and does not send a test message to the webhook.
func Register(name, webhookURL string, options ...Option) (*Service, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, ErrInvalidName
	}

	canonicalURL, err := validateWebhookURL(webhookURL)
	if err != nil {
		return nil, err
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

	service := &Service{
		name:       name,
		webhookURL: canonicalURL,
		client:     config.client,
		limiter:    newLimiter(config.rateLimit, config.rateInterval),
	}

	services.Lock()
	defer services.Unlock()
	if _, exists := services.byName[name]; exists {
		return nil, fmt.Errorf("%w: name %q", ErrBotRegistered, name)
	}
	if _, exists := services.byWebhook[canonicalURL]; exists {
		return nil, fmt.Errorf("%w: webhook is already registered", ErrBotRegistered)
	}
	services.byName[name] = service
	services.byWebhook[canonicalURL] = service

	return service, nil
}

// Get returns the bot registered with name.
func Get(name string) (*Service, bool) {
	services.RLock()
	defer services.RUnlock()
	service, exists := services.byName[name]
	return service, exists
}

func validateWebhookURL(rawURL string) (string, error) {
	webhookURL, err := url.Parse(rawURL)
	if err != nil || webhookURL.Scheme != "https" || webhookURL.Host != "qyapi.weixin.qq.com" || webhookURL.Path != "/cgi-bin/webhook/send" || webhookURL.Query().Get("key") == "" {
		return "", ErrInvalidWebhookURL
	}
	return webhookURL.String(), nil
}
