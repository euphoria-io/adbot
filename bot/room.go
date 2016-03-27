package bot

import (
	"fmt"
	"sync"

	"euphoria.io/heim-client/client"
	"euphoria.io/heim/proto"
	"euphoria.io/scope"
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
	Config        *Config
	Name          string
	SpeechHandler SpeechHandler

	c               *client.Client
	joined          bool
	hosts           SessionSet
	sessionsByIdEra map[string]SessionSet
}

func (r *Room) IsControlRoom() bool { return r.Name == r.Config.ControlRoom }

func (r *Room) Dial(ctx scope.Context) error {
	r.Lock()
	defer r.Unlock()

	r.hosts = SessionSet{}
	r.sessionsByIdEra = map[string]SessionSet{}
	if r.c != nil {
		r.c.Close()
		r.c = nil
	}

	conn, err := client.DialRoom(ctx, r.Config.BaseURL, r.Name)
	if err != nil {
		return err
	}

	r.c = conn
	r.c.Add(r)
	return nil
}

func (r *Room) SnapshotEvent(event *proto.SnapshotEvent) error {
	r.Lock()
	defer r.Unlock()

	r.joined = true

	for _, session := range event.Listing {
		r.addSession(session)
	}

	_, err := r.c.Send(proto.NickType, proto.NickCommand{Name: r.Config.DefaultNick})
	return err
}

func (r *Room) addSession(session proto.SessionView) {
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
}

func (r *Room) JoinEvent(event *proto.PresenceEvent) error {
	r.addSession(proto.SessionView(*event))
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
