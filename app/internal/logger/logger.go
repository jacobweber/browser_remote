package logger

import (
	"fmt"
	"io"
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

func NewFile() *Logger {
	file, err := os.OpenFile("browser_remote.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Println("Unable to create and/or open log file.")
		os.Exit(1)
	}
	return New(file, file, nil)
}

func NewStdout() *Logger {
	return New(os.Stdout, os.Stderr, nil)
}

func New(traceHandle io.Writer, errorHandle io.Writer, file *os.File) *Logger {
	trace := log.New(traceHandle, "TRACE: ", log.Ldate|log.Ltime|log.Lshortfile)
	error := log.New(errorHandle, "ERROR: ", log.Ldate|log.Ltime|log.Lshortfile)

	return &Logger{
		file:  file,
		Trace: trace,
		Error: error,
	}
}

func (l *Logger) Cleanup() {
	if l.file != nil {
		l.file.Close()
	}
}
