package server

import (
	authhandler "f1/internal/auth/handler"
	"f1/internal/config"
	webhttp "f1/internal/web/handler/http"
	jwtmw "f1/pkg/middleware/jwt"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func setupRouter(
	cfg config.Config,
	authHandler *authhandler.AuthHandler,
	draftHandler *webhttp.DraftHandler,
	h *webhttp.HttpHandler,
	middleware *jwtmw.JWTAuthMiddleware,
) *gin.Engine {
	r := gin.Default()

	r.Use(cors.New(cors.Config{
		AllowOrigins:     cfg.CORSOrigins,
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
	}))

	v1 := r.Group("/api/v1")

	// /auth/register, /auth/login, /auth/refresh — публичные; /auth/logout — под middleware.
	authHandler.RegisterRoutes(v1, middleware)
	draftHandler.RegisterRoutes(v1, middleware)

	game := v1.Group("")
	game.Use(middleware.Handler())
	{
		game.GET("/ws", h.HandleWs)

		// симуляция
		game.POST("/setup", h.ChooseSetup)
		game.GET("/race-result", h.GetRaceResult)
		game.GET("/standing", h.GetStanding)
		game.POST("/rounds/:stage/init", h.InitRound)

		// межсезонье
		game.POST("/updates", h.MakeUpdate)
		game.POST("/token-setup", h.MakeSetup)
		game.POST("/base", h.UpdateBase)
		game.POST("/transfers/pilot", h.PilotTransfer)
		game.POST("/transfers/principal", h.PrincipalTransfer)

		// данные
		game.GET("/pilots", h.GetPilots)
		game.GET("/teams", h.GetTeams)
		game.GET("/principals", h.GetPrincipals)
		game.GET("/track", h.GetTrackInfo)
		game.GET("/my-team", h.GetMyTeam)
		game.GET("/players", h.GetPlayers)
		game.GET("/players/squads", h.GetPlayersSquad)

		// группы
		game.POST("/groups", h.RegisterGroup)
		game.POST("/groups/join", h.JoinGroup)
	}

	return r
}
