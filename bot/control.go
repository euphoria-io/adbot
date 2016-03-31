package bot

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
	"text/tabwriter"

	"euphoria.io/adbot/sys"
	"euphoria.io/heim/proto"
)

type ControlRoomCommands struct {
	GeneralCommands
}

func (c *ControlRoomCommands) CmdAdminCampaign(caller *Caller, cmd *Command, reply ReplyFunc) error {
	fmtKeywords := func(keywords sys.WordList) string {
		buf := &bytes.Buffer{}
		for k, _ := range keywords {
			if buf.Len() > 0 {
				buf.WriteRune(',')
			}
			buf.WriteString(k)
		}
		keywordString := buf.String()
		if len(keywordString) > 50 {
			keywordString = keywordString[:50] + "..."
		}
		return keywordString
	}

	allCampaigns := func() error {
		buf := &bytes.Buffer{}
		w := tabwriter.NewWriter(buf, 5, 0, 2, ' ', 0)
		fmt.Fprintln(w, "Account\tCreative\tMax Bid\tKeywords")
		err := sys.MapSpends(c.Bot.DB, func(spend sys.Spend) error {
			_, id := spend.UserID.Parse()
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", id, spend.CreativeName, spend.MaxBid, fmtKeywords(spend.Keywords))
			return nil
		})
		if err != nil {
			return reply("error: %s", err)
		}
		w.Flush()
		return reply(buf.String())
	}

	userCampaigns := func() error {
		userID := proto.UserID(cmd.Args[0])
		spends, err := sys.Spends(c.Bot.DB, userID)
		if err != nil {
			return reply("error: %s", err)
		}
		buf := &bytes.Buffer{}
		w := tabwriter.NewWriter(buf, 5, 0, 2, ' ', 0)
		fmt.Fprintln(w, "Creative\tMax Bid\tKeywords")
		for _, spend := range spends {
			fmt.Fprintf(w, "%s\t%s\t%s\n", spend.CreativeName, spend.MaxBid, fmtKeywords(spend.Keywords))
		}
		w.Flush()
		return reply(buf.String())
	}

	switch len(cmd.Args) {
	case 0:
		return allCampaigns()
	case 1:
		return userCampaigns()
	default:
		return reply("usage: !campaign [USERID]")
	}
}

func (c *ControlRoomCommands) CmdAdminCredit(caller *Caller, cmd *Command, reply ReplyFunc) error {
	if len(cmd.Args) < 2 {
		return reply("usage: !credit USERID AMOUNT [MEMO]")
	}

	userID := proto.UserID(cmd.Args[0])
	credit, err := ParseCents(cmd.Args[0])
	if err != nil {
		return reply("error: %s", err)
	}

	_, toBalance, err := sys.Transfer(c.Bot.DB, credit, sys.House, userID, cmd.Rest(3), true)
	if err != nil {
		return reply("error: %s", err)
	}

	return reply("credited %s for %s, balance now %s", userID, credit, toBalance)
}

func (c *ControlRoomCommands) CmdAdminDisable(caller *Caller, cmd *Command, reply ReplyFunc) error {
	return c.setEnabled(caller, cmd, reply)
}

func (c *ControlRoomCommands) CmdAdminEnable(caller *Caller, cmd *Command, reply ReplyFunc) error {
	return c.setEnabled(caller, cmd, reply)
}

