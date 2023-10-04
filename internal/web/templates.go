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

const navbarClasses = "px-3 max-sm:py-2 py-1 rounded-md flex items-center gap-2 transition-colors hocus:text-white"
const navbarInactiveClasses = "hocus:bg-gray-800"
const navbarActiveClasses = "text-white bg-gray-700"

func tplNavbar() string {
	return navbarClasses
}

func tplNavbarActive(activeTab, tab string) string {
	if activeTab != tab {
		return navbarInactiveClasses
	}

	return navbarActiveClasses
}
