package bot

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"github.com/bwmarrin/discordgo"
	"github.com/charmbracelet/log"
	"wherd.dev/chad/internal/openrouter"
)

var mentionRegex = regexp.MustCompile(`@(\w+)`)

func (b *Bot) engageWithMessage(s *discordgo.Session, m *discordgo.MessageCreate) {
	b.mutex.RLock()
	o := &openrouter.OpenRouter{
		Key:                  b.config.OpenRouter.Key,
		MaxMessagesInContext: b.config.OpenRouter.MaxMessagesInContext,
		Model:                b.config.OpenRouter.Model,
	}

	req := o.NewRequest()
	req.AddMessages(b.messageHistory[m.ChannelID])
	b.mutex.RUnlock()

	req.AddMessage("user", fmt.Sprintf(
		"Continue as Chad. Direct, concise, simple > complex.\n\nRespond with:\n- Full answer / short phrase / just emoji (👍 🤔 🚀)\n- Clarifying question if needed\n- Tag users with relevant experience\n\nCurrent %s message: %s\n\nDon't repeat previous points.",
		m.Author.Username,
		m.Content))

	response, err := o.Send(req)
	if err != nil {
		log.Errorf("Failed to send request: %v", err)
		return
	}

	if len(response.Choices) == 0 {
		log.Error("No choices in response")
		return
	}

	content := response.Choices[0].Message.Content

	// Check if response is an emoji or a message
	ch := []rune(content)[0]
	if unicode.IsLetter(ch) || unicode.IsDigit(ch) || unicode.IsPunct(ch) {
		// Check if response mentions users
		// if it does we need to convert the @username to <@userID>
		b.mutex.RLock()
		content = mentionRegex.ReplaceAllStringFunc(content, func(match string) string {
			username := strings.TrimPrefix(match, "@")
			if userID, ok := b.memberCache[username]; ok {
				return fmt.Sprintf("<@%s>", userID)
			}
			return match
		})
		b.mutex.RUnlock()

		if _, err := s.ChannelMessageSend(m.ChannelID, content); err != nil {
			log.Printf("Failed to send message: %v", err)
			return
		}
	} else {
		if err := s.MessageReactionAdd(m.ChannelID, m.ID, content); err != nil {
			log.Printf("Failed to add reaction: %v", err)
		}
	}
}
