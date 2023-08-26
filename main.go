package main

import (
	"AlainDebot/db"
	"AlainDebot/bot"
	"os"

	_ "github.com/jackc/pgx/v5/stdlib"
	"go.uber.org/zap"
)

func initLogger(isDev bool) {
	var logger *zap.Logger
	if isDev {
		logger = zap.Must(zap.NewDevelopment())
	} else {
		logger = zap.Must(zap.NewProduction())
	}

	zap.ReplaceGlobals(logger)
}

func main() {
	dev := os.Getenv("DEV")
	connStr := os.Getenv("POSTGRES_URI")
	tgToken := os.Getenv("TG_TOKEN")

	initLogger(dev == "")
	db.Init(connStr)
	bot.Init(tgToken)

	bot.Run()
}
