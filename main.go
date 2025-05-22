package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/bwmarrin/discordgo"
)

// Configuration structure
type Config struct {
	Token            string `json:"token"`
	StudyChannelID   string `json:"studyChannelID"`
	DatabasePath     string `json:"databasePath"`
	ReminderTime     string `json:"reminderTime"`     // Format: "15:04" (24h)
	CheckInFrequency int    `json:"checkInFrequency"` // In hours
}

// User activity tracking
type UserActivity struct {
	UserID      string      `json:"userID"`
	Username    string      `json:"username"`
	LastCheckIn time.Time   `json:"lastCheckIn"`
	CheckIns    []time.Time `json:"checkIns"`
}

// Database structure
type Database struct {
	UserActivities map[string]UserActivity `json:"userActivities"` // userID -> activity
}

var (
	config   Config
	database Database
)

func main() {
	// Load configuration
	configFile, err := os.ReadFile("config.json")
	if err != nil {
		log.Fatalf("Error reading config file: %v", err)
	}

	err = json.Unmarshal(configFile, &config)
	if err != nil {
		log.Fatalf("Error parsing config file: %v", err)
	}

	// Validate required config
	if config.Token == "" {
		log.Fatalf("Discord token is required in config.json")
	}
	if config.StudyChannelID == "" {
		log.Fatalf("Study channel ID is required in config.json")
	}

	// Set defaults
	if config.DatabasePath == "" {
		config.DatabasePath = "study_data.json"
	}
	if config.ReminderTime == "" {
		config.ReminderTime = "09:00"
	}
	if config.CheckInFrequency == 0 {
		config.CheckInFrequency = 24
	}

	// Initialize database
	database.UserActivities = make(map[string]UserActivity)
	loadDatabase()

	// Create Discord session
	dg, err := discordgo.New("Bot " + config.Token)
	if err != nil {
		log.Fatalf("Error creating Discord session: %v", err)
	}

	// Register event handlers
	dg.AddHandler(messageCreate)
	dg.AddHandler(ready)

	// Open Discord session
	err = dg.Open()
	if err != nil {
		log.Fatalf("Error opening Discord connection: %v", err)
	}
	defer dg.Close()

	// Start reminder routine
	go reminderRoutine(dg)

	// Wait for a CTRL+C signal
	fmt.Println("Study accountability bot is now running. Press CTRL+C to exit.")
	fmt.Printf("Monitoring channel ID: %s\n", config.StudyChannelID)
	fmt.Println("Tracking user: kevin.you")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc

	// Save database before exiting
	saveDatabase()
	fmt.Println("Bot shutting down...")
}

func ready(s *discordgo.Session, event *discordgo.Ready) {
	log.Printf("Logged in as: %v#%v", s.State.User.Username, s.State.User.Discriminator)

	// Set the playing status
	err := s.UpdateGameStatus(0, "Tracking kevin.you's study progress!")
	if err != nil {
		log.Printf("Error setting status: %v", err)
	}
}

func messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	// Ignore bot's own messages
	if m.Author.ID == s.State.User.ID {
		return
	}

	// Only track messages from user "kevin.you" in the studying-updates channel
	if m.ChannelID == config.StudyChannelID && m.Author.Username == "kevin.you" {
		// Record this check-in
		recordCheckIn(m.Author.ID, m.Author.Username)

		// Send a quick acknowledgment (optional)
		s.MessageReactionAdd(m.ChannelID, m.ID, "✅")

		log.Printf("Check-in recorded for %s (%s)", m.Author.Username, m.Author.ID)
	}
}

func recordCheckIn(userID, username string) {
	// Get or create user activity
	activity, exists := database.UserActivities[userID]
	if !exists {
		activity = UserActivity{
			UserID:   userID,
			Username: username,
			CheckIns: []time.Time{},
		}
	}

	// Update username in case it changed
	activity.Username = username

	// Record check-in
	now := time.Now()
	activity.LastCheckIn = now
	activity.CheckIns = append(activity.CheckIns, now)

	// Keep only the last 30 check-ins to prevent unlimited growth
	if len(activity.CheckIns) > 30 {
		activity.CheckIns = activity.CheckIns[len(activity.CheckIns)-30:]
	}

	// Update database
	database.UserActivities[userID] = activity

	// Save to database
	saveDatabase()
}

func saveDatabase() {
	data, err := json.MarshalIndent(database, "", "  ")
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

	err = json.Unmarshal(data, &database)
	if err != nil {
		log.Printf("Error unmarshaling database: %v", err)
		return
	}

	log.Printf("Loaded %d user activities from database", len(database.UserActivities))
}

func reminderRoutine(s *discordgo.Session) {
	// Check every hour
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		checkAndSendReminders(s)
	}
}

func checkAndSendReminders(s *discordgo.Session) {
	now := time.Now()

	// Parse reminder time (e.g., "09:00")
	reminderHour := 9
	reminderMinute := 0

	_, err := fmt.Sscanf(config.ReminderTime, "%d:%d", &reminderHour, &reminderMinute)
	if err != nil {
		log.Printf("Error parsing reminder time: %v", err)
		return
	}

	// Check if it's the right time to send reminders (within 5 minutes of target time)
	if now.Hour() != reminderHour || now.Minute() < reminderMinute || now.Minute() > reminderMinute+5 {
		return
	}

	// Check all users for overdue check-ins
	for userID, activity := range database.UserActivities {
		hoursSinceLastCheckIn := now.Sub(activity.LastCheckIn).Hours()

		// If user hasn't checked in within the frequency period
		if hoursSinceLastCheckIn > float64(config.CheckInFrequency) {
			sendReminder(s, userID, activity.Username, int(hoursSinceLastCheckIn))
		}
	}
}

func sendReminder(s *discordgo.Session, userID, username string, hoursSinceLastCheckIn int) {
	// Send reminder in the study channel
	message := fmt.Sprintf("📚 Hey <@%s>! It's been %d hours since your last study check-in. How's your progress going today?", userID, hoursSinceLastCheckIn)

	_, err := s.ChannelMessageSend(config.StudyChannelID, message)
	if err != nil {
		log.Printf("Error sending reminder to %s: %v", username, err)
	} else {
		log.Printf("Sent reminder to %s (%d hours overdue)", username, hoursSinceLastCheckIn)
	}
}
