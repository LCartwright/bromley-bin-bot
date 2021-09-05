package main

import (
	"go.uber.org/zap"
	"os"
	"strconv"
	"time"

	"github.com/lcartwright/bromley-bin-bot/services/binfetcher"
	"github.com/lcartwright/bromley-bin-bot/services/bot"
)

var (
	logger, _ = zap.NewDevelopment()
)

func main() {
	fetchURL, err := strconv.ParseBool(os.Getenv("FETCH_URL"))
	if err != nil {
		logger.Fatal("error fetching FETCH_URL env var", zap.Error(err))
	}
	apiToken := os.Getenv("API_TOKEN")
	if apiToken == "" {
		logger.Fatal("error fetching API_TOKEN env var", zap.Error(err))
	}
	bf := binfetcher.NewBinFetcher(fetchURL)
	ticker := time.NewTicker(24 * time.Hour)
	go func() {
		bf.UpdateBins()
		for {
			<- ticker.C
			bf.UpdateBins()
		}
	}()
	b := bot.NewBot(apiToken, bf)
	b.StartAndListen()
}


