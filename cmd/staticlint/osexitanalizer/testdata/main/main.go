package main

import (
	"flag"
	"fmt"
	"log"
	"os"
)

func main() {
	fmt.Println("main called")
	var environment string
	var logLevel string
	flag.StringVar(&logLevel, "l", "info", "logging level")
	flag.StringVar(&environment, "e", "dev", "environment")
	flag.Parse()
	if environment == "" {
		log.Fatal("environment not set")
	}
	if logLevel == "" {
		panic("log level not set")
	}
	os.Exit(123) // want "direct call to os.Exit in main function is forbidden"
}
