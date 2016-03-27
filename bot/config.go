package bot

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

type Config struct {
	BaseURL     string
	ControlRoom string
	DBPath      string
	DefaultNick string
}

func (cfg *Config) FlagSet() *flag.FlagSet {
	cmdName := filepath.Base(os.Args[0])
	flags := flag.NewFlagSet(cmdName, flag.ContinueOnError)
	flags.StringVar(&cfg.BaseURL, "baseURL", "https://euphoria.io", "base websocket URL for euphoria")
	flags.StringVar(&cfg.ControlRoom, "controlRoom", "ads", "name of room where admin commands are given")
	flags.StringVar(&cfg.DBPath, "db", "adbot.db", "path to database file")
	flags.StringVar(&cfg.DefaultNick, "defaultNick", "AdBot", "name to use in control room")
	flags.Usage = func() {
		fmt.Printf("usage: %s OPTIONS\n\n", cmdName)
		flags.PrintDefaults()
		fmt.Println()
	}
	return flags
}
