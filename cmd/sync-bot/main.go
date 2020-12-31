package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	d2g "github.com/densestvoid/discord-to-groupme"
)

func main() {
	var configFilename string
	flag.StringVar(&configFilename, "c", "config.json", "specifies the name of the config file to use")
	flag.Parse()
	config, err := d2g.ReadConfig(configFilename)
	if err != nil {
		fmt.Println("failed to start: ", err)
		return
	}

	app := d2g.NewApp(config)
	finsihSig, err := app.Start()
	if err != nil {
		fmt.Println("failed to start: ", err)
		return
	}
	sysSigs := make(chan os.Signal, 1)
	signal.Notify(sysSigs, syscall.SIGINT, syscall.SIGTERM)
	select {
	case <-sysSigs:
		app.Stop()
	case <-finsihSig:
	}
}
