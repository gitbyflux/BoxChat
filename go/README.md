# BoxChat Go Backend

Это бэкенд для BoxChat, переписанный с Python на Go.

## Структура проекта

```
go/
├── cmd/
│   └── server/
│       └── main.go          # Точка входа
├── internal/
│   ├── config/              # Конфигурация
│   ├── database/            # DB подключение, миграции
│   ├── models/              # GORM модели
│   ├── handlers/
│   │   ├── http/            # HTTP handlers (REST API)
│   │   │   ├── auth.go      # Аутентификация
│   │   │   ├── api.go       # Основное API
│   │   │   ├── friends.go   # Друзья
│   │   │   ├── search.go    # Поиск
│   │   │   └── upload.go    # Загрузка файлов
│   │   └── socketio/        # Socket.IO v4 handlers
│   ├── middleware/          # Auth, CORS, Logger
│   ├── services/            # Бизнес-логика
│   │   ├── auth.go          # Аутентификация
│   │   ├── moderation.go    # Модерация
│   │   ├── mentions.go      # Упоминания
│   │   ├── giphy.go         # Giphy API
│   │   └── ...
│   └── utils/               # Утилиты
├── go.mod
└── go.sum
```

## Технологический стек

- **Web фреймворк:** Gin
- **WebSocket:** gorilla/websocket + Socket.IO v4 (ручная реализация)
- **ORM:** GORM
- **База данных:** SQLite
- **Аутентификация:** Сессии на cookie + bcrypt

## Сборка и запуск

### Требования

- Go 1.21 или выше

### Сборка

```bash
cd go
go mod tidy
go build -o ../boxchat-go ./cmd/server
```

### Запуск

⚠️ **Важно:** Для корректной работы необходимо установить переменные окружения:

```bash
# Из корня проекта (с переменными окружения)
ADMIN_PASSWORD="YourPassword123!" \
SECRET_KEY="$(openssl rand -hex 32)" \
ALLOWED_ORIGINS="http://localhost,http://127.0.0.1" \
./boxchat-go
```

Или из директории go:

```bash
cd go

# С переменными окружения
ADMIN_PASSWORD="YourPassword123!" \
SECRET_KEY="$(openssl rand -hex 32)" \
ALLOWED_ORIGINS="http://localhost,http://127.0.0.1" \
go run ./cmd/server
```

Без переменных `ADMIN_PASSWORD` и `SECRET_KEY` сервер запустится, но будет использовать случайные значения.

## Конфигурация

Конфигурация загружается из `config.json` в корне проекта.

Основные переменные окружения:
- `SQLALCHEMY_DATABASE_URI` - строка подключения к БД
- `SECRET_KEY` - секретный ключ
- `GIPHY_API_KEY` - API ключ Giphy
- `SERVER_HOST` - хост сервера (по умолчанию 127.0.0.1)
- `SERVER_PORT` - порт сервера (по умолчанию 5000)

## API Endpoints

### Аутентификация
- `POST /api/v1/auth/login` - вход
- `POST /api/v1/auth/register` - регистрация
- `GET /api/v1/auth/session` - проверка сессии
- `GET /logout` - выход

### Пользователь
- `GET /api/v1/user/me` - текущий пользователь
- `PATCH /api/v1/user/settings` - обновление настроек
- `POST /api/v1/user/avatar` - загрузка аватара
- `DELETE /api/v1/user/avatar` - удаление аватара
- `POST /api/v1/user/delete` - удаление аккаунта

### Друзья
- `GET /api/v1/friends/status/:user_id` - проверить статус дружбы
- `POST /api/v1/friends/request` - отправить запрос в друзья
- `GET /api/v1/friends/requests` - список запросов
- `POST /api/v1/friends/requests/:id/respond` - ответить на запрос (accept/decline)
- `DELETE /api/v1/friends/requests/:id` - отменить запрос
- `GET /api/v1/friends` - список друзей
- `DELETE /api/v1/friends/:id` - удалить из друзей
- `POST /api/v1/dm/:user_id/create` - создать DM комнату

### Поиск
- `GET /api/v1/search/users?q=query&limit=20` - поиск пользователей
- `GET /api/v1/search/servers?q=query&limit=20` - поиск серверов
- `GET /api/v1/search?q=query&limit=10` - глобальный поиск (users, rooms, messages)

### Комнаты и каналы
- `GET /api/v1/rooms` - список комнат
- `GET /api/v1/room/:room_id` - информация о комнате
- `POST /api/v1/room/:room_id/join` - присоединиться к комнате
- `GET /api/v1/room/:room_id/members` - участники комнаты
- `GET /api/v1/room/:room_id/roles` - роли в комнате
- `GET /api/v1/channel/:channel_id/messages` - сообщения канала
- `POST /api/v1/channel/:channel_id/mark_read` - отметить как прочитанное
- `GET /channels/accessible` - доступные каналы пользователя

