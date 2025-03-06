package twitter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/michimani/gotwi"
	"github.com/michimani/gotwi/fields"
	"github.com/michimani/gotwi/tweet/managetweet"
	mttypes "github.com/michimani/gotwi/tweet/managetweet/types"
	"github.com/michimani/gotwi/tweet/timeline"
	ttypes "github.com/michimani/gotwi/tweet/timeline/types"
	"github.com/michimani/gotwi/user/userlookup"
	ultypes "github.com/michimani/gotwi/user/userlookup/types"
)

type configFile struct {
	Twitter Credentials `json:"twitter"`
}

type Credentials struct {
	APIKey            string `json:"api_key"`
	APIKeySecret      string `json:"api_key_secret"`
	AccessToken       string `json:"access_token"`
	AccessTokenSecret string `json:"access_token_secret"`
	Username          string `json:"username"`
}

type Config struct {
	Username string
}

func loadCredentials(path string) (*Credentials, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %v", err)
	}

	var config configFile
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %v", err)
	}

	creds := &config.Twitter
	if creds.APIKey == "" || creds.APIKeySecret == "" || creds.AccessToken == "" || creds.AccessTokenSecret == "" {
		return nil, fmt.Errorf("missing required credentials in config file")
	}

	return creds, nil
}

func (c *Config) Validate() error {
	if c.Username == "" {
		return errors.New("username is required")
	}
	return nil
}

type Client struct {
	client *gotwi.Client
	userID string
	config *Config
}

func NewClient(config *Config) (*Client, error) {
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %v", err)
	}

	creds, err := loadCredentials("config.json")
	if err != nil {
		return nil, err
	}

	in := &gotwi.NewClientInput{
		AuthenticationMethod: gotwi.AuthenMethodOAuth1UserContext,
		OAuthToken:           creds.AccessToken,
		OAuthTokenSecret:     creds.AccessTokenSecret,
		APIKey:               creds.APIKey,
		APIKeySecret:         creds.APIKeySecret,
	}

	client, err := gotwi.NewClient(in)
	if err != nil {
		return nil, fmt.Errorf("failed to create Twitter client: %v\nPlease verify your API credentials are correct and have the necessary permissions", err)
	}

	// Get user ID from username
	p := &ultypes.GetByUsernameInput{
		Username: config.Username,
	}

	res, err := userlookup.GetByUsername(context.Background(), client, p)
	if err != nil {
		var gtwErr *gotwi.GotwiError
		if errors.As(err, &gtwErr) {
			if gtwErr.StatusCode == 401 {
				return nil, fmt.Errorf("authentication failed: please check your API credentials and ensure they have the necessary permissions")
			}
			if gtwErr.StatusCode == 404 {
				return nil, fmt.Errorf("user '%s' not found: please verify the username", config.Username)
			}
			if gtwErr.StatusCode == 429 {
				waitForRateLimit(err)
			}
		}
		return nil, fmt.Errorf("failed to get user ID: %v", err)
	}

	if res == nil {
		return nil, fmt.Errorf("user data not found for username: %s", config.Username)
	}

	return &Client{
		client: client,
		userID: gotwi.StringValue(res.Data.ID),
		config: config,
	}, nil
}

func waitForRateLimit(err error) {
	var gtwErr *gotwi.GotwiError
	if errors.As(err, &gtwErr) && gtwErr.StatusCode == 429 {
		// Use a fixed wait time since the Twitter API doesn't provide reset time in the error
		waitTime := 15 * time.Minute
		fmt.Printf("\nRate limit reached. Waiting for %v before continuing...\n", waitTime)
		time.Sleep(waitTime)
	}
}

func (c *Client) DeleteContent(contentType string, cutoffDate time.Time) (int, int, error) {

	tweetsDeleted := 0
	repliesDeleted := 0
	ctx := context.Background()

	params := &ttypes.ListTweetsInput{
		ID:         c.userID,
		MaxResults: ttypes.ListMaxResults(20), // Maximum allowed per page
		TweetFields: fields.TweetFieldList{
			fields.TweetFieldCreatedAt,
			fields.TweetFieldReferencedTweets,
			fields.TweetFieldText, // Add text field to get tweet content
		},
		Expansions: fields.ExpansionList{
			fields.ExpansionReferencedTweetsID,
		},
	}

	baseDelay := 5 * time.Second
	maxRetries := 3

	for {
		var tweets *ttypes.ListTweetsOutput
		var err error

		tweets, err = timeline.ListTweets(ctx, c.client, params)
		if err != nil {
			var gtwErr *gotwi.GotwiError
			if errors.As(err, &gtwErr) && gtwErr.StatusCode == 429 {
				waitForRateLimit(err)
				continue // Retry the same request after waiting
			}
			return tweetsDeleted, repliesDeleted, fmt.Errorf("failed to fetch tweets: %v", err)
		}

		fmt.Printf("tweets: %+v\n", tweets)

		// Safely check for nil tweets response
		if tweets == nil {
			return tweetsDeleted, repliesDeleted, fmt.Errorf("received nil response from Twitter API")
		}

		fmt.Printf("Found %d tweets to delete\n", len(tweets.Data))

		// Check for empty data
		if len(tweets.Data) == 0 {
			break
		}

		for _, t := range tweets.Data {
			createdAt := t.CreatedAt
			if createdAt.Before(cutoffDate) {
				isReply := false
				if t.ReferencedTweets != nil {
					for _, ref := range t.ReferencedTweets {
						if gotwi.StringValue(ref.Type) == "replied_to" {
							isReply = true
							break
						}
					}
				}

				tweetID := gotwi.StringValue(t.ID)
				if tweetID == "" {
					continue // Skip if tweet ID is empty
				}

				tweetText := gotwi.StringValue(t.Text)
				fmt.Printf("Found %s from %s (ID: %s)\nContent: %s\n",
					map[bool]string{true: "reply", false: "tweet"}[isReply],
					createdAt.Format("2006-01-02"),
					tweetID,
					tweetText,
				)

				if contentType == "all" ||
					(contentType == "tweets" && !isReply) ||
					(contentType == "replies" && isReply) {

					deleteParams := &mttypes.DeleteInput{
						ID: tweetID,
					}

					// Retry loop for deleting tweets
					var deleteErr error
					for retry := 0; retry < maxRetries; retry++ {
						_, deleteErr = managetweet.Delete(ctx, c.client, deleteParams)
						if deleteErr == nil {
							break
						}

						var gtwErr *gotwi.GotwiError
						if errors.As(deleteErr, &gtwErr) && gtwErr.StatusCode == 429 {
							waitForRateLimit(deleteErr)
							continue
						}

						fmt.Printf("Error deleting tweet %s: %v\n", tweetID, deleteErr)
						break
					}

					if deleteErr == nil {
						fmt.Printf("Successfully deleted %s from %s\nContent: %s\n---\n",
							map[bool]string{true: "reply", false: "tweet"}[isReply],
							createdAt.Format("2006-01-02"),
							tweetText,
						)

						if isReply {
							repliesDeleted++
						} else {
							tweetsDeleted++
						}
					}
				}
			}
		}

		// Handle pagination using next_token
		nextToken := gotwi.StringValue(tweets.Meta.NextToken)
		if nextToken == "" {
			break
		}
		params.PaginationToken = nextToken

		// Add a base delay between requests to prevent rate limiting
		time.Sleep(baseDelay)
	}

	return tweetsDeleted, repliesDeleted, nil
}
