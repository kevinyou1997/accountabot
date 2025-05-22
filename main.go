package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/bwmarrin/discordgo"
)

// Configuration structure
type Config struct {
	Token             string            `json:"token"`
	TrackedChannels   map[string]string `json:"trackedChannels"` // channelID -> projectName
	ReminderChannelID string            `json:"reminderChannelID"`
	CheckInFrequency  time.Duration     `json:"checkInFrequency"` // In hours
	ReminderTime      string            `json:"reminderTime"`     // Format: "15:04" (24h)
	DatabasePath      string            `json:"databasePath"`
}

// User activity tracking
type UserActivity struct {
	LastCheckIn time.Time         `json:"lastCheckIn"`
	CheckIns    []time.Time       `json:"checkIns"`
	Tickets     map[string]Ticket `json:"tickets"`
	ProjectName string            `json:"projectName"`
}

// Ticket structure
type Ticket struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Status      string    `json:"status"` // "open", "in_progress", "done"
	CreatedAt   time.Time `json:"createdAt"`
	CompletedAt time.Time `json:"completedAt"`
	ProjectName string    `json:"projectName"`
}

// Database structure
type Database struct {
	UserActivities map[string]map[string]UserActivity `json:"userActivities"` // userID -> channelID -> activity
	mutex          sync.RWMutex
}

var (
	config   Config
	database Database
)

func main() {
	// Load configuration
	configFile, err := os.ReadFile("config.json")
	if err != nil {
		log.Println("Warning: Could not read config file. Using default configuration.")
		config = Config{
			TrackedChannels:  make(map[string]string),
			CheckInFrequency: 24, // Default to daily check-ins
			ReminderTime:     "09:00",
			DatabasePath:     "accountability_data.json",
		}
	} else {
		err = json.Unmarshal(configFile, &config)
		if err != nil {
			log.Fatalf("Error parsing config file: %v", err)
		}
	}

	// Initialize database
	database.UserActivities = make(map[string]map[string]UserActivity)
	loadDatabase()

	// Create Discord session
	dg, err := discordgo.New("Bot " + config.Token)
	if err != nil {
		log.Fatalf("Error creating Discord session: %v", err)
	}

	// Register event handlers
	dg.AddHandler(messageCreate)
	dg.AddHandler(ready)

	// Register commands
	dg.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		if i.Type == discordgo.InteractionApplicationCommand {
			handleSlashCommand(s, i)
		}
	})

	// Open Discord session
	err = dg.Open()
	if err != nil {
		log.Fatalf("Error opening Discord connection: %v", err)
	}
	defer dg.Close()

	// Register slash commands
	registerCommands(dg)

	// Start reminder routine
	go reminderRoutine(dg)

	// Wait for a CTRL+C signal
	fmt.Println("Accountability bot is now running. Press CTRL+C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc

	// Save database before exiting
	saveDatabase()
}

func ready(s *discordgo.Session, event *discordgo.Ready) {
	log.Printf("Logged in as: %v#%v", s.State.User.Username, s.State.User.Discriminator)

	// Set the playing status
	err := s.UpdateGameStatus(0, "Tracking your progress!")
	if err != nil {
		log.Printf("Error setting status: %v", err)
	}
}

func messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	// Ignore bot's own messages
	if m.Author.ID == s.State.User.ID {
		return
	}

	// Check if the message is in a tracked channel
	if projectName, ok := config.TrackedChannels[m.ChannelID]; ok {
		// Record this check-in
		recordCheckIn(m.Author.ID, m.ChannelID, projectName)

		// Check if message contains ticket syntax
		if strings.HasPrefix(m.Content, "!ticket") {
			handleTicketCommand(s, m)
		}
	}
}

