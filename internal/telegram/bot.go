package telegram

import (
	"context"
	"fmt"
	"log/slog"

	"dev-work-tracker/internal/service"
	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

type Bot struct {
	client   *bot.Bot
	service  *service.Service
	logger   *slog.Logger
	sessions *sessionStore
}

func New(token string, allowedUserID int64, appService *service.Service, logger *slog.Logger) (*Bot, error) {
	app := &Bot{service: appService, logger: logger, sessions: newSessionStore()}
	authorizer := Authorizer{AllowedUserID: allowedUserID}
	client, err := bot.New(token,
		bot.WithNotAsyncHandlers(),
		bot.WithMiddlewares(accessMiddleware(authorizer, logger)),
		bot.WithDefaultHandler(app.handleUpdate),
		bot.WithErrorsHandler(func(err error) { logger.Error("Telegram bot error", "error", err) }),
	)
	if err != nil {
		return nil, fmt.Errorf("create Telegram bot: %w", err)
	}
	app.client = client
	return app, nil
}

func (b *Bot) Start(ctx context.Context) {
	_, err := b.client.SetMyCommands(ctx, &bot.SetMyCommandsParams{Commands: []models.BotCommand{
		{Command: "start", Description: "Главное меню"}, {Command: "new", Description: "Добавить работу"},
		{Command: "report", Description: "Отчёт за месяц"}, {Command: "export", Description: "Выгрузить Excel"},
		{Command: "projects", Description: "Проекты"}, {Command: "payments", Description: "Оплаты"},
		{Command: "cancel", Description: "Отменить текущий сценарий"},
	}})
	if err != nil {
		b.logger.Warn("failed to set Telegram commands", "error", err)
	}
	b.client.Start(ctx)
}

type Authorizer struct{ AllowedUserID int64 }

func (a Authorizer) Allowed(userID int64) bool { return userID == a.AllowedUserID }

func accessMiddleware(authorizer Authorizer, logger *slog.Logger) bot.Middleware {
	return func(next bot.HandlerFunc) bot.HandlerFunc {
		return func(ctx context.Context, client *bot.Bot, update *models.Update) {
			userID, ok := updateUserID(update)
			if !ok {
				return
			}
			if !authorizer.Allowed(userID) {
				logger.Warn("unauthorized Telegram access attempt", "telegram_user_id", userID)
				denyAccess(ctx, client, update, logger)
				return
			}
			next(ctx, client, update)
		}
	}
}
func updateUserID(update *models.Update) (int64, bool) {
	if update.Message != nil && update.Message.From != nil {
		return update.Message.From.ID, true
	}
	if update.CallbackQuery != nil {
		return update.CallbackQuery.From.ID, true
	}
	return 0, false
}
func denyAccess(ctx context.Context, client *bot.Bot, update *models.Update, logger *slog.Logger) {
	if update.Message != nil {
		if _, err := client.SendMessage(ctx, &bot.SendMessageParams{ChatID: update.Message.Chat.ID, Text: "Access denied"}); err != nil {
			logger.Error("failed to send Telegram access denial", "error", err)
		}
		return
	}
	if update.CallbackQuery != nil {
		if _, err := client.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{CallbackQueryID: update.CallbackQuery.ID, Text: "Access denied", ShowAlert: true}); err != nil {
			logger.Error("failed to answer unauthorized Telegram callback", "error", err)
		}
	}
}
