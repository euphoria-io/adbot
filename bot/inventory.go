package bot

import (
	"fmt"
	"sync/atomic"

	"euphoria.io/adbot/sys"
	"euphoria.io/heim/proto"
)

func MinBid(userCount, msgsSinceLastAd int) sys.Cents {
	cost := sys.Cents(10 * userCount)
	if msgsSinceLastAd < 20 {
		cost *= 20 - sys.Cents(msgsSinceLastAd)
	}
	return cost
}

type InventorySpeechHandler struct {
	Bot             *Bot
	Room            *Room
	msgsSinceLastAd uint64
}

func (ish *InventorySpeechHandler) HandleSpeech(msg *proto.Message, reply ReplyFunc) error {
	impressions := ish.Room.UserCount()

	creative, cost, err := sys.Select(ish.Bot.DB, msg.Content, MinBid(impressions, int(ish.msgsSinceLastAd)))
	if err != nil {
		return err
	}
	if creative == nil {
		atomic.AddUint64(&ish.msgsSinceLastAd, 1)
		return nil
	}

	if creative.UserID != sys.House {
		memo := fmt.Sprintf("display %s at CPI of %s", creative.Name, cost/sys.Cents(impressions))
		_, _, err = sys.Transfer(ish.Bot.DB, cost, creative.UserID, sys.House, memo, true)
		if err != nil {
			return err
		}
	}

	atomic.StoreUint64(&ish.msgsSinceLastAd, 0)
	return reply("$$$ %s", creative.Content)
}
