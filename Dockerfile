# Используем базовый образ Golang для сборки приложения
FROM golang:1.22.7-alpine AS builder

# Устанавливаем необходимые зависимости
RUN apk add --no-cache git

# Устанавливаем рабочую директорию внутри контейнера
WORKDIR /app

# Копируем файлы go.mod и go.sum для управления зависимостями
COPY go.mod go.sum ./

# Загружаем зависимости
RUN go mod download

# Копируем все исходные файлы приложения
COPY . .

# Собираем приложение, отключая CGO и указывая целевую ОС Linux
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/user ./cmd/main.go

# Создаем финальный образ на основе Alpine Linux
FROM alpine:3.19

# Устанавливаем необходимые зависимости для запуска приложения
RUN apk --no-cache add ca-certificates

# Устанавливаем рабочую директорию
WORKDIR /app

# Копируем собранный бинарный файл из предыдущего этапа сборки
COPY --from=builder /app/user .

# Копируем файл .env с переменными окружения
COPY ./cmd/.env .

# Создаем директорию для логов
RUN mkdir -p /app/logs

# Открываем порт, который будет прослушивать приложение
EXPOSE 50052

# Запускаем приложение
CMD ["./user"]
