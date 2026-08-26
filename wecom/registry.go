package wecom

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/Cyvadra/utils/internal/ratelimit"
	"github.com/Cyvadra/utils/internal/registry"
)

var services = struct {
	byName    *registry.Registry[*Service]
	byWebhook *registry.Registry[*Service]
}{
	byName:    registry.New[*Service](),
	byWebhook: registry.New[*Service](),
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
		limiter:    ratelimit.New(config.rateLimit, config.rateInterval),
	}

	if err := services.byName.Register(name, service); err != nil {
		return nil, fmt.Errorf("%w: name %q", ErrBotRegistered, name)
	}
	if err := services.byWebhook.Register(canonicalURL, service); err != nil {
		services.byName.Delete(name)
		return nil, fmt.Errorf("%w: webhook is already registered", ErrBotRegistered)
	}
	return service, nil
}

// Get returns the bot registered with name.
func Get(name string) (*Service, bool) {
	return services.byName.Get(strings.TrimSpace(name))
}

func validateWebhookURL(rawURL string) (string, error) {
	webhookURL, err := url.Parse(rawURL)
	if err != nil || webhookURL.Scheme != "https" || webhookURL.Host != "qyapi.weixin.qq.com" || webhookURL.Path != "/cgi-bin/webhook/send" || webhookURL.Query().Get("key") == "" {
		return "", ErrInvalidWebhookURL
	}
	return webhookURL.String(), nil
}
