# Testing Guidelines for beef-briefing

This document provides comprehensive guidelines for writing and maintaining tests in the beef-briefing project. It complements the testing patterns documentation in [CLAUDE.md](CLAUDE.md#testing-patterns).

## Table of Contents

- [Quick Start](#quick-start)
- [Testing Philosophy](#testing-philosophy)
- [Test Structure](#test-structure)
- [Layer-Specific Guidelines](#layer-specific-guidelines)
- [Writing Effective Tests](#writing-effective-tests)
- [Common Patterns](#common-patterns)
- [Testing Tools](#testing-tools)
- [Debugging Tests](#debugging-tests)
- [Best Practices](#best-practices)
- [Code Review Checklist](#code-review-checklist)

## Quick Start

### Running Tests

```bash
cd apps/api-service

# Run all tests (parallel mode - fastest)
go test ./...

# Run tests sequentially (catches race conditions)
go test -p 1 ./...

# Run with race detector
go test -race ./...

# Run specific package
go test -v ./internal/repository

# Run specific test function
go test -v -run TestFunctionName ./internal/repository

# Generate coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html
```

### Test File Naming

- Repository layer tests: `*_repo_test.go` (e.g., `user_repo_test.go`)
- Service layer tests: `*_service_test.go` (e.g., `card_service_test.go`)
- Handler layer tests: `*_handler_test.go` (e.g., `arena_handler_test.go`)
- Generic utilities: `*_test.go` (e.g., `helpers_test.go`)

### Test Function Naming

Name test functions to describe what they test, not how they test it:

```go
// Good
func TestUpsertUser_NewUser(t *testing.T) { }
func TestUpsertUser_UpdateExistingUser(t *testing.T) { }
func TestUpsertUser_NullFieldHandling(t *testing.T) { }

// Avoid
func TestUpsertUser1(t *testing.T) { }
func TestUpsertUserFail(t *testing.T) { }
```

## Testing Philosophy

### Three-Layer Testing Strategy

The project uses a bottom-up testing approach with three distinct layers:

1. **Repository Layer** (Real Database)
   - Tests interact with actual PostgreSQL database
   - Uses transaction rollback for isolation
   - Catches database-specific issues (constraints, type conversions)
   - Most realistic tests but slower (~1-2s per package)

2. **Service Layer** (Mocked Repositories)
   - Tests business logic in isolation
   - Repositories are mocked via interfaces
   - Fast execution (~0.1-0.5s per package)
   - Focuses on application logic, not database

3. **Handler Layer** (HTTP Testing)
   - Tests HTTP request/response handling
   - Services are mocked via interfaces
   - Uses `httptest` package for request simulation
   - Validates parameter parsing, error handling, response format

### Coverage Goals

| Layer | Target | Why |
|-------|--------|-----|
| Repository | 65%+ | Real DB testing catches many bugs; aim for >70% on critical functions |
| Service | 70%+ | Business logic is critical; test happy path + all error scenarios |
| Handler | 75%+ | HTTP layer complexity; test auth, validation, error responses |
| Overall | 70%+ | Multi-layer integration requires comprehensive coverage |

### Test Isolation and Independence

All tests must be:

1. **Order-Independent**: Tests can run in any order
2. **Stateless**: No shared state between tests
3. **Deterministic**: Same results on every run
4. **Fast**: Total suite runs in <60 seconds

This is verified by:
- Running with `go test -p 1 ./...` (sequential)
- Running with `go test -race ./...` (race detector)
- Running the full suite multiple times

## Test Structure

### Basic Test Template

Every test should follow this structure:

```go
func TestFeatureName_ScenarioDescription(t *testing.T) {
    // SETUP: Prepare test fixtures and dependencies

    // EXECUTE: Perform the operation being tested

    // ASSERT: Verify the results

    // CLEANUP: (handled by defer or transaction rollback)
}
```

### Minimal Example

```go
func TestUserRepository_UpsertUser_NewUser(t *testing.T) {
    // Setup
    db := testutil.SetupTestDB(t)
    defer testutil.TeardownTestDB(t, db)

    testutil.WithTestTransaction(t, db, func(tx *sql.Tx) {
        // Execute
        repo := repository.NewUserRepository(tx)
        user := testutil.SampleUser()
        err := repo.UpsertUser(context.Background(), &user)

        // Assert
        if err != nil {
            t.Fatalf("UpsertUser failed: %v", err)
        }

        // Verify in database
        var retrieved User
        row := tx.QueryRow("SELECT id, first_name FROM users WHERE id = $1", user.ID)
        if err := row.Scan(&retrieved.ID, &retrieved.FirstName); err != nil {
            t.Fatalf("User not in database: %v", err)
        }
        if retrieved.FirstName != user.FirstName {
            t.Errorf("Expected %q, got %q", user.FirstName, retrieved.FirstName)
        }
    })
}
```

## Layer-Specific Guidelines

### Repository Layer Testing

**Characteristics**:
- Tests run against real PostgreSQL database
- Transactions auto-rollback after test
- Can verify exact SQL behavior
- Slower but most realistic

**Key Patterns**:

```go
func TestMessageRepository_InsertMessage_WithAttachments(t *testing.T) {
    db := testutil.SetupTestDB(t)
    defer testutil.TeardownTestDB(t, db)

    testutil.WithTestTransaction(t, db, func(tx *sql.Tx) {
        // Arrange: Insert prerequisite data
        userRepo := repository.NewUserRepository(tx)
        user := testutil.SampleUser()
        userRepo.UpsertUser(context.Background(), &user)

        chatRepo := repository.NewChatRepository(tx)
        chat := testutil.SampleChat()
        chatRepo.UpsertChat(context.Background(), &chat)

        // Act: Insert message with attachments
        msgRepo := repository.NewMessageRepository(tx)
        msg := testutil.SampleMessage()
        msg.From = &user
        msg.Chat = chat
        err := msgRepo.InsertMessage(context.Background(), &msg)

        // Assert
        if err != nil {
            t.Fatalf("InsertMessage failed: %v", err)
        }

        // Verify in database
        var msgID int64
        row := tx.QueryRow("SELECT message_id FROM messages WHERE chat_id = $1", chat.ID)
        if err := row.Scan(&msgID); err != nil {
            t.Fatalf("Message not found: %v", err)
        }
    })
}
```

**Best Practices**:

1. Always setup prerequisite data (users, chats) before testing
2. Verify changes directly in database after operation
3. Test both valid inputs and constraint violations
4. Use transactions for automatic cleanup
5. Test NULL fields for optional columns

### Service Layer Testing

**Characteristics**:
- Tests use mocked repositories
- Fast execution without database
- Focuses on business logic
- Mock setup/teardown is critical

**Key Patterns**:

```go
func TestCardService_GetUserCard_UserNotFound(t *testing.T) {
    // Arrange
    mockCardRepo := new(MockCardRepository)
    mockCardRepo.On("GetUserCard", mock.Anything, int64(999)).
        Return(nil, nil) // User not found

    service := NewCardService(mockCardRepo)

    // Act
    card, err := service.GetUserCard(context.Background(), 999)

    // Assert
    if err != nil {
        t.Errorf("Expected no error, got: %v", err)
    }
    if card != nil {
        t.Errorf("Expected nil card, got: %v", card)
    }
    mockCardRepo.AssertExpectations(t)
}
```

**Best Practices**:

1. Use interfaces for all dependencies (not concrete types)
2. Set up mocks with expected calls before execution
3. Verify mock expectations after assertions
4. Test error paths as thoroughly as happy paths
5. Use `mock.Anything` for inputs that don't matter
6. Use `mock.MatchedBy` for complex argument matching

### Handler Layer Testing

**Characteristics**:
- Tests use `httptest.NewRecorder` for responses
- Mocked services for dependencies
- Validates HTTP semantics (status codes, headers)
- Tests parameter parsing and validation

**Key Patterns**:

```go
func TestCardHandler_GetUserCard_Success(t *testing.T) {
    // Arrange
    mockService := new(MockCardService)
    expectedCard := &Card{ID: 123, Week: "2024-01"}
    mockService.On("GetUserCard", mock.Anything, int64(456)).
        Return(expectedCard, nil)

    handler := NewCardHandler(mockService)

    // Act
    req, _ := http.NewRequest("GET", "/cards/456?chat_id=789", nil)
    rec := httptest.NewRecorder()
    handler.GetUserCard(rec, req)

    // Assert
    if rec.Code != http.StatusOK {
        t.Errorf("Expected 200, got %d", rec.Code)
    }

    var resp Card
    json.Unmarshal(rec.Body.Bytes(), &resp)
    if resp.ID != expectedCard.ID {
        t.Errorf("Expected card ID %d, got %d", expectedCard.ID, resp.ID)
    }

    mockService.AssertExpectations(t)
}
```

**Best Practices**:

1. Test both valid and invalid parameters
2. Test all error status codes (400, 401, 403, 404, 500)
3. Verify response JSON structure
4. Test authentication/authorization checks
5. Test edge cases (missing fields, invalid types)

## Writing Effective Tests

### Table-Driven Tests

Use table-driven tests for multiple similar scenarios:

```go
func TestNullString(t *testing.T) {
    tests := []struct {
        name    string
        input   *string
        want    sql.NullString
        wantErr bool
    }{
        {
            name:  "nil input returns NULL",
            input: nil,
            want:  sql.NullString{Valid: false},
        },
        {
            name:  "empty string returns NULL",
            input: stringPtr(""),
            want:  sql.NullString{Valid: false},
        },
        {
            name:  "non-empty string returns value",
            input: stringPtr("hello"),
            want:  sql.NullString{String: "hello", Valid: true},
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := helpers.NullString(tt.input)
            if got != tt.want {
                t.Errorf("NullString(%v) = %v, want %v", tt.input, got, tt.want)
            }
        })
    }
}
```

**Benefits**:
- Easy to add new test cases
- Clear separation of inputs and expected outputs
- Self-documenting test logic
- Easier to spot missing scenarios

### Error Testing

Always test error scenarios:

```go
func TestMatchService_CreateMatch_NotEnoughCards(t *testing.T) {
    mockRepo := testutil.NewMockGameRepository()
    mockRepo.CreateMatchError = errors.New("user has no cards")

    service := NewMatchService(mockRepo)

    _, err := service.CreateMatch(context.Background(), &Match{})

    if err == nil {
        t.Error("Expected error for user with no cards")
    }
    if !strings.Contains(err.Error(), "no cards") {
        t.Errorf("Wrong error: %v", err)
    }
}
```

### Concurrent Testing

When testing concurrent operations:

```go
func TestShop_ConcurrentPurchases(t *testing.T) {
    mockRepo := testutil.NewMockGameRepository()
    shop := NewShop(mockRepo)

    // Run 10 concurrent purchases
    var wg sync.WaitGroup
    errors := make(chan error, 10)

    for i := 0; i < 10; i++ {
        wg.Add(1)
        go func(cardID int64) {
            defer wg.Done()
            _, err := shop.BuyCard(context.Background(), cardID)
            if err != nil {
                errors <- err
            }
        }(int64(i))
    }

    wg.Wait()
    close(errors)

    // Collect any errors
    var errs []error
    for err := range errors {
        errs = append(errs, err)
    }

    if len(errs) > 0 {
        t.Errorf("Concurrent purchases failed: %v", errs)
    }
}
```

### Testing NULL Handling

Be explicit about NULL value handling:

```go
func TestUserRepository_UpsertUser_NullFields(t *testing.T) {
    db := testutil.SetupTestDB(t)
    defer testutil.TeardownTestDB(t, db)

    testutil.WithTestTransaction(t, db, func(tx *sql.Tx) {
        repo := repository.NewUserRepository(tx)

        // User with optional fields empty (will be NULL)
        user := models.User{
            ID:        123456789,
            IsBot:     false,
            FirstName: "Test",
            LastName:  "",      // Empty = NULL
            Username:  "",      // Empty = NULL
        }

        err := repo.UpsertUser(context.Background(), &user)
        if err != nil {
            t.Fatalf("UpsertUser failed: %v", err)
        }

        // Verify NULL values in database
        var lastName sql.NullString
        row := tx.QueryRow("SELECT last_name FROM users WHERE id = $1", user.ID)
        row.Scan(&lastName)

        if lastName.Valid {
            t.Error("Expected last_name to be NULL, got: " + lastName.String)
        }
    })
}
```

## Common Patterns

### Setting Up Common Test Data

```go
// Create a user in the database
func setupUser(t *testing.T, tx *sql.Tx, user *models.User) {
    repo := repository.NewUserRepository(tx)
    if err := repo.UpsertUser(context.Background(), user); err != nil {
        t.Fatalf("Failed to setup user: %v", err)
    }
}

// Create a chat in the database
func setupChat(t *testing.T, tx *sql.Tx, chat *models.Chat) {
    repo := repository.NewChatRepository(tx)
    if err := repo.UpsertChat(context.Background(), chat); err != nil {
        t.Fatalf("Failed to setup chat: %v", err)
    }
}

// Create a message in the database
func setupMessage(t *testing.T, tx *sql.Tx, msg *models.Message) {
    repo := repository.NewMessageRepository(tx)
    if err := repo.InsertMessage(context.Background(), msg); err != nil {
        t.Fatalf("Failed to setup message: %v", err)
    }
}
```

### Mock Repository Setup

```go
// Create a mock that succeeds
mockRepo := testutil.NewMockGameRepository()

// Create a mock that fails
mockRepo := testutil.NewMockGameRepository()
mockRepo.CreateMatchError = errors.New("database error")

// Verify a specific call was made
if mockRepo.CreateMatchCalls != 1 {
    t.Errorf("Expected CreateMatch to be called once, got %d", mockRepo.CreateMatchCalls)
}

// Check stored state
if stored, ok := mockRepo.Matches[matchID]; !ok {
    t.Error("Match not found in repository")
} else {
    if stored.Status != expected.Status {
        t.Errorf("Expected status %v, got %v", expected.Status, stored.Status)
    }
}
```

### Response Validation

```go
func validateCardResponse(t *testing.T, rec *httptest.ResponseRecorder) {
    if rec.Code != http.StatusOK {
        t.Errorf("Expected 200, got %d", rec.Code)
    }

    contentType := rec.Header().Get("Content-Type")
    if contentType != "application/json" {
        t.Errorf("Expected JSON content type, got %s", contentType)
    }

    var resp CardResponse
    if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
        t.Fatalf("Failed to unmarshal response: %v", err)
    }

    if resp.Card == nil {
        t.Error("Expected card in response, got nil")
    }
}
```

## Testing Tools

### Using testutil Mocks

```go
// MinIO mock for file uploads
mockMinIO := testutil.NewMockMinIOClient()
mockMinIO.UploadError = errors.New("upload failed")

// NewRelic mock for instrumentation
mockNR := testutil.NewMockNewRelicApp()
events := mockNR.GetCustomEvents()

// Game repository mock
mockGame := testutil.NewMockGameRepository()
mockGame.Reset() // Clear state between tests
```

### Using Testutil Fixtures

```go
// Sample data factories
user := testutil.SampleUser()
chat := testutil.SampleChat()
message := testutil.SampleMessage()
update := testutil.SampleTelegramUpdate()
match := testutil.SampleMatch()

// Customized fixtures
user := testutil.SampleUserWithID(999)
chat := testutil.SampleChatWithID(-1001234567890)
match := testutil.SampleMatchWithStatus(repository.MatchStatusShopPhase)
```

## Debugging Tests

### Viewing Test Output

```bash
# Show all test output
go test -v ./internal/handlers

# Show only failed tests
go test -v ./internal/handlers | grep FAIL

# Run a single test with output
go test -v -run TestCardHandler_GetUserCard ./internal/handlers

# Show panics and errors
go test -v -race ./internal/handlers 2>&1 | grep -A5 panic
```

### Finding Flaky Tests

```bash
# Run tests multiple times
for i in {1..5}; do
    go test -p 1 ./... || echo "Failed on run $i"
done

# Run with race detector (catches data races)
go test -race ./...

# Run in randomized order
go test -shuffle=on ./...
```

### Debugging Hangs

```bash
# Run with timeout
go test -timeout 5s ./...

# See what tests are running
go test -v ./... | grep RUN

# Get goroutine traces
go test -v -run TestHangingTest -timeout 2s ./... 2>&1 | tail -20
```

### Database Connection Issues

```bash
# Check test database is running
psql -h localhost -U postgres -d beef_briefing -c "SELECT 1"

# Verify environment variables
echo "TEST_DB_HOST=$TEST_DB_HOST"
echo "TEST_DB_PORT=$TEST_DB_PORT"
echo "TEST_DB_USER=$TEST_DB_USER"
echo "TEST_DB_NAME=$TEST_DB_NAME"

# Test database setup
go test -v -run TestSetupDatabase ./internal/testutil
```

## Best Practices

### 1. Test Behavior, Not Implementation

```go
// Good: Tests what the function does
func TestGetUserCard_ReturnsUserCard(t *testing.T) {
    // ...
}

// Bad: Tests how it's implemented
func TestGetUserCard_UsesQueryRow(t *testing.T) {
    // ...
}
```

### 2. One Assertion Per Concept

```go
// Good: Clear what's being tested
func TestUpsertUser(t *testing.T) {
    if err != nil {
        t.Errorf("Unexpected error: %v", err)
    }
    if storedUser.FirstName != expectedUser.FirstName {
        t.Errorf("FirstName mismatch")
    }
}

// Not ideal: Multiple things in one assertion
func TestUpsertUser(t *testing.T) {
    if err != nil && storedUser.FirstName != expectedUser.FirstName {
        t.Error("Something is wrong")
    }
}
```

### 3. Use Helper Functions

```go
// Create reusable test helpers
func createMatchInDB(t *testing.T, tx *sql.Tx, match *Match) string {
    repo := repository.NewGameRepository(tx)
    id, err := repo.CreateMatch(context.Background(), match)
    if err != nil {
        t.Fatalf("Failed to create match: %v", err)
    }
    return id
}

func assertMatchStatus(t *testing.T, repo *GameRepository, matchID string, expected MatchStatus) {
    match, err := repo.GetMatch(context.Background(), matchID)
    if err != nil {
        t.Fatalf("Failed to get match: %v", err)
    }
    if match.Status != expected {
        t.Errorf("Expected status %v, got %v", expected, match.Status)
    }
}

// Usage
func TestMatch_Lifecycle(t *testing.T) {
    // ...
    matchID := createMatchInDB(t, tx, match)
    assertMatchStatus(t, repo, matchID, MatchStatusOpen)
}
```

### 4. Test Edge Cases

```go
// Test boundaries
func TestParticipant_Coins(t *testing.T) {
    tests := []int{0, 1, 10, 999, 1000, math.MaxInt64}
    for _, coins := range tests {
        participant := &Participant{CoinsRemaining: coins}
        // Test with each boundary
    }
}

// Test special values
func TestTimestamp(t *testing.T) {
    times := []time.Time{
        time.Time{},                    // Zero time
        time.Now(),                     // Current
        time.Unix(0, 0),                // Epoch
        time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC), // Y2K
    }
    for _, tm := range times {
        // Test with each time
    }
}
```

### 5. Keep Tests Fast

```go
// Good: Only test what's necessary
func TestUpsertUser_InsertOnly(t *testing.T) {
    user := testutil.SampleUser()
    // Test insert only

    // Clean test with no extra database queries
}

// Avoid: Unnecessary complexity
func TestUpsertUser_ComplexScenario(t *testing.T) {
    // Setup 100 users
    // Create 50 chats
    // Insert 1000 messages
    // Test one upsert in the middle
    // This is too slow and tests multiple things
}
```

## Code Review Checklist

When submitting tests in a PR, ensure:

- [ ] Test function names clearly describe what's being tested
- [ ] Tests are organized by layer (repository, service, handler)
- [ ] Each test is independent (doesn't depend on execution order)
- [ ] Tests use `testutil` fixtures and helpers
- [ ] Mock setup is explicit and visible
- [ ] Assertions include error messages with expected/actual values
- [ ] Both happy path and error scenarios are tested
- [ ] Database tests use transactions with rollback
- [ ] Service tests mock all external dependencies
- [ ] Handler tests validate HTTP semantics
- [ ] Tests run fast (< 1s for service/handler, < 5s for repository packages)
- [ ] No hardcoded IDs or timestamps (use fixtures)
- [ ] No test interdependencies or shared state
- [ ] Coverage is tracked (aim for >70% overall)
- [ ] Tests pass with `go test -race ./...`

## Additional Resources

- [CLAUDE.md - Testing Patterns](CLAUDE.md#testing-the-system)
- [Go Testing Best Practices](https://golang.org/doc/effective_go#testing)
- [Table-Driven Tests](https://github.com/golang/go/wiki/TableDrivenTests)
- [testutil Package Documentation](CLAUDE.md#test-utilities-testutil-package)

## Questions or Issues?

If you have questions about testing patterns or encounter issues:

1. Check existing tests in `apps/api-service/internal/*/` for examples
2. Review the `testutil` package documentation
3. Look at the "Testing Patterns" section in CLAUDE.md
4. Check the progress notes in `plans/more-tests/progress.txt`