func handleTicketCommand(s *discordgo.Session, m *discordgo.MessageCreate) {
	parts := strings.SplitN(m.Content, " ", 3)
	if len(parts) < 2 {
		s.ChannelMessageSend(m.ChannelID, "Usage: !ticket create <title> | <description> or !ticket done <ticket-id>")
		return
	}

	action := parts[1]

	switch action {
	case "create":
		if len(parts) < 3 {
			s.ChannelMessageSend(m.ChannelID, "Usage: !ticket create <title> | <description>")
			return
		}

		// Split the title and description
		titleDesc := strings.SplitN(parts[2], "|", 2)
		title := strings.TrimSpace(titleDesc[0])
		description := ""
		if len(titleDesc) > 1 {
			description = strings.TrimSpace(titleDesc[1])
		}

		ticketID := createTicket(m.Author.ID, m.ChannelID, title, description)
		s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("✅ Created ticket **#%s**: %s", ticketID, title))

	case "done":
		if len(parts) < 3 {
			s.ChannelMessageSend(m.ChannelID, "Usage: !ticket done <ticket-id>")
			return
		}

		ticketID := strings.TrimSpace(parts[2])
		success := completeTicket(m.Author.ID, m.ChannelID, ticketID)

		if success {
			s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("🎉 Completed ticket **#%s**! Great job!", ticketID))
		} else {
			s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("❌ Could not find ticket **#%s**", ticketID))
		}

	case "list":
		tickets := listTickets(m.Author.ID, m.ChannelID)

		if len(tickets) == 0 {
			s.ChannelMessageSend(m.ChannelID, "No tickets found for this project")
			return
		}

		// Format tickets
		var message strings.Builder
		message.WriteString("**Your Tickets:**\n")

		for _, ticket := range tickets {
			status := "⏳ In Progress"
			if ticket.Status == "done" {
				status = "✅ Done"
			}

			message.WriteString(fmt.Sprintf("**#%s**: %s - %s\n", ticket.ID, ticket.Title, status))
		}

		s.ChannelMessageSend(m.ChannelID, message.String())

	default:
		s.ChannelMessageSend(m.ChannelID, "Unknown ticket command. Use: !ticket create, !ticket done, or !ticket list")
	}
}

func createTicket(userID, channelID, title, description string) string {
	database.mutex.Lock()
	defer database.mutex.Unlock()

	// Initialize user map if it doesn't exist
	if _, ok := database.UserActivities[userID]; !ok {
		database.UserActivities[userID] = make(map[string]UserActivity)
	}

	// Initialize channel activity if it doesn't exist
	if _, ok := database.UserActivities[userID][channelID]; !ok {
		projectName := config.TrackedChannels[channelID]
		database.UserActivities[userID][channelID] = UserActivity{
			LastCheckIn: time.Now(),
			CheckIns:    []time.Time{},
			Tickets:     make(map[string]Ticket),
			ProjectName: projectName,
		}
	}

	// Generate ticket ID (simple incrementing number)
	ticketID := fmt.Sprintf("%d", len(database.UserActivities[userID][channelID].Tickets)+1)

	// Create the ticket
	userActivity := database.UserActivities[userID][channelID]
	userActivity.Tickets[ticketID] = Ticket{
		ID:          ticketID,
		Title:       title,
		Description: description,
		Status:      "open",
		CreatedAt:   time.Now(),
		ProjectName: userActivity.ProjectName,
	}

	database.UserActivities[userID][channelID] = userActivity

	// Save to database
	saveDatabase()

	return ticketID
}

func completeTicket(userID, channelID, ticketID string) bool {
	database.mutex.Lock()
	defer database.mutex.Unlock()

	// Check if user exists
	userActivities, ok := database.UserActivities[userID]
	if !ok {
		return false
	}

	// Check if channel exists
	activity, ok := userActivities[channelID]
	if !ok {
		return false
	}

	// Check if ticket exists
	ticket, ok := activity.Tickets[ticketID]
	if !ok {
		return false
	}

	// Mark as done
	ticket.Status = "done"
	ticket.CompletedAt = time.Now()
	activity.Tickets[ticketID] = ticket

	database.UserActivities[userID][channelID] = activity

	// Save to database
	saveDatabase()

	return true
}

func listTickets(userID, channelID string) []Ticket {
	database.mutex.RLock()
	defer database.mutex.RUnlock()

	var tickets []Ticket

	// Check if user exists
	userActivities, ok := database.UserActivities[userID]
	if !ok {
		return tickets
	}

	// Check if channel exists
	activity, ok := userActivities[channelID]
	if !ok {
		return tickets
	}

	// Get all tickets
	for _, ticket := range activity.Tickets {
		tickets = append(tickets, ticket)
	}

	return tickets
}

