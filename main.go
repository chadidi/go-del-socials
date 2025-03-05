package main

import (
	"bufio"
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

type DeleteOption struct {
	Name        string
	Description string
}

var deleteOptions = []DeleteOption{
	{"all", "Delete both posts and comments"},
	{"posts", "Delete only posts"},
	{"comments", "Delete only comments"},
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

func promptChoice(prompt string, options []string, defaultOption string) (string, error) {
	reader := bufio.NewReader(os.Stdin)

	fmt.Println(prompt)
	for i, opt := range options {
		if opt == defaultOption {
			fmt.Printf("%d. %s (default)\n", i+1, opt)
		} else {
			fmt.Printf("%d. %s\n", i+1, opt)
		}
	}

	fmt.Printf("Enter your choice (1-%d) or press Enter for default: ", len(options))
	input, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}

	input = strings.TrimSpace(input)
	if input == "" {
		return defaultOption, nil
	}

	choice := 0
	_, err = fmt.Sscanf(input, "%d", &choice)
	if err != nil || choice < 1 || choice > len(options) {
		return "", fmt.Errorf("invalid choice")
	}

	return options[choice-1], nil
}

func promptDate(prompt string, defaultDate time.Time) (time.Time, error) {
	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("%s (YYYY or YYYY-MM or YYYY-MM-DD) [default: %s]: ", prompt, defaultDate.Format("2006"))

	input, err := reader.ReadString('\n')
	if err != nil {
		return time.Time{}, err
	}

	input = strings.TrimSpace(input)
	if input == "" {
		return defaultDate, nil
	}

	// Try different date formats
	var t time.Time

	switch len(strings.Split(input, "-")) {
	case 1: // Year only (YYYY)
		year := 0
		if _, err := fmt.Sscanf(input, "%d", &year); err != nil {
			return time.Time{}, fmt.Errorf("invalid year format: %v", err)
		}
		t = time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC)

	case 2: // Year and month (YYYY-MM)
		year, month := 0, 0
		if _, err := fmt.Sscanf(input, "%d-%d", &year, &month); err != nil {
			return time.Time{}, fmt.Errorf("invalid year-month format: %v", err)
		}
		if month < 1 || month > 12 {
			return time.Time{}, fmt.Errorf("invalid month: must be between 1 and 12")
		}
		t = time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)

	case 3: // Full date (YYYY-MM-DD)
		var err error
		t, err = time.Parse("2006-01-02", input)
		if err != nil {
			return time.Time{}, fmt.Errorf("invalid date format: %v", err)
		}

	default:
		return time.Time{}, fmt.Errorf("invalid date format. Use YYYY or YYYY-MM or YYYY-MM-DD")
	}

	return t, nil
}

func deleteUserContent(client *RedditClient, contentType string, cutoffDate time.Time) (int, int, error) {
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
			posts, resp, err := client.User.Posts(context.Background(), &postsOpts)
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

					if err := client.deleteContent(fullname); err != nil {
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
			comments, resp, err := client.User.Comments(context.Background(), &commentsOpts)
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

					if err := client.deleteContent(fullname); err != nil {
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
	fmt.Printf("Authenticated as user: %s\n\n", user.Name)

	// Prompt for content type with default "all"
	contentType, err := promptChoice("What would you like to delete?", []string{"all", "posts", "comments"}, "all")
	if err != nil {
		log.Fatalf("Failed to get content type choice: %v", err)
	}

	// Prompt for cutoff date with default 2020
	defaultDate := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	cutoffDate, err := promptDate("Enter the date before which to delete content", defaultDate)
	if err != nil {
		log.Fatalf("Failed to get cutoff date: %v", err)
	}

	fmt.Printf("\nDeleting %s before %s...\n\n", contentType, cutoffDate.Format("2006-01-02"))

	postsDeleted, commentsDeleted, err := deleteUserContent(client, contentType, cutoffDate)
	if err != nil {
		log.Fatalf("Error during deletion: %v", err)
	}

	fmt.Printf("\nDeletion Summary:\n")
	fmt.Printf("- Posts deleted: %d\n", postsDeleted)
	fmt.Printf("- Comments deleted: %d\n", commentsDeleted)
	fmt.Printf("Total items deleted: %d\n", postsDeleted+commentsDeleted)
}
