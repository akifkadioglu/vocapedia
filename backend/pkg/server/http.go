package server

import (
	"io/fs"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"github.com/go-chi/jwtauth/v5"

	"github.com/akifkadioglu/vocapedia/pkg/controllers/auth"
	"github.com/akifkadioglu/vocapedia/pkg/controllers/chapters"
	"github.com/akifkadioglu/vocapedia/pkg/controllers/payment"
	"github.com/akifkadioglu/vocapedia/pkg/controllers/stream"
	"github.com/akifkadioglu/vocapedia/pkg/controllers/user"
	"github.com/akifkadioglu/vocapedia/pkg/embed"
	customMiddleware "github.com/akifkadioglu/vocapedia/pkg/middleware"
	"github.com/akifkadioglu/vocapedia/pkg/token"
)

type Server struct {
	Router *chi.Mux
}

func CreateNewServer() *Server {
	s := &Server{}
	s.Router = chi.NewRouter()
	return s
}

func (s *Server) MountHandlers(host string, port int, allowMethods []string, allowOrigins []string, allowHeaders []string) {
	s.Router.Use(customMiddleware.Logger)
	s.Router.Use(cors.Handler(cors.Options{
		AllowedOrigins: allowOrigins,
		AllowedMethods: allowMethods,
		AllowedHeaders: allowHeaders,
		MaxAge:         300,
	}))
	s.Router.Use(customMiddleware.Language)
	s.Router.Use(customMiddleware.SecurityHeaders)
	//s.Router.Use(customMiddleware.RateLimiter(6, time.Second))
	s.Router.Use(jwtauth.Verifier(token.TokenAuth()))

	s.Router.Route("/v1", func(api chi.Router) {
		api.Group(func(api chi.Router) {
			api.Use(customMiddleware.HandleToken)
			api.Use(jwtauth.Authenticator(token.TokenAuth()))

			api.Route("/chapters", func(api chi.Router) {
				api.Get("/favorite", chapters.Favorites)
				api.Post("/favorite", chapters.Favorite)
				api.Delete("/favorite", chapters.DeleteFavorite)
				api.Post("/compose-by-excel", chapters.ComposeByExcel)
				api.Post("/compose", chapters.Compose)
				api.Put("/compose", chapters.Update)
				api.Delete("/archive/{id}", chapters.Archive)
				// Discarded chapters endpoints
				api.Get("/discarded", chapters.GetDiscardedChapters)
				api.Post("/restore/{id}", chapters.RestoreChapter)
				api.Delete("/delete/{id}", chapters.PermanentlyDeleteChapter)
			})
			api.Route("/user", func(api chi.Router) {
				api.Get("/token", user.Tokens)
				api.Put("/", user.EditUser)
				api.Delete("/token/{id}", user.DeleteToken)
				api.Get("/check", user.Check)
				api.Put("/vocatoken", user.UpdateVocaToken)
				api.Get("/vocatoken", user.GetVocaToken)
				api.Get("/tokens", user.GetUserTokens)
				api.Get("/streak", user.DailyStreak)
				api.Post("/request-teacher", user.RequestTeacher)
				api.Put("/language-preferences", user.UpdateLanguagePreferences)
				api.Get("/language-preferences", user.GetLanguagePreferences)
			})
			api.Route("/auth", func(api chi.Router) {
				api.Delete("/logout", auth.Logout)
			})
			api.Route("/stream", func(api chi.Router) {
				api.Post("/create", stream.CreateStream)                      // Create new stream (authenticated)
				api.Get("/{room}", stream.GetStreamByID)                      // Get stream by ID
				api.Post("/{room}/start", stream.StartStream)                 // Start stream
				api.Post("/{room}/join", stream.JoinStream)                   // Join stream (upcoming only)
				api.Delete("/{room}/cancel", stream.CancelStreamRegistration) // Cancel registration
				api.Post("/{room}/end", stream.EndStream)                     // End stream
				// List streams
				api.Get("/active", stream.GetActiveStreams)     // Past 12h
				api.Get("/recent", stream.GetRecentStreams)     // Ended last 12h
				api.Get("/upcoming", stream.GetUpcomingStreams) // Next 12h
				api.Get("/joined", stream.GetUserJoinedStreams) // User's joined streams
			})
			api.Route("/payment", func(api chi.Router) {
				api.Post("/token", payment.PurchaseTokens)
				api.Get("/history", payment.GetPaymentHistory)
				api.Get("/stats", payment.GetPaymentStats)
				api.Get("/{paymentID}", payment.GetPaymentStatus)
			})
			api.Route("/webhooks", func(api chi.Router) {
				api.Post("/lemonsqueezy", payment.HandleLemonSqueezyWebhook)
			})

		})
		api.Route("/public", func(api chi.Router) {
			api.Route("/auth", func(api chi.Router) {
				api.Post("/send-otp", auth.SendOTP)
				api.Post("/verify-otp", auth.VerifyOTP)
			})
			api.Route("/chapters", func(api chi.Router) {
				api.Get("/user", chapters.UserChapters)
				api.Get("/{id}", chapters.GetByID)
				api.Get("/search", chapters.Search)
				api.Get("/trends", chapters.GetTrendingChapters)
				api.Get("/game-format/{id}", chapters.GameFormat)
				api.Get("/hangman/{id}", chapters.GameHangman)
			})
			api.Route("/user", func(api chi.Router) {
				api.Get("/", user.GetByUsername)
				api.Get("/search", user.SearchUsers)
			})
			// Webhook endpoint for payment confirmations

			//api.Post("/speech-to-text", speechtotext.SpeechToText)
		})
		api.Route("/usage", func(api chi.Router) {
			//api.Use(customMiddleware.HandleVocatoken)
			api.Get("/extension/notifier/{id}", chapters.Extension)
		})
	})

	s.Router.HandleFunc("/auth", auth.Token)

	staticsFS, _ := fs.Sub(embed.StaticsFS(), "statics")
	fileServerStatics := http.FileServer(http.FS(staticsFS))

	s.Router.Handle("/og-image.png", http.StripPrefix("/", fileServerStatics))
	s.Router.Handle("/favicon.ico", http.StripPrefix("/", fileServerStatics))

	log.Default().Printf("HTTP Server is running on http://%s:%d", host, port)

}
