package main

import (
	"os"

	"agentattest.dev/agentattest/internal/app"
)

func main() {
	os.Exit(app.Main(os.Args[1:], os.Stdout, os.Stderr))
}
