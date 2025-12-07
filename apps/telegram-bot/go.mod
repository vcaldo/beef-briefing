module beef-briefing/apps/telegram-bot

go 1.25

replace beef-briefing/pkg/config => ../../pkg/config

require (
	beef-briefing/pkg/config v0.0.0-00010101000000-000000000000
	github.com/go-telegram/bot v1.11.1
)

require (
	github.com/joho/godotenv v1.5.1 // indirect
	github.com/kelseyhightower/envconfig v1.4.0 // indirect
)
