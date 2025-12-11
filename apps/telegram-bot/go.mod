module beef-briefing/apps/telegram-bot

go 1.25

replace beef-briefing/pkg/config => ../../pkg/config

require (
	beef-briefing/pkg/config v0.0.0-00010101000000-000000000000
	github.com/go-telegram/bot v1.11.1
	github.com/newrelic/go-agent/v3 v3.35.1
)

require (
	github.com/joho/godotenv v1.5.1 // indirect
	github.com/kelseyhightower/envconfig v1.4.0 // indirect
	golang.org/x/net v0.25.0 // indirect
	golang.org/x/sys v0.20.0 // indirect
	golang.org/x/text v0.15.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240528184218-531527333157 // indirect
	google.golang.org/grpc v1.65.0 // indirect
	google.golang.org/protobuf v1.34.2 // indirect
)
