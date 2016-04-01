package bot

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"euphoria.io/adbot/sys"
	"euphoria.io/heim-client/client"
	"euphoria.io/heim/proto"
	"euphoria.io/scope"
)

const (
	MinBackoff = time.Second
	MaxBackoff = 10 * time.Second
)

type SessionSet map[string]struct{}

func (ss *SessionSet) Add(sessionID string) { (*ss)[sessionID] = struct{}{} }

func (ss *SessionSet) Remove(sessionID string) {
	if ss != nil {
		delete(*ss, sessionID)
	}
}

func (ss SessionSet) Contains(sessionID string) bool {
	_, ok := ss[sessionID]
	return ok
}

type Room struct {
	sync.Mutex
	Bot           *Bot
	Config        *Config
	CookieJar     http.CookieJar
	Name          string
	SpeechHandler SpeechHandler

	c               *client.Client
	ctx             scope.Context
	backoff         time.Duration
	joined          bool
	hosts           SessionSet
	sessionsByIdEra map[string]SessionSet
}

func (r *Room) IsControlRoom() bool {
	for _, roomName := range strings.Split(r.Config.ControlRooms, ",") {
		if roomName == r.Name {
			return true
		}
	}
	return false
}

func (r *Room) Close() error {
	r.ctx.Terminate(fmt.Errorf("parting"))
	return nil
}

func (r *Room) Dial(ctx scope.Context) {
	r.ctx = ctx
	r.Redial()
}

func (r *Room) Redial() {
	r.Lock()
	if r.c != nil {
		r.c.Close()
		r.c = nil
	}
	r.hosts = SessionSet{}
	r.sessionsByIdEra = map[string]SessionSet{}
	r.Unlock()

	for {
		delay := r.backoff
		r.backoff *= 2
		if r.backoff < MinBackoff {
			r.backoff = MinBackoff
		} else if r.backoff > MaxBackoff {
			r.backoff = MaxBackoff
		}

		select {
		case <-time.After(delay):
		case <-r.ctx.Done():
			return
		}

		conn, err := client.DialRoom(r.ctx, r.Config.BaseURL, r.Name, r.CookieJar)
		if err != nil {
			fmt.Printf("error dialing %s: %s", r.Name, err)
			continue
		}

		r.c = conn
		break
	}

	r.c.Add(r)
}

func (r *Room) UserCount() int {
	n := 0
	for _, ss := range r.sessionsByIdEra {
		n += len(ss)
	}
	return n
}

func (r *Room) DisconnectEvent(event *proto.DisconnectEvent) error {
	fmt.Printf("disconnected from %s due to '%s', reconnecting\n", r.Name, event.Reason)
	go func() {
		if event.Reason == "authentication changed" {
			r.backoff = 0
		} else {
			r.backoff = MinBackoff
		}
		r.Redial()
	}()
	return nil
}

func (r *Room) SnapshotEvent(event *proto.SnapshotEvent) error {
	r.Lock()
	defer r.Unlock()

	r.joined = true

	for _, session := range event.Listing {
		if err := r.addSession(session); err != nil {
			return err
		}
	}

	if r.Config.Ghost && !r.IsControlRoom() {
		return nil
	}
	_, err := r.c.Send(proto.NickType, proto.NickCommand{Name: r.Config.DefaultNick})
	return err
}

func (r *Room) addSession(session proto.SessionView) error {
	if kind, _ := session.ID.Parse(); kind == "bot" {
		return nil
	}
	key := fmt.Sprintf("%s:%s", session.ServerID, session.ServerEra)
	sessions, ok := r.sessionsByIdEra[key]
	if !ok {
		sessions = SessionSet{}
		r.sessionsByIdEra[key] = sessions
	}
	sessions.Add(session.SessionID)

	if session.IsManager {
		r.hosts.Add(session.SessionID)
	}

	kind, _ := session.ID.Parse()
	if kind == "account" && r.IsControlRoom() {
		return sys.SetNick(r.Bot.DB, session.ID, session.Name)
	}
	return nil
}

func (r *Room) JoinEvent(event *proto.PresenceEvent) error {
	return r.addSession(proto.SessionView(*event))
}

func (r *Room) NickEvent(event *proto.NickEvent) error {
	kind, _ := event.ID.Parse()
	if kind == "account" && r.IsControlRoom() {
		return sys.SetNick(r.Bot.DB, event.ID, event.To)
	}
	return nil
}

func (r *Room) PartEvent(event *proto.PresenceEvent) error {
	r.Lock()
	defer r.Unlock()

	r.hosts.Remove(event.SessionID)

	if sessions, ok := r.sessionsByIdEra[fmt.Sprintf("%s:%s", event.ServerID, event.ServerEra)]; ok {
		sessions.Remove(event.SessionID)
	}

	return nil
}

func (r *Room) NetworkEvent(event *proto.NetworkEvent) error {
	r.Lock()
	defer r.Unlock()

	if event.Type == "partition" {
		key := fmt.Sprintf("%s:%s", event.ServerID, event.ServerEra)
		for sessionID, _ := range r.sessionsByIdEra[key] {
			r.hosts.Remove(sessionID)
		}
		delete(r.sessionsByIdEra, key)
	}

	return nil
}

func (r *Room) SendEvent(event *proto.SendEvent) error {
	if r.SpeechHandler == nil {
		return nil
	}

	reply := func(format string, args ...interface{}) error {
		msg := proto.Message{
			Parent: event.ID,
		}
		if len(args) == 0 {
			msg.Content = format
		} else {
			msg.Content = fmt.Sprintf(format, args...)
		}
		_, err := r.c.AsyncSend(proto.SendType, msg)
		return err
	}

	return r.SpeechHandler.HandleSpeech((*proto.Message)(event), reply)
}
