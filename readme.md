# BoxChat Messenger

Самописный мессенджер с real-time сообщениями.

## Стек

- **Backend:** Go 1.21+, Gin, GORM, gorilla/websocket (Socket.IO v4)
- **Frontend:** Vite, React 19, TypeScript, Material UI
- **Database:** SQLite

## Быстрый старт

### Бэкенд

```bash
cd go
go mod tidy
go build -o ../boxchat-go ./cmd/server

# Запуск
ADMIN_PASSWORD="YourPassword123!" \
SECRET_KEY="$(openssl rand -hex 32)" \
ALLOWED_ORIGINS="http://localhost,http://127.0.0.1" \
./boxchat-go
```

### Фронтенд

```bash
cd frontend
npm install
npm run dev
```

## Структура

```
BoxChat/
├── go/              # Бэкенд
├── frontend/        # Фронтенд
├── config.json      # Конфигурация
├── uploads/         # Файлы
└── instance/        # БД
```

## API Endpoints

### Аутентификация
- `POST /api/v1/auth/login` — Вход
- `POST /api/v1/auth/register` — Регистрация
- `GET /api/v1/auth/session` — Проверка сессии
- `GET /logout` — Выход

### Пользователь
- `GET /api/v1/user/me` — Текущий пользователь
- `PATCH /api/v1/user/settings` — Обновить настройки
- `POST /api/v1/user/avatar` — Загрузить аватар
- `DELETE /api/v1/user/avatar` — Удалить аватар
- `POST /api/v1/user/delete` — Удалить аккаунт

### Друзья
- `GET /api/v1/friends/status/:user_id` — Статус дружбы
- `POST /api/v1/friends/request` — Отправить запрос
- `GET /api/v1/friends/requests` — Список запросов
- `POST /api/v1/friends/requests/:id/respond` — Ответить
- `DELETE /api/v1/friends/requests/:id` — Отменить
- `GET /api/v1/friends` — Список друзей
- `DELETE /api/v1/friends/:id` — Удалить друга
- `POST /api/v1/dm/:user_id/create` — Создать DM

### Поиск
- `GET /api/v1/search/users?q=&limit=20` — Пользователи
- `GET /api/v1/search/servers?q=&limit=20` — Серверы
- `GET /api/v1/search?q=&limit=10` — Глобальный поиск

### Комнаты и каналы
- `GET /api/v1/rooms` — Список комнат
- `GET /api/v1/room/:room_id` — Инфо о комнате
- `POST /api/v1/room/:room_id/join` — Войти
- `GET /api/v1/room/:room_id/members` — Участники
- `GET /api/v1/room/:room_id/roles` — Роли
- `GET /api/v1/channel/:channel_id/messages` — Сообщения
- `POST /api/v1/channel/:channel_id/mark_read` — Отметить прочитанным

### Управление каналами
- `POST /api/v1/room/:room_id/add_channel` — Создать канал
- `PATCH /api/v1/channel/:channel_id/edit` — Редактировать
- `DELETE /api/v1/channel/:channel_id/delete` — Удалить
- `PATCH /api/v1/channel/:channel_id/permissions` — Права доступа

### Управление комнатами
- `GET /api/v1/room/:room_id/settings` — Настройки
- `PATCH /api/v1/room/:room_id/settings` — Обновить
- `POST /api/v1/room/:room_id/avatar/delete` — Удалить аватар
- `DELETE /api/v1/room/:room_id/delete` — Удалить комнату
- `GET /api/v1/room/:room_id/bans` — Баны
- `POST /api/v1/room/:room_id/unban/:user_id` — Разбанить

### Роли
- `POST /api/v1/room/:room_id/roles` — Создать роль
- `PATCH /api/v1/room/:room_id/roles/:role_id` — Обновить
- `DELETE /api/v1/room/:room_id/roles/:role_id` — Удалить
- `PATCH /api/v1/room/:room_id/roles/:role_id/permissions` — Права
- `POST /api/v1/room/:room_id/members/:user_id/roles` — Назначить
- `DELETE /api/v1/room/:room_id/members/:user_id/roles/:role_id` — Снять

### Сообщения
- `POST /api/v1/message/:id/reaction` — Реакция
- `POST /api/v1/message/:id/delete` — Удалить
- `POST /api/v1/message/:id/edit` — Редактировать
- `POST /api/v1/message/:id/forward` — Переслать

