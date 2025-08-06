package bot

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"github.com/bwmarrin/discordgo"
	"github.com/charmbracelet/log"
	"wherd.dev/chad/internal/openrouter"
	"wherd.dev/chad/internal/websearch"
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

func (b *Bot) engageFromMention(s *discordgo.Session, m *discordgo.MessageCreate) {
	msg, _ := s.ChannelMessageSend(m.ChannelID, "💭 Thinking...")

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
		"Chad - you were mentioned. Reply as needed.\n\nOptions: answer / question / emoji / tag others\nSimple > complex\n\n%s said: %s",
		m.Author.Username,
		m.Content))

	response, err := o.Send(req)
	if err != nil {
		maybeEditMessage(s, m.ChannelID, msg, "Sorry I'm unable to think right now.", nil)
		log.Errorf("Failed to send request: %v", err)
		return
	}

	if len(response.Choices) == 0 {
		maybeEditMessage(s, m.ChannelID, msg, "Sorry I'm unable to think right now.", nil)
		log.Error("No choices in response")
		return
	}

	if len(response.Choices[0].Message.TollCalls) > 0 {
		for _, toolCall := range response.Choices[0].Message.TollCalls {
			req.Messages = append(req.Messages, &openrouter.Message{
				Role:      "assistant",
				TollCalls: []openrouter.ToolCall{toolCall},
			})

			if message, err := b.processToolCall(&toolCall); err == nil {
				req.Messages = append(req.Messages, message)
			} else {
				log.Errorf("Failed to process tool call: %v", err)
			}
		}

		response, err = o.Send(req)
		if err != nil {
			maybeEditMessage(s, m.ChannelID, msg, "Sorry I'm unable to think right now.", nil)
			log.Errorf("Failed to send request: %v", err)
			return
		}
	}

	content := response.Choices[0].Message.Content

	// Check if response is an emoji or a message
	ch := []rune(content)[0]
	if unicode.IsLetter(ch) || unicode.IsDigit(ch) || unicode.IsPunct(ch) || len(content) > 2 {
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

		if err := maybeEditMessage(s, m.ChannelID, msg, content, nil); err != nil {
			log.Printf("Failed to send message: %v", err)
			return
		}
	} else {
		s.ChannelMessageDelete(m.ChannelID, msg.ID)
		if err := s.MessageReactionAdd(m.ChannelID, m.ID, content); err != nil {
			log.Printf("Failed to add reaction: %v", err)
		}
	}
}

func (b *Bot) processToolCall(toolCalls *openrouter.ToolCall) (*openrouter.Message, error) {
	if toolCalls.Function.Name == "search" {
		b.mutex.RLock()
		apikey := b.config.SearchApiKey
		b.mutex.RUnlock()

		var args struct {
			Query string `json:"query"`
		}

		if err := json.Unmarshal([]byte(toolCalls.Function.Arguments), &args); err != nil {
			return nil, err
		}

		searchResults, err := websearch.Search(apikey, args.Query)
		if err != nil {
			return nil, err
		}

		content, err := json.Marshal(searchResults)
		if err != nil {
			return nil, err
		}

		message := &openrouter.Message{
			Role:       "tool",
			ToolCallID: toolCalls.ID,
			Name:       "search",
			Content:    string(content),
		}

		return message, nil
	}

	return nil, fmt.Errorf("unknown function: %s", toolCalls.Function.Name)
}
