package templates

import (
	"testing"
)

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
	
	if flash.Message != "test success" {
		t.Errorf("Expected 'test success', got '%s'", flash.Message)
	}
	
	if flash.Type != FlashTypeSuccess {
		t.Errorf("Expected FlashTypeSuccess, got %v", flash.Type)
	}
	
	if !flash.IsSuccess() {
		t.Error("Expected IsSuccess() to return true")
	}
	
	if flash.IsError() {
		t.Error("Expected IsError() to return false")
	}
	
	if flash.IsInfo() {
		t.Error("Expected IsInfo() to return false")
	}
}

func TestErrorFlash(t *testing.T) {
	flash := ErrorFlash("test error")
	
	if flash.Message != "test error" {
		t.Errorf("Expected 'test error', got '%s'", flash.Message)
	}
	
	if flash.Type != FlashTypeError {
		t.Errorf("Expected FlashTypeError, got %v", flash.Type)
	}
	
	if flash.IsSuccess() {
		t.Error("Expected IsSuccess() to return false")
	}
	
	if !flash.IsError() {
		t.Error("Expected IsError() to return true")
	}
	
	if flash.IsInfo() {
		t.Error("Expected IsInfo() to return false")
	}
}

func TestInfoFlash(t *testing.T) {
	flash := InfoFlash("test info")
	
	if flash.Message != "test info" {
		t.Errorf("Expected 'test info', got '%s'", flash.Message)
	}
	
	if flash.Type != FlashTypeInfo {
		t.Errorf("Expected FlashTypeInfo, got %v", flash.Type)
	}
	
	if flash.IsSuccess() {
		t.Error("Expected IsSuccess() to return false")
	}
	
	if flash.IsError() {
		t.Error("Expected IsError() to return false")
	}
	
	if !flash.IsInfo() {
		t.Error("Expected IsInfo() to return true")
	}
}

func TestFlashBorderColor(t *testing.T) {
	t.Run("success border color", func(t *testing.T) {
		flash := SuccessFlash("test")
		expected := "border-green-500"
		
		if flash.BorderColor() != expected {
			t.Errorf("Expected '%s', got '%s'", expected, flash.BorderColor())
		}
	})
	
	t.Run("error border color", func(t *testing.T) {
		flash := ErrorFlash("test")
		expected := "border-red-500"
		
		if flash.BorderColor() != expected {
			t.Errorf("Expected '%s', got '%s'", expected, flash.BorderColor())
		}
	})
	
	t.Run("info border color", func(t *testing.T) {
		flash := InfoFlash("test")
		expected := "border-blue-500"
		
		if flash.BorderColor() != expected {
			t.Errorf("Expected '%s', got '%s'", expected, flash.BorderColor())
		}
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