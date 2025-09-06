package templates

import (
	"testing"
)

// Test helpers for flash testing patterns
func assertFlashBasicProperties(t *testing.T, flash Flash, expectedMessage string, expectedType FlashType) {
	t.Helper()
	if flash.Message != expectedMessage {
		t.Errorf("Expected '%s', got '%s'", expectedMessage, flash.Message)
	}
	if flash.Type != expectedType {
		t.Errorf("Expected %s, got %v", expectedType, flash.Type)
	}
}

func assertFlashTypeChecks(t *testing.T, flash Flash, shouldBeSuccess, shouldBeError, shouldBeInfo bool) {
	t.Helper()
	if flash.IsSuccess() != shouldBeSuccess {
		t.Errorf("Expected IsSuccess() to return %v", shouldBeSuccess)
	}
	if flash.IsError() != shouldBeError {
		t.Errorf("Expected IsError() to return %v", shouldBeError)
	}
	if flash.IsInfo() != shouldBeInfo {
		t.Errorf("Expected IsInfo() to return %v", shouldBeInfo)
	}
}

func assertFlashBorderColor(t *testing.T, flashConstructor func(string) Flash, expectedColor string) {
	t.Helper()
	flash := flashConstructor("test")
	if flash.BorderColor() != expectedColor {
		t.Errorf("Expected '%s', got '%s'", expectedColor, flash.BorderColor())
	}
}

func TestFlashes(t *testing.T) {
	t.Run("empty flashes", func(t *testing.T) {
		flashes := Flashes()
		if len(flashes) != 0 {
			t.Errorf("Expected empty slice, got %d items", len(flashes))
		}
	})

	t.Run("multiple flashes", func(t *testing.T) {
		flash1 := SuccessFlash("success message")
		flash2 := ErrorFlash("error message")

		flashes := Flashes(flash1, flash2)
		if len(flashes) != 2 {
			t.Errorf("Expected 2 flashes, got %d", len(flashes))
		}

		if flashes[0].Message != "success message" {
			t.Errorf("Expected 'success message', got '%s'", flashes[0].Message)
		}

		if flashes[1].Message != "error message" {
			t.Errorf("Expected 'error message', got '%s'", flashes[1].Message)
		}
	})
}

func TestSuccessFlash(t *testing.T) {
	flash := SuccessFlash("test success")

	assertFlashBasicProperties(t, flash, "test success", FlashTypeSuccess)
	assertFlashTypeChecks(t, flash, true, false, false)
}

func TestErrorFlash(t *testing.T) {
	flash := ErrorFlash("test error")

	assertFlashBasicProperties(t, flash, "test error", FlashTypeError)
	assertFlashTypeChecks(t, flash, false, true, false)
}

func TestInfoFlash(t *testing.T) {
	flash := InfoFlash("test info")

	assertFlashBasicProperties(t, flash, "test info", FlashTypeInfo)
	assertFlashTypeChecks(t, flash, false, false, true)
}

func TestFlashBorderColor(t *testing.T) {
	t.Run("success border color", func(t *testing.T) {
		assertFlashBorderColor(t, SuccessFlash, "border-green-500")
	})

	t.Run("error border color", func(t *testing.T) {
		assertFlashBorderColor(t, ErrorFlash, "border-red-500")
	})

	t.Run("info border color", func(t *testing.T) {
		assertFlashBorderColor(t, InfoFlash, "border-blue-500")
	})

	t.Run("unknown flash type panics", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("Expected panic for unknown flash type")
			}
		}()

		flash := Flash{Message: "test", Type: "unknown"}
		flash.BorderColor()
	})
}

func TestFlashConstants(t *testing.T) {
	if FlashTypeSuccess != "success" {
		t.Errorf("Expected FlashTypeSuccess to be 'success', got '%s'", FlashTypeSuccess)
	}

	if FlashTypeError != "error" {
		t.Errorf("Expected FlashTypeError to be 'error', got '%s'", FlashTypeError)
	}

	if FlashTypeInfo != "info" {
		t.Errorf("Expected FlashTypeInfo to be 'info', got '%s'", FlashTypeInfo)
	}
}
