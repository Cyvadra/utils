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
