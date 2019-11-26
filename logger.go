package main

import (
	log4 "github.com/fire988/log4go"
	"log"
)

// initLogger 初始化日志句柄。
func initLogger() {
	logger = make(log4.Logger)

	fileLogWriter := log4.NewNetLogWriter(cfg.App, cfg.SendURL)
	consoleWriter := log4.NewConsoleLogWriter()
	consoleWriter.SetFormat("[%T] (%S) %M")

	logger.AddFilter("stdout", log4.FINEST, consoleWriter)
	logger.AddFilter("logfile", log4.INFO, fileLogWriter)

	log.Println("Logger init")

	return
}
