package main

import (
	"fmt"
	"github.com/ShutovAndrey/weblocation/internal/database"
	"github.com/ShutovAndrey/weblocation/internal/server"
	"github.com/ShutovAndrey/weblocation/internal/services/collector"
	"github.com/ShutovAndrey/weblocation/internal/services/logger"
	"github.com/joho/godotenv"
	"github.com/robfig/cron/v3"
)

func init() {
	// get .env
	godotenv.Load()
}

func main() {
	logger.CreateLogger()
	defer logger.Close()

	if err := database.New(); err != nil {
		logger.Error(err)
	}

	fmt.Println("Collecting data. Please wait..")
	collector.GetOrUpdateData()

	c := cron.New()
	c.AddFunc("@daily", collector.GetOrUpdateData)
	c.Start()

	fmt.Println("Listening on :80")
	if err := server.Create(); err != nil {
		c.Stop()
		logger.Error(err)
	}
}
