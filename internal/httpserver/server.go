package httpserver

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

type Server struct {
	echo *echo.Echo
}

type databasePinger interface {
	Ping(context.Context) error
}

func New(address string, pool *pgxpool.Pool, logger *slog.Logger) *Server {
	return newServer(address, pool, logger)
}

func newServer(address string, database databasePinger, logger *slog.Logger) *Server {
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true
	e.Server.Addr = address
	e.GET("/health", func(c echo.Context) error {
		if err := database.Ping(c.Request().Context()); err != nil {
			logger.Error("health check failed", "component", "postgresql", "error", err)
			return c.JSON(http.StatusServiceUnavailable, map[string]string{"status": "unavailable"})
		}
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})
	return &Server{echo: e}
}

func (s *Server) Start() error {
	return s.echo.StartServer(s.echo.Server)
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.echo.Shutdown(ctx)
}
