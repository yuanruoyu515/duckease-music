package app

import (
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/egoist/mygo"
)

// logLimit is how large app.log grows before it is moved to app.log.1,
// replacing the one there, when the app starts.
const logLimit = 1 << 20

// logToFile writes the log to app.log in the app's logs directory too
// (~/Library/Logs/<name> on macOS), as well as to standard error, which an
// app opened from Finder shows nobody: why songs failed to play stays
// there to read.
func logToFile() {
	dir, err := mygo.App.Path(mygo.PathLogs)
	if err != nil {
		return
	}
	path := filepath.Join(dir, "app.log")
	if info, err := os.Stat(path); err == nil && info.Size() > logLimit {
		os.Rename(path, path+".1")
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return
	}
	log.SetOutput(io.MultiWriter(os.Stderr, f))
}
