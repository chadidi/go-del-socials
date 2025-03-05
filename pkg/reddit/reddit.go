package reddit

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/vartanbeno/go-reddit/v2/reddit"
)

type Config struct {
	ClientID     string
	ClientSecret string
	Username     string
	Password     string
	UserAgent    string
}

type Client struct {
	*reddit.Client
	accessToken string
	httpClient  *http.Client
	config      *Config
}

func NewClient(config *Config) (*Client, error) {
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

	return &Client{
		Client:      client,
		accessToken: tokenResp.AccessToken,
		httpClient:  httpClient,
		config:      config,
	}, nil
}

func (c *Client) deleteContent(fullname string) error {
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

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("delete request failed: %s", resp.Status)
	}

	return nil
}

func (c *Client) DeleteContent(contentType string, cutoffDate time.Time) (int, int, error) {
	postsDeleted := 0
	commentsDeleted := 0

	// Delete posts if requested
	if contentType == "all" || contentType == "posts" {
		postsOpts := reddit.ListUserOverviewOptions{
			ListOptions: reddit.ListOptions{
				Limit: 100,
			},
		}

		for {
			posts, resp, err := c.User.Posts(context.Background(), &postsOpts)
			if err != nil {
				return postsDeleted, commentsDeleted, fmt.Errorf("failed to fetch posts: %v", err)
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

					if err := c.deleteContent(fullname); err != nil {
						fmt.Printf("Error deleting post %s: %v\n", fullname, err)
						continue
					}

					fmt.Printf("Successfully deleted post: %s\n", post.Title)
					postsDeleted++
				}
			}

			if resp.After == "" {
				break
			}

			postsOpts.After = resp.After
			time.Sleep(2 * time.Second)
		}
	}

	// Delete comments if requested
	if contentType == "all" || contentType == "comments" {
		commentsOpts := reddit.ListUserOverviewOptions{
			ListOptions: reddit.ListOptions{
				Limit: 100,
			},
		}

		for {
			comments, resp, err := c.User.Comments(context.Background(), &commentsOpts)
			if err != nil {
				return postsDeleted, commentsDeleted, fmt.Errorf("failed to fetch comments: %v", err)
			}

			if len(comments) == 0 {
				break
			}

			for _, comment := range comments {
				commentTime := time.Unix(comment.Created.Unix(), 0)

				if commentTime.Before(cutoffDate) {
					fullname := fmt.Sprintf("t1_%s", comment.ID)
					fmt.Printf("Attempting to delete comment from %s (Fullname: %s)\n", commentTime.Format("2006-01-02"), fullname)

					if err := c.deleteContent(fullname); err != nil {
						fmt.Printf("Error deleting comment %s: %v\n", fullname, err)
						continue
					}

					fmt.Printf("Successfully deleted comment from %s\n", commentTime.Format("2006-01-02"))
					commentsDeleted++
				}
			}

			if resp.After == "" {
				break
			}

			commentsOpts.After = resp.After
			time.Sleep(2 * time.Second)
		}
	}

	return postsDeleted, commentsDeleted, nil
}
