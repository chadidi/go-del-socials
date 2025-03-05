package twitter

import (
	"fmt"
	"time"
)

type Config struct {
	APIKey            string
	APIKeySecret      string
	AccessToken       string
	AccessTokenSecret string
	BearerToken       string
	Username          string
}

type Client struct {
	config *Config
}

func NewClient(config *Config) (*Client, error) {
	if config.BearerToken == "" {
		return nil, fmt.Errorf("bearer token is required")
	}

	return &Client{
		config: config,
	}, nil
}

func (c *Client) DeleteContent(contentType string, cutoffDate time.Time) (int, int, error) {
	return 0, 0, fmt.Errorf("twitter deletion functionality coming soon")
}
