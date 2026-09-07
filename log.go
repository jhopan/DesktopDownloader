package main

import (
	"fmt"
	"os"
	"path/filepath"
)

var logFile *os.File

func logInit() {
	os.MkdirAll(appDir, 0755)
	p := filepath.Join(appDir, "app.log")
	os.MkdirAll(appDir, 0755)
	f, err := os.OpenFile(p, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return
	}
	logFile = f
}

func logf(format string, args ...interface{}) {
	if logFile == nil {
		return
	}
	fmt.Fprintf(logFile, format+"\n", args...)
}

func logClose() {
	if logFile != nil {
		logFile.Close()
	}
}