func recordCheckIn(userID, channelID, projectName string) {
	database.mutex.Lock()
	defer database.mutex.Unlock()

	// Initialize user map if it doesn't exist
	if _, ok := database.UserActivities[userID]; !ok {
		database.UserActivities[userID] = make(map[string]UserActivity)
	}

	// Get or initialize user activity for this channel
	activity, ok := database.UserActivities[userID][channelID]
	if !ok {
		activity = UserActivity{
			CheckIns:    []time.Time{},
			Tickets:     make(map[string]Ticket),
			ProjectName: projectName,
		}
	}

	// Record check-in
	now := time.Now()
	activity.LastCheckIn = now
	activity.CheckIns = append(activity.CheckIns, now)

	// Update database
	database.UserActivities[userID][channelID] = activity

	// Save to database
	saveDatabase()
}

func saveDatabase() {
	// Create a temporary copy without the mutex
	dbCopy := struct {
		UserActivities map[string]map[string]UserActivity `json:"userActivities"`
	}{
		UserActivities: database.UserActivities,
	}

	data, err := json.MarshalIndent(dbCopy, "", "  ")
	if err != nil {
		log.Printf("Error marshaling database: %v", err)
		return
	}

	err = os.WriteFile(config.DatabasePath, data, 0644)
	if err != nil {
		log.Printf("Error writing database file: %v", err)
	}
}

func loadDatabase() {
	data, err := os.ReadFile(config.DatabasePath)
	if err != nil {
		if os.IsNotExist(err) {
			log.Println("No existing database found. Starting fresh.")
		} else {
			log.Printf("Error reading database file: %v", err)
		}
		return
	}

	var dbCopy struct {
		UserActivities map[string]map[string]UserActivity `json:"userActivities"`
	}

	err = json.Unmarshal(data, &dbCopy)
	if err != nil {
		log.Printf("Error unmarshaling database: %v", err)
		return
	}

	database.UserActivities = dbCopy.UserActivities
}

func reminderRoutine(s *discordgo.Session) {
	ticker := time.NewTicker(1 * time.Hour)

	for range ticker.C {
		now := time.Now()

		// Parse reminder time
		reminderTimeParts := strings.Split(config.ReminderTime, ":")
		if len(reminderTimeParts) != 2 {
			log.Println("Invalid reminder time format in config")
			continue
		}

		reminderHour := 9 // Default 9 AM
		reminderMinute := 0

		fmt.Sscanf(reminderTimeParts[0], "%d", &reminderHour)
		fmt.Sscanf(reminderTimeParts[1], "%d", &reminderMinute)

		// Check if it's reminder time
		if now.Hour() == reminderHour && now.Minute() < reminderMinute+5 && now.Minute() >= reminderMinute {
			sendDailyReminders(s)
		}
	}
}

func sendDailyReminders(s *discordgo.Session) {
	database.mutex.RLock()
	defer database.mutex.RUnlock()

	// Skip if reminder channel is not configured
	if config.ReminderChannelID == "" {
		return
	}

	now := time.Now()

	// Check all users
	for userID, userActivities := range database.UserActivities {
		for channelID, activity := range userActivities {
			// Check if user hasn't checked in within the frequency period
			if now.Sub(activity.LastCheckIn) > config.CheckInFrequency*time.Hour {
				// Send reminder
				mention := fmt.Sprintf("<@%s>", userID)
				message := fmt.Sprintf("%s, you haven't checked in on project **%s** for %d hours. Remember to update your progress!",
					mention,
					activity.ProjectName,
					int(now.Sub(activity.LastCheckIn).Hours()))

				s.ChannelMessageSend(config.ReminderChannelID, message)
			}
		}
	}
}

// Register slash commands
func registerCommands(s *discordgo.Session) {
	commands := []*discordgo.ApplicationCommand{
		{
			Name:        "track",
			Description: "Track a channel for project updates",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "project-name",
					Description: "The name of the project to track",
					Required:    true,
				},
			},
		},
		{
			Name:        "stats",
			Description: "Show your project stats",
		},
		{
			Name:        "progress",
			Description: "Show progress bar for completion of tickets",
		},
	}

	for _, command := range commands {
		_, err := s.ApplicationCommandCreate(s.State.User.ID, "", command)
		if err != nil {
			log.Printf("Error creating slash command %s: %v", command.Name, err)
		}
	}
}

// Handle slash commands
func handleSlashCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	switch i.ApplicationCommandData().Name {
	case "track":
		handleTrackCommand(s, i)
	case "stats":
		handleStatsCommand(s, i)
	case "progress":
		handleProgressCommand(s, i)
	}
}

func handleTrackCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	options := i.ApplicationCommandData().Options
	projectName := options[0].StringValue()

	// Add channel to tracked channels
	config.TrackedChannels[i.ChannelID] = projectName

	// Save config
	configData, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		log.Printf("Error marshaling config: %v", err)
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Error saving configuration",
			},
		})
		return
	}

	err = os.WriteFile("config.json", configData, 0644)
	if err != nil {
		log.Printf("Error writing config file: %v", err)
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Error saving configuration",
			},
		})
		return
	}

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: fmt.Sprintf("Now tracking this channel for project **%s**!\n\nUse this channel for daily updates, and I'll keep track of your progress.", projectName),
		},
	})
}

func handleStatsCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	userID := i.Member.User.ID

	database.mutex.RLock()
	defer database.mutex.RUnlock()

	// Check if user has any tracked projects
	userActivities, ok := database.UserActivities[userID]
	if !ok || len(userActivities) == 0 {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "You don't have any tracked projects yet. Use `/track` in a channel to start tracking.",
			},
		})
		return
	}

	// Build stats message
	var response strings.Builder
	response.WriteString("# Your Project Stats\n\n")

	for channelID, activity := range userActivities {
		// Get project stats
		totalTickets := len(activity.Tickets)
		completedTickets := 0

		for _, ticket := range activity.Tickets {
			if ticket.Status == "done" {
				completedTickets++
			}
		}

		// Calculate check-in streaks
		now := time.Now()
		lastCheckIn := activity.LastCheckIn
		daysSinceCheckIn := int(now.Sub(lastCheckIn).Hours() / 24)

		// Count check-ins in the last week
		checkInsLastWeek := 0
		weekAgo := now.AddDate(0, 0, -7)

		for _, checkIn := range activity.CheckIns {
			if checkIn.After(weekAgo) {
				checkInsLastWeek++
			}
		}

		// Format the stats
		response.WriteString(fmt.Sprintf("## %s\n", activity.ProjectName))
		response.WriteString(fmt.Sprintf("- **Tickets**: %d/%d completed (%.1f%%)\n",
			completedTickets, totalTickets,
			getPercentage(completedTickets, totalTickets)))
		response.WriteString(fmt.Sprintf("- **Last Check-in**: %d days ago\n", daysSinceCheckIn))
		response.WriteString(fmt.Sprintf("- **Check-ins Last Week**: %d\n\n", checkInsLastWeek))
	}

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: response.String(),
		},
	})
}

func handleProgressCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	userID := i.Member.User.ID

	database.mutex.RLock()
	defer database.mutex.RUnlock()

	// Check if user has any tracked projects
	userActivities, ok := database.UserActivities[userID]
	if !ok || len(userActivities) == 0 {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "You don't have any tracked projects yet. Use `/track` in a channel to start tracking.",
			},
		})
		return
	}

	// Build progress message
	var response strings.Builder
	response.WriteString("# Your Project Progress\n\n")

	for _, activity := range userActivities {
		// Get project stats
		totalTickets := len(activity.Tickets)
		completedTickets := 0

		for _, ticket := range activity.Tickets {
			if ticket.Status == "done" {
				completedTickets++
			}
		}

		// Skip projects with no tickets
		if totalTickets == 0 {
			continue
		}

		// Calculate percentage
		percentage := getPercentage(completedTickets, totalTickets)

		// Create progress bar (10 blocks)
		progressBar := createProgressBar(percentage)

		response.WriteString(fmt.Sprintf("## %s\n", activity.ProjectName))
		response.WriteString(fmt.Sprintf("%s %.1f%% (%d/%d)\n\n",
			progressBar, percentage, completedTickets, totalTickets))
	}

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: response.String(),
		},
	})
}

func getPercentage(completed, total int) float64 {
	if total == 0 {
		return 0.0
	}
	return float64(completed) / float64(total) * 100.0
}

func createProgressBar(percentage float64) string {
	const barLength = 10
	filledBlocks := int((percentage / 100.0) * float64(barLength))

	// Create the progress bar
	progressBar := "["

	for i := 0; i < barLength; i++ {
		if i < filledBlocks {
			progressBar += "█"
		} else {
			progressBar += "░"
		}
	}

	progressBar += "]"

	return progressBar
}
