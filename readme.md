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

# Запуск с переменными окружения
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
├── go/                    # Бэкенд на Go
│   ├── cmd/
│   │   └── server/        # Точка входа (main.go)
│   └── internal/          # Внутренние пакеты
│       ├── config/        # Конфигурация
│       ├── database/      # Работа с БД
│       ├── handlers/      # HTTP обработчики
│       ├── middleware/    # Middleware (CORS, auth, logger)
│       ├── models/        # Модели данных
│       ├── services/      # Бизнес-логика
│       ├── utils/         # Утилиты
│       └── testutil/      # Тестовые хелперы
├── frontend/              # Фронтенд на React
├── config.yaml            # Конфигурация приложения
├── uploads/               # Загруженные файлы
└── instance/              # SQLite база данных
```

## Конфигурация

**Файл:** `config.yaml` (скопируйте `config.yaml.example`)

Основные поля:

| Секция | Поле | Тип | Описание | Default |
|--------|------|-----|----------|---------|
| `database` | `path` | string | Путь к SQLite базе | `instance/boxchat.db` |
| `server` | `host` | string | Хост сервера | `127.0.0.1` |
| `server` | `port` | int | Порт сервера | `5000` |
| `security` | `secret_key` | string | Ключ сессий/JWT | автогенерация |
| `upload` | `folder` | string | Папка загрузок | `uploads` |
| `upload` | `max_size` | int | Макс. размер файла (байты) | `52428800` (50MB) |
| `session` | `lifetime_days` | int | Время жизни сессии (дни) | `30` |
| `giphy` | `api_key` | string | Giphy API ключ | — |

Полный пример — в [`config.yaml.example`](config.yaml.example).

### Переменные окружения

Приоритет над `config.yaml`. Можно использовать для sensitive данных.

| Переменная | Описание | Default |
|------------|----------|---------|
| `SECRET_KEY` | Секретный ключ для сессий | автогенерация |
| `SERVER_HOST` | Хост сервера | `127.0.0.1` |
| `SERVER_PORT` | Порт сервера | `5000` |
| `DATABASE_PATH` | Путь к SQLite базе | `instance/boxchat.db` |
| `UPLOAD_FOLDER` | Папка для загрузок | `uploads` |
| `MAX_CONTENT_LENGTH` | Макс. размер файла (байты) | `52428800` (50MB) |
| `GIPHY_API_KEY` | API ключ Giphy | — |
| `ALLOWED_ORIGINS` | CORS origin (через запятую) | — |
| `ADMIN_PASSWORD` | Пароль первого админа | — |

Пример запуска:

```bash
SECRET_KEY="my_super_secret_key" \
SERVER_PORT="8080" \
./boxchat-go
```

---

## API Endpoints

### Аутентификация
| Метод | Endpoint | Описание |
|-------|----------|----------|
| `POST` | `/api/v1/auth/login` | Вход |
| `POST` | `/api/v1/auth/register` | Регистрация |
| `GET` | `/api/v1/auth/session` | Проверка сессии |
| `GET` | `/logout` | Выход |

### Пользователь
| Метод | Endpoint | Описание |
|-------|----------|----------|
| `GET` | `/api/v1/user/me` | Текущий пользователь |
| `PATCH` | `/api/v1/user/settings` | Обновить настройки |
| `POST` | `/api/v1/user/avatar` | Загрузить аватар |
| `DELETE` | `/api/v1/user/avatar` | Удалить аватар |
| `POST` | `/api/v1/user/delete` | Удалить аккаунт |

### Друзья
| Метод | Endpoint | Описание |
|-------|----------|----------|
| `GET` | `/api/v1/friends/status/:user_id` | Статус дружбы |
| `POST` | `/api/v1/friends/request` | Отправить запрос |
| `GET` | `/api/v1/friends/requests` | Список запросов |
| `POST` | `/api/v1/friends/requests/:id/respond` | Ответить на запрос |
| `DELETE` | `/api/v1/friends/requests/:id` | Отменить запрос |
| `GET` | `/api/v1/friends` | Список друзей |
| `DELETE` | `/api/v1/friends/:id` | Удалить друга |
| `POST` | `/api/v1/dm/:user_id/create` | Создать DM |

### Поиск
| Метод | Endpoint | Описание |
|-------|----------|----------|
| `GET` | `/api/v1/search/users` | Поиск пользователей (`?q=`, `?limit=`) |
| `GET` | `/api/v1/search/servers` | Поиск серверов |
| `GET` | `/api/v1/search` | Глобальный поиск |

### Комнаты и каналы
| Метод | Endpoint | Описание |
|-------|----------|----------|
| `GET` | `/api/v1/rooms` | Список комнат |
| `GET` | `/api/v1/room/:room_id` | Инфо о комнате |
| `POST` | `/api/v1/room/:room_id/join` | Войти в комнату |
| `GET` | `/api/v1/room/:room_id/members` | Участники |
| `GET` | `/api/v1/room/:room_id/roles` | Роли |
| `GET` | `/api/v1/channel/:channel_id/messages` | Сообщения канала |
| `POST` | `/api/v1/channel/:channel_id/mark_read` | Отметить прочитанным |

### Управление каналами
| Метод | Endpoint | Описание |
|-------|----------|----------|
| `POST` | `/api/v1/room/:room_id/add_channel` | Создать канал |
| `PATCH` | `/api/v1/channel/:channel_id/edit` | Редактировать |
| `DELETE` | `/api/v1/channel/:channel_id/delete` | Удалить |
| `PATCH` | `/api/v1/channel/:channel_id/permissions` | Права доступа |

### Управление комнатами
| Метод | Endpoint | Описание |
|-------|----------|----------|
| `GET` | `/api/v1/room/:room_id/settings` | Настройки |
| `PATCH` | `/api/v1/room/:room_id/settings` | Обновить |
| `POST` | `/api/v1/room/:room_id/avatar/delete` | Удалить аватар |
| `DELETE` | `/api/v1/room/:room_id/delete` | Удалить комнату |
| `GET` | `/api/v1/room/:room_id/bans` | Баны |
| `POST` | `/api/v1/room/:room_id/unban/:user_id` | Разбанить |

### Роли
| Метод | Endpoint | Описание |
|-------|----------|----------|
| `POST` | `/api/v1/room/:room_id/roles` | Создать роль |
| `PATCH` | `/api/v1/room/:room_id/roles/:role_id` | Обновить |
| `DELETE` | `/api/v1/room/:room_id/roles/:role_id` | Удалить |
| `PATCH` | `/api/v1/room/:room_id/roles/:role_id/permissions` | Права роли |
| `POST` | `/api/v1/room/:room_id/members/:user_id/roles` | Назначить роль |
| `DELETE` | `/api/v1/room/:room_id/members/:user_id/roles/:role_id` | Снять роль |

### Сообщения
| Метод | Endpoint | Описание |
|-------|----------|----------|
| `POST` | `/api/v1/message/:id/reaction` | Добавить реакцию |
| `POST` | `/api/v1/message/:id/delete` | Удалить сообщение |
| `POST` | `/api/v1/message/:id/edit` | Редактировать сообщение |
| `POST` | `/api/v1/message/:id/forward` | Переслать сообщение |

### Музыка
| Метод | Endpoint | Описание |
|-------|----------|----------|
| `POST` | `/api/v1/music/add` | Добавить трек |
| `GET` | `/api/v1/user/music` | Список треков |
| `POST` | `/api/v1/music/:id/delete` | Удалить трек |

### Стикеры
| Метод | Endpoint | Описание |
|-------|----------|----------|
| `POST` | `/api/v1/sticker_packs` | Создать пак |
| `GET` | `/api/v1/sticker_packs` | Список паков |
| `GET` | `/api/v1/sticker_packs/:id` | Инфо о паке |
| `PATCH` | `/api/v1/sticker_packs/:id` | Обновить пак |
| `DELETE` | `/api/v1/sticker_packs/:id` | Удалить пак |
| `POST` | `/api/v1/sticker_packs/:id/stickers` | Добавить стикер |
| `DELETE` | `/api/v1/stickers/:id` | Удалить стикер |

### Администрирование
| Метод | Endpoint | Описание |
|-------|----------|----------|
| `POST` | `/api/v1/admin/user/:id/ban` | Забанить |
| `POST` | `/api/v1/admin/user/:id/unban` | Разбанить |
| `POST` | `/api/v1/admin/user/:id/kick_from_room/:room_id` | Кикнуть из комнаты |
| `POST` | `/api/v1/admin/user/:id/mute_in_room/:room_id` | Замутить в комнате |
| `POST` | `/api/v1/admin/user/:id/unmute_in_room/:room_id` | Снять мут |
| `POST` | `/api/v1/admin/user/:id/promote` | Повысить |
| `POST` | `/api/v1/admin/user/:id/demote` | Понизить |
| `POST` | `/api/v1/admin/user/:id/delete_messages` | Удалить сообщения |

### Дополнительно
| Метод | Endpoint | Описание |
|-------|----------|----------|
| `POST` | `/api/v1/room/:room_id/invite` | Создать инвайт |
| `GET` | `/api/v1/join/:token` | Войти по инвайту |
| `POST` | `/api/v1/room/:room_id/leave` | Покинуть комнату |
| `GET` | `/api/v1/gifs/trending` | Trending GIF |
| `GET` | `/api/v1/gifs/search` | Поиск GIF (`?q=`) |
| `POST` | `/upload_file` | Загрузить файл |
| `GET` | `/uploads/:path` | Получить файл |

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

**Требуемые разрешения:** `mute_members`, `kick_members`, `ban_members`

---

## Упоминания

- `@username` — упомянуть пользователя
- `@role` — упомянуть роль (требуется разрешение)
- `@everyone` — упомянуть всех в комнате (требуется разрешение)

---

## Разрешения ролей

| Разрешение | Описание |
|------------|----------|
| `manage_server` | Управление сервером |
| `manage_roles` | Создание и редактирование ролей |
| `manage_channels` | Управление каналами |
| `invite_members` | Приглашение участников |
| `delete_server` | Удаление сервера |
| `delete_messages` | Удаление чужих сообщений |
| `kick_members` | Кик участников |
| `ban_members` | Бан участников |
| `mute_members` | Мут участников |

---

## Безопасность

- **Пароли:** хешируются через `bcrypt`
- **Сессии:** JWT + httpOnly cookies
- **CORS:** настраиваемый список разрешённых origin
- **Rate limiting:** защита от brute-force
- **Security headers:** X-Frame-Options, X-Content-Type-Options, CSP

---

## Тестирование

```bash
cd go
go test ./... -v
```

Запуск с покрытием:

```bash
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out
```

---

## Примеры API запросов

### Регистрация

```bash
curl -X POST http://localhost:5000/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{"username": "user", "password": "password123", "email": "user@example.com"}'
```

### Вход

```bash
curl -X POST http://localhost:5000/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username": "user", "password": "password123"}' \
  -c cookies.txt
```

### Отправка сообщения (WebSocket)

```javascript
const socket = io('http://localhost:5000/socket.io');

socket.on('connect', () => {
  socket.emit('send_message', {
    room_id: 1,
    channel_id: 2,
    msg: 'Hello, World!',
    message_type: 'text'
  });
});
```
