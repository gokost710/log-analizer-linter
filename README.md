# gologanalizer
Golang линтер для анализа логов.

## Описание
Линтер анализирует вызовы логгеров и проверяет текст лог-сообщений на соответствие заданным правилам.

Поддерживаемые логгеры:
- [log/slog](https://pkg.go.dev/log/slog)
- [go.uber.org/zap](https://github.com/uber-go/zap)

---

## Проверяемые правила
1. **Сообщения должны начинаться со строчной буквы**
    
    <table>
    <thead><tr><th>❌ Неправильно</th><th>✅ Правильно</th></tr></thead>
    <tbody>
    <tr><td>
    
    ```go
    log.Info("Starting server on port 8080")
    slog.Error("Failed to connect to database")
    ```
    
    </td><td>
    
    ```go
    log.Info("starting server on port 8080")
    slog.Error("failed to connect to database")
    ```
    
    </td></tr>
    
    </tbody>
    </table>

2. **Сообщения должны быть на английском языке**
    
    <table>
    <thead><tr><th>❌ Неправильно</th><th>✅ Правильно</th></tr></thead>
    <tbody>
    <tr><td>
    
    ```go
    log.Info("запуск сервера")
    log.Error("ошибка подключения к базе данных")
    ```
    
    </td><td>
    
    ```go
    log.Info("starting server")
    log.Error("failed to connect to database")
    ```
    
    </td></tr>
    
    </tbody>
    </table>

3. **Запрещены спецсимволы и эмодзи**
    
    <table>
    <thead><tr><th>❌ Неправильно</th><th>✅ Правильно</th></tr></thead>
    <tbody>
    <tr><td>
    
    ```go
    log.Info("server started!🚀")
    log.Error("connection failed!!!")
    log.Warn("warning: something went wrong...")
    ```
    
    </td><td>
    
    ```go
    log.Info("server started")
    log.Error("connection failed")
    log.Warn("something went wrong")
    ```
    
    </td></tr>
    
    </tbody>
    </table>

4. Запрещены потенциально чувствительные данные
    
    <table>
    <thead><tr><th>❌ Неправильно</th><th>✅ Правильно</th></tr></thead>
    <tbody>
    <tr><td>
    
    ```go
    log.Info("user password: " + password)
    log.Debug("api_key=" + apiKey)
    log.Info("token: " + token)
    ```
    
    </td><td>
    
    ```go
    log.Info("user authenticated successfully")
    log.Debug("api request completed")
    log.Info("token validated")
    ```
    
    </td></tr>
    
    </tbody>
    </table>

---

## Требования

 - [golang](https://github.com/golang/go) v1.26+
 - [golangci-lint](https://github.com/golangci/golangci-lint) v2+

---
## Установка и подключение

Линтер подключается как модульный плагин golangci-lint.

1. **Создать файл `.custom-gcl.yml`**
   ```yaml
   version: v2.10.1
   name: custom-gcl
   destination: ./bin

   plugins:
     - module: github.com/gokost710/log-analizer-linter
       import: github.com/gokost710/log-analizer-linter
       version: v1.0.0
   ```


2. **Собрать кастомный golangci-lint**
   ```bash
   golangci-lint custom
   ```

   _Будет создан бинарник:_

   ```bash
   ./bin/custom-gcl
   ```

3. **Включить линтер в .golangci.yml**
   ```yaml
   linters:
     enable:
       - gologanalyzer

   linters-settings:
     gologanalyzer:
       check-lowercase: true
       check-english: true
       check-symbols: true
       check-sensitive: true
       sensitive-patterns:
         - "(?i)password"
         - "(?i)token"
         - "(?i)api[_-]?key"
   ```

4. **Запуск**
   ```bash
   ./bin/custom-gcl run ./...
   ```

## Конфигурация
   
   | Параметр           | Описание                        | Значение по умолчанию |
   | ------------------ | ------------------------------- |-----------------------|
   | check-lowercase    | Проверка строчной буквы         | true                  |
   | check-english      | Проверка английского языка      | true                  |
   | check-symbols      | Проверка спецсимволов           | true                  |
   | check-sensitive    | Проверка чувствительных данных  | true                  |
   | sensitive-patterns | Пользовательские regex-паттерны | -                     |

## Авто-исправление
Для автоматического исправления запускать с флагом `--fix`
```shell
./bin/custom-gcl run --fix ./...
```

## CI/CD

В репозитории настроен CI, который:
- собирает плагин
- запускает unit-тесты
- проверяет корректность сборки

CI запускается при push и pull request.