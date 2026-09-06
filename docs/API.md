# Yandex Music API - используемые эндпоинты

База: `https://api.music.yandex.net/`. Авторизация: заголовок `Authorization: OAuth <token>`.
Клиент притворяется Android-приложением (`YandexMusicAndroid`, `okhttp`).
Все пути ниже - относительно базы, `{uid}` - id текущего пользователя.

## Аккаунт

| Метод | Путь | Назначение |
|---|---|---|
| GET | `account/status` | Проверка токена, получение `uid` при старте |

## Треки

| Метод | Путь | Назначение |
|---|---|---|
| POST | `/tracks` | Пачка треков по id (`track-ids`, `with-positions=false`) |
| GET | `/tracks/{id}/download-info` | Варианты качества/ссылки для скачивания |
| GET | `/tracks/{id}/lyrics` | Мета текстов (`sign`+`timeStamp` HMAC, `format=LRC`), дальше GET `downloadUrl` с LRC |
| POST | `/play-audio` | Событие «слушаю» (`uid`, `play-id`, `track-id`, `track-length-seconds`, …) |

Скачивание файла - двухшаговое: `downloadInfoUrl + &format=json` отдаёт хост/путь/подпись,
прямая ссылка собирается как `https://{host}/get-{codec}/{hash}/{ts}{path}`.

## Лайки и пины

| Метод | Путь                                     | Назначение                            |
| ----- | ---------------------------------------- | ------------------------------------- |
| GET   | `/users/{uid}/likes/tracks`              | Id лайкнутых треков                   |
| POST  | `/users/{uid}/likes/tracks/add-multiple` | Лайк (`track-ids`)                    |
| POST  | `/users/{uid}/likes/tracks/remove`       | Анлайк (`track-ids`)                  |
| GET   | `/users/{uid}/likes/albums`              | Лайкнутые альбомы                     |
| GET   | `/pins`                                  | Закреплённое (фильтруем `album_item`) |

## Плейлисты

| Метод | Путь | Назначение |
|---|---|---|
| GET | `/users/{uid}/playlists/list` | Список плейлистов пользователя |
| GET | `/users/{uid}/playlists` | Треки плейлиста (`kinds`, `mixed`, `rich-tracks=true`) |
| GET | `/users/{uid}/playlists/{kind}` | Один плейлист |
| POST | `/users/{uid}/playlists/create` | Создать (`title`, `visibility`) |
| POST | `/users/{uid}/playlists/{kind}/name` | Переименовать (`value`) |
| POST | `/users/{uid}/playlists/{kind}/delete` | Удалить |
| POST | `/users/{uid}/playlists/{kind}/change-relative` | Вставка/удаление треков (`diff` JSON + `revision`) |

## Радио / rotor-сессии

| Метод | Путь | Назначение |
|---|---|---|
| GET | `/rotor/stations/list` | Все станции (`language`) |
| GET | `/rotor/station/{type:tag}/tracks` | Треки станции, deprecated (есть `queue`) |
| POST | `/rotor/station/{type:tag}/feedback` | Фидбэк старой схемы, deprecated |
| POST | `/rotor/session/new` | Новая сессия (`seeds`, `includeTracksInResponse`) |
| POST | `/rotor/session/{id}/tracks` | Следующая пачка (`feedbacks`, `queue` вида `trackId:albumId`) |
| POST | `/rotor/session/{id}/feedback` | События: старт/финиш радио, старт/финиш/скип/лайк/дизлайк трека |

## Артисты и альбомы

| Метод | Путь | Назначение |
|---|---|---|
| GET | `/artists/{id}/tracks` | Треки артиста (`page`, `page-size`) |
| GET | `/artists/{id}/track-ids-by-rating` | Популярные треки (только id) |
| GET | `/albums/{id}` | Альбом |
| GET | `/albums/{id}/with-tracks` | Альбом с `volumes` треков |

## Поиск

| Метод | Путь | Назначение |
|---|---|---|
| GET | `/search` | Поиск (`text`, `page=0`, `type`: `all`/`track`/`artist`/`album`) |
| GET | `/search/suggest` | Подсказки (`part`) |

## Служебное

- Шаринг трека: `https://music.yandex.ru/album/{albumId}/track/{trackId}` (собирается локально).
- Обложки: `https://{coverUri с %% → SIZE x SIZE}`, обычно `200x200` в плеере.
- Ответы API обёрнуты в `{invocationInfo, result}`; ошибки - `400 {error:{name,message}}`, `401 {error,…}`.
- Известная нестабильность схемы: сервер местами отдаёт число вместо строки и наоборот
  (`Label.id`, `Track.id`, `Artist.id`, `lyricId`, `exec-duration-millis`, битрейт) -
  в коде это закрыто типами `FlexString` / `FlexInt` / `FlexUint64` (`api/types.go`).
