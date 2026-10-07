package client

import (
	"context"
	"net/http"
)

// ServiceAccount mirrors the API's UserSummaryDto for a service account.
type ServiceAccount struct {
	ID         string `json:"id"`
	Username   string `json:"username"`
	Role       string `json:"role"`
	IsDisabled bool   `json:"isDisabled"`
}

func (c *Client) CreateServiceAccount(ctx context.Context, name, role string) (ServiceAccount, error) {
	var out ServiceAccount
	err := c.do(ctx, http.MethodPost, "/api/service-accounts", map[string]string{"name": name, "role": role}, &out)
	return out, err
}

func (c *Client) GetServiceAccount(ctx context.Context, id string) (ServiceAccount, error) {
	var out ServiceAccount
	err := c.do(ctx, http.MethodGet, "/api/service-accounts/"+id, nil, &out)
	return out, err
}

// DeleteServiceAccount is permanent: the account, its tokens and its memberships go.
func (c *Client) DeleteServiceAccount(ctx context.Context, id string) error {
	return c.do(ctx, http.MethodDelete, "/api/service-accounts/"+id, nil, nil)
}

type userList struct {
	Users []struct {
		ServiceAccount
		AuthProvider string `json:"authProvider"`
	} `json:"users"`
}

// FindServiceAccountByName resolves a service account by its (unique, case-insensitive) name.
func (c *Client) FindServiceAccountByName(ctx context.Context, name string) (ServiceAccount, error) {
	var out userList
	if err := c.do(ctx, http.MethodGet, "/api/users", nil, &out); err != nil {
		return ServiceAccount{}, err
	}
	for _, u := range out.Users {
		if u.AuthProvider == "ServiceAccount" && equalFold(u.Username, name) {
			return u.ServiceAccount, nil
		}
	}
	return ServiceAccount{}, &APIError{Status: http.StatusNotFound, Detail: "no service account named " + name}
}
