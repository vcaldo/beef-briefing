package main

import (
	"bufio"
	"crypto/rand"
	"encoding/base64"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"

	"golang.org/x/crypto/bcrypt"
	"golang.org/x/term"
)

const (
	defaultSessionSecretLength = 32
	minPasswordLength          = 8
	passwordHashFilename       = "admin_password_hash"
	sessionSecretFilename      = "session_secret"
)

type outputMode string

const (
	outputModeEnv   outputMode = "env"
	outputModeFiles outputMode = "files"
)

func main() {
	// Parse command line flags
	envFile := flag.String("file", "", "Path to .env file to update (for env mode)")
	secretsDir := flag.String("secrets-dir", "", "Path to directory where secret files will be written (for files mode)")
	mode := flag.String("mode", "env", "Output mode: 'env' for .env file, 'files' for separate secret files")
	passwordOnly := flag.Bool("password-only", false, "Only update password hash, skip session secret")
	sessionOnly := flag.Bool("session-only", false, "Only update session secret, skip password")
	noInteractive := flag.Bool("no-interactive", false, "Read password from stdin (non-interactive mode)")
	showHelp := flag.Bool("help", false, "Show help message")

	flag.Parse()

	if *showHelp {
		printHelp()
		return
	}

	if *passwordOnly && *sessionOnly {
		fmt.Fprintf(os.Stderr, "Error: cannot use both -password-only and -session-only\n")
		os.Exit(1)
	}

	// Determine output mode
	var outMode outputMode
	switch *mode {
	case "env":
		outMode = outputModeEnv
	case "files":
		outMode = outputModeFiles
	default:
		fmt.Fprintf(os.Stderr, "Error: invalid mode '%s'. Must be 'env' or 'files'\n", *mode)
		os.Exit(1)
	}

	// Validate parameters based on mode
	if outMode == outputModeEnv {
		if *envFile == "" {
			fmt.Fprintf(os.Stderr, "Error: -file parameter is required for env mode\n\n")
			printHelp()
			os.Exit(1)
		}
		if _, err := os.Stat(*envFile); os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "Error: file not found: %s\n", *envFile)
			os.Exit(1)
		}
	} else {
		if *secretsDir == "" {
			fmt.Fprintf(os.Stderr, "Error: -secrets-dir parameter is required for files mode\n\n")
			printHelp()
			os.Exit(1)
		}
		// Create secrets directory if it doesn't exist
		if err := os.MkdirAll(*secretsDir, 0700); err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to create secrets directory: %v\n", err)
			os.Exit(1)
		}
	}

	fmt.Println("========================================")
	fmt.Println("Admin Panel Secret Generator")
	fmt.Println("========================================")
	fmt.Println()

	var passwordHash string
	var sessionSecret string

	// Generate password hash
	if !*sessionOnly {
		var err error
		passwordHash, err = generatePasswordHash(*noInteractive)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("✓ Password hash generated")
	}

	// Generate session secret
	if !*passwordOnly {
		var err error
		sessionSecret, err = generateSessionSecret()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error generating session secret: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("✓ Session secret generated")
	}

	// Write secrets based on mode
	if outMode == outputModeEnv {
		if err := updateEnvFile(*envFile, passwordHash, sessionSecret, *passwordOnly, *sessionOnly); err != nil {
			fmt.Fprintf(os.Stderr, "Error updating env file: %v\n", err)
			os.Exit(1)
		}
		fmt.Println()
		fmt.Printf("✓ Successfully updated %s\n", *envFile)
	} else {
		if err := writeSecretFiles(*secretsDir, passwordHash, sessionSecret, *passwordOnly, *sessionOnly); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing secret files: %v\n", err)
			os.Exit(1)
		}
		fmt.Println()
		fmt.Printf("✓ Successfully wrote secrets to %s\n", *secretsDir)
	}

	fmt.Println()
	if !*sessionOnly {
		fmt.Println("ADMIN_PASSWORD_HASH has been set")
	}
	if !*passwordOnly {
		fmt.Println("SESSION_SECRET has been set")
	}
}

func printHelp() {
	fmt.Println("Admin Panel Secret Generator and Updater")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  go run update_secrets.go [options]")
	fmt.Println()
	fmt.Println("Options:")
	fmt.Println("  -mode string         Output mode: 'env' or 'files' (default: env)")
	fmt.Println("  -file string         Path to .env file to update (required for env mode)")
	fmt.Println("  -secrets-dir string  Path to directory for secret files (required for files mode)")
	fmt.Println("  -password-only       Only update password hash, skip session secret")
	fmt.Println("  -session-only        Only update session secret, skip password")
	fmt.Println("  -no-interactive      Read password from stdin (for scripting)")
	fmt.Println("  -help                Show this help message")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  # Update .env file (interactive mode):")
	fmt.Println("  go run update_secrets.go -file infrastructure/.env.dev")
	fmt.Println()
	fmt.Println("  # Write to separate files:")
	fmt.Println("  go run update_secrets.go -mode=files -secrets-dir infrastructure/secrets")
	fmt.Println()
	fmt.Println("  # Update only password in files mode:")
	fmt.Println("  go run update_secrets.go -mode=files -secrets-dir infrastructure/secrets -password-only")
	fmt.Println()
	fmt.Println("  # Non-interactive mode:")
	fmt.Println("  echo 'mypassword' | go run update_secrets.go -mode=files -secrets-dir infrastructure/secrets -no-interactive")
}

