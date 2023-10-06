package web

import (
	"embed"
)

//go:embed views layouts
var templates embed.FS

type InputOpts struct {
	Name         string
	Placeholder  string
	Type         string
	Autocomplete string
}

func tplInputOpts(name, placeholder, type_, autocomplete string) InputOpts {
	if type_ != "password" && type_ != "text" {
		panic("InputOpts type must be either `password` or `text`")
	}

	return InputOpts{
		name,
		placeholder,
		type_,
		autocomplete,
	}
}

const navbarClasses = "px-3 py-1 rounded-md flex items-center gap-2 transition-colors focus:outline-none hocus:text-white max-sm:px-2 max-sm:py-2 "
const navbarInactiveClasses = "hocus:bg-gray-700/50"
const navbarActiveClasses = "text-white bg-gray-700/80"

func tplNavbar() string {
	return navbarClasses
}

func tplNavbarActive(activeTab, tab string) string {
	if activeTab != tab {
		return navbarInactiveClasses
	}

	return navbarActiveClasses
}

type FlashType string

const (
	FlashTypeSuccess FlashType = "success"
	FlashTypeError   FlashType = "error"
	FlashTypeInfo    FlashType = "info"
)

type Flash struct {
	Message string
	Type    FlashType
}

func NewFlash(type_ FlashType, message string) Flash {
	return Flash{
		Message: message,
		Type:    type_,
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
