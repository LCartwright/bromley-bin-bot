package bot

import (
	"fmt"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"go.uber.org/zap"
	"log"
	"strings"

	"github.com/lcartwright/bromley-bin-bot/services/binfetcher"
)

type Bot struct {
	botAPI     *tgbotapi.BotAPI
	binFetcher binfetcher.BinFetcherer
}

var (
	logger, _ = zap.NewDevelopment()
)

func NewBot(apiToken string, bf binfetcher.BinFetcherer) *Bot {
	bot, err := tgbotapi.NewBotAPI(apiToken)
	if err != nil {
		logger.Fatal("error starting new bot", zap.Error(err))
	}
	bot.Debug = true
	logger.Sugar().Infof("Authorized on account %s", bot.Self.UserName)
	return &Bot{
		botAPI:     bot,
		binFetcher: bf,
	}
}

func (b *Bot) StartAndListen() {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates, err := b.botAPI.GetUpdatesChan(u)
	if err != nil {
		logger.Fatal("error getting initial updates for bot", zap.Error(err))
	}
	for update := range updates {
		if update.Message == nil { // ignore any non-Message Updates
			continue
		}
		log.Printf("[%s] %s", update.Message.From.UserName, update.Message.Text)
		nextCollectionMessage := nextCollectionMessage(b.binFetcher.NextCollection())
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, nextCollectionMessage)
		msg.ReplyToMessageID = update.Message.MessageID
		b.botAPI.Send(msg)
	}
}

func nextCollectionMessage(bc *binfetcher.BinCollections) string {
	if bc == nil || bc.Date.IsZero() {
		return "Can't find out the next collections, oh no!"
	}
	return fmt.Sprintf("Next collection date is %s and will be collecting %s.", bc.Date.Format("Monday, 2 January 2006"), strings.Join(bc.CollectionTypes, ", "))
}
