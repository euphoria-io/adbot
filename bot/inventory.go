package bot

import (
	"fmt"
	"sync/atomic"

	"euphoria.io/adbot/sys"
	"euphoria.io/heim/proto"
)

func MinBid(userCount, msgsSinceLastAd int) sys.Cents {
	cost := sys.Cents(5 * userCount)
	if msgsSinceLastAd < 20 {
		cost *= 5 * (20 - sys.Cents(msgsSinceLastAd))
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

	if err := sys.Bill(ish.Bot.DB, ish.Room.Name, creative.UserID, cost, creative.Name, impressions); err != nil {
		return err
	}

	adv, err := sys.GetAdvertiser(ish.Bot.DB, creative.UserID)
	if err != nil {
		return err
	}

	content := fmt.Sprintf("/me delivered creative %s by %s to &%s at a price of %s", creative.Name, adv.Nick, ish.Room.Name, cost)
	if creative.UserID == sys.House {
		content = fmt.Sprintf("/me delivered house creative %s to &%s at a price of %s", creative.Name, ish.Room.Name, cost)
	}
	if ish.Bot.Config.Ghost {
		content += " (simulated)"
	}
	for _, room := range ish.Bot.ctrlRooms {
		_, err = room.c.AsyncSend(proto.SendType, proto.Message{
			Content: content,
		})
	}

	atomic.StoreUint64(&ish.msgsSinceLastAd, 0)

	if ish.Bot.Config.Ghost {
		return nil
	}
	return reply("sponsored message: %s", creative.Content)
}
