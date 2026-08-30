package config

import "gopkg.in/yaml.v3"

type CacheType uint

const (
	CACHE_NONE CacheType = iota
	CACHE_LIKED_ONLY
	CACHE_ALL
)

type AudioQuality string

const (
	QUALITY_BEST   AudioQuality = "best"
	QUALITY_HIGH   AudioQuality = "high"
	QUALITY_MEDIUM AudioQuality = "medium"
	QUALITY_LOW    AudioQuality = "low"
)

var audioQualityToLabel = map[AudioQuality]string{
	QUALITY_BEST:   "best",
	QUALITY_HIGH:   "high",
	QUALITY_MEDIUM: "medium",
	QUALITY_LOW:    "low",
}

func (q AudioQuality) Label() string {
	if l, ok := audioQualityToLabel[q]; ok {
		return l
	}
	return "best"
}

func (q AudioQuality) Next() AudioQuality {
	switch q {
	case QUALITY_BEST:
		return QUALITY_HIGH
	case QUALITY_HIGH:
		return QUALITY_MEDIUM
	case QUALITY_MEDIUM:
		return QUALITY_LOW
	default:
		return QUALITY_BEST
	}
}

var cacheValueToEnum = map[string]CacheType{
	"disable": CACHE_NONE,
	"false":   CACHE_NONE,
	"none":    CACHE_NONE,
	"off":     CACHE_NONE,
	"likes":   CACHE_LIKED_ONLY,
	"liked":   CACHE_LIKED_ONLY,
	"all":     CACHE_ALL,
}

var cacheEnumToValue = map[CacheType]string{
	CACHE_NONE:       "none",
	CACHE_LIKED_ONLY: "likes",
	CACHE_ALL:        "all",
}

func (t *CacheType) UnmarshalYAML(value *yaml.Node) error {
	*t = cacheValueToEnum[value.Value]
	return nil
}

func (t CacheType) MarshalYAML() (interface{}, error) {
	if t > CACHE_ALL {
		t = CACHE_NONE
	}
	return cacheEnumToValue[t], nil
}

type Icons struct {
	Play       string `yaml:"play"`
	Stop       string `yaml:"stop"`
	Liked      string `yaml:"liked"`
	NotLiked   string `yaml:"not-liked"`
	Cached     string `yaml:"cached"`
	LyricsDot  string `yaml:"lyrics-dot"`
	VolumeOff  string `yaml:"volume-off"`
	VolumeLow  string `yaml:"volume-low"`
	VolumeMid  string `yaml:"volume-mid"`
	VolumeHigh string `yaml:"volume-high"`
}

type Colors struct {
	Accent            string `yaml:"accent"`
	Error             string `yaml:"error"`
	Border            string `yaml:"border"`
	Background        string `yaml:"background"`
	PlaylistSelection string `yaml:"playlist-selection"`
	ActiveText        string `yaml:"active-text"`
	NormalText        string `yaml:"normal-text"`
	InactiveText      string `yaml:"inactive-text"`
	TrackTitleText    string `yaml:"track-title-text"`
	TrackVersionText  string `yaml:"track-version-text"`
	TrackArtistText   string `yaml:"track-artist-text"`
	LyricsPrevious    string `yaml:"lyrics-previous"`
	LyricsCurrent     string `yaml:"lyrics-current"`
	LyricsNext        string `yaml:"lyrics-next"`
}

type Style struct {
	VolumeIndicatorWidth    int     `yaml:"volume-indicator-width"`
	VolumeIndicatorAutohide int     `yaml:"volume-indicator-autohide-at"`
	SidePanelWidth          int     `yaml:"side-panel-width"`
	SidePanelAutohide       int     `yaml:"side-panel-autohide-at"`
	SearchModalWidth        int     `yaml:"search-modal-width"`
	Icons                   *Icons  `yaml:"icons"`
	Colors                  *Colors `yaml:"colors"`
}

