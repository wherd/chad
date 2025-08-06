package bot

import (
	"encoding/json"
	"os"
	"time"

	"github.com/charmbracelet/log"
)

// The version of the data format. If this changes, the data is considered incompatible and a new file is created.
const dataVersion = "1.0"

type Settings struct {
	Timestamp       int64       `json:"timestamp"`
	Version         string      `json:"version"`
	Reminders       []*Reminder `json:"reminders"`
	ReminderCounter int64       `json:"reminder_counter"`
}

func (b *Bot) saveSettings() error {
	b.mutex.RLock()
	settings := Settings{
		Timestamp:       time.Now().Unix(),
		Version:         dataVersion,
		Reminders:       b.reminders,
		ReminderCounter: b.reminderCounter,
	}
	b.mutex.RUnlock()

	jsondata, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}

	tempFile := "chad_memory.json.tmp"
	if err := os.WriteFile(tempFile, jsondata, 0644); err != nil {
		return err
	}

	if err := os.Rename(tempFile, "chad_memory.json"); err != nil {
		os.Remove(tempFile)
		return err
	}

	return nil
}

func (b *Bot) loadSettings() error {
	data := Settings{}

	jsondata, err := os.ReadFile("chad_memory.json")
	if err != nil {
		return err
	}

	if err := json.Unmarshal(jsondata, &data); err != nil {
		return err
	}

	if time.Now().Unix()-data.Timestamp > 60*60*24*7 {
		log.Warnf("Data is too old, starting fresh")
		return nil
	}

	if data.Version != dataVersion {
		log.Warnf("Data version mismatch, starting fresh")
		return nil
	}

	b.mutex.Lock()
	b.reminders = data.Reminders
	b.reminderCounter = data.ReminderCounter
	b.mutex.Unlock()

	log.Debugf("Loaded data from %s (version %s)", time.Unix(data.Timestamp, 0).Format("2006-01-02 15:04:05"), data.Version)
	return nil
}
