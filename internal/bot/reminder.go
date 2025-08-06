package bot

import (
	"fmt"
	"time"

	"github.com/charmbracelet/log"
)

type Reminder struct {
	ID        string `json:"id"`
	ChannelID string `json:"channel_id"`
	UserID    string `json:"user_id"`
	Message   string `json:"message"`
	Time      int64  `json:"time"`
}

func (b *Bot) scheduleReminder(reminder *Reminder) {
	duration := time.Until(time.Unix(reminder.Time, 0))
	if duration <= 0 {
		// Reminder is already due, send immediately
		b.sendReminder(reminder)
		return
	}

	timer := time.AfterFunc(duration, func() {
		b.sendReminder(reminder)
		b.removeReminder(reminder.ID)
	})

	b.reminderTimers[reminder.ID] = timer
}

func (b *Bot) removeReminder(reminderID string) {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	delete(b.reminderTimers, reminderID)
	for i, r := range b.reminders {
		if r.ID == reminderID {
			b.reminders = append(b.reminders[:i], b.reminders[i+1:]...)
			break
		}
	}
}

func (b *Bot) sendReminder(reminder *Reminder) {
	content := fmt.Sprintf("<@%s> You asked me to remind you about this: %s", reminder.UserID, reminder.Message)
	if _, err := b.session.ChannelMessageSend(reminder.ChannelID, content); err != nil {
		log.Errorf("Failed to send reminder to channel %s: %v", reminder.ChannelID, err)
	}
}

func (b *Bot) initializeReminders() {
	b.mutex.Lock()
	for _, reminder := range b.reminders {
		if reminder.Time > time.Now().Unix() {
			b.scheduleReminder(reminder)
		}
	}
	b.mutex.Unlock()
}

func (b *Bot) shutdownReminders() {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	// Cancel all active timers
	for _, timer := range b.reminderTimers {
		timer.Stop()
	}
	b.reminderTimers = make(map[string]*time.Timer)
	log.Info("All reminder timers stopped")
}
