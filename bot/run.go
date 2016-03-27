package bot

import (
	"flag"
	"fmt"
	"os"
	"os/signal"

	"euphoria.io/scope"
)

func Run() {
	cfg := &Config{}
	flags := cfg.FlagSet()
	if err := flags.Parse(os.Args[1:]); err != nil {
		if err == flag.ErrHelp {
			os.Exit(0)
		}
		os.Exit(1)
	}

	ctx := scope.New()
	interrupt := make(chan os.Signal)
	signal.Notify(interrupt, os.Interrupt, os.Kill)

	bot := New(cfg)
	bot.Serve(ctx)

	exitCode := 0
loop:
	for {
		select {
		case sig := <-interrupt:
			err := fmt.Errorf("%s signal", sig)
			fmt.Println()
			fmt.Println(err.Error())
			ctx.Terminate(err)
			break loop
		case <-ctx.Done():
			fmt.Printf("terminated: %s\n", ctx.Err())
			exitCode = 3
			break loop
		}
	}

	fmt.Println("Shutting down...")
	ctx.WaitGroup().Wait()
	os.Exit(exitCode)
}
