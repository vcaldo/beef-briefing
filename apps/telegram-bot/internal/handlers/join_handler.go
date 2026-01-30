package handlers

import (
	"context"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

type JoinHandler struct {
	matchHandler *MatchHandler
}

func NewJoinHandler(matchHandler *MatchHandler) *JoinHandler {
	return &JoinHandler{matchHandler: matchHandler}
}

func (h *JoinHandler) Handle(ctx context.Context, b *bot.Bot, update *models.Update) {
	h.matchHandler.HandleMatchOrJoin(ctx, b, update)
}