### Управление каналами
- `POST /api/v1/room/:room_id/add_channel` - создать канал
- `PATCH /api/v1/channel/:channel_id/edit` - редактировать канал
- `DELETE /api/v1/channel/:channel_id/delete` - удалить канал
- `PATCH /api/v1/channel/:channel_id/permissions` - изменить права записи

### Управление комнатами
- `GET /api/v1/room/:room_id/settings` - настройки комнаты
- `PATCH /api/v1/room/:room_id/settings` - обновить настройки комнаты
- `POST /api/v1/room/:room_id/avatar/delete` - удалить аватар комнаты
- `DELETE /api/v1/room/:room_id/delete` - удалить комнату
- `GET /api/v1/room/:room_id/bans` - список забаненных
- `POST /api/v1/room/:room_id/unban/:user_id` - разбанить

### Управление ролями
- `POST /api/v1/room/:room_id/roles` - создать роль
- `GET /api/v1/room/:room_id/roles/:role_id` - информация о роли
- `PATCH /api/v1/room/:room_id/roles/:role_id` - обновить роль
- `DELETE /api/v1/room/:room_id/roles/:role_id` - удалить роль
- `PATCH /api/v1/room/:room_id/roles/:role_id/permissions` - обновить разрешения роли
- `POST /api/v1/room/:room_id/roles/mention_permissions` - добавить разрешение на упоминание
- `DELETE /api/v1/room/:room_id/roles/mention_permissions` - удалить разрешение на упоминание
- `POST /api/v1/room/:room_id/members/:member_user_id/roles` - назначить роль участнику
- `DELETE /api/v1/room/:room_id/members/:member_user_id/roles/:role_id` - снять роль с участника

### Сообщения
- `POST /api/v1/message/:message_id/reaction` - добавить реакцию
- `POST /api/v1/message/:message_id/delete` - удалить сообщение
- `POST /api/v1/message/:message_id/edit` - редактировать сообщение
- `POST /api/v1/message/:message_id/forward` - переслать сообщение

### Музыкальная библиотека
- `POST /api/v1/music/add` - добавить трек
- `GET /api/v1/user/music` - список треков
- `POST /api/v1/music/:music_id/delete` - удалить трек

### Стикерпаки
- `POST /api/v1/sticker_packs` - создать стикерпак
- `GET /api/v1/sticker_packs` - список стикерпаков
- `GET /api/v1/sticker_packs/:pack_id` - информация о стикерпаке
- `PATCH /api/v1/sticker_packs/:pack_id` - обновить стикерпак
- `DELETE /api/v1/sticker_packs/:pack_id` - удалить стикерпак
- `POST /api/v1/sticker_packs/:pack_id/stickers` - добавить стикер
- `DELETE /api/v1/stickers/:sticker_id` - удалить стикер

### Администрирование
- `POST /api/v1/admin/user/:user_id/ban` - забанить (глобально или в комнате)
- `POST /api/v1/admin/user/:user_id/unban` - разбанить
- `GET /api/v1/admin/banned_users` - список забаненных
- `GET /api/v1/admin/banned_ips` - список забаненных IP
- `POST /api/v1/admin/user/:user_id/kick_from_room/:room_id` - кикнуть
- `POST /api/v1/admin/user/:user_id/mute_in_room/:room_id` - замьютить
- `POST /api/v1/admin/user/:user_id/unmute_in_room/:room_id` - размьютить
- `POST /api/v1/admin/user/:user_id/promote` - повысить
- `POST /api/v1/admin/user/:user_id/demote` - понизить
- `POST /api/v1/admin/user/:user_id/delete_messages` - удалить сообщения
- `POST /api/v1/admin/user/:user_id/change_password` - сменить пароль
- `POST /api/v1/user/change_password` - сменить свой пароль

### Дополнительно
- `POST /api/v1/room/:room_id/invite` - создать инвайт
- `GET /api/v1/join/:token` - войти по инвайту
- `POST /api/v1/room/:room_id/leave` - покинуть комнату
- `POST /api/v1/room/:room_id/delete_dm` - удалить DM
- `GET /api/v1/user/:user_id/profile` - профиль пользователя
- `GET /api/v1/statistics` - статистика (суперюзер)
- `POST /api/v1/room/:room_id/banner` - баннер комнаты
- `DELETE /api/v1/room/:room_id/banner/delete` - удалить баннер

### GIF (Giphy)
- `GET /api/v1/gifs/trending?limit=24&offset=0` - трендовые GIF
- `GET /api/v1/gifs/search?q=query&limit=24&offset=0` - поиск GIF

### Загрузка файлов
- `POST /upload_file` - загрузить файл
- `GET /uploads/:filepath` - получить файл

### WebSocket (Socket.IO v4)
- `GET /socket.io` - Socket.IO подключение (требует аутентификации)

#### События (клиент → сервер):
- `join` - присоединиться к каналу
  ```json
  {"channel_id": 1}
  ```
