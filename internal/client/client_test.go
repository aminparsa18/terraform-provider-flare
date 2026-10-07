package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newTestClient(t *testing.T, h http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	c, err := New(srv.URL, "flr_pat_test", "test")
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestNewRejectsBadEndpoint(t *testing.T) {
	for _, e := range []string{"", "flare.local", "ftp://x", "http://"} {
		if _, err := New(e, "t", "ua"); err == nil {
			t.Errorf("New(%q) = nil error, want error", e)
		}
	}
}

func TestRequestsCarryBearerTokenAndJSON(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer flr_pat_test" {
			t.Errorf("Authorization = %q", got)
		}
		if r.Method == http.MethodPost && r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Content-Type = %q", r.Header.Get("Content-Type"))
		}
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(NotificationChannel{ID: "1", Name: "x", Type: "Webhook"})
	})
	got, err := c.CreateNotificationChannel(context.Background(), NotificationChannel{Name: "x", Type: "Webhook"})
	if err != nil || got.ID != "1" {
		t.Fatalf("got %+v, %v", got, err)
	}
}

func TestProblemDetailSurfacesInError(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"title":"Conflict","detail":"A notification channel named 'x' already exists."}`))
	})
	_, err := c.CreateNotificationChannel(context.Background(), NotificationChannel{Name: "x", Type: "Webhook"})
	if err == nil || !strings.Contains(err.Error(), "already exists") || !strings.Contains(err.Error(), "409") {
		t.Fatalf("err = %v", err)
	}
}

func TestNotFound(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) { http.NotFound(w, r) })
	_, err := c.GetNotificationChannel(context.Background(), "nope")
	if !IsNotFound(err) {
		t.Fatalf("IsNotFound(%v) = false", err)
	}
}

func TestOmittedOptionalFieldsAreNotSent(t *testing.T) {
	var body map[string]any
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&body)
		_, _ = w.Write([]byte(`{}`))
	})
	_, _ = c.UpdateNotificationChannel(context.Background(), "1", NotificationChannel{Name: "n", Type: "Email"})
	if _, present := body["webhookUrl"]; present {
		t.Errorf("webhookUrl was sent: %v", body)
	}
}

func channelsServer(channels ...NotificationChannel) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(notificationChannelList{Channels: channels})
	}
}

func TestFindByNameIsCaseInsensitive(t *testing.T) {
	c := newTestClient(t, channelsServer(NotificationChannel{ID: "a", Name: "Oncall Slack"}, NotificationChannel{ID: "b", Name: "email"}))
	got, err := c.FindNotificationChannelByName(context.Background(), " oncall slack ")
	if err != nil || got.ID != "a" {
		t.Fatalf("got %+v, %v", got, err)
	}
}

func TestFindByNameMissingIsNotFound(t *testing.T) {
	c := newTestClient(t, channelsServer())
	if _, err := c.FindNotificationChannelByName(context.Background(), "x"); !IsNotFound(err) {
		t.Fatalf("err = %v", err)
	}
}

func TestFindByNameAmbiguousIsAnErrorNotAGuess(t *testing.T) {
	c := newTestClient(t, channelsServer(NotificationChannel{ID: "a", Name: "dup"}, NotificationChannel{ID: "b", Name: "DUP"}))
	_, err := c.FindNotificationChannelByName(context.Background(), "dup")
	if err == nil || IsNotFound(err) || !strings.Contains(err.Error(), "2 notification channels") {
		t.Fatalf("err = %v", err)
	}
}
