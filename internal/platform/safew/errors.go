package safew

import (
	"errors"
	"strings"
)

// permanentChatMarkers are lower-cased fragments of SafeW/Telegram-style error
// descriptions meaning the bot can never post to that chat again (until a human
// fixes it). Retrying such errors is pointless.
var permanentChatMarkers = []string{
	"channel_private",
	"chat_not_found",
	"chat not found",
	"bot was kicked",
	"bot was blocked",
	"user_banned_in_channel",
	"chat_write_forbidden",
	"chat_admin_required",
	"bot is not a member",
	"not a member of the",
	"peer_id_invalid",
	"chat_id_invalid",
	"group chat was deleted",
	"group chat was upgraded",
	"group is deactivated",
	"chat was deleted",
	"have no rights to send",
	"need administrator rights",
	"forbidden",
}

// IsPermanentChatError reports whether err means the chat is unreachable for
// good (private/deleted channel, bot kicked or banned, HTTP 403, ...).
func IsPermanentChatError(err error) bool {
	if err == nil {
		return false
	}
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		if apiErr.HTTPStatus == 403 || apiErr.Code == 403 {
			return true
		}
	}
	msg := strings.ToLower(err.Error())
	for _, m := range permanentChatMarkers {
		if strings.Contains(msg, m) {
			return true
		}
	}
	return false
}