type Controls struct {
	// Main control
	Quit        *Key `yaml:"quit"`
	Apply       *Key `yaml:"apply"`
	Cancel      *Key `yaml:"cancel"`
	CursorUp    *Key `yaml:"cursor-up"`
	CursorDown  *Key `yaml:"cursor-down"`
	Reload      *Key `yaml:"reload"`
	ShowAllKeys *Key `yaml:"show-all-keys"`
	// Playlists control
	PlaylistsUp     *Key `yaml:"playlists-up"`
	PlaylistsDown   *Key `yaml:"playlists-down"`
	PlaylistsRename *Key `yaml:"playlists-rename"`
	PlaylistsHide   *Key `yaml:"playlists-hide"`
	PlaylistsRadio  *Key `yaml:"playlists-radio"`
	// Track list control
	TracksNextPage           *Key `yaml:"tracks-next-page"`
	TracksPrevPage           *Key `yaml:"tracks-previous-page"`
	TracksLike               *Key `yaml:"tracks-like"`
	TracksAddToPlaylist      *Key `yaml:"tracks-add-to-playlist"`
	TracksRemoveFromPlaylist *Key `yaml:"tracks-remove-from-playlist"`
	TracksShare              *Key `yaml:"tracks-share"`
	TracksShuffle            *Key `yaml:"tracks-shuffle"`
	TracksSearch             *Key `yaml:"tracks-search"`
	TracksSearchTab         *Key `yaml:"tracks-search-tab"`
	TracksBack               *Key `yaml:"tracks-back"`
	TracksHide               *Key `yaml:"tracks-hide"`
	TracksMoveUp             *Key `yaml:"tracks-move-up"`
	TracksMoveDown           *Key `yaml:"tracks-move-down"`
	TracksJumpToPlaying      *Key `yaml:"tracks-jump-to-playing"`
	TracksArtistBrowse       *Key `yaml:"tracks-artist-browse"`
	TracksShowQueue          *Key `yaml:"tracks-show-queue"`
	TracksInfo               *Key `yaml:"tracks-info"`
	TracksGoToAlbum          *Key `yaml:"tracks-go-to-album"`
	TracksDislike            *Key `yaml:"tracks-dislike"`
	TracksSort               *Key `yaml:"tracks-sort"`
	TracksFilter             *Key `yaml:"tracks-filter"`
	TracksRemoveFromQueue    *Key `yaml:"tracks-remove-from-queue"`
	TracksExport             *Key `yaml:"tracks-export"`
	TracksUpload             *Key `yaml:"tracks-upload"`
	TracksStats              *Key `yaml:"tracks-stats"`
	TracksPlayNext           *Key `yaml:"tracks-play-next"`
	// Player control
	PlayerPause          *Key `yaml:"player-pause"`
	PlayerNext           *Key `yaml:"player-next"`
	PlayerPrevious       *Key `yaml:"player-previous"`
	PlayerRewindForward  *Key `yaml:"player-rewind-forward"`
	PlayerRewindBackward *Key `yaml:"player-rewind-backward"`
	PlayerLike           *Key `yaml:"player-like"`
	PlayerCache          *Key `yaml:"player-cache"`
	PlayerVolUp          *Key `yaml:"player-vol-up"`
	PlayerVolDown        *Key `yaml:"player-vol-down"`
	PlayerToggleLyrics   *Key `yaml:"player-toggle-lyrics"`
	PlayerHide           *Key `yaml:"player-hide"`
	PlayerCacheAllLiked  *Key `yaml:"player-cache-all-liked"`
	PlayerDownload       *Key `yaml:"player-download"`
	PlayerMute           *Key `yaml:"player-mute"`
	PlayerQualityCycle   *Key `yaml:"player-quality-cycle"`
	PlayerRepeatMode     *Key `yaml:"player-repeat-mode"`
	PlayerSleepTimer     *Key `yaml:"player-sleep-timer"`
	PlayerDislike        *Key `yaml:"player-dislike"`
}

type Search struct {
	Artists   bool `yaml:"artists"`
	Albums    bool `yaml:"albums"`
	Playlists bool `yaml:"playlists"`
}

type Config struct {
	Token          string      `yaml:"token"`
	BufferSize     float64     `yaml:"buffer-size-ms"`
	RewindDuration float64     `yaml:"rewind-duration-s"`
	Volume         float64     `yaml:"volume"`
	VolumeStep     float64     `yaml:"volume-step"`
	SuppressErrors bool        `yaml:"suppress-errors"`
	ShowLyrics     bool        `yaml:"show-lyrics"`
	AudioQuality   AudioQuality `yaml:"audio-quality"`
	CacheTracks    CacheType   `yaml:"cache-tracks"`
	CacheDir       string      `yaml:"cache-dir"`
	DownloadDir    string      `yaml:"download-dir"`
	Proxy          string      `yaml:"proxy"`
	Search         *Search     `yaml:"search"`
	Controls       *Controls   `yaml:"controls"`
	Style          *Style      `yaml:"style"`
}

