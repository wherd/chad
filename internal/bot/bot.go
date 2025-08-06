package bot

import (
	"context"
	"fmt"
	"math/rand/v2"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/charmbracelet/log"
	"wherd.dev/chad/internal/config"
	"wherd.dev/chad/internal/openrouter"
)

type Bot struct {
	config   *config.Config
	settings *Settings

	session *discordgo.Session

	ctx    context.Context
	cancel context.CancelFunc

	mutex          sync.RWMutex
	rateLimits     map[string][]int64
	memberCache    map[string]string
	messageHistory map[string][]*openrouter.Message
	reminders      []*Reminder
}

type Reminder struct {
	ChannelID string `json:"channel_id"`
	UserID    string `json:"user_id"`
	Message   string `json:"message"`
	Time      int64  `json:"time"`
}

func New(config *config.Config) *Bot {
	ctx, cancel := context.WithCancel(context.Background())
	return &Bot{
		config:   config,
		settings: &Settings{},

		ctx:    ctx,
		cancel: cancel,

		mutex:          sync.RWMutex{},
		rateLimits:     map[string][]int64{},
		memberCache:    map[string]string{},
		messageHistory: map[string][]*openrouter.Message{},
		reminders:      []*Reminder{},
	}
}

func (b *Bot) Run() error {
	if b.config.DiscordToken == "" {
		return fmt.Errorf("Discord token is not set")
	}

	if b.config.OpenRouter.Key == "" {
		return fmt.Errorf("OpenRouter key is not set")
	}

	var err error
	if b.session, err = discordgo.New("Bot " + b.config.DiscordToken); err != nil {
		return err
	}

	b.session.AddHandler(b.ready)
	b.session.AddHandler(b.guildCreate)
	b.session.AddHandler(b.guildDelete)
	b.session.AddHandler(b.memberJoin)
	b.session.AddHandler(b.memberUpdate)
	b.session.AddHandler(b.memberLeave)
	b.session.AddHandler(b.messageCreate)

	b.session.Identify.Intents = discordgo.IntentsGuildMessages |
		discordgo.IntentsMessageContent |
		discordgo.IntentsGuilds |
		discordgo.IntentsGuildMembers

	if err = b.session.Open(); err != nil {
		return err
	}

	// load existing data
	if err = b.loadSettings(); err != nil {
		log.Warnf("Could not load existing data: %v", err)
	}

	go b.reminderChecker()
	// go b.cleanupTasks()
	go b.autoSaveData()

	log.Infof("Bot is running! Press Ctrl+C to exit")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc

	// Gracefully shutdown
	b.shutdown()
	return b.session.Close()
}

func (b *Bot) ready(s *discordgo.Session, event *discordgo.Ready) {
	log.Infof("Bot logged in as %s", event.User.String())
	log.Infof("Bot is in %d servers", len(s.State.Guilds))

	// Set bot status
	s.UpdateGameStatus(0, "!help for commands")
}

func (b *Bot) guildCreate(s *discordgo.Session, event *discordgo.GuildCreate) {
	log.Infof("Joined server: %s (%d members)", event.Name, event.MemberCount)

	b.mutex.Lock()

	// Cache members
	for _, member := range event.Guild.Members {
		b.memberCache[member.User.Username] = member.User.ID
		if member.Nick != "" {
			b.memberCache[member.Nick] = member.User.ID
		}
	}

	b.mutex.Unlock()
}

func (b *Bot) guildDelete(s *discordgo.Session, event *discordgo.GuildDelete) {
	log.Infof("Left server: %s", event.Name)

	// Trigger immediate save after leaving a server
	if err := b.saveSettings(); err != nil {
		log.Printf("Failed to save data after leaving server: %v", err)
	}
}

func (b *Bot) memberJoin(s *discordgo.Session, event *discordgo.GuildMemberAdd) {
	log.Infof("New member joined: %s", event.Member.User.String())

	// Cache member username and nickname
	b.mutex.Lock()
	b.memberCache[event.Member.User.Username] = event.Member.User.ID
	if event.Member.Nick != "" {
		b.memberCache[event.Member.Nick] = event.Member.User.ID
	}
	b.mutex.Unlock()
}

func (b *Bot) memberUpdate(s *discordgo.Session, event *discordgo.GuildMemberUpdate) {
	log.Infof("Member updated: %s", event.User.String())
	b.mutex.Lock()

	if event.BeforeUpdate != nil {
		delete(b.memberCache, event.BeforeUpdate.User.Username)
		if event.BeforeUpdate.Nick != "" {
			delete(b.memberCache, event.BeforeUpdate.Nick)
		}
	}

	b.memberCache[event.Member.User.Username] = event.Member.User.ID
	if event.Member.Nick != "" {
		b.memberCache[event.Member.Nick] = event.Member.User.ID
	}

	b.mutex.Unlock()
}

