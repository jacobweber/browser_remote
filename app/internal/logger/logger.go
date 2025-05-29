package logger

import (
	"fmt"
	"log"
	"os"
)

type Logger struct {
	file *os.File
	// Trace logs general information messages.
	Trace *log.Logger
	// Error logs error messages.
	Error *log.Logger
}

func NewLogger() Logger {
	file, err := os.OpenFile("browser_remote.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Println("Unable to create and/or open log file.")
		os.Exit(1)
	}
	trace := log.New(file, "TRACE: ", log.Ldate|log.Ltime|log.Lshortfile)
	error := log.New(file, "ERROR: ", log.Ldate|log.Ltime|log.Lshortfile)

	return Logger{
		file:  file,
		Trace: trace,
		Error: error,
	}
}

func (l *Logger) Cleanup() {
	l.file.Close()
}
