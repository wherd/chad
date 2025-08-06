# Chad Discord Bot

Chad is a Discord bot written in Go that integrates with OpenRouter for AI-powered conversations. The bot handles rate limiting, persists data across restarts, and maintains conversation context without requiring complex infrastructure.

Built this because I wanted a Discord bot that just works. Most projects either focus on tons of features but break under load, or stay super simple but aren't actually useful. Chad aims for the middle ground—smart enough for real conversations, simple enough to deploy and maintain.

## What it actually does

**AI-powered conversations** through OpenRouter API with support for multiple models. The bot can engage naturally in Discord channels while respecting server-specific settings and user preferences.

**Intelligent rate limiting** with per-user tracking and automatic timeouts for spam prevention. Uses timestamp arrays to manage request windows without external dependencies — simple approach that scales well for typical Discord server sizes.

**Persistent conversation memory** that survives restarts. Stores recent messages per channel to provide context to AI models, plus server settings and user data in a simple JSON file. Data older than 7 days gets automatically cleaned up.

**Web search integration** when the AI needs current information beyond its training data. Helps provide accurate, up-to-date responses instead of making educated guesses about recent events.

**Server-specific configuration** for prefixes, disabled commands, moderator roles, and welcome channels. Each Discord server can customize Chad's behavior without affecting others.

## Why these technical choices matter

**Go** because it compiles to a single binary, has great concurrency support for handling multiple Discord connections, and doesn't require managing runtime dependencies. Deploy means copying one file and running it.

**OpenRouter integration** provides access to multiple AI models through one API instead of managing separate connections to different providers. Flexibility to use different models for different types of requests.

**JSON file persistence** instead of a database because Discord bot data is relatively simple and doesn't need complex queries. File-based storage eliminates database setup and maintenance while providing easy backups and debugging.

**In-memory rate limiting** with disk persistence strikes a balance between performance and reliability. Fast lookups during operation, but state survives restarts without external caching layers.

## Getting started

You'll need Go 1.24.5+, a Discord bot token from the Discord Developer Portal, and an OpenRouter API key.

```bash
git clone https://github.com/wherd/chad.git
cd chad
go mod tidy
go build
```

Create a `config.json` file based on the provided `.chad.default` template:

```json
{
    "discord_token": "your_discord_bot_token",
    "search_api": "your_brave_search_api_key", 
    "prefix": "!",
    "auto_save_interval": 60,
    "open_router": {
        "key": "your_openrouter_api_key",
        "system_prompt": "You are Chad, a helpful Discord bot assistant.",
        "temperature": 0.7,
        "max_tokens": 1024,
        "max_messages_in_context": 10,
        "model": "anthropic/claude-3-sonnet"
    },
    "rate_limit": {
        "max_requests": 10,
        "window": 60,
        "mute_time": 60
    }
}
```

Run with default config:
```bash
./chad
```

Or specify a custom config file:
```bash
./chad path/to/custom-config.json
```

## Architecture overview

**main.go** loads configuration and initializes the bot connection to Discord.

**internal/bot/bot.go** handles Discord events, message processing, and conversation management. Contains the core logic for determining when and how to respond.

**internal/openrouter/openrouter.go** manages API communication with OpenRouter, including model selection and response formatting.

**internal/websearch/websearch.go** provides web search capabilities when the AI needs current information.

**Data persistence** automatically saves bot state to `chad_memory.json` every 60 seconds and on shutdown. Includes versioning and data validation to handle updates gracefully.

The rate limiting system tracks per-user request timestamps and uses Discord's built-in timeout functionality for enforcement. Simple but effective approach that doesn't require external services.

## Configuration options

- **discord_token**: Bot token from Discord Developer Portal
- **search_api**: Brave Search API key for web search functionality  
- **prefix**: Command prefix for bot interactions (default: "!")
- **auto_save_interval**: How often to save state in seconds
- **open_router.model**: Which AI model to use for responses
- **rate_limit.max_requests**: Maximum requests per user in the time window
- **rate_limit.window**: Time window in seconds for rate limiting
- **rate_limit.mute_time**: How long to timeout users who exceed limits

## Development

Standard Go development workflow:

```bash
go build          # Build executable
go mod tidy       # Clean up dependencies
go fmt ./...      # Format code
go vet ./...      # Static analysis
go test ./...     # Run tests
```

The codebase uses standard Go practices—clear interfaces, minimal external dependencies, and straightforward error handling. Most of the complexity is in managing Discord's event system and OpenRouter's API responses.

## Why this approach works

**Simplicity enables reliability.** The bot does what it needs to do without unnecessary complexity. Easy to understand, debug, and modify when requirements change.

**Single binary deployment** eliminates most operational issues. No dependency management, no runtime version conflicts, no complex container setups. Copy the file, run it, done.

**Graceful degradation** when external services are unavailable. The bot can still function for basic operations even if OpenRouter or search APIs are temporarily down.

**Reasonable resource usage** through careful management of conversation context and automatic cleanup of old data. Scales well for typical Discord server sizes without requiring dedicated infrastructure.

The goal was building a Discord bot that just works consistently without requiring constant maintenance. Sometimes the boring technical choices are the ones that actually solve problems long-term.

## Contributing

Contributions welcome—just keep the focus on reliability and simplicity. Fork the repo, create a feature branch, and submit a pull request with a clear description of what you're changing and why.

## License

MIT License - see [LICENSE](LICENSE) file for details.

## Support

If Chad has been useful for your Discord server, consider supporting development:

[![ko-fi](https://ko-fi.com/img/githubbutton_sm.svg)](https://ko-fi.com/wherd)