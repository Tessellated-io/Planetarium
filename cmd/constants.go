package cmd

import "fmt"

// Binary name
const (
	binaryName        = "planetarium"
	productName       = "Planetarium"
	binaryIcon        = "🪐"
	defaultListenPort = 5353
)

// Versions and such
var (
	ProductVersion string
	GoVersion      string
	GitRevision    string
)

// Commands
var (
	rootHelp  = fmt.Sprintf("%s is a server to run the cosmos chain directory", productName)
	startHelp = fmt.Sprintf("Start a %s server", productName)
)
