# 📚 Swagger UI для Auth Service

## ✅ Установка завершена!

Swagger UI успешно интегрирован в Auth Service.

---

## 🚀 Как использовать

### 1. Запустите сервис:
```bash
cd service/Auth
go run cmd/main.go
```

### 2. Откройте Swagger UI в браузере:
```
http://localhost:8081/swagger/index.html
```

---

## 📝 Доступные endpoints в Swagger

### Public (без авторизации):
- `GET /health` - Health check
- `POST /auth/register` - Регистрация
- `POST /auth/login` - Вход
- `POST /auth/refresh` - Обновление токена
- `POST /auth/logout` - Выход

### Protected (требуется JWT токен):
- `GET /api/me` - Информация о текущем пользователе

### Admin (только для админов):
- `GET /admin/users` - Список пользователей
- `POST /admin/users` - Создать пользователя
- `PUT /admin/users/:id/role` - Изменить роль
- `DELETE /admin/users/:id` - Удалить пользователя

---

## 🔑 Как тестировать protected endpoints

### 1. Зарегистрируйтесь:
```bash
POST /auth/register
{
  "email": "test@example.com",
  "password": "password123"
}
```

### 2. Получите токен:
```bash
POST /auth/login
{
  "email": "test@example.com",
  "password": "password123"
}

# Ответ:
{
  "access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "refresh_token": "...",
  "expires_in": 900
}
```

### 3. В Swagger UI:
1. Нажмите **Authorize** (кнопка с замочком вверху справа)
2. Введите: `Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...`
3. Нажмите **Authorize**
4. Теперь можете тестировать protected endpoints!

---

## 🔄 Обновление документации

### После изменения API:
```bash
cd service/Auth
swag init -g cmd/main.go -o ./docs
```

### Автоматизация через Makefile:
Создайте `Makefile`:
```makefile
.PHONY: swagger
swagger:
	swag init -g cmd/main.go -o ./docs

.PHONY: run
run: swagger
	go run cmd/main.go

.PHONY: dev
dev:
	air  # Если используете air для hot-reload
```

---

## 📦 Установленные пакеты

- `github.com/swaggo/gin-swagger` - Swagger UI для Gin
- `github.com/swaggo/files` - Статические файлы Swagger
- `github.com/swaggo/swag` - CLI для генерации документации

---

## 🎨 Swagger аннотации

В `cmd/main.go`:
```go
// @title Auth Service API
// @version 1.0
// @description JWT-based authentication service with refresh tokens
// @host localhost:8081
// @BasePath /
```

В контроллерах (если нужно добавить):
```go
// @Summary Регистрация пользователя
// @Tags auth
// @Accept json
// @Produce json
// @Param body body RegisterRequest true "Данные для регистрации"
// @Success 201 {object} TokenPair
// @Router /auth/register [post]
func (ac *AuthController) Register(c *gin.Context) {
    // ...
}
```

---

## 📂 Структура файлов

```
service/Auth/
├── cmd/
│   └── main.go              ← Swagger аннотации здесь
├── docs/                    ← Сгенерированные файлы
│   ├── docs.go
│   ├── swagger.json
│   └── swagger.yaml
└── internal/
    └── controllers/
        ├── auth.go          ← Можно добавить аннотации
        └── admin.go
```

---

## 🔍 Примеры запросов в Swagger

### Register:
```json
{
  "email": "user@example.com",
  "password": "SecurePass123"
}
```

### Login:
```json
{
  "email": "user@example.com",
  "password": "SecurePass123"
}
```

### Create User (Admin):
```json
{
  "email": "newuser@example.com",
  "password": "password123",
  "role": "user"
}
```

### Update Role (Admin):
```json
{
  "role": "admin"
}
```

---

## 🐛 Troubleshooting

### Swagger UI не открывается:
1. Проверьте, что сервис запущен: `curl http://localhost:8081/health`
2. Убедитесь, что документация сгенерирована: `ls docs/`
3. Пересоздайте документацию: `swag init -g cmd/main.go -o ./docs`

### Endpoints не отображаются:
- Добавьте Swagger аннотации в контроллеры
- Пересгенерируйте: `swag init -g cmd/main.go -o ./docs`
- Перезапустите сервис

### Ошибка импорта:
```bash
go mod tidy
go build ./cmd/main.go
```

---

## 🎯 Production настройки

В production отключите Swagger или ограничьте доступ:

```go
// Только для dev окружения
if os.Getenv("ENV") == "development" {
    r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
}

// Или с авторизацией
swagger := r.Group("/swagger")
swagger.Use(middleware.BasicAuth())
{
    swagger.GET("/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
}
```

---

## ✨ Готово!

**Swagger UI доступен:** http://localhost:8081/swagger/index.html

Приятного тестирования! 🚀
