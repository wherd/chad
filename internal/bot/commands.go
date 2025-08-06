package bot

import (
	"fmt"
	"math/rand/v2"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/charmbracelet/log"
	"wherd.dev/chad/internal/openrouter"
	"wherd.dev/chad/internal/websearch"
)

func (b *Bot) handleCommand(s *discordgo.Session, m *discordgo.MessageCreate) {
	if strings.HasPrefix(m.Content, "!help") {
		b.handleHelp(s, m)
		return
	}

	if strings.HasPrefix(m.Content, "!ask ") {
		b.handleAsk(s, m)
		return
	}

	if strings.HasPrefix(m.Content, "!flip") {
		b.handleCoinFlip(s, m)
		return
	}

	if strings.HasPrefix(m.Content, "!roll") {
		b.handleDiceRoll(s, m)
		return
	}

	if strings.HasPrefix(m.Content, "!remind") {
		b.handleRemind(s, m)
		return
	}

	if strings.HasPrefix(m.Content, "!factcheck ") {
		b.handleFactcheck(s, m)
		return
	}
}

func (b *Bot) handleAsk(s *discordgo.Session, m *discordgo.MessageCreate) {
	m.Content = strings.TrimPrefix(m.Content, "!ask ")

	if len(m.Content) == 0 {
		s.ChannelMessageSend(m.ChannelID, "Usage: `!ask <your question>` (eg. !ask What is the meaning of life?)")
		return
	}

	res, err := s.ChannelMessageSend(m.ChannelID, "💭 Thinking...")

	b.mutex.RLock()
	o := &openrouter.OpenRouter{
		Key:                  b.config.OpenRouter.Key,
		MaxMessagesInContext: b.config.OpenRouter.MaxMessagesInContext,
		Model:                b.config.OpenRouter.Model,
	}

	req := o.NewRequest()
	req.AddMessages(b.messageHistory[m.ChannelID])
	b.mutex.RUnlock()

	response, err := o.Send(req)
	if err != nil {
		log.Errorf("Failed to send request: %v", err)
		if err = maybeEditMessage(s, m.ChannelID, res, "❌ Sorry I'm unable to think right now.", nil); err != nil {
			log.Errorf("Failed to send error message: %v", err)
		}
		return
	}

	if len(response.Choices) == 0 {
		log.Error("No choices in response")
		if err = maybeEditMessage(s, m.ChannelID, res, "❌ Sorry I'm unable to think right now.", nil); err != nil {
			log.Errorf("Failed to send error message: %v", err)
		}
		return
	}

	content := response.Choices[0].Message.Content

	b.mutex.RLock()
	content = mentionRegex.ReplaceAllStringFunc(content, func(match string) string {
		username := strings.TrimPrefix(match, "@")
		if userID, ok := b.memberCache[username]; ok {
			return fmt.Sprintf("<@%s>", userID)
		}
		return match
	})
	b.mutex.RUnlock()

	if err = maybeEditMessage(s, m.ChannelID, res, content, nil); err != nil {
		log.Errorf("Failed to send message: %v", err)
	}
}

func (b *Bot) handleFactcheck(s *discordgo.Session, m *discordgo.MessageCreate) {
	m.Content = strings.TrimPrefix(m.Content, "!factcheck ")

	if len(m.Content) == 0 {
		b.session.ChannelMessageSendEmbed(m.ChannelID, &discordgo.MessageEmbed{
			Title:       "🔍 Factcheck Command Usage",
			Description: "Verify claims with web search!",
			Color:       0x3498db,
			Fields: []*discordgo.MessageEmbedField{
				{Name: "Usage", Value: "`!factcheck <claim>`", Inline: false},
				{Name: "Example", Value: "`!factcheck The moon is made of cheese`", Inline: false},
			},
		})
		return
	}

	res, err := s.ChannelMessageSend(m.ChannelID, "💭 Thinking...")

	b.mutex.RLock()
	searchResults, err := websearch.Search(b.config.SearchApiKey, "fact check "+m.Content)
	b.mutex.RUnlock()

	if err != nil {
		log.Printf("Web search error: %v", err)
		if err = maybeEditMessage(s, m.ChannelID, res, "❌ Failed to search for information. Please try again later.", nil); err != nil {
			log.Errorf("Failed to send error message: %v", err)
		}
		return
	}

	searchContext := &strings.Builder{}
	for _, result := range searchResults {
		searchContext.WriteString(fmt.Sprintf("Title: %s, URL: %s, Content: %s\n", result.Title, result.URL, result.Description))
	}

	prompt := fmt.Sprintf(`Fact-check this claim using the search results below.

CLAIM: "%s"

SEARCH RESULTS:
%s

Respond with:

VERDICT: [True/False/Partially True/Unclear] - one sentence why

EVIDENCE: 
- Supporting: [specific quotes/data from sources]
- Contradicting: [specific quotes/data from sources]

CONTEXT: [missing context that changes the claim's validity]

CONFIDENCE: [High/Medium/Low] based on source quality and consensus

Rules:
- Quote exact evidence, don't paraphrase
- Name the source for each piece of evidence
- If sources conflict, show both sides
- "Unclear" if evidence is insufficient
- Skip sections if not applicable (e.g., no contradicting evidence)`, m.Content, searchContext)

	b.mutex.RLock()
	o := &openrouter.OpenRouter{
		Key:                  b.config.OpenRouter.Key,
		MaxMessagesInContext: b.config.OpenRouter.MaxMessagesInContext,
		Model:                b.config.OpenRouter.Model,
	}

	req := o.NewRequest()
	req.AddMessage("user", prompt)
	b.mutex.RUnlock()

	response, err := o.Send(req)
	if err != nil || len(response.Choices) == 0 {
		log.Errorf("Fact-check AI error: %v", err)
		if err = maybeEditMessage(s, m.ChannelID, res, "❌ Failed to analyze the fact-check. Please try again later.", nil); err != nil {
			log.Errorf("Failed to send error message: %v", err)
		}
		return
	}

	content := response.Choices[0].Message.Content

	// Determine verdict color
	verdictColor := 0x95a5a6 // Gray default
	if strings.Contains(strings.ToLower(content), "verdict: true") {
		verdictColor = 0x27ae60 // Green
	} else if strings.Contains(strings.ToLower(content), "verdict: false") {
		verdictColor = 0xe74c3c // Red
	} else if strings.Contains(strings.ToLower(content), "partially true") {
		verdictColor = 0xf39c12 // Orange
	}

	// Build source list from search results
	sources := ""
	for i, result := range searchResults {
		if i >= 3 { // Limit to top 3 sources
			break
		}
		sources += fmt.Sprintf("• [%s](%s)\n", result.Title, result.URL)
	}

	embed := &discordgo.MessageEmbed{
		Title:       "🔍 Fact Check Analysis",
		Description: fmt.Sprintf("**Claim:** %s\n\n%s", m.Content, content),
		Color:       verdictColor,
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:   "📚 Sources Checked",
				Value:  sources,
				Inline: false,
			},
		},
		Footer: &discordgo.MessageEmbedFooter{
			Text: fmt.Sprintf("Fact-checked by %s • Always verify with multiple sources", m.Author.Username),
		},
		Timestamp: time.Now().Format(time.RFC3339),
	}

	if err = maybeEditMessage(s, m.ChannelID, res, "", embed); err != nil {
		log.Errorf("Failed to send error message: %v", err)
	}
}

