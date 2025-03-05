package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/vartanbeno/go-reddit/v2/reddit"
)

type Config struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	Username     string `json:"username"`
	Password     string `json:"password"`
	UserAgent    string `json:"user_agent"`
}

type RedditClient struct {
	*reddit.Client
	accessToken string
	httpClient  *http.Client
	config      *Config
}

func loadConfig() (*Config, error) {
	file, err := os.ReadFile("config.json")
	if err != nil {
		return nil, fmt.Errorf("error reading config file: %v", err)
	}

	var config Config
	if err := json.Unmarshal(file, &config); err != nil {
		return nil, fmt.Errorf("error parsing config file: %v", err)
	}

	return &config, nil
}

func newRedditClient(config *Config) (*RedditClient, error) {
	credentials := reddit.Credentials{
		ID:       config.ClientID,
		Secret:   config.ClientSecret,
		Username: config.Username,
		Password: config.Password,
	}

	client, err := reddit.NewClient(credentials, reddit.WithUserAgent(config.UserAgent))
	if err != nil {
		return nil, fmt.Errorf("failed to create Reddit client: %v", err)
	}

	// Get OAuth2 token
	data := url.Values{}
	data.Set("grant_type", "password")
	data.Set("username", config.Username)
	data.Set("password", config.Password)

	req, err := http.NewRequest("POST", "https://www.reddit.com/api/v1/access_token", strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create token request: %v", err)
	}

	req.SetBasicAuth(config.ClientID, config.ClientSecret)
	req.Header.Set("User-Agent", config.UserAgent)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	httpClient := &http.Client{}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get token: %v", err)
	}
	defer resp.Body.Close()

	var tokenResp struct {
		AccessToken string `json:"access_token"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, fmt.Errorf("failed to decode token response: %v", err)
	}

	return &RedditClient{
		Client:      client,
		accessToken: tokenResp.AccessToken,
		httpClient:  httpClient,
		config:      config,
	}, nil
}

func (c *RedditClient) deleteContent(fullname string) error {
	data := url.Values{}
	data.Set("id", fullname)

	req, err := http.NewRequest("POST", "https://oauth.reddit.com/api/del", strings.NewReader(data.Encode()))
	if err != nil {
		return fmt.Errorf("failed to create delete request: %v", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.accessToken)
	req.Header.Set("User-Agent", c.config.UserAgent)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send delete request: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("delete request failed: %s - %s", resp.Status, string(body))
	}

	return nil
}

func deleteUserContent(client *RedditClient, cutoffDate time.Time) (int, error) {
	deletedCount := 0

	// Delete posts
	postsOpts := reddit.ListUserOverviewOptions{
		ListOptions: reddit.ListOptions{
			Limit: 100,
		},
	}

	for {
		posts, resp, err := client.User.Posts(context.Background(), &postsOpts)
		if err != nil {
			return deletedCount, fmt.Errorf("failed to fetch posts: %v", err)
		}

		if len(posts) == 0 {
			break
		}

		for _, post := range posts {
			postTime := time.Unix(post.Created.Unix(), 0)
			fmt.Printf("Found post: %s (posted on %s)\n", post.Title, postTime.Format("2006-01-02"))

			if postTime.Before(cutoffDate) {
				fullname := fmt.Sprintf("t3_%s", post.ID)
				fmt.Printf("Attempting to delete post: %s (Fullname: %s)\n", post.Title, fullname)

				if err := client.deleteContent(fullname); err != nil {
					fmt.Printf("Error deleting post %s: %v\n", fullname, err)
					continue
				}

				fmt.Printf("Successfully deleted post: %s\n", post.Title)
				deletedCount++
			}
		}

		if resp.After == "" {
			break
		}

		postsOpts.After = resp.After
		time.Sleep(2 * time.Second)
	}

	// Delete comments
	commentsOpts := reddit.ListUserOverviewOptions{
		ListOptions: reddit.ListOptions{
			Limit: 100,
		},
	}

	for {
		comments, resp, err := client.User.Comments(context.Background(), &commentsOpts)
		if err != nil {
			return deletedCount, fmt.Errorf("failed to fetch comments: %v", err)
		}

		if len(comments) == 0 {
			break
		}

		for _, comment := range comments {
			commentTime := time.Unix(comment.Created.Unix(), 0)

			if commentTime.Before(cutoffDate) {
				fullname := fmt.Sprintf("t1_%s", comment.ID)
				fmt.Printf("Attempting to delete comment from %s (Fullname: %s)\n", commentTime.Format("2006-01-02"), fullname)

				if err := client.deleteContent(fullname); err != nil {
					fmt.Printf("Error deleting comment %s: %v\n", fullname, err)
					continue
				}

				fmt.Printf("Successfully deleted comment from %s\n", commentTime.Format("2006-01-02"))
				deletedCount++
			}
		}

		if resp.After == "" {
			break
		}

		commentsOpts.After = resp.After
		time.Sleep(2 * time.Second)
	}

	return deletedCount, nil
}

func main() {
	// Load configuration
	config, err := loadConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Create Reddit client
	client, err := newRedditClient(config)
	if err != nil {
		log.Fatalf("Failed to create Reddit client: %v", err)
	}

	// Verify authentication
	user, _, err := client.User.Get(context.Background(), config.Username)
	if err != nil {
		log.Fatalf("Failed to authenticate: %v", err)
	}
	fmt.Printf("Authenticated as user: %s\n", user.Name)

	cutoffDate := time.Date(2018, 1, 1, 0, 0, 0, 0, time.UTC)
	deletedCount, err := deleteUserContent(client, cutoffDate)
	if err != nil {
		log.Fatalf("Error during deletion: %v", err)
	}

	fmt.Printf("Finished! Deleted %d items older than 2018\n", deletedCount)
}
