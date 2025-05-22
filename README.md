# Discord Accountability Bot

A Discord bot built in Go that helps keep you accountable for your projects by tracking daily updates, managing tickets, and providing progress statistics.

## Features

- **Daily update tracking**: The bot tracks your activity in designated project channels
- **Ticket management**: Create and complete tickets for your projects
- **Progress visualization**: See progress bars and statistics for your projects
- **Check-in reminders**: Get notified when you haven't checked in for a while
- **Slash commands**: Easy-to-use Discord slash commands for interaction

## Setup

### Prerequisites

- Go 1.18 or higher
- A Discord bot token (create one at [Discord Developer Portal](https://discord.com/developers/applications))
- Bot permissions: Send Messages, Read Message History, Use Slash Commands

### Installation

1. Clone this repository
2. Create a `config.json` file based on the example provided
3. Replace `YOUR_DISCORD_BOT_TOKEN` with your actual Discord bot token
4. Build and run the bot:

```bash
go mod init accountability-bot
go get github.com/bwmarrin/discordgo
go build
./accountability-bot
```

## Configuration

Edit the `config.json` file:

```json
{
  "token": "YOUR_DISCORD_BOT_TOKEN",
  "trackedChannels": {
    "channel-id-1": "Project 1",
    "channel-id-2": "Project 2"
  },
  "reminderChannelID": "reminder-channel-id",
  "checkInFrequency": 24,
  "reminderTime": "09:00",
  "databasePath": "accountability_data.json"
}
```

- `token`: Your Discord bot token
- `trackedChannels`: Map of channel IDs to project names (you can also use the `/track` command to add channels)
- `reminderChannelID`: Channel ID where reminders will be sent
- `checkInFrequency`: How often (in hours) check-ins are expected
- `reminderTime`: When daily reminders are sent (24-hour format)
- `databasePath`: Where the bot stores its data

## Usage

### Slash Commands

- `/track <project-name>`: Start tracking the current channel for a project
- `/stats`: Show your project statistics
- `/progress`: Show progress bars for your projects

### Text Commands

In any tracked channel:

- `!ticket create <title> | <description>`: Create a new ticket
- `!ticket done <ticket-id>`: Mark a ticket as completed
- `!ticket list`: List all your tickets for the current project

### How it Works

1. Use `/track` in a channel to start tracking it for a project
2. Post daily updates in that channel to record check-ins
3. Create tickets using `!ticket create` when you start working on something
4. Mark tickets as done using `!ticket done` when you complete them
5. Check your progress with `/stats` and `/progress`
6. The bot will remind you if you haven't checked in for the configured time

## Example

```
# Start tracking a channel
/track My Awesome Project

# Check in by posting updates
Just started working on the new login feature!

# Create a ticket
!ticket create Implement login UI | Create the login form with username and password fields

# Mark a ticket as done
!ticket done 1

# See your progress
/progress

# View detailed stats
/stats
```

## Data Storage

The bot stores data in a JSON file as configured in `databasePath`. The data includes:
- Check-in timestamps
- Ticket information
- Project progress

## Customization

Feel free to modify the code to add additional features or customize existing ones!