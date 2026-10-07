package client

import (
	"context"
	"fmt"
	"net/http"
)

// NotificationChannel mirrors the API's NotificationChannel / NotificationChannelRequest (camelCase JSON,
// enum as a string). Optional fields are pointers so an omitted value stays omitted on write; on read the
// API returns "" for unset strings, and credentials come back masked.
type NotificationChannel struct {
	ID                  string  `json:"id,omitempty"`
	Name                string  `json:"name"`
	Description         *string `json:"description,omitempty"`
	Type                string  `json:"type"`
	WebhookURL          *string `json:"webhookUrl,omitempty"`
	TelegramBotToken    *string `json:"telegramBotToken,omitempty"`
	TelegramChatID      *string `json:"telegramChatId,omitempty"`
	EmailTo             *string `json:"emailTo,omitempty"`
	PagerDutyRoutingKey *string `json:"pagerDutyRoutingKey,omitempty"`
	SendResolved        *bool   `json:"sendResolved,omitempty"`
	JiraBaseURL         *string `json:"jiraBaseUrl,omitempty"`
	JiraEmail           *string `json:"jiraEmail,omitempty"`
	JiraAPIToken        *string `json:"jiraApiToken,omitempty"`
	JiraProjectKey      *string `json:"jiraProjectKey,omitempty"`
	JiraIssueType       *string `json:"jiraIssueType,omitempty"`
	IncidentIoToken     *string `json:"incidentIoToken,omitempty"`
	JsmOpsAPIKey        *string `json:"jsmOpsApiKey,omitempty"`
}

type notificationChannelList struct {
	Channels []NotificationChannel `json:"channels"`
}

func (c *Client) CreateNotificationChannel(ctx context.Context, in NotificationChannel) (NotificationChannel, error) {
	var out NotificationChannel
	err := c.do(ctx, http.MethodPost, "/api/notification-channels", in, &out)
	return out, err
}

func (c *Client) GetNotificationChannel(ctx context.Context, id string) (NotificationChannel, error) {
	var out NotificationChannel
	err := c.do(ctx, http.MethodGet, "/api/notification-channels/"+id, nil, &out)
	return out, err
}

func (c *Client) UpdateNotificationChannel(ctx context.Context, id string, in NotificationChannel) (NotificationChannel, error) {
	var out NotificationChannel
	err := c.do(ctx, http.MethodPut, "/api/notification-channels/"+id, in, &out)
	return out, err
}

func (c *Client) DeleteNotificationChannel(ctx context.Context, id string) error {
	return c.do(ctx, http.MethodDelete, "/api/notification-channels/"+id, nil, nil)
}

func (c *Client) ListNotificationChannels(ctx context.Context) ([]NotificationChannel, error) {
	var out notificationChannelList
	if err := c.do(ctx, http.MethodGet, "/api/notification-channels", nil, &out); err != nil {
		return nil, err
	}
	return out.Channels, nil
}

// FindNotificationChannelByName resolves a channel name to the channel. Names are unique server-side, but
// rows created before that rule can still collide, so an ambiguous name is an error rather than a guess.
func (c *Client) FindNotificationChannelByName(ctx context.Context, name string) (NotificationChannel, error) {
	all, err := c.ListNotificationChannels(ctx)
	if err != nil {
		return NotificationChannel{}, err
	}
	var matches []NotificationChannel
	for _, ch := range all {
		if equalFold(ch.Name, name) {
			matches = append(matches, ch)
		}
	}
	switch len(matches) {
	case 0:
		return NotificationChannel{}, &APIError{Status: http.StatusNotFound, Detail: fmt.Sprintf("no notification channel named %q", name)}
	case 1:
		return matches[0], nil
	default:
		return NotificationChannel{}, fmt.Errorf("%d notification channels are named %q; rename the duplicates in Flare", len(matches), name)
	}
}
