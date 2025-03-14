package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"go-del-socials/pkg/reddit"
	"go-del-socials/pkg/twitter"
)

type Config struct {
	Reddit struct {
		ClientID     string `json:"client_id"`
		ClientSecret string `json:"client_secret"`
		Username     string `json:"username"`
		Password     string `json:"password"`
		UserAgent    string `json:"user_agent"`
	} `json:"reddit"`
	Twitter struct {
		APIKey            string `json:"api_key"`
		APIKeySecret      string `json:"api_key_secret"`
		AccessToken       string `json:"access_token"`
		AccessTokenSecret string `json:"access_token_secret"`
		Username          string `json:"username"`
	} `json:"twitter"`
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

func promptChoice(prompt string, options []string, defaultOption string) (string, error) {
	reader := bufio.NewReader(os.Stdin)

	fmt.Println(prompt)
	for i, opt := range options {
		if defaultOption != "" && opt == defaultOption {
			fmt.Printf("%d. %s (default)\n", i+1, opt)
		} else {
			fmt.Printf("%d. %s\n", i+1, opt)
		}
	}

	if defaultOption != "" {
		fmt.Printf("Enter your choice (1-%d) or press Enter for default: ", len(options))
	} else {
		fmt.Printf("Enter your choice (1-%d): ", len(options))
	}

	input, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}

	input = strings.TrimSpace(input)
	if input == "" {
		if defaultOption != "" {
			return defaultOption, nil
		}
		return "", fmt.Errorf("a choice is required")
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

func runRedditDeletion(config *Config) error {
	redditConfig := &reddit.Config{
		ClientID:     config.Reddit.ClientID,
		ClientSecret: config.Reddit.ClientSecret,
		Username:     config.Reddit.Username,
		Password:     config.Reddit.Password,
		UserAgent:    config.Reddit.UserAgent,
	}

	client, err := reddit.NewClient(redditConfig)
	if err != nil {
		return fmt.Errorf("failed to create Reddit client: %v", err)
	}

	// Prompt for content type
	contentType, err := promptChoice("What would you like to delete?", []string{"all", "posts", "comments"}, "all")
	if err != nil {
		return fmt.Errorf("failed to get content type choice: %v", err)
	}

	// Prompt for cutoff date
	defaultDate := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	cutoffDate, err := promptDate("Enter the date before which to delete content", defaultDate)
	if err != nil {
		return fmt.Errorf("failed to get cutoff date: %v", err)
	}

	fmt.Printf("\nDeleting %s before %s...\n\n", contentType, cutoffDate.Format("2006-01-02"))

	postsDeleted, commentsDeleted, err := client.DeleteContent(contentType, cutoffDate)
	if err != nil {
		return fmt.Errorf("error during deletion: %v", err)
	}

	fmt.Printf("\nReddit Deletion Summary:\n")
	fmt.Printf("- Posts deleted: %d\n", postsDeleted)
	fmt.Printf("- Comments deleted: %d\n", commentsDeleted)
	fmt.Printf("Total items deleted: %d\n", postsDeleted+commentsDeleted)

	return nil
}

func runTwitterDeletion(config *Config) error {
	twitterConfig := &twitter.Config{
		Username: config.Twitter.Username,
	}

	client, err := twitter.NewClient(twitterConfig)
	if err != nil {
		return fmt.Errorf("failed to create Twitter client: %v", err)
	}

	// Prompt for content type
	contentType, err := promptChoice("What would you like to delete?", []string{"all", "tweets", "replies"}, "all")
	if err != nil {
		return fmt.Errorf("failed to get content type choice: %v", err)
	}

	// Prompt for cutoff date
	defaultDate := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	cutoffDate, err := promptDate("Enter the date before which to delete content", defaultDate)
	if err != nil {
		return fmt.Errorf("failed to get cutoff date: %v", err)
	}

	fmt.Printf("\nDeleting %s before %s...\n\n", contentType, cutoffDate.Format("2006-01-02"))

	tweetsDeleted, repliesDeleted, err := client.DeleteContent(contentType, cutoffDate)
	if err != nil {
		return fmt.Errorf("error during deletion: %v", err)
	}

	fmt.Printf("\nTwitter Deletion Summary:\n")
	fmt.Printf("- Tweets deleted: %d\n", tweetsDeleted)
	fmt.Printf("- Replies deleted: %d\n", repliesDeleted)
	fmt.Printf("Total items deleted: %d\n", tweetsDeleted+repliesDeleted)

	return nil
}

func main() {
	// Load configuration
	config, err := loadConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Choose platform
	platform, err := promptChoice("Choose platform:", []string{"reddit", "twitter"}, "")
	if err != nil {
		log.Fatalf("Failed to get platform choice: %v", err)
	}

	// Run the appropriate deletion function
	var runErr error
	switch platform {
	case "reddit":
		runErr = runRedditDeletion(config)
	case "twitter":
		fmt.Println("\n⚠️  Important Notice about Twitter/X Deletion ⚠️")
		fmt.Println("Twitter/X has significantly restricted their API access for free accounts.")
		fmt.Println("As a result, this tool may no longer work reliably with Twitter.")
		fmt.Println("\nRecommended Alternative:")
		fmt.Printf("Please use DeleteTweets: %s\n", "https://github.com/Lyfhael/DeleteTweets")
		fmt.Println("\nWould you like to:")
		choice, err := promptChoice("", []string{"Continue anyway", "Exit"}, "Exit")
		if err != nil {
			log.Fatalf("Failed to get choice: %v", err)
		}
		if choice == "Exit" {
			fmt.Println("Exiting. Please check out the recommended alternative tool.")
			os.Exit(0)
		}
		runErr = runTwitterDeletion(config)
	}

	if runErr != nil {
		log.Fatalf("Error: %v", runErr)
	}
}
