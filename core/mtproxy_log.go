package core

import (
	"strings"

	"github.com/sagernet/sing-box/log"
)

// MTProxy's logger adapter labels rejected authentication as errors and normal
// stream shutdown as info. Keep those diagnostics at debug without hiding other
// handshake or transport failures.
func mtproxyLogLevel(level log.Level, args []any) log.Level {
	if len(args) == 2 && level == log.LevelError {
		message, ok := args[0].(string)
		err, isError := args[1].(error)
		if ok && isError && err != nil && message == "cannot read client hello" && err.Error() == "incorrect client random" {
			return log.LevelDebug
		}
	}
	if len(args) != 1 || level != log.LevelInfo {
		return level
	}
	message, ok := args[0].(string)
	if !ok {
		return level
	}
	switch message {
	case "Stream has been started", "Stream has been finished", "client -> telegram has been finished", "telegram -> client has been finished":
		return log.LevelDebug
	}
	if (strings.HasPrefix(message, "client -> telegram has been finished (written ") || strings.HasPrefix(message, "telegram -> client has been finished (written ")) && strings.HasSuffix(message, " bytes): io: read/write on closed pipe") {
		return log.LevelDebug
	}
	return level
}