func (c *ControlRoomCommands) setEnabled(caller *Caller, cmd *Command, reply ReplyFunc) error {
	setUserSpend := func() error {
		userID := proto.UserID(cmd.Args[0])
		if err := sys.SetUserEnabled(c.Bot.DB, userID, cmd.Name == "enable"); err != nil {
			return reply("error: %s", err)
		}
		return reply("%sd all spends by %s", cmd.Name, userID)
	}

	setSpend := func() error {
		userID := proto.UserID(cmd.Args[0])
		creative := cmd.Args[1]
		if err := sys.SetSpendEnabled(c.Bot.DB, userID, creative, cmd.Name == "enable"); err != nil {
			return reply("error: %s", err)
		}
		return reply("%sd spend %s by %s", cmd.Name, creative, userID)
	}

	switch len(cmd.Args) {
	case 1:
		return setUserSpend()
	case 2:
		return setSpend()
	default:
		return reply("usage: !%s USERID [CREATIVE]", cmd.Name)
	}
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

func (c *ControlRoomCommands) CmdAdminPart(caller *Caller, cmd *Command, reply ReplyFunc) error {
	if len(cmd.Args) != 1 {
		return reply("usage: !part ROOM")
	}

	roomName := cmd.Args[0]
	removed, err := c.Bot.Part(roomName)
	if err != nil {
		return reply("error: %s", err)
	}
	if !removed {
		return reply("not tracking &%s", roomName)
	}
	return reply("no longer tracking &%s", roomName)
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

func (c *ControlRoomCommands) CmdAdminReset(caller *Caller, cmd *Command, reply ReplyFunc) error {
	resetBalances := func() error {
		if err := sys.ResetBalances(c.Bot.DB); err != nil {
			return reply("error: %s", err)
		}
		return reply("reset all balances")
	}

	resetCampaigns := func() error {
		if err := sys.ResetCampaigns(c.Bot.DB); err != nil {
			return reply("error: %s", err)
		}
		return reply("reset all campaigns")
	}

	if len(cmd.Args) != 1 {
		return reply("usage: !reset balances|campaigns")
	}
	switch cmd.Args[0] {
	case "balances":
		return resetBalances()
	case "campaigns":
		return resetCampaigns()
	default:
		return reply("usage: !reset balances|campaigns")
	}
}

func (c *ControlRoomCommands) CmdAdminRooms(caller *Caller, cmd *Command, reply ReplyFunc) error {
	rooms, err := sys.Rooms(c.Bot.DB)
	if err != nil {
		return reply("error: %s", err)
	}
	if len(rooms) == 0 {
		return reply("no rooms configured")
	}
	return reply("&" + strings.Join(rooms, ", &"))
}

func (c *ControlRoomCommands) CmdAdminShutdown(caller *Caller, cmd *Command, reply ReplyFunc) error {
	if err := reply("bye!"); err != nil {
		c.Bot.ctx.Terminate(fmt.Errorf("error during shutdown requested by %s(%s): %s", caller.Nick, caller.UserID, err))
		return err
	}
	c.Bot.ctx.Terminate(fmt.Errorf("shutdown requested by %s(%s)", caller.Nick, caller.UserID))
	return nil
}

func (c *ControlRoomCommands) CmdAdminStimulate(caller *Caller, cmd *Command, reply ReplyFunc) error {
	if len(cmd.Args) != 1 {
		return reply("usage: !stimulate AMOUNT")
	}

	stimulusStr := cmd.Args[0]
	stimulus, err := ParseCents(stimulusStr)
	if err != nil {
		return reply("invalid stimulus: %s", stimulusStr)
	}

	if err := sys.AddStimulus(c.Bot.DB, stimulus); err != nil {
		return reply("error: %s", err)
	}

	return reply("stimulus package rolled out")
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

func (c *ControlRoomCommands) CmdScoreboard(caller *Caller, cmd *Command, reply ReplyFunc) error {
	sb := Scoreboard{}
	if err := sb.Load(c.Bot.DB); err != nil {
		return reply("error: %s", err)
	}
	sort.Reverse(sb)

	if len(sb) > 10 {
		sb = sb[:10]
	}
	for i, entry := range sb {
		user, err := sys.GetAdvertiser(c.Bot.DB, entry.UserID)
		if err != nil {
			return reply("error: %s", err)
		}
		sb[i].Name = user.Nick
	}

	buf := &bytes.Buffer{}
	if err := sb.WriteTo(buf); err != nil {
		return reply("error: %s", err)
	}
	return reply(buf.String())
}

func (c *ControlRoomCommands) CmdSpend(caller *Caller, cmd *Command, reply ReplyFunc) error {
	if len(cmd.Args) < 6 || cmd.Args[0] != "up" || cmd.Args[1] != "to" || cmd.Args[3] != "on" {
		return reply("usage: !spend up to MAXBID on CREATIVE KEYWORDS...")
	}
	maxBidStr := cmd.Args[2]
	maxBid, err := ParseCents(maxBidStr)
	if err != nil {
		return reply("invalid max bid: %s", maxBidStr)
	}
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

func (c *ControlRoomCommands) CmdStats(caller *Caller, cmd *Command, reply ReplyFunc) error {
	userID := caller.UserID
	if caller.Host {
		userID = sys.House
	}
	if len(cmd.Args) > 0 {
		userID = proto.UserID(cmd.Args[0])
	}
	m, err := sys.LoadMetrics(c.Bot.DB, userID)
	if err != nil {
		return reply("error: %s", err)
	}

	buf := &bytes.Buffer{}
	w := tabwriter.NewWriter(buf, 5, 0, 2, ' ', tabwriter.AlignRight)
	fmt.Fprintf(w, "Ads displayed:\t%d\n", m.AdsDisplayed)
	fmt.Fprintf(w, "Impressions:\t%d\n", m.Impressions)
	if userID == sys.System {
		fmt.Fprintf(w, "Total revenue:\t%s\n", sys.Cents(m.AmountSpent-m.AmountSpentByHouse))
	} else {
		fmt.Fprintf(w, "Total spent:\t%s\n", sys.Cents(m.AmountSpent))
	}
	if m.Impressions > 0 {
		fmt.Fprintf(w, "CPI:\t%s\n", sys.Cents(m.AmountSpent/m.Impressions))
	}
	w.Flush()
	return reply(buf.String())
}
