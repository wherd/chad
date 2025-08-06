package openrouter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type OpenRouter struct {
	Key                  string  `json:"key"`
	SystemPrompt         string  `json:"system_prompt"`
	Temperature          float64 `json:"temperature"`
	MaxTokens            int     `json:"max_tokens"`
	MaxMessagesInContext int     `json:"max_messages_in_context"`
	Model                string  `json:"model"`
}

type Request struct {
	Messages       []*Message `json:"messages,omitempty"`
	Prompt         string     `json:"prompt,omitempty"`
	Model          string     `json:"model,omitempty"`           // See "Supported Models" section
	ResponseFormat string     `json:"response_format,omitempty"` // response_format?: { type: 'json_object' };
	MaxTokens      int        `json:"max_tokens,omitempty"`      // Range: [1, context_length)
	Temperature    float64    `json:"temperature,omitempty"`     // Range: [0, 2]
	Tools          []Tool     `json:"tools,omitempty"`           // tools?: Tool[];
}

type Message struct {
	Role       string     `json:"role"` // role: 'user' | 'assistant' | 'system';
	Name       string     `json:"name,omitempty"`
	Content    string     `json:"content"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	TollCalls  []ToolCall `json:"tool_calls,omitempty"`
}

type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function FunctionCall `json:"function"`
}

type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type Tool struct {
	Type     string   `json:"type"`
	Fucntion Function `json:"function"`
}

type Function struct {
	Name        string                `json:"name"`
	Description string                `json:"description"`
	Parameters  map[string]Parameters `json:"parameters"`
}

type Parameters struct {
	Type       string          `json:"type"`
	Properties map[string]Type `json:"properties"`
	Required   []string        `json:"required,omitempty"`
}

type Type struct {
	Type string `json:"type"`
}

type Response struct {
	Choices []*Choice `json:"choices,omitempty"`
	Error   *Error    `json:"error,omitempty"`
}

type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type Choice struct {
	Message Message `json:"message"`
}

func New(key string, systemPrompt string, temperature float64, maxTokens int, maxMessagesInContext int, model string) *OpenRouter {
	return &OpenRouter{
		Key:                  key,
		SystemPrompt:         systemPrompt,
		Temperature:          temperature,
		MaxTokens:            maxTokens,
		MaxMessagesInContext: maxMessagesInContext,
		Model:                model,
	}
}

func (o *OpenRouter) Send(r *Request) (*Response, error) {
	if len(r.Messages) <= 1 && r.Prompt == "" {
		return nil, fmt.Errorf("empty prompt provided")
	}

	jsonData, err := json.Marshal(r)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", "https://openrouter.ai/api/v1/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+o.Key)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Title", "Chad Discord Bot")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	response := &Response{}
	if err := json.NewDecoder(resp.Body).Decode(response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if response.Error != nil {
		return nil, fmt.Errorf("API error: %s", response.Error.Message)
	}

	return response, nil
}

func (r *OpenRouter) NewRequest() *Request {
	return &Request{
		Model:       r.Model,
		MaxTokens:   r.MaxTokens,
		Temperature: r.Temperature,
		Messages: []*Message{
			{Role: "system", Content: r.SystemPrompt},
		},
		Tools: []Tool{
			{
				Type: "function",
				Fucntion: Function{
					Name:        "search",
					Description: "Search the internet for information",
					Parameters: map[string]Parameters{
						"query": {
							Type: "string",
							Properties: map[string]Type{
								"query": {Type: "string"},
							},
							Required: []string{"query"},
						},
					},
				},
			},
		},
	}
}

func (r *Request) AddMessage(role string, content string) {
	r.Messages = append(r.Messages, &Message{Role: role, Content: content})
}

func (r *Request) AddMessages(messages []*Message) {
	r.Messages = append(r.Messages, messages...)
}
