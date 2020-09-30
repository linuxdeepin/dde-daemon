package shortcuts

func GetQtKeycodeMap() map[string]string {
	var qtKeycodMap = map[string]string{
		"messenger":           "Qt::Key_Messenger",         // XF86Messenger
		"save":                "Qt::Key_Save",              // XF86Save
		"new":                 "Qt::Key_New",               // XF86New
		"wake-up":             "Qt::Key_WakeUp",            // XF86WakeUp
		"audio-rewind":        "Qt::Key_AudioRewind",       // XF86AudioRewind
		"audio-mute":          "Qt::Key_VolumeMute",        // XF86AudioMute
		"mon-brightness-up":   "Qt::Key_MonBrightnessUp",   // XF86MonBrightnessUp
		"wlan":                "Qt::Key_WLAN",              // XF86WLAN
		"audio-media":         "Qt::Key_AudioMedia",        // XF86AudioMedia
		"reply":               "Qt::Key_Reply",             // XF86Reply
		"favorites":           "Qt::Key_Favorites",         // XF86Favorites
		"audio-play":          "Qt::Key_MediaPlay",         // XF86AudioPlay
		"audio-mic-mute":      "Qt::Key_MicMute",           // XF86AudioMicMute
		"audio-pause":         "Qt::Key_AudioPause",        // XF86AudioPause
		"audio-stop":          "Qt::Key_AudioStop",         // XF86AudioStop
		"power-off":           "Qt::Key_PowerOff",          // XF86PowerOff
		"documents":           "Qt::Key_Documents",         // XF86Documents
		"game":                "Qt::Key_Game",              // XF86Game
		"search":              "Qt::Key_Search",            // XF86Search
		"audio-record":        "Qt::Key_AudioRecord",       // XF86AudioRecord
		"display":             "Qt::Key_Display",           // XF86Display
		"reload":              "Qt::Key_Reload",            // XF86Reload
		"explorer":            "Qt::Key_Explorer",          // XF86Explorer
		"calculator":          "Qt::Key_Calculator",        // XF86Calculator
		"calendar":            "Qt::Key_Calendar",          // XF86Calendar
		"forward":             "Qt::Key_Forward",           // XF86Forward
		"cut":                 "Qt::Key_Cut",               // XF86Cut
		"mon-brightness-down": "Qt::Key_MonBrightnessDown", // XF86MonBrightnessDown
		"copy":                "Qt::Key_Copy",              // XF86Copy
		"tools":               "Qt::Key_Tools",             // XF86Tools
		"audio-raise-volume":  "Qt::Key_VolumeUp",          // XF86AudioRaiseVolume
		"close":               "Qt::Key_Close",             // XF86Close
		"www":                 "Qt::Key_WWW",               // XF86WWW
		"home-page":           "Qt::Key_HomePage",          // XF86HomePage
		"sleep":               "Qt::Key_Sleep",             // XF86Sleep
		"audio-lower-volume":  "Qt::Key_VolumeDown",        // XF86AudioLowerVolume
		"audio-prev":          "Qt::Key_MediaPrevious",         // XF86AudioPrev
		"audio-next":          "Qt::Key_MediaNext",         // XF86AudioNext
		"paste":               "Qt::Key_Paste",             // XF86Paste
		"open":                "Qt::Key_Open",              // XF86Open
		"send":                "Qt::Key_Send",              // XF86Send
		"my-computer":         "Qt::Key_MyComputer",        // XF86MyComputer
		"mail":                "Qt::Key_MailForward",       // XF86Mail
		"adjust-brightness":   "Qt::Key_BrightnessAdjust",  // XF86BrightnessAdjust
		"log-off":             "Qt::Key_LogOff",            // XF86LogOff
		"pictures":            "Qt::Key_Pictures",          // XF86Pictures
		"terminal":            "Qt::Key_Terminal",          // XF86Terminal
		"video":               "Qt::Key_Video",             // XF86Video
		"music":               "Qt::Key_Music",             // XF86Music
		"app-left":            "Qt::Key_ApplicationLeft",   // XF86ApplicationLeft
		"app-right":           "Qt::Key_ApplicationRight",  // XF86ApplicationRight
		"meeting":             "Qt::Key_Meeting",           // XF86Meeting
		"switch-monitors":      "<Super>P",
		"numlock":             "Qt::Key_NumLock",
		"capslock":            "Qt::Key_CapsLock",
	}
	return qtKeycodMap
}
