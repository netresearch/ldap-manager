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

const NavbarItemBaseClass = "px-2 py-1 "

func tplNavbarActive(activeTab, tab string) string {
	if activeTab == tab {
		return NavbarItemBaseClass + "text-white font-bold bg-gray-700 rounded-md"
	}

	return NavbarItemBaseClass
}
