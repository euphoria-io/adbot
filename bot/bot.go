package bot

import (
	"strings"
	"sync"

	"euphoria.io/scope"

	"euphoria.io/adbot/sys"
)

func New(cfg *Config) (*Bot, error) {
	db, err := sys.Open(cfg.DBPath)
	if err != nil {
		return nil, err
	}

	bot := &Bot{
		Config: cfg,
		DB:     db,
	}
	return bot, nil
}

type Bot struct {
	sync.Mutex
	Config *Config
	DB     *sys.DB

	ctx      scope.Context
	ctrlRoom *Room
	rooms    map[string]*Room
}

func (b *Bot) NewRoom(roomName string) *Room {
	return &Room{
		Config:    b.Config,
		CookieJar: sys.CookieJar(b.DB),
		Name:      roomName,
	}
}

func (b *Bot) Serve(ctx scope.Context) error {
	b.Lock()
	defer b.Unlock()

	rooms, err := sys.Rooms(b.DB)
	if err != nil {
		return err
	}

	b.ctx = ctx
	b.ctrlRoom = b.NewRoom(b.Config.ControlRoom)
	b.ctrlRoom.SpeechHandler = BindCommands(&ControlRoomCommands{GeneralCommands{Bot: b}})
	b.ctrlRoom.Dial(b.ctx.Fork())

	b.rooms = map[string]*Room{}
	for _, roomName := range rooms {
		b.rooms[roomName] = b.NewRoom(roomName)
		b.rooms[roomName].Dial(b.ctx.Fork())
		b.rooms[roomName].SpeechHandler = &InventorySpeechHandler{
			Bot:  b,
			Room: b.rooms[roomName],
		}
	}

	return nil
}

func (b *Bot) Join(roomName string) (bool, error) {
	b.Lock()
	defer b.Unlock()

	roomName = strings.ToLower(roomName)
	ok, err := sys.Join(b.DB, roomName)
	if !ok || err != nil {
		return ok, err
	}

	if _, ok := b.rooms[roomName]; ok {
		return false, nil
	}

	if b.rooms == nil {
		b.rooms = map[string]*Room{}
	}
	b.rooms[roomName] = b.NewRoom(roomName)
	b.rooms[roomName].Dial(b.ctx.Fork())
	b.rooms[roomName].SpeechHandler = &InventorySpeechHandler{
		Bot:  b,
		Room: b.rooms[roomName],
	}
	return true, nil
}
