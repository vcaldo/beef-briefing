package jsonutil

import (
	"errors"
	"testing"
)

func TestMarshal_Success(t *testing.T) {
	type testStruct struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	data := testStruct{Name: "Alice", Age: 30}
	result, err := Marshal(data)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	expected := `{"name":"Alice","age":30}`
	if string(result) != expected {
		t.Fatalf("expected %s, got %s", expected, string(result))
	}
}

func TestMarshal_Error(t *testing.T) {
	// Channels cannot be marshaled to JSON
	ch := make(chan int)
	_, err := Marshal(ch)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestUnmarshal_Success(t *testing.T) {
	type testStruct struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	data := []byte(`{"name":"Bob","age":25}`)
	var result testStruct
	err := Unmarshal(data, &result)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if result.Name != "Bob" || result.Age != 25 {
		t.Fatalf("expected {Bob 25}, got {%s %d}", result.Name, result.Age)
	}
}

func TestUnmarshal_Error(t *testing.T) {
	data := []byte(`{invalid json}`)
	var result map[string]interface{}
	err := Unmarshal(data, &result)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestErrorWrapping(t *testing.T) {
	// Test that errors are properly wrapped with %w
	data := []byte(`{invalid}`)
	var result map[string]interface{}
	err := Unmarshal(data, &result)

	if err == nil {
		t.Fatal("expected error, got nil")
	}

	// Verify error is wrapped by checking it can be unwrapped
	unwrapped := errors.Unwrap(err)
	if unwrapped == nil {
		t.Fatal("error should be wrapped and unwrappable")
	}
}
