package bot

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"

	"euphoria.io/adbot/sys"
	"euphoria.io/heim/proto"
)

type Caller struct {
	Nick    string
	UserID  proto.UserID
	Host    bool
	Account bool
}

type Command struct {
	Name string
	Args []string

	line    string
	indices []int
}

func (cmd *Command) Rest(idx int) string {
	if idx < 0 {
		idx = 0
	}
	if idx >= len(cmd.indices) {
		return ""
	}
	return cmd.line[cmd.indices[idx]:]
}

type CommandHandler func(*Caller, *Command, ReplyFunc) error

type CommandSpeechHandler struct {
	AdminCommands   map[string]CommandHandler
	AccountCommands map[string]CommandHandler
	GeneralCommands map[string]CommandHandler
}

func (ch *CommandSpeechHandler) HandleSpeech(msg *proto.Message, reply ReplyFunc) error {
	line := strings.TrimSpace(msg.Content)
	if !strings.HasPrefix(line, "!") {
		return nil
	}

	kind, _ := msg.Sender.ID.Parse()
	caller := &Caller{
		Nick:    msg.Sender.Name,
		UserID:  msg.Sender.ID,
		Host:    msg.Sender.IsManager,
		Account: kind == "account",
	}

	cmd := Parse(line)
	if caller.Host {
		if h, ok := ch.AdminCommands[cmd.Name]; ok {
			return h(caller, cmd, reply)
		}
	}

	if caller.Account {
		if h, ok := ch.AccountCommands[cmd.Name]; ok {
			return h(caller, cmd, reply)
		}
	}

	if h, ok := ch.GeneralCommands[cmd.Name]; ok {
		return h(caller, cmd, reply)
	}

	return reply("I don't know the !%s command, try !help for help", cmd.Name)
}

var commandArgPattern = regexp.MustCompile(`(\s*)(\S+)`)

func Parse(line string) *Command {
	parts := commandArgPattern.FindAllStringSubmatch(line, -1)
	idxs := commandArgPattern.FindAllStringSubmatchIndex(line, -1)
	cmd := &Command{
		line:    line,
		indices: make([]int, len(idxs)),
	}
	if parts == nil || idxs == nil {
		return cmd
	}

	for i, x := range idxs {
		cmd.indices[i] = x[4]
	}

	if len(parts) > 0 {
		cmd.Args = make([]string, len(parts)-1)
	}
	for i, part := range parts {
		if i == 0 {
			cmd.Name = strings.ToLower(part[2][1:])
		} else {
			cmd.Args[i-1] = part[2]
		}
	}
	return cmd
}

func BindCommands(handler interface{}) *CommandSpeechHandler {
	commandHandlerType := reflect.TypeOf(CommandHandler(nil))

	handlerValue := reflect.ValueOf(handler)
	handlerType := reflect.TypeOf(handler)
	for handlerType.Kind() == reflect.Ptr {
		handlerType = handlerType.Elem()
	}
	if handlerType.Kind() != reflect.Struct {
		panic(fmt.Sprintf("expected struct or pointer to struct, got %T", handler))
	}

	csh := &CommandSpeechHandler{
		AdminCommands:   map[string]CommandHandler{},
		AccountCommands: map[string]CommandHandler{},
		GeneralCommands: map[string]CommandHandler{},
	}

	handlerType = reflect.PtrTo(handlerType)
	n := handlerType.NumMethod()
	for i := 0; i < n; i++ {
		method := handlerType.Method(i)
		if !strings.HasPrefix(method.Name, "Cmd") {
			continue
		}

		methodValue := handlerValue.MethodByName(method.Name)
		if !methodValue.IsValid() || !methodValue.Type().ConvertibleTo(commandHandlerType) {
			continue
		}

		cmdHandler, ok := methodValue.Convert(commandHandlerType).Interface().(CommandHandler)
		if !ok {
			continue
		}

		if strings.HasPrefix(method.Name, "CmdAdmin") {
			name := strings.ToLower(method.Name[len("CmdAdmin"):])
			csh.AdminCommands[name] = cmdHandler
		} else if strings.HasPrefix(method.Name, "CmdGeneral") {
			name := strings.ToLower(method.Name[len("CmdGeneral"):])
			csh.GeneralCommands[name] = cmdHandler
		} else {
			name := strings.ToLower(method.Name[len("Cmd"):])
			csh.AccountCommands[name] = cmdHandler
		}
	}

	return csh
}

type GeneralCommands struct {
	Bot *Bot
}

func (c *GeneralCommands) CmdGeneralHelp(caller *Caller, cmd *Command, reply ReplyFunc) error {
	return reply("no help available")
}

type ControlRoomCommands struct {
	GeneralCommands
}

func (c *ControlRoomCommands) CmdAdminJoin(caller *Caller, cmd *Command, reply ReplyFunc) error {
	if len(cmd.Args) != 1 {
		return reply("usage: !join ROOM")
	}

	roomName := cmd.Args[0]
	joined, err := c.Bot.Join(roomName)
	if err != nil {
		return reply("error joining %s: %s", roomName, err)
	}

	if joined {
		return reply("now tracking &%s", roomName)
	} else {
		return reply("already tracking &%s", roomName)
	}
}

func (c *ControlRoomCommands) CmdAdminShutdown(caller *Caller, cmd *Command, reply ReplyFunc) error {
	if err := reply("bye!"); err != nil {
		c.Bot.ctx.Terminate(fmt.Errorf("error during shutdown requested by %s(%s): %s", caller.Nick, caller.UserID, err))
		return err
	}
	c.Bot.ctx.Terminate(fmt.Errorf("shutdown requested by %s(%s)", caller.Nick, caller.UserID))
	return nil
}

func (c *ControlRoomCommands) CmdAdminRegister(caller *Caller, cmd *Command, reply ReplyFunc) error {
	if len(cmd.Args) != 1 {
		return reply("usage: !register EMAIL")
	}
	email := cmd.Args[0]
	if err := sys.Register(c.Bot.DB, c.Bot.ctrlRoom.c, email); err != nil {
		return reply("error: %s", err)
	}
	return reply(`registered account under %s, please check email for verification URL and tell me "!verify URL"`, email)
}

func (c *ControlRoomCommands) CmdAdminVerify(caller *Caller, cmd *Command, reply ReplyFunc) error {
	if len(cmd.Args) != 1 {
		return reply("usage: !verify URL")
	}
	url := cmd.Args[0]
	if err := sys.Verify(c.Bot.DB, c.Bot.ctrlRoom.c, url); err != nil {
		return reply("error: %s", err)
	}
	return reply("verified!")
}
