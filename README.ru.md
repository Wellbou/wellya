<p align="center">
  <img src=".assets/banner.png" width="800" alt="баннер wellya">
</p>

# <img src=".assets/logo.png" width="48" alt="логотип wellya"> wellya

[Read in English](README.md)

[![GitHub License](https://img.shields.io/github/license/wellbou/wellya)](LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/wellbou/wellya)](https://goreportcard.com/report/github.com/wellbou/wellya)

Терминальный клиент Яндекс Музыки. Форк [yamusic-tui](https://github.com/DECE2183/yamusic-tui) с дополнительными возможностями и исправлениями.<br>
Основан на [yandex-music-open-api](https://github.com/acherkashin/yandex-music-open-api).

![скриншот](.assets/screenshot.png)

### Требования

Действующий аккаунт Яндекс Музыки и токен доступа. Проще всего получить токен через расширение для браузера ([Chrome](https://chrome.google.com/webstore/detail/yandex-music-token/lcbjeookjibfhjjopieifgjnhlegmkib), [Firefox](https://addons.mozilla.org/en-US/firefox/addon/yandex-music-token/)).

### Возможности

- Плеер: play/pause, следующий/предыдущий трек, прогресс-бар, перемотка, громкость, мьют (`m`)
- Режимы повтора (`r`): выкл / всё / один. Таймер сна (`n`): от 15 до 120 минут
- Лайк/анлайк (`l` / `L`), дизлайк (`d` / `D`), копирование ссылки (`ctrl+s`)
- Синхронные тексты (`t`), переключение качества звука (`q`): best / high / medium / low с отображением битрейта
- Кэширование треков (`c`), кэширование всех лайков (`C`), скачивание в файл (`ctrl+w`)
- Вкладки: Playlists, Radio (`R`), Search (`S`)
- Моя волна остаётся в общем списке; остальные станции — на вкладке Radio
- История прослушиваний (последние 100 треков), просмотр очереди (`tab`), переход к играющему (`N`)
- Просмотр исполнителя (`i`), переход к альбому (`A`), информация о треке (`I`)
- Удаление из очереди без анлайка (`X`), перемещение треков (`ctrl+u` / `ctrl+d`)
- Сортировка (`s`): название / исполнитель / длительность. Фильтр списка (`/` или просто печать, `esc` очищает)
- Глобальный поиск: `enter` — играть сейчас, `p` — следующим
- Экспорт плейлиста в M3U (`E`), импорт локальных файлов (`U`): MP3/M3U-файлы и папки
- Тост со статистикой (`ctrl+g`), всплывающие уведомления, подтверждения удаления (`y`/`n`)
- Полная справка по горячим клавишам (`F1`), все клавиши переназначаются в конфиге
- Устойчивый парсинг API: числовые поля и id переживают возврат Яндексом чисел вместо строк и наоборот (FlexString/FlexInt/FlexUint64)

### Установка

```bash
go install github.com/wellbou/wellya@latest
```

Или собрать вручную:

```bash
git clone https://github.com/wellbou/wellya
cd wellya
go build -o wellya .
./wellya
```

Для обновления: снова выполнить `go install github.com/wellbou/wellya@latest`.

### Конфигурация

Файл конфигурации — `~/.config/wellya/config.yaml`. Создаётся с настройками по умолчанию при первом запуске. Конфиг из `~/.config/yamusic-tui/config.yaml` подхватывается автоматически, если есть.

```yaml
token: <ваш токен яндекс музыки>
buffer-size-ms: 80
rewind-duration-s: 5
volume: 0.5
volume-step: 0.05
suppress-errors: false
show-lyrics: false
audio-quality: best # best/high/medium/low
cache-tracks: likes # none/likes/all
cache-dir: ""
download-dir: "" # по умолчанию: XDG music dir (~/Music или ~/Музыка)
resume-on-start: true # восстанавливать очередь, трек и позицию при запуске
proxy: "" # URL прокси; иначе берутся HTTP_PROXY и HTTPS_PROXY
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
    reload: ctrl+\
    show-all-keys: ?
    keys-help: f1
    playlists-up: ctrl+up
    playlists-down: ctrl+down
    playlists-rename: ctrl+r
    playlists-hide: ctrl+b
    playlists-radio: R
    playlists-share: Y
    tracks-page-up: pgup
    tracks-page-down: pgdown
    tracks-like: l
    tracks-add-to-playlist: a
    tracks-quick-add: y
    tracks-remove-from-playlist: ctrl+a
    tracks-remove-from-queue: X
    tracks-export: E
    tracks-upload: U
    tracks-stats: ctrl+g
    tracks-play-next: p
    tracks-share: ctrl+s
    tracks-shuffle: ctrl+x
    tracks-search: ctrl+f
    tracks-search-tab: S
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
    player-cache: c
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
```

На одно действие можно назначить несколько клавиш через запятую.

Кэшированные треки лежат в системном кэше (`~/.cache/wellya` в Linux), если не задан `cache-dir`. Если звук заикается — увеличьте `buffer-size-ms`.

### Локальные файлы

Нажмите `U` и введите путь к MP3/M3U-файлу или папке. Файлы разбираются по ID3-тегам, копируются в кэш и добавляются в Cached tracks — можно слушать сразу. Загрузка на сервер Яндекса через API невозможна, так что это остаётся локальным. Для выгрузки в облако используйте сайт `music.yandex.ru` → Коллекция → Загрузить трек.

### Системные медиа-клавиши

![win11-smtc-example](.assets/smtc-win11.png)

MPRIS в Linux, SMTC в Windows, MPRemoteCommandCenter в macOS. Отключается сборкой с тегом `nomedia`:

```bash
go build -tags='nomedia' -o wellya .
```

На macOS для работы медиа-клавиш нужен внешний линковщик:

```bash
go build -ldflags="-linkmode=external" -o wellya .
```

---

<p align="center">
  <img src=".assets/OriginOfSymmetry.png" width="600" alt="Muse - Origin of Symmetry, очень крутой альбом">
  <br>
  <em>Слушайте на здоровье! Спасибо, что выбрали именно мой софт. Muse, Origin of Symmetry (2001) (надо же заполнить чем то футер)</em>
</p>
