package telegram

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

const startMessage = "DevTimeTracker запущен.\n\nСистема готова к работе."

type Bot struct {
	client *bot.Bot
}

func New(token string, allowedUserID int64, logger *slog.Logger) (*Bot, error) {
	authorizer := Authorizer{AllowedUserID: allowedUserID}
	client, err := bot.New(token,
		bot.WithMiddlewares(accessMiddleware(authorizer, logger)),
		bot.WithDefaultHandler(ignoreHandler),
		bot.WithErrorsHandler(func(err error) {
			logger.Error("Telegram bot error", "error", err)
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("create Telegram bot: %w", err)
	}
	client.RegisterHandler(bot.HandlerTypeMessageText, "/start", bot.MatchTypeExact, startHandler(logger))
	return &Bot{client: client}, nil
}

func ignoreHandler(context.Context, *bot.Bot, *models.Update) {}

func (b *Bot) Start(ctx context.Context) {
	b.client.Start(ctx)
}

type Authorizer struct {
	AllowedUserID int64
}

func (a Authorizer) Allowed(userID int64) bool {
	return userID == a.AllowedUserID
}

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
		_, err := client.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "Access denied",
		})
		if err != nil {
			logger.Error("failed to send Telegram access denial", "error", err)
		}
		return
	}
	if update.CallbackQuery != nil {
		_, err := client.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: update.CallbackQuery.ID,
			Text:            "Access denied",
			ShowAlert:       true,
		})
		if err != nil {
			logger.Error("failed to answer unauthorized Telegram callback", "error", err)
		}
	}
}

func startHandler(logger *slog.Logger) bot.HandlerFunc {
	return func(ctx context.Context, client *bot.Bot, update *models.Update) {
		if update.Message == nil {
			return
		}
		_, err := client.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   startMessage,
		})
		if err != nil {
			logger.Error("failed to send Telegram start response", "error", err)
		}
	}
}
