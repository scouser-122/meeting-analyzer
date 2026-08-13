package pkg1

import (
	"log"
	"os"
)

func osExitCheckFunc() {
	os.Exit(123)
}

func logFatalCheckFunc(environment string) {
	if environment == "" {
		log.Fatal("environment not set") // want "direct call to log.Fatal not in main function is forbidden"
	}
}

func panicCheckFunc(logLevel string) {
	if logLevel == "" {
		panic("logLevel not set") // want "direct call to panic not in main function is forbidden"
	}
}