func (b *Bot) memberLeave(s *discordgo.Session, event *discordgo.GuildMemberRemove) {
	log.Infof("Member left: %s", event.User.String())

	b.mutex.Lock()
	delete(b.memberCache, event.User.Username)
	if event.Member != nil && event.Member.Nick != "" {
		delete(b.memberCache, event.Member.Nick)
	}
	b.mutex.Unlock()
}

func (b *Bot) messageCreate(s *discordgo.Session, event *discordgo.MessageCreate) {
	if event.Author.Bot {
		return
	}

	// Rate limiting check
	if b.isRateLimited(event.Author.ID) {
		if err := s.MessageReactionAdd(event.ChannelID, event.ID, "⏰"); err != nil {
			log.Printf("Failed to add warning reaction: %v", err)
		}

		timeoutUntil := time.Now().Add(time.Duration(b.config.RateLimit.MuteTime))
		err := s.GuildMemberTimeout(event.GuildID, event.Author.ID, &timeoutUntil)
		if err != nil {
			log.Errorf("Failed to timeout user %s: %v", event.Author.Username, err)
		}

		return
	}

	b.storeMessageForContext(event.ChannelID, &openrouter.Message{
		Role:    "user",
		Content: fmt.Sprintf("%s: %s", event.Author.Username, event.Content),
	})

	// Check if message starts with bot prefix
	if strings.HasPrefix(event.Content, b.config.Prefix) {
		b.handleCommand(s, event)
		return
	}

	// Check if message mensions the bot
	// if slices.Contains(event.Mentions, b.session.State.User) {
	// 	// TODO: Handle bot mentions
	// 	return
	// }

	// Randomly respond to messages
	if rand.Float32() < 0.1 {
		b.engageWithMessage(s, event)
	}
}

func (b *Bot) isRateLimited(userID string) bool {
	now := time.Now().Unix()

	b.mutex.Lock()
	defer b.mutex.Unlock()

	if timestamps, ok := b.rateLimits[userID]; ok {
		for i, t := range timestamps {
			if t < now {
				timestamps[i] = now + b.config.RateLimit.Window
				return false
			}
		}
	} else {
		b.rateLimits[userID] = make([]int64, b.config.RateLimit.MaxRequests)
		b.rateLimits[userID][0] = now + b.config.RateLimit.Window
		return false
	}

	return true
}

func (b *Bot) storeMessageForContext(channelID string, message *openrouter.Message) {
	b.mutex.Lock()

	if _, ok := b.messageHistory[channelID]; !ok {
		b.messageHistory[channelID] = make([]*openrouter.Message, 0, b.config.OpenRouter.MaxMessagesInContext)
	}

	history := b.messageHistory[channelID]
	history = append(history, message)

	// Keep only last {MaxMessagesInContext} messages
	if len(history) > b.config.OpenRouter.MaxMessagesInContext {
		history = history[len(history)-b.config.OpenRouter.MaxMessagesInContext:]
	}

	b.messageHistory[channelID] = history
	b.mutex.Unlock()
}

func (b *Bot) reminderChecker() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		now := time.Now().Unix()

		b.mutex.Lock()
		for i := len(b.reminders) - 1; i >= 0; i-- {
			reminder := b.reminders[i]
			if reminder.Time <= now {
				content := fmt.Sprintf("<@%s> You asked me to remind you about this: %s", reminder.UserID, reminder.Message)
				b.session.ChannelMessageSend(reminder.ChannelID, content)
				b.reminders = append(b.reminders[:i], b.reminders[i+1:]...)
			}
		}
		b.mutex.Unlock()
	}
}

func (b *Bot) autoSaveData() {
	ticker := time.NewTicker(time.Duration(b.config.AutoSaveInterval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := b.saveSettings(); err != nil {
				log.Errorf("Auto-save failed: %v", err)
			}
		case <-b.ctx.Done():
			log.Info("Shutting down auto-save routine")
			return
		}
	}
}

func (b *Bot) shutdown() {
	log.Print("Initiating shutdown...")

	// Cancel background tasks
	b.cancel()

	// Set status to offline
	if b.session != nil {
		if err := b.session.UpdateGameStatus(0, "Shutting down..."); err != nil {
			log.Errorf("Failed to update game status during shutdown: %v", err)
		}
		time.Sleep(1 * time.Second) // Give time for status to update
	}

	// Save data before exiting
	if err := b.saveSettings(); err != nil {
		log.Errorf("Failed to save data during shutdown: %v", err)
	}

	log.Print("Shutdown complete")
}