func generatePasswordHash(nonInteractive bool) (string, error) {
	var password string

	if nonInteractive || !term.IsTerminal(int(syscall.Stdin)) {
		// Non-interactive mode - read from stdin
		reader := bufio.NewReader(os.Stdin)
		input, err := reader.ReadString('\n')
		if err != nil {
			return "", fmt.Errorf("reading password from stdin: %w", err)
		}
		password = strings.TrimSpace(input)
	} else {
		// Interactive mode - hide password input
		fmt.Print("Enter admin password: ")
		passwordBytes, err := term.ReadPassword(int(syscall.Stdin))
		if err != nil {
			return "", fmt.Errorf("reading password: %w", err)
		}
		fmt.Println()

		fmt.Print("Confirm password: ")
		confirmBytes, err := term.ReadPassword(int(syscall.Stdin))
		if err != nil {
			return "", fmt.Errorf("reading confirmation: %w", err)
		}
		fmt.Println()

		if string(passwordBytes) != string(confirmBytes) {
			return "", fmt.Errorf("passwords do not match")
		}

		password = string(passwordBytes)
	}

	// Validate password
	if len(password) == 0 {
		return "", fmt.Errorf("password cannot be empty")
	}

	if len(password) < minPasswordLength {
		fmt.Fprintf(os.Stderr, "Warning: password is less than %d characters\n", minPasswordLength)
	}

	// Generate bcrypt hash
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("generating bcrypt hash: %w", err)
	}

	return string(hash), nil
}

func generateSessionSecret() (string, error) {
	bytes := make([]byte, defaultSessionSecretLength)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("generating random bytes: %w", err)
	}

	return base64.StdEncoding.EncodeToString(bytes), nil
}

func writeSecretFiles(dirPath, passwordHash, sessionSecret string, passwordOnly, sessionOnly bool) error {
	// Write password hash file
	if !sessionOnly && passwordHash != "" {
		passwordFile := filepath.Join(dirPath, passwordHashFilename)
		if err := os.WriteFile(passwordFile, []byte(passwordHash), 0600); err != nil {
			return fmt.Errorf("writing password hash file: %w", err)
		}
	}

	// Write session secret file
	if !passwordOnly && sessionSecret != "" {
		secretFile := filepath.Join(dirPath, sessionSecretFilename)
		if err := os.WriteFile(secretFile, []byte(sessionSecret), 0600); err != nil {
			return fmt.Errorf("writing session secret file: %w", err)
		}
	}

	return nil
}

func updateEnvFile(filePath, passwordHash, sessionSecret string, passwordOnly, sessionOnly bool) error {
	// Read the current file
	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("reading file: %w", err)
	}

	lines := strings.Split(string(content), "\n")
	updatedLines := make([]string, 0, len(lines))
	passwordHashUpdated := false
	sessionSecretUpdated := false

	// Regular expressions to match existing entries
	passwordHashRegex := regexp.MustCompile(`^ADMIN_PASSWORD_HASH=`)
	sessionSecretRegex := regexp.MustCompile(`^SESSION_SECRET=`)

	// Update existing lines or track if we need to add new ones
	for _, line := range lines {
		if !sessionOnly && passwordHashRegex.MatchString(line) {
			updatedLines = append(updatedLines, fmt.Sprintf("ADMIN_PASSWORD_HASH=\"%s\"", passwordHash))
			passwordHashUpdated = true
		} else if !passwordOnly && sessionSecretRegex.MatchString(line) {
			updatedLines = append(updatedLines, fmt.Sprintf("SESSION_SECRET=\"%s\"", sessionSecret))
			sessionSecretUpdated = true
		} else {
			updatedLines = append(updatedLines, line)
		}
	}

	// Add new entries if they didn't exist
	if !sessionOnly && !passwordHashUpdated {
		// Add a blank line if file doesn't end with one
		if len(updatedLines) > 0 && updatedLines[len(updatedLines)-1] != "" {
			updatedLines = append(updatedLines, "")
		}
		timestamp := time.Now().Format("2006-01-02 15:04:05")
		updatedLines = append(updatedLines, fmt.Sprintf("# Generated by update_secrets.go on %s", timestamp))
		updatedLines = append(updatedLines, fmt.Sprintf("ADMIN_PASSWORD_HASH=\"%s\"", passwordHash))
	}

	if !passwordOnly && !sessionSecretUpdated {
		// Add comment only if we didn't just add it for password hash
		if sessionOnly || passwordHashUpdated {
			if len(updatedLines) > 0 && updatedLines[len(updatedLines)-1] != "" {
				updatedLines = append(updatedLines, "")
			}
			timestamp := time.Now().Format("2006-01-02 15:04:05")
			updatedLines = append(updatedLines, fmt.Sprintf("# Generated by update_secrets.go on %s", timestamp))
		}
		updatedLines = append(updatedLines, fmt.Sprintf("SESSION_SECRET=\"%s\"", sessionSecret))
	}

	// Write updated content back to file
	newContent := strings.Join(updatedLines, "\n")
	if err := os.WriteFile(filePath, []byte(newContent), 0644); err != nil {
		return fmt.Errorf("writing file: %w", err)
	}

	return nil
}
