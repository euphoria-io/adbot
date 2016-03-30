package bot

import (
	"bytes"
	"fmt"
	"strconv"
	"text/tabwriter"

	"euphoria.io/adbot/sys"
	"euphoria.io/heim/proto"
)

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
	for _, room := range c.Bot.ctrlRooms {
		if err := sys.Register(c.Bot.DB, room.c, email); err != nil {
			return reply("error: %s", err)
		}
		break
	}
	return nil
}

func (c *ControlRoomCommands) CmdAdminVerify(caller *Caller, cmd *Command, reply ReplyFunc) error {
	if len(cmd.Args) != 1 {
		return reply("usage: !verify URL")
	}
	url := cmd.Args[0]
	for _, room := range c.Bot.ctrlRooms {
		if err := sys.Verify(c.Bot.DB, room.c, url); err != nil {
			return reply("error: %s", err)
		}
		break
	}
	return reply("verified!")
}

func (c *ControlRoomCommands) CmdAdminCredit(caller *Caller, cmd *Command, reply ReplyFunc) error {
	if len(cmd.Args) < 2 {
		return reply("usage: !credit USERID AMOUNT [MEMO]")
	}

	userID := proto.UserID(cmd.Args[0])

	f, err := strconv.ParseFloat(cmd.Args[1], 64)
	if err != nil {
		return reply("error: %s", err)
	}

	credit := sys.Cents(f * 100)

	_, toBalance, err := sys.Transfer(c.Bot.DB, credit, sys.House, userID, cmd.Rest(3), true)
	if err != nil {
		return reply("error: %s", err)
	}

	return reply("credited %s for %s, balance now %s", userID, credit, toBalance)
}

func (c *ControlRoomCommands) CmdGeneralBalance(caller *Caller, cmd *Command, reply ReplyFunc) error {
	userID := caller.UserID
	if caller.Host {
		userID = sys.House
	}
	advertiser, err := sys.GetAdvertiser(c.Bot.DB, userID)
	if err != nil {
		return reply("error: %s", err)
	}
	return reply("your balance is %s", advertiser.Balance)
}

func (c *ControlRoomCommands) CmdGeneralLedger(caller *Caller, cmd *Command, reply ReplyFunc) error {
	userID := caller.UserID
	if caller.Host {
		if len(cmd.Args) > 0 {
			userID = proto.UserID(cmd.Args[0])
		} else {
			userID = sys.House
		}
	}

	ledger, err := sys.Ledger(c.Bot.DB, userID, 25)
	if err != nil {
		return reply("error: %s", err)
	}

	if len(ledger) == 0 {
		return reply("no transactions")
	}

	buf := &bytes.Buffer{}
	w := tabwriter.NewWriter(buf, 5, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tFrom\tTo\tMemo\tAmount\tBalance")
	for _, entry := range ledger {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n", entry.TxID, entry.From, entry.To, entry.Memo, entry.Cents, entry.Balance)
	}
	w.Flush()
	return reply(buf.String())
}

func (c *ControlRoomCommands) CmdCreative(caller *Caller, cmd *Command, reply ReplyFunc) error {
	if len(cmd.Args) < 2 {
		return reply("usage: !creative NAME COPY...")
	}
	userID := caller.UserID
	if caller.Host {
		userID = sys.House
	}
	name := cmd.Args[0]
	_, replaced, err := sys.NewCreative(c.Bot.DB, userID, name, cmd.Rest(2))
	if err != nil {
		return reply("error: %s", err)
	}
	verb := "added"
	if replaced {
		verb = "replaced"
	}
	return reply("%s creative %s, remove with !delete %s", verb, name, name)
}

func (c *ControlRoomCommands) CmdDelete(caller *Caller, cmd *Command, reply ReplyFunc) error {
	if len(cmd.Args) < 1 {
		return reply("usage: !delete CREATIVE")
	}
	userID := caller.UserID
	if caller.Host {
		userID = sys.House
	}
	name := cmd.Args[0]
	deleted, err := sys.DeleteCreative(c.Bot.DB, userID, name)
	if err != nil {
		return reply("error: %s")
	}
	if deleted {
		return reply("deleted creative %s", name)
	} else {
		return reply("creative %s does not exist", name)
	}
}

func (c *ControlRoomCommands) CmdSpend(caller *Caller, cmd *Command, reply ReplyFunc) error {
	if len(cmd.Args) < 6 || cmd.Args[0] != "up" || cmd.Args[1] != "to" || cmd.Args[3] != "on" {
		return reply("usage: !spend up to MAXBID on CREATIVE KEYWORDS...")
	}
	maxBidStr := cmd.Args[2]
	f, err := strconv.ParseFloat(maxBidStr, 64)
	if err != nil {
		return reply("invalid max bid: %s", maxBidStr)
	}
	maxBid := sys.Cents(f * 100)
	creativeName := cmd.Args[4]
	userID := caller.UserID
	if caller.Host {
		userID = sys.House
	}
	_, replaced, err := sys.NewSpend(c.Bot.DB, userID, creativeName, cmd.Rest(6), maxBid)
	if err != nil {
		return reply("error: %s", err)
	}
	verb := "added"
	if replaced {
		verb = "replaced"
	}
	return reply("%s spend on creative %s, remove with !cancel %s", verb, creativeName, creativeName)
}

func (c *ControlRoomCommands) CmdCancel(caller *Caller, cmd *Command, reply ReplyFunc) error {
	if len(cmd.Args) < 1 {
		return reply("usage: !cancel CREATIVE")
	}
	userID := caller.UserID
	if caller.Host {
		userID = sys.House
	}
	name := cmd.Args[0]
	deleted, err := sys.DeleteSpend(c.Bot.DB, userID, name)
	if err != nil {
		return reply("error: %s")
	}
	if deleted {
		return reply("cancelled spend %s", name)
	} else {
		return reply("spend on %s does not exist", name)
	}
}