- `send_message` - отправить сообщение
  ```json
  {"room_id": 1, "channel_id": 1, "msg": "Hello", "message_type": "text"}
  ```
- `read` - отметить прочитанным
  ```json
  {"channel_id": 1}
  ```

#### События (сервер → клиент):
- `connect` - подключение установлено
- `receive_message` - новое сообщение
- `message_deleted` - сообщение удалено
- `message_edited` - сообщение отредактировано
- `presence_updated` - статус пользователя изменился
- `member_mute_updated` - пользователь замьючен
- `member_removed` - пользователь удалён из комнаты
- `force_redirect` - перенаправление (после kick/ban)
- `command_result` - результат модерационной команды
- `read_status_updated` - статус прочтения обновлён
- `new_dm_created` - создана новая DM комната
- `new_dm_message` - новое сообщение в DM
- `server_removed` - пользователь удалён из сервера
- `bulk_messages_deleted` - массовое удаление сообщений
- `room_state_refresh` - обновить состояние комнаты
- `friend_request_updated` - статус запроса в друзья обновлён
- `error` - ошибка

## Модерационные команды

Доступны в WebSocket через сообщения, начинающиеся с `/`:

### /mute
Замьютить пользователя на указанное время.
```
/mute @username 30m [reason]
/mute @username 2h
/mute @username 1d причина
```

### /unmute
Снять мут с пользователя.
```
/unmute @username
```

### /kick
Выгнать пользователя из комнаты.
```
/kick @username [reason]
```

### /ban
Забанить пользователя в комнате (опционально на время).
```
/ban @username [duration] [reason]
/ban @username 2h спам
/ban @username причина
```

**Разрешения:**
- `mute_members` - для /mute и /unmute
- `kick_members` - для /kick
- `ban_members` - для /ban
- Superuser и room admin имеют все разрешения

## Система упоминаний

Автоматический парсинг упоминаний в сообщениях:

- `@username` - упоминание пользователя
- `@role` - упоминание роли (если есть разрешение)
- `@everyone` - упоминание всех в комнате (если есть разрешение)

Сервер возвращает структуру `mentions` в событии `receive_message`:
```json
{
  "mention_everyone": false,
  "mentioned_user_ids": [1, 2, 3],
  "mentioned_usernames": ["user1", "user2"],
  "mentioned_role_ids": [1],
  "mentioned_role_tags": ["admin"],
  "denied_role_tags": []
}
```

## Отличия от Python версии

1. **WebSocket:** Socket.IO v4 (ручная реализация на gorilla/websocket) вместо Flask-SocketIO
2. **Пароли:** bcrypt вместо scrypt (есть fallback для legacy паролей)
3. **Производительность:** Значительно выше благодаря Go
4. **Безопасность:** HttpOnly cookies, foreign key constraints, транзакции
5. **Функционал:** Полностью соответствует Python версии + дополнительные возможности

## Реализованный функционал

✅ **Аутентификация и авторизация**
- Логин/регистрация с защитой от брутфорса
- Сессии на cookie
- Удаление аккаунта

✅ **Друзья**
- Запросы в друзья
- Принятие/отклонение запросов
- Удаление друзей
- Автоматическое создание DM при принятии запроса

✅ **Комнаты и каналы**
- CRUD для комнат и каналов
- Настройки комнат (аватар, название, описание)
- Управление каналами (создание, редактирование, удаление)
- Права на запись в каналах по ролям

✅ **Роли и разрешения**
- Система ролей с 9 разрешениями
- Управление ролями (CRUD)
- Назначение ролей участникам
- Разрешения на упоминание ролей
- Системные роли (everyone, admin)

✅ **Модерация**
- Мут/анмут с длительностью
- Кик и бан с причиной
- Временные баны
- RoomBan API

✅ **Контент**
- Загрузка файлов (изображения, музыка, видео, документы)
- Музыкальная библиотека
- Стикерпаки и стикеры
- Реакции на сообщения
- GIF через Giphy API

✅ **Поиск**
- Поиск пользователей
- Глобальный поиск (сообщения, комнаты, пользователи)

✅ **WebSocket**
- Присутствие (online/offline)
- Отправка сообщений
- Упоминания (@username, @role, @everyone)
- Модерационные команды
- Real-time уведомления (message edit/delete)
- Friend request уведомления
- DM уведомления
- Read status обновления
- Server removed уведомления
- Bulk message delete уведомления

✅ **Администрирование**
- Глобальный бан/разбан пользователей
- Бан в комнате
- Кик, мут, unmute
- Повышение/понижение ролей
- Удаление сообщений
- Смена пароля пользователю

✅ **Дополнительно**
- Forward сообщений
- Инвайт система
- Leave room / Delete DM
- Профиль пользователя
- Статистика сервера
- Баннеры комнат

## TODO

- [ ] Интеграция с Python scrypt для миграции паролей
