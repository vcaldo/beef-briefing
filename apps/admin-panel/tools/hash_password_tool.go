package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"syscall"

	"golang.org/x/crypto/bcrypt"
	"golang.org/x/term"
)

func main() {
	fmt.Println("Beef Briefing Admin Panel - Password Hash Generator")
	fmt.Println("====================================================")
	fmt.Println()

	reader := bufio.NewReader(os.Stdin)

	// Check if input is from terminal or pipe
	var password string
	if term.IsTerminal(int(syscall.Stdin)) {
		// Interactive mode - hide password input
		fmt.Print("Enter password: ")
		passwordBytes, err := term.ReadPassword(int(syscall.Stdin))
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading password: %v\n", err)
			os.Exit(1)
		}
		fmt.Println()

		fmt.Print("Confirm password: ")
		confirmBytes, err := term.ReadPassword(int(syscall.Stdin))
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading confirmation: %v\n", err)
			os.Exit(1)
		}
		fmt.Println()

		if string(passwordBytes) != string(confirmBytes) {
			fmt.Fprintf(os.Stderr, "Error: passwords do not match\n")
			os.Exit(1)
		}

		password = string(passwordBytes)
	} else {
		// Pipe mode - read from stdin
		input, err := reader.ReadString('\n')
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading from stdin: %v\n", err)
			os.Exit(1)
		}
		password = strings.TrimSpace(input)
	}

	// Validate password
	if len(password) == 0 {
		fmt.Fprintf(os.Stderr, "Error: password cannot be empty\n")
		os.Exit(1)
	}

	if len(password) < 8 {
		fmt.Fprintf(os.Stderr, "Warning: password is less than 8 characters\n")
	}

	// Generate bcrypt hash
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error generating hash: %v\n", err)
		os.Exit(1)
	}

	fmt.Println()
	fmt.Println("Generated bcrypt hash:")
	fmt.Println(string(hash))
	fmt.Println()
	fmt.Println("Add this to your .env file:")
	fmt.Printf("ADMIN_PASSWORD_HASH=%s\n", string(hash))
}
