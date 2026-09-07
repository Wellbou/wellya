package main

import (
	"fmt"
	"os"

	"github.com/godbus/dbus/v5"
)

var cliCommands = map[string]string{
	"--next":   "Next",
	"--prev":   "Previous",
	"--play":   "Play",
	"--pause":  "Pause",
	"--toggle": "PlayPause",
	"--stop":   "Stop",
}

func handleCliCommand() bool {
	if len(os.Args) < 2 {
		return false
	}
	method, ok := cliCommands[os.Args[1]]
	if !ok {
		return false
	}
	if err := sendMPRIS(method); err != nil {
		fmt.Fprintln(os.Stderr, "wellya:", err)
		os.Exit(1)
	}
	return true
}

func sendMPRIS(method string) error {
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return err
	}
	defer conn.Close()

	obj := conn.Object("org.mpris.MediaPlayer2.wellya", "/org/mpris/MediaPlayer2")
	call := obj.Call("org.mpris.MediaPlayer2.Player."+method, 0)
	return call.Err
}
