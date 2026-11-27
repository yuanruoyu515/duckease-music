package main

import (
	"log"
	"os"

	"github.com/egoist/mygo"

	"netease-music-mygo/internal/app"
	"netease-music-mygo/internal/mcp"
	"netease-music-mygo/internal/netease"
)

func main() {
	// `DuckEase Music mcp` serves agents over stdio instead of opening
	// the app, with the session the app keeps.
	if len(os.Args) > 1 && os.Args[1] == "mcp" {
		log.SetOutput(os.Stderr)
		netease.RestoreSession()
		if err := mcp.Serve(os.Stdin, os.Stdout, mygo.App.Version()); err != nil {
			log.Fatal(err)
		}
		return
	}
	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
