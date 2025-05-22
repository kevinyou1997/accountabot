# Discord Study Accountability Bot

A simple Discord bot built in Go that tracks your daily study updates and keeps you accountable by monitoring your check-ins in a dedicated study channel.

## Features

- **Daily Study Tracking**: Monitors your messages in the `studying-updates` channel
- **Check-in Recording**: Automatically records when you post study updates
- **Accountability Reminders**: Sends reminders when you haven't checked in for a while
- **Progress Persistence**: Saves your check-in history to a local database

## How It Works

1. The bot monitors a specific Discord channel (configured as `studyChannelID`)
2. Every time you post a message in that channel, it counts as a "check-in"
3. The bot adds a ✅ reaction to acknowledge your update
4. If you haven't checked in within the configured time period, it sends you a reminder
5. All your check-in data is saved locally for persistence

## Setup

### Prerequisites

- Go 1.18 or higher
- A Discord bot token (create one at [Discord Developer Portal](https://discord.com/developers/applications))
- Bot permissions: Send Messages, Read Message History, Add Reactions

### Installation

1. Clone/download the code
2. Create a `config.json` file with your settings
3. Build and run:

```bash
go mod init study-bot
go get github.com/bwmarrin/discordgo
go build -o study-bot
./study-bot
```

## Configuration

Create a `config.json` file:

```json
{
  "token": "YOUR_DISCORD_BOT_TOKEN",
  "studyChannelID": "1234567890123456789",
  "databasePath": "study_data.json",
  "reminderTime": "09:00",
  "checkInFrequency": 24
}
```

### Configuration Options

- `token`: Your Discord bot token
- `studyChannelID`: The ID of your studying-updates channel
- `databasePath`: Where to save your study data (defaults to "study_data.json")
- `reminderTime`: When to send daily reminders in 24-hour format (defaults to "09:00")
- `checkInFrequency`: How many hours between expected check-ins (defaults to 24)

### Getting Your Channel ID

1. Enable Developer Mode in Discord (User Settings > Advanced > Developer Mode)
2. Right-click on your studying-updates channel
3. Select "Copy ID"
4. Paste this ID as the `studyChannelID` in your config

## Usage

1. **Start the bot**: Run the compiled executable
2. **Post study updates**: Write messages in your studying-updates channel
   - "Just finished reviewing calculus chapter 3!"
   - "Completed 2 hours of Python coding practice"
   - "Read 20 pages of my textbook today"
3. **Get reminders**: The bot will remind you if you haven't posted in a while
4. **Check logs**: The bot logs all check-ins to the console

## Example

```
# In your studying-updates channel:
"Just started my morning study session with linear algebra!"
# Bot adds ✅ reaction and logs: "Check-in recorded for YourUsername"

# If you don't post for 24+ hours:
# Bot sends: "📚 Hey @YourUsername! It's been 26 hours since your last study check-in. How's your progress going today?"
```

## Data Storage

The bot saves your check-in data in a JSON file. Example structure:

```json
{
  "userActivities": {
    "123456789": {
      "userID": "123456789",
      "username": "YourUsername",
      "lastCheckIn": "2024-01-15T14:30:00Z",
      "checkIns": [
        "2024-01-14T09:15:00Z",
        "2024-01-15T14:30:00Z"
      ]
    }
  }
}
```

## Future Features

This is a minimal version focused on daily tracking. Future versions will include:
- Ticket/task management
- Progress statistics and visualizations
- Multi-project tracking
- Slash commands for better interaction

## Troubleshooting

- **Bot not responding**: Check that the bot token is correct and the bot is online
- **No check-ins recorded**: Verify the `studyChannelID` matches your channel
- **Reminders not working**: Check that the `reminderTime` format is correct (HH:MM)