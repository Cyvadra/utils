> sugar for golang development

## WeCom webhooks

The `wecom` package sends text and markdown messages through WeCom group bot
webhooks. Register each bot once with a unique name, then retain its returned
service or retrieve it later by name.

```go
package main

import (
	"context"

	"github.com/Cyvadra/utils/wecom"
)

func main() {
	bot, err := wecom.Register(
		"production-alerts",
		"https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=YOUR_SECRET_KEY",
	)
	if err != nil {
		panic(err)
	}

	err = bot.Send(context.Background(), "deployment completed", wecom.Mention{
		UserIDs:       []string{"zhangsan"},
		MobileNumbers: []string{"13800001111"},
		All:           true,
	})
	if err != nil {
		panic(err)
	}

	_ = bot.SendMarkdown(context.Background(), "# Deployment\nService is healthy")
}
```

Registering a duplicate name or webhook returns an error. `wecom.Get(name)`
retrieves a previously registered service. Webhook URLs are secrets; provide
them through configuration and do not commit them to source control.

Text messages larger than 2,048 bytes and markdown messages larger than 4,096
bytes are split into ordered requests, preferring newline boundaries and then
valid UTF-8 boundaries. A failure after one or more chunks can result in partial
delivery.

Each bot is independently limited to 20 requests per minute by default. Sending
waits for capacity and observes context cancellation. Use
`wecom.WithRateLimit(limit, interval)` during `Register` to set a different
per-bot limit when needed.

## Telegram notifications

The `telegram` package sends plain text, MarkdownV2, and HTML notifications to
a registered default chat. Create a bot with BotFather, add it to the target
chat or channel, and use that bot's token with the numeric chat ID or public
`@username`.

```go
package main

import (
	"context"

	"github.com/Cyvadra/utils/telegram"
)

func main() {
	bot, err := telegram.Register(
		"production-alerts",
		"BOTFATHER_TOKEN",
		"-1001234567890",
	)
	if err != nil {
		panic(err)
	}

	_, err = bot.SendMarkdown(context.Background(), "*Deployment complete*")
	if err != nil {
		panic(err)
	}

	_, err = bot.SendHTML(context.Background(), "<b>Service is healthy</b>")
	if err != nil {
		panic(err)
	}
}
```

Registering the same name or token/chat pair returns an error. A bot token may
be registered for multiple destinations, and `telegram.Get(name)` retrieves an
existing service. Bot tokens are secrets; load them from configuration and never
commit them to source control.

Messages longer than Telegram's 4,096-character limit are split in order. Plain
text prefers newline boundaries. MarkdownV2 and HTML are validated and split
with active formatting closed and reopened for each chunk. Invalid markup fails
before delivery; a later request failure can still yield partial delivery, so
send methods return all successful `telegram.Message` results with the error.

By default each destination is limited to 20 requests per minute, while every
service using the same token shares a 30 requests-per-second cap. Both waits
respect context cancellation. A Telegram 429 response containing `retry_after`
is retried once after the requested delay. Override limits during registration
with `telegram.WithBotRateLimit` and `telegram.WithChatRateLimit` for known
traffic patterns.