func (b *Bot) handleCoinFlip(s *discordgo.Session, m *discordgo.MessageCreate) {
	result := "🪙 Heads"
	if rand.Float32() < 0.5 {
		result = "🎯 Tails"
	}

	s.ChannelMessageSend(m.ChannelID, result)
}

func (b *Bot) handleDiceRoll(s *discordgo.Session, m *discordgo.MessageCreate) {
	m.Content = strings.TrimPrefix(m.Content, "!roll ")

	sides := 6
	count := 1

	if len(m.Content) > 0 {
		arg := m.Content
		if strings.Contains(arg, "d") {
			parts := strings.Split(arg, "d")
			if len(parts) == 2 {
				if c, err := strconv.Atoi(parts[0]); err == nil && c <= 10 {
					count = c
				}
				if s, err := strconv.Atoi(parts[1]); err == nil && s <= 100 {
					sides = s
				}
			}
		} else if s, err := strconv.Atoi(arg); err == nil && s <= 100 {
			sides = s
		}
	}

	var rolls []int
	total := 0

	for i := 0; i < count; i++ {
		roll := rand.IntN(sides) + 1
		rolls = append(rolls, roll)
		total += roll
	}

	rollsStr := make([]string, len(rolls))
	for i, roll := range rolls {
		rollsStr[i] = strconv.Itoa(roll)
	}

	s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("🎲 Rolled %dd%d: %s (Total: %d)", count, sides, strings.Join(rollsStr, ", "), total))
}

func (b *Bot) handleRemind(s *discordgo.Session, m *discordgo.MessageCreate) {
	args := strings.Split(m.Content, " ")
	if len(args) < 3 {
		s.ChannelMessageSend(m.ChannelID, "Usage: `!remind 5m Take a break` or `!remind 2h Meeting with team`")
		return
	}

	duration, err := time.ParseDuration(args[1])
	if err != nil || duration.Minutes() < 1 {
		s.ChannelMessageSend(m.ChannelID, "Invalid time format. Use: 5m, 2h, 1d (minutes, hours, days)")
		return
	}

	reminderText := strings.Join(args[2:], " ")
	reminderTime := time.Now().Add(duration)

	reminder := &Reminder{
		Message:   reminderText,
		Time:      reminderTime.Unix(),
		ChannelID: m.ChannelID,
		UserID:    m.Author.ID,
	}

	b.mutex.Lock()
	b.reminders = append(b.reminders, reminder)
	b.mutex.Unlock()

	s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("<@!%s> I'll remind you in %s about: \"%s\"", m.Author.ID, args[1], reminderText))
}

func (b *Bot) handleHelp(s *discordgo.Session, m *discordgo.MessageCreate) {
	help := `I can help you with the following:

  **AI & Knowledge**
  !ask <question> - Ask the AI
  !factcheck <claim> - Verify claims with web search

  **Utilities**  
  !remind 5m <message> - Set reminder

  **Fun & Social**
  !flip - Flip a coin
  !roll [dice] - Roll dice (eg. 2d6 or 20)

  You can also mention me to get my attention.`

	s.ChannelMessageSend(m.ChannelID, help)
}

func maybeEditMessage(s *discordgo.Session, channelID string, m *discordgo.Message, content string, embed *discordgo.MessageEmbed) error {
	var err error
	if m != nil {
		if embed != nil {
			_, err = s.ChannelMessageEditEmbed(m.ChannelID, m.ID, embed)
		} else {
			_, err = s.ChannelMessageEdit(m.ChannelID, m.ID, content)
		}
	} else if embed != nil {
		_, err = s.ChannelMessageSendEmbed(channelID, embed)
	} else {
		_, err = s.ChannelMessageSend(channelID, content)
	}

	return err
}
