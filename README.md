# WellYaMusic CLI — `wellya`

[![GitHub License](https://img.shields.io/github/license/wellbou/wellya)](LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/wellbou/wellya)](https://goreportcard.com/report/github.com/wellbou/wellya)

> **Фиолетовый форк** [yamusic-tui](https://github.com/DECE2183/yamusic-tui) в стиле экосистемы [wellutils](https://github.com/wellbou/wellutils) — `wellutils`, `wellya`, `wellsensors` единый бренд.

An unofficial Yandex Music terminal client with purple theme and extended features.<br>
Based on [yandex-music-open-api](https://github.com/acherkashin/yandex-music-open-api) and `Dece2183/yamusic-tui`.

![screenshot](.assets/screenshot.png)

### Well-семейство

```bash
wellutils  # →  wellfetch / wellper / wellgpu / wellsensors / wellya
wellya     # Yandex Music в терминале, фиолетовый стиль #AB47BC
```

Все утилиты `well*` ставятся как `wellutils --self-update` и `go install github.com/wellbou/wellya@latest`.

### Requirements

Valid Yandex Music account + access token. Самый простой способ — расширение браузера ([Chrome](https://chrome.google.com/webstore/detail/yandex-music-token/lcbjeookjibfhjjopieifgjnhlegmkib), [Firefox](https://addons.mozilla.org/en-US/firefox/addon/yandex-music-token/)).

### Что нового в форке vs оригинал

- [x] Фиолетовая тема `#AB47BC` / `#2C1A4A` / `#4A148C` — в тон `wellutils --fetch`
- [x] **Удалить из очереди, не из любимых** — `X` (локально, без API `UnlikeTrack`)
- [x] **M3U экспорт** — `E` → `~/Music/wellya/<playlist>.m3u` (`#EXTM3U` + `#EXTINF` + `https://music.yandex.ru/...`)
- [x] **Загрузка локальных файлов** — `U` → ввод пути `~/Music/file.mp3` или директории, парсинг ID3, кэш в `~/.cache/wellya`, добавление в `Cached tracks` (поддержка UGC/OWN — Yamaha хранит как `trackSource:"UGC"`)
- [x] Версия в футтере `dev-queue-m3u`
- [x] Остальные фичи форка (см. ниже) — повтор, мьют, станции, битрейт, история, jump-to-playing, тосты, таймер сна, артист-брауз, очередь, фильтры поиска, инфо, go-to-album, дизлайк, релоад конфига, номера треков, счётчик проигрываний, сортировка, фильтр

### Implemented features (полный список)

 - [x] Player
    - [x] Play/pause, next/prev, progress, rewind, volume, mute (`m`), repeat (`r` OFF/ALL/1), sleep timer (`n`)
    - [x] Like/unlike (`l`/`L`), dislike (`d`/`D` → `EV_TRACK_DISLIKED`), share (`ctrl+s` → `ShareTrackLink`)
    - [x] Synced lyrics (`t`), битрейт (`q` → best/high/medium/low + `320k`)
    - [x] Cache / download (`S`/`C`/`ctrl+w` → `~/Music/wellya`)
 - [x] Radio
    - [x] My wave + **все станции** (`Stations()` → `STATION` плейлист, rotor `RotorSessionTracks`)
 - [x] Likes / History
    - [x] Liked tracks, liked albums, **History** (100, dedup), Artist browse (`i` → `ArtistPopularTracks`)
 - [x] Playlists
    - [x] Display user playlists, play, add/remove (`a`/`ctrl+a`), create/remove/rename (`ctrl+r`)
    - [x] Move (`ctrl+u`/`ctrl+d`), shuffle (`ctrl+x`), sort (`s` → title/artist/duration), filter (`/`), page (`pgup/pgdown`)
    - [x] Queue (`tab` → Up Next), jump to playing (`N`), track info (`I`), go to album (`A`)
    - [x] **Remove from queue only** (`X`), **Export M3U** (`E`), **Upload local MP3** (`U`)
    - [x] Total duration `[H:MM:SS]` в заголовке, счётчик треков `(42)` в сайдбаре, `x5` playcount
 - [x] Search (`ctrl+f`) + фильтры `tab` All/Tracks/Albums/Artists/Playlists
 - [x] Caching (race/sentinel/двойной `Seek` фиксы), прокси `httpClient.Do`

## Installation

```bash
# как утилита well-семейства
go install github.com/wellbou/wellya@latest
# бинарь будет как `wellya` и `wellmusic` (alias)
sudo install -m 755 ~/go/bin/wellya /usr/local/bin/wellya
sudo ln -sf /usr/local/bin/wellya /usr/local/bin/wellmusic

# или собрать локально
git clone https://github.com/wellbou/wellya
cd wellya
go build -o wellya .
./wellya
```

Также `wellutils` подтянет `wellya` при `wellutils --self-update` (если настроен).

## Configuration

Файл теперь `~/.config/wellya/config.yaml` (миграция со старого `~/.config/yamusic-tui/config.yaml` автоматическая).

```yaml
token: <your yandex music token>
buffer-size-ms: 80
rewind-duration-s: 5
volume: 0.5
volume-step: 0.05
show-errors: false
show-lyrics: false
audio-quality: best # best/high/medium/low
cache-tracks: likes # none/likes/all
cache-dir: ""
download-dir: "" # default ~/Music/wellya
proxy: ""
search:
    artists: true
    albums: false
    playlists: false
controls:
    quit: ctrl+q,ctrl+c
    apply: enter
    cancel: esc
    cursor-up: up
    cursor-down: down
    show-all-keys: ?
    playlists-up: ctrl+up
    playlists-down: ctrl+down
    playlists-rename: ctrl+r
    playlists-hide: ctrl+b
    tracks-next-page: pgup
    tracks-previous-page: pgdown
    tracks-like: l
    tracks-add-to-playlist: a
    tracks-remove-from-playlist: ctrl+a
    tracks-remove-from-queue: X
    tracks-export: E
    tracks-upload: U
    tracks-share: ctrl+s
    tracks-shuffle: ctrl+x
    tracks-search: ctrl+f
    tracks-back: backspace
    tracks-hide: ctrl+t
    tracks-move-up: ctrl+u
    tracks-move-down: ctrl+d
    tracks-jump-to-playing: N
    tracks-artist-browse: i
    tracks-show-queue: tab
    tracks-info: I
    tracks-go-to-album: A
    tracks-dislike: d
    tracks-sort: s
    tracks-filter: /
    player-pause: space
    player-next: right
    player-previous: left
    player-rewind-forward: ctrl+right
    player-rewind-backward: ctrl+left
    player-like: L
    player-cache: S
    player-vol-up: +,=
    player-vol-down: '-'
    player-toggle-lyrics: t
    player-hide: ctrl+p
    player-cache-all-liked: C
    player-download: ctrl+w
    player-quality-cycle: q
    player-mute: m
    player-repeat-mode: r
    player-sleep-timer: n
    player-dislike: D
    reload: ctrl+\
style:
    volume-indicator-width: 16
    volume-indicator-autohide-at: 58
    side-panel-width: 32
    side-panel-autohide-at: 96
    search-modal-width: 56
    icons:
        play: ▶
        stop: ■
        liked: 💛
        not-liked: 🤍
        cached: 💿
        lyrics-dot: •
        volume-off: 🔇
        volume-low: 🔈
        volume-mid: 🔉
        volume-high: 🔊
    colors:
        accent: '#AB47BC'
        error: '#E91E63'
        border: '#6A1B9A'
        background: '#2C1A4A'
        playlist-selection: '#4A148C'
        active-text: '#EDE7F6'
        normal-text: '#D1C4E9'
        inactive-text: '#7E57C2'
        track-title-text: '#E1BEE7'
        track-version-text: '#B39DDB'
        track-artist-text: '#CE93D8'
        lyrics-previous: '#4A148C'
        lyrics-current: '#E040FB'
        lyrics-next: '#7B1FA2'
```

Кэш по умолчанию `~/.cache/wellya` (Linux) / `~/AppData/Local/wellya` (Windows).

### Загрузка своих треков (UGC)

Яндекс Музыка поддерживает `trackSource: "UGC"` / `"OWN"` — загруженные MP3 хранятся как `storageDir: "247309_u/..."`, `desiredVisibility:"private"`, `canPublish:false`.

В `wellya` это эмулируется локально без API-загрузки:

1. `U` → ввод `~/Music/my.mp3` или `~/Music/Album/` (рекурсивно `.mp3/.flac/.ogg/.m4a`)
2. Парсинг ID3 (`github.com/bogem/id3v2`) → `title/artist/album/genre/year/TLEN`
3. `id = local_<sha1(path+modtime+size)[:8]>`, `cache.Write(id)` + `writeTrackID3Tag` + `io.Copy`
4. Добавление в `Cached tracks` (`playlist.LOCAL`), сразу играбельно ( `cache.Read(id)` в `playControl.go:288` )

Для выгрузки в облако Яндекса используй веб-интерфейс `music.yandex.ru` → Коллекция → Загрузить трек.

## System media controls

MPRIS (Linux), SMTC (Windows), MPRemoteCommandCenter (macOS). Отключается `go build -tags='nomedia'`.

```bash
go build -tags='nomedia' -o wellya .
go build -ldflags="-linkmode=external" -o wellya . # macOS 15+
```
