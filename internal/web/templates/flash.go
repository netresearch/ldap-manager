package templates

type FlashType string

const (
	FlashTypeSuccess FlashType = "success"
	FlashTypeError   FlashType = "error"
	FlashTypeInfo    FlashType = "info"
)

func Flashes(templates ...Flash) []Flash {
	return templates
}

type Flash struct {
	Message string
	Type    FlashType
}

func SuccessFlash(message string) Flash {
	return Flash{
		Message: message,
		Type:    FlashTypeSuccess,
	}
}

func ErrorFlash(message string) Flash {
	return Flash{
		Message: message,
		Type:    FlashTypeError,
	}
}

func InfoFlash(message string) Flash {
	return Flash{
		Message: message,
		Type:    FlashTypeInfo,
	}
}

func (f Flash) IsSuccess() bool {
	return f.Type == FlashTypeSuccess
}

func (f Flash) IsError() bool {
	return f.Type == FlashTypeError
}

func (f Flash) IsInfo() bool {
	return f.Type == FlashTypeInfo
}

const flashSuccessClasses = "border-green-500"
const flashErrorClasses = "border-red-500"
const flashInfoClasses = "border-blue-500"

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
