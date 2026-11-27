package main

import (
	"log"
	"os"
	"time"

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
		// The keychain may ask the listener first, a new build each time:
		// answer the client meanwhile, which would give up on a server
		// silent until then.
		ready := make(chan struct{})
		go func() {
			netease.RestoreSession()
			close(ready)
		}()
		mcp.SessionReady = ready
		// An agent may read playlist after playlist: space them, as the
		// app's pages never ask so fast.
		netease.ReadGap = 250 * time.Millisecond
		if err := mcp.Serve(os.Stdin, os.Stdout, mygo.App.Version()); err != nil {
			log.Fatal(err)
		}
		return
	}
	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
