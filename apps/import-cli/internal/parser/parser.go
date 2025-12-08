package parser

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"beef-briefing/apps/import-cli/internal/models"
)

// ChatMetadata holds the chat information from the export root
type ChatMetadata struct {
	ID   int64
	Name string
	Type string
}

// Parser handles streaming parsing of Telegram export JSON
type Parser struct {
	filePath string
}

// New creates a new Parser instance
func New(filePath string) *Parser {
	return &Parser{
		filePath: filePath,
	}
}

// ParseMetadata extracts chat metadata from the export file
func (p *Parser) ParseMetadata() (*ChatMetadata, error) {
	file, err := os.Open(p.filePath)
	if err != nil {
		return nil, fmt.Errorf("opening export file: %w", err)
	}
	defer file.Close()

	decoder := json.NewDecoder(file)

	// Read opening brace
	token, err := decoder.Token()
	if err != nil {
		return nil, fmt.Errorf("reading opening token: %w", err)
	}
	if token != json.Delim('{') {
		return nil, fmt.Errorf("expected '{', got %v", token)
	}

	metadata := &ChatMetadata{}

	// Read key-value pairs until we have all metadata
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return nil, fmt.Errorf("reading key token: %w", err)
		}

		key, ok := token.(string)
		if !ok {
			continue
		}

		switch key {
		case "name":
			var name string
			if err := decoder.Decode(&name); err != nil {
				return nil, fmt.Errorf("decoding name: %w", err)
			}
			metadata.Name = name
		case "type":
			var chatType string
			if err := decoder.Decode(&chatType); err != nil {
				return nil, fmt.Errorf("decoding type: %w", err)
			}
			metadata.Type = chatType
		case "id":
			var id int64
			if err := decoder.Decode(&id); err != nil {
				return nil, fmt.Errorf("decoding id: %w", err)
			}
			metadata.ID = id
		case "messages":
			// We've reached messages, we have all the metadata we need
			return metadata, nil
		default:
			// Skip unknown fields
			var skip json.RawMessage
			if err := decoder.Decode(&skip); err != nil {
				return nil, fmt.Errorf("skipping field %s: %w", key, err)
			}
		}
	}

	return metadata, nil
}

// StreamMessages streams messages from the export file
// It calls the callback function for each message parsed
func (p *Parser) StreamMessages(callback func(msg *models.ExportMessage) error) error {
	file, err := os.Open(p.filePath)
	if err != nil {
		return fmt.Errorf("opening export file: %w", err)
	}
	defer file.Close()

	decoder := json.NewDecoder(file)

	// Navigate to the messages array
	if err := p.navigateToMessages(decoder); err != nil {
		return fmt.Errorf("navigating to messages: %w", err)
	}

	// Read messages one by one
	for decoder.More() {
		var msg models.ExportMessage
		if err := decoder.Decode(&msg); err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("decoding message: %w", err)
		}

		if err := callback(&msg); err != nil {
			return fmt.Errorf("processing message %d: %w", msg.ID, err)
		}
	}

	return nil
}

// navigateToMessages moves the decoder to the start of the messages array
func (p *Parser) navigateToMessages(decoder *json.Decoder) error {
	// Read opening brace
	token, err := decoder.Token()
	if err != nil {
		return fmt.Errorf("reading opening token: %w", err)
	}
	if token != json.Delim('{') {
		return fmt.Errorf("expected '{', got %v", token)
	}

	// Find the "messages" key
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return fmt.Errorf("reading key token: %w", err)
		}

		key, ok := token.(string)
		if !ok {
			continue
		}

		if key == "messages" {
			// Read opening bracket of messages array
			token, err := decoder.Token()
			if err != nil {
				return fmt.Errorf("reading messages array token: %w", err)
			}
			if token != json.Delim('[') {
				return fmt.Errorf("expected '[', got %v", token)
			}
			return nil
		}

		// Skip other fields
		var skip json.RawMessage
		if err := decoder.Decode(&skip); err != nil {
			return fmt.Errorf("skipping field %s: %w", key, err)
		}
	}

	return fmt.Errorf("messages array not found")
}

// CountMessages counts the total number of messages in the export file
func (p *Parser) CountMessages() (int, error) {
	count := 0
	err := p.StreamMessages(func(msg *models.ExportMessage) error {
		count++
		return nil
	})
	if err != nil {
		return 0, err
	}
	return count, nil
}
