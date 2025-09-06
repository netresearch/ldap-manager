// Package templates provides flash message functionality for the web interface.
// Handles success, error, and info messages with associated styling classes.
package templates

// FlashType represents the category of flash message for styling and behavior.
type FlashType string

const (
	// FlashTypeSuccess represents a successful operation or positive feedback.
	FlashTypeSuccess FlashType = "success"
	// FlashTypeError represents an error condition or failed operation.
	FlashTypeError FlashType = "error"
	// FlashTypeInfo represents informational messages or neutral feedback.
	FlashTypeInfo FlashType = "info"
)

// Flashes creates a slice of flash messages from the provided Flash instances.
// Utility function for collecting multiple flash messages into a single collection.
func Flashes(templates ...Flash) []Flash {
	return templates
}

// Flash represents a user-facing message with associated type and styling.
type Flash struct {
	Message string
	Type    FlashType
}

// SuccessFlash creates a success flash message with the provided text.
func SuccessFlash(message string) Flash {
	return Flash{
		Message: message,
		Type:    FlashTypeSuccess,
	}
}

// ErrorFlash creates an error flash message with the provided text.
func ErrorFlash(message string) Flash {
	return Flash{
		Message: message,
		Type:    FlashTypeError,
	}
}

// InfoFlash creates an informational flash message with the provided text.
func InfoFlash(message string) Flash {
	return Flash{
		Message: message,
		Type:    FlashTypeInfo,
	}
}

// IsSuccess returns true if this flash message represents a successful operation.
func (f Flash) IsSuccess() bool {
	return f.Type == FlashTypeSuccess
}

// IsError returns true if this flash message represents an error condition.
func (f Flash) IsError() bool {
	return f.Type == FlashTypeError
}

// IsInfo returns true if this flash message represents informational content.
func (f Flash) IsInfo() bool {
	return f.Type == FlashTypeInfo
}

const (
	flashSuccessClasses = "border-green-500"
	flashErrorClasses   = "border-red-500"
	flashInfoClasses    = "border-blue-500"
)

// BorderColor returns the appropriate CSS border class for the flash message type.
func (f Flash) BorderColor() string {
	switch f.Type {
	case FlashTypeSuccess:
		return flashSuccessClasses
	case FlashTypeError:
		return flashErrorClasses
	case FlashTypeInfo:
		return flashInfoClasses
	default:
		panic("unknown flash type")
	}
}
