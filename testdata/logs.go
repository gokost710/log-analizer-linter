package testdata

import (
	"context"
	"log/slog"

	"go.uber.org/zap"
)

func badLowercase(ctx context.Context, logger *zap.Logger) {
	slog.Info("Starting server on port 8080")
	slog.Error("Failed to connect to database")

	logger.Info("User authenticated successfully")
	logger.Error("Connection timeout occurred")
}

func goodLowercase(ctx context.Context, logger *zap.Logger) {
	slog.Info("starting server on port 8080")
	slog.Error("failed to connect to database")

	logger.Info("user authenticated successfully")
	logger.Error("connection timeout occurred")
}

func badNonEnglish(ctx context.Context, logger *zap.Logger) {
	slog.Info("запуск сервера")
	slog.Error("ошибка подключения к базе данных")

	logger.Info("пользователь вошел")
	logger.Error("не удалось получить данные")
}

func goodEnglish(ctx context.Context, logger *zap.Logger) {
	slog.Info("starting server")
	slog.Error("failed to connect to database")

	logger.Info("user logged in")
	logger.Error("failed to fetch data")
}

func badSymbols(ctx context.Context, logger *zap.Logger) {
	slog.Info("server started! 🚀")
	slog.Error("connection failed!!!")
	slog.Warn("warning: something went wrong...")

	logger.Info("operation completed ✅")
	logger.Error("critical failure ❌")
}

func goodSymbols(ctx context.Context, logger *zap.Logger) {
	slog.Info("server started")
	slog.Error("connection failed")
	slog.Warn("something went wrong")

	logger.Info("operation completed")
	logger.Error("critical failure")
}

func badSensitive(ctx context.Context, logger *zap.Logger) {
	slog.Info("user password is 123456")
	slog.Info("api_key=XYZ-SECRET-KEY")
	slog.Info("token expired")

	logger.Info("PASSWORD leaked")
	logger.Warn("access token: abcdef123")
	logger.Error("user password mismatch")

	slog.InfoContext(ctx, "api-key invalid")

	sugar := logger.Sugar()
	sugar.Infof("invalid token for user %s", "bob")
}

func goodSensitive(ctx context.Context, logger *zap.Logger) {
	slog.Info("user authenticated successfully")
	slog.Info("api request completed")
	slog.Info("token validated")

	logger.Info("login completed")
	logger.Warn("request timeout")
	logger.Error("operation failed")

	sugar := logger.Sugar()
	sugar.Infof("request completed for user %s", "bob")
}

func chaosTest(ctx context.Context, logger *zap.Logger) {
	slog.Info("Starting сервер 🚀 with api_key and TOKEN")
	logger.Info("PASSWORD 🔥 leaked")
	slog.InfoContext(ctx, "Ошибка password mismatch")
}