var defaultConfig = Config{
	BufferSize:     80,
	RewindDuration: 5,
	Volume:         0.5,
	VolumeStep:     0.05,
	ShowLyrics:     false,
	AudioQuality:   QUALITY_BEST,
	CacheTracks:    CACHE_LIKED_ONLY,
	CacheDir:       "",
	DownloadDir:    "",
	SuppressErrors: false,
	Search: &Search{
		Artists:   true,
		Albums:    false,
		Playlists: false,
	},
	Controls: &Controls{
		Quit:                     NewKey("ctrl+q,ctrl+c"),
		Apply:                    NewKey("enter"),
		Cancel:                   NewKey("esc"),
		CursorUp:                 NewKey("up"),
		CursorDown:               NewKey("down"),
		Reload:                   NewKey("ctrl+\\"),
		ShowAllKeys:              NewKey("?"),
		PlaylistsUp:              NewKey("ctrl+up"),
		PlaylistsDown:            NewKey("ctrl+down"),
		PlaylistsRename:          NewKey("ctrl+r"),
		PlaylistsHide:            NewKey("ctrl+b"),
		PlaylistsRadio:           NewKey("R"),
		TracksNextPage:           NewKey("pgup"),
		TracksPrevPage:           NewKey("pgdown"),
		TracksLike:               NewKey("l"),
		TracksAddToPlaylist:      NewKey("a"),
		TracksRemoveFromPlaylist: NewKey("ctrl+a"),
		TracksSearch:             NewKey("ctrl+f"),
		TracksSearchTab:         NewKey("S"),
		TracksShuffle:            NewKey("ctrl+x"),
		TracksShare:              NewKey("ctrl+s"),
		TracksBack:               NewKey("backspace"),
		TracksHide:               NewKey("ctrl+t"),
		TracksMoveUp:             NewKey("ctrl+u"),
		TracksMoveDown:           NewKey("ctrl+d"),
		TracksJumpToPlaying:      NewKey("N"),
		TracksArtistBrowse:       NewKey("i"),
		TracksShowQueue:          NewKey("tab"),
		TracksInfo:               NewKey("I"),
		TracksGoToAlbum:          NewKey("A"),
		TracksDislike:            NewKey("d"),
		TracksSort:               NewKey("s"),
		TracksFilter:             NewKey("/"),
		TracksRemoveFromQueue:    NewKey("X"),
		TracksExport:             NewKey("E"),
		TracksUpload:             NewKey("U"),
		TracksStats:              NewKey("ctrl+g"),
		TracksPlayNext:           NewKey("n"),
		PlayerPause:              NewKey("space"),
		PlayerNext:               NewKey("right"),
		PlayerPrevious:           NewKey("left"),
		PlayerRewindForward:      NewKey("ctrl+right"),
		PlayerRewindBackward:     NewKey("ctrl+left"),
		PlayerLike:               NewKey("L"),
		PlayerToggleLyrics:       NewKey("t"),
		PlayerCache:              NewKey("S"),
		PlayerVolUp:              NewKey("+,="),
		PlayerVolDown:            NewKey("-"),
		PlayerHide:               NewKey("ctrl+p"),
		PlayerCacheAllLiked:      NewKey("C"),
		PlayerDownload:           NewKey("ctrl+w"),
		PlayerQualityCycle:       NewKey("q"),
		PlayerMute:              NewKey("m"),
		PlayerRepeatMode:        NewKey("r"),
		PlayerSleepTimer:        NewKey("n"),
		PlayerDislike:           NewKey("D"),
	},
	Style: &Style{
		VolumeIndicatorWidth:    16,
		VolumeIndicatorAutohide: 58,
		SidePanelWidth:          32,
		SidePanelAutohide:       96,
		SearchModalWidth:        56,
		Icons: &Icons{
			Play:       "▶",
			Stop:       "■",
			Liked:      "💛",
			NotLiked:   "🤍",
			Cached:     "💿",
			LyricsDot:  "•",
			VolumeOff:  "🔇",
			VolumeLow:  "🔈",
			VolumeMid:  "🔉",
			VolumeHigh: "🔊",
		},
		Colors: &Colors{
			Accent:            "#AB47BC",
			Error:             "#E91E63",
			Border:            "#6A1B9A",
			Background:        "#2C1A4A",
			PlaylistSelection: "#4A148C",
			ActiveText:        "#EDE7F6",
			NormalText:        "#D1C4E9",
			InactiveText:      "#7E57C2",
			TrackTitleText:    "#E1BEE7",
			TrackVersionText:  "#B39DDB",
			TrackArtistText:   "#CE93D8",
			LyricsPrevious:    "#4A148C",
			LyricsCurrent:     "#E040FB",
			LyricsNext:        "#7B1FA2",
		},
	},
}

const DirName = "wellya"
const AppName = "WellYaMusic CLI"
const AppNameShort = "wellya"
