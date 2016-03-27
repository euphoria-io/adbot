package bot

import (
	"strings"
	"sync"

	"euphoria.io/scope"
)

func New(cfg *Config) *Bot {
	return &Bot{
		Config: cfg,
	}
}

type Bot struct {
	sync.Mutex
	Config *Config

	ctx      scope.Context
	ctrlRoom *Room
	rooms    map[string]*Room
}

func (b *Bot) Serve(ctx scope.Context) {
	b.ctrlRoom = &Room{
		Config:        b.Config,
		Name:          b.Config.ControlRoom,
		SpeechHandler: BindCommands(&ControlRoomCommands{GeneralCommands{Bot: b}}),
	}
	b.ctx = ctx
	if err := b.ctrlRoom.Dial(b.ctx.Fork()); err != nil {
		ctx.Terminate(err)
	}
}

func (b *Bot) Join(roomName string) (bool, error) {
	b.Lock()
	defer b.Unlock()

	roomName = strings.ToLower(roomName)
	if _, ok := b.rooms[roomName]; ok {
		return false, nil
	}

	if b.rooms == nil {
		b.rooms = map[string]*Room{}
	}
	b.rooms[roomName] = &Room{
		Config: b.Config,
		Name:   roomName,
	}
	if err := b.rooms[roomName].Dial(b.ctx.Fork()); err != nil {
		return false, err
	}
	return true, nil
}
