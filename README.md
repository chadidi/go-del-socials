# Social Media Content Deletion Tool

This Go tool helps you automatically delete your old social media content. Currently supports:
- Reddit posts and comments deletion
- Twitter tweets and replies deletion

## Prerequisites

- Go 1.23.6 or later
- Reddit account and API credentials (for Reddit deletion)
- Twitter API credentials (for Twitter deletion)

## Setup

### Reddit Setup
1. Create a Reddit application to get API credentials:
   - Go to https://www.reddit.com/prefs/apps
   - Click "create another app..."
   - Select "script"
   - Fill in the name and description
   - Set the redirect URI to http://localhost:8080
   - Click "create app"

### Configuration
1. Copy the `config.json.example` to `config.json` and fill in your credentials:

```json
{
    "reddit": {
        "client_id": "your_client_id",
        "client_secret": "your_client_secret",
        "username": "your_reddit_username",
        "password": "your_reddit_password",
        "user_agent": "RedditDelete/1.0.0"
    },
    "twitter": {
        "api_key": "YOUR_API_KEY",
        "api_key_secret": "YOUR_API_KEY_SECRET",
        "username": "YOUR_TWITTER_USERNAME"
    }
}
```

#### Reddit Configuration Fields
- `client_id`: The string under "personal use script" from your Reddit app settings
- `client_secret`: The "secret" field from your Reddit app settings
- `username`: Your Reddit account username
- `password`: Your Reddit account password
- `user_agent`: User agent string for API requests (can be left as default)

#### Twitter Configuration Fields
- `api_key`: Your Twitter API key from the developer portal
- `api_key_secret`: Your Twitter API key secret from the developer portal
- `username`: Your Twitter username

### Twitter Setup
1. Create a Twitter Developer account and get API credentials:
   - Go to https://developer.twitter.com/
   - Sign up for a developer account if you haven't already
   - Create a new project and app
   - Get your API Key and Secret from the app settings
   - Make sure you have read and write permissions for your app

## Usage

1. Make sure your credentials are properly set in `config.json`
2. Run the script:
   ```bash
   go run main.go
   ```

The script will:
- Load your social media content (posts and comments for Reddit)
- Check each item's date
- Delete content older than the cutoff date
- Show progress as it runs

## Features

### Reddit
- Deletes both posts and comments
- Shows detailed progress for each deletion
- Includes a 2-second delay between API calls to avoid rate limiting
- Provides error logging for failed deletions
- Shows count of deleted posts and comments at the end

### Twitter
- Deletes both tweets and replies
- Shows detailed progress including tweet content and dates
- Includes a 2-second delay between API calls to avoid rate limiting
- Verifies credentials and username before starting
- Shows separate counts for deleted tweets and replies
- Handles pagination to process all available tweets
- Provides error logging for failed deletions

## Safety Features

- Rate limiting protection with built-in delays between API calls
- Detailed logging of all operations
- Error handling for failed deletions
- Progress tracking during deletion process

## Note

Please be careful when using this script as content deletion is permanent and cannot be undone. Make sure to:
- Double check your cutoff date before running
- Backup any important content you want to keep
- Review the deletion progress as it runs