### Музыка
- `POST /api/v1/music/add` — Добавить трек
- `GET /api/v1/user/music` — Список треков
- `POST /api/v1/music/:id/delete` — Удалить

### Стикеры
- `POST /api/v1/sticker_packs` — Создать пак
- `GET /api/v1/sticker_packs` — Список
- `GET /api/v1/sticker_packs/:id` — Инфо
- `PATCH /api/v1/sticker_packs/:id` — Обновить
- `DELETE /api/v1/sticker_packs/:id` — Удалить
- `POST /api/v1/sticker_packs/:id/stickers` — Добавить стикер
- `DELETE /api/v1/stickers/:id` — Удалить

### Администрирование
- `POST /api/v1/admin/user/:id/ban` — Бан
- `POST /api/v1/admin/user/:id/unban` — Разбан
- `POST /api/v1/admin/user/:id/kick_from_room/:room_id` — Кик
- `POST /api/v1/admin/user/:id/mute_in_room/:room_id` — Мут
- `POST /api/v1/admin/user/:id/unmute_in_room/:room_id` — Анмут
- `POST /api/v1/admin/user/:id/promote` — Повысить
- `POST /api/v1/admin/user/:id/demote` — Понизить
- `POST /api/v1/admin/user/:id/delete_messages` — Удалить сообщения

### Дополнительно
- `POST /api/v1/room/:room_id/invite` — Инвайт
- `GET /api/v1/join/:token` — Войти по инвайту
- `POST /api/v1/room/:room_id/leave` — Покинуть комнату
- `GET /api/v1/gifs/trending` — Trending GIF
- `GET /api/v1/gifs/search?q=` — Поиск GIF
- `POST /upload_file` — Загрузить файл
- `GET /uploads/:path` — Получить файл

---

## WebSocket (Socket.IO v4)

**Подключение:** `GET /socket.io` (требуется аутентификация)

### Клиент → Сервер
| Событие | Payload |
|---------|---------|
| `join` | `{ channel_id }` |
| `send_message` | `{ room_id, channel_id, msg, message_type, file_url?, reply_to? }` |
| `read` | `{ channel_id }` |

### Сервер → Клиент
| Событие | Описание |
|---------|----------|
| `connect` | Подключение установлено |
| `receive_message` | Новое сообщение |
| `message_deleted` | Сообщение удалено |
| `message_edited` | Сообщение отредактировано |
| `presence_updated` | Статус пользователя изменился |
| `member_mute_updated` | Пользователь замьючен |
| `member_removed` | Удалён из комнаты |
| `force_redirect` | Kick/ban |
| `command_result` | Результат команды модерации |
| `read_status_updated` | Статус прочтения |
| `new_dm_created` | Создана DM комната |
| `new_dm_message` | Новое сообщение в DM |
| `server_removed` | Удалён из сервера |
| `bulk_messages_deleted` | Массовое удаление сообщений |
| `room_state_refresh` | Обновление состояния комнаты |
| `friend_request_updated` | Статус запроса в друзья |
| `error` | Ошибка |

---

## Модерационные команды

Отправляются как сообщения в WebSocket (начинаются с `/`):

| Команда | Синтаксис |
|---------|-----------|
| `/mute` | `/mute @username 30m [reason]` |
| `/unmute` | `/unmute @username` |
| `/kick` | `/kick @username [reason]` |
| `/ban` | `/ban @username [duration] [reason]` |

**Разрешения:** `mute_members`, `kick_members`, `ban_members`

---

## Упоминания

- `@username` — пользователь
- `@role` — роль (если есть разрешение)
- `@everyone` — все в комнате (если есть разрешение)

---

## Разрешения ролей

`manage_server`, `manage_roles`, `manage_channels`, `invite_members`, `delete_server`, `delete_messages`, `kick_members`, `ban_members`, `mute_members`

---

## Конфигурация

**Файл:** `config.json`

**Переменные окружения:**
- `ADMIN_PASSWORD` — пароль админа
- `SECRET_KEY` — ключ сессии
- `ALLOWED_ORIGINS` — CORS (через запятую)
- `SERVER_HOST` — хост (default: 127.0.0.1)
- `SERVER_PORT` — порт (default: 5000)
