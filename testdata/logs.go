package testdata

import (
  "log/slog"

  "go.uber.org/zap"
)

func badLowercase(logger *zap.Logger) {
  slog.Info("Starting server on port 8080")
  slog.Error("Failed to connect to database")

  logger.Info("User authenticated successfully")
  logger.Error("Connection timeout occurred")
}

func badNonEnglish(logger *zap.Logger) {
  slog.Info("run сервер")
  slog.Error("hello мир")

  logger.Info("user ㋿ sign in")
  logger.Error("не удалось получить данные")
}

func badSymbols(logger *zap.Logger) {
  slog.Info("server started! 🚀")
  slog.Error("connection failed!!!")
  slog.Warn("warning: something went wrong...")

  logger.Info("operation completed ✅")
  logger.Error("critical failure ❌")
}

func badSensitive(logger *zap.Logger) {
  slog.Info("user password is qweqwe123asd")
  slog.Info("token is fndjnfdnjifdnifdv")

  logger.Info("secret asd")
  logger.Error("user credentials mismatch")
}
