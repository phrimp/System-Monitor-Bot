package logger

import (
	"log"
	"os"
)

var (
	infoLogger  *log.Logger
	errorLogger *log.Logger
	warnLogger  *log.Logger
)

func Init() {
	infoLogger = log.New(os.Stdout, "INFO: ", log.Ldate|log.Ltime|log.Lshortfile)
	errorLogger = log.New(os.Stderr, "ERROR: ", log.Ldate|log.Ltime|log.Lshortfile)
	warnLogger = log.New(os.Stdout, "WARN: ", log.Ldate|log.Ltime|log.Lshortfile)
	Info("Logger initialized successfully")
}

func Info(v ...interface{}) {
	infoLogger.Println(v...)
}

func Error(v ...interface{}) {
	errorLogger.Println(v...)
}

func Warn(v ...interface{}) {
	warnLogger.Println(v...)
}

func Fatal(v ...interface{}) {
	errorLogger.Fatal(v...)
}
