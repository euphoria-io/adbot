package bot

import "euphoria.io/heim/proto"

type ReplyFunc func(string, ...interface{}) error

type SpeechHandler interface {
	HandleSpeech(*proto.Message, ReplyFunc) error
}
