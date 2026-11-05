package server

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"webhook-telegram-bot/internal/config"
	"webhook-telegram-bot/internal/handlers"
	"webhook-telegram-bot/internal/services/github"
	"webhook-telegram-bot/internal/services/gitlab"
	telegramGithub "webhook-telegram-bot/internal/services/telegram/github"
	telegramGitlab "webhook-telegram-bot/internal/services/telegram/gitlab"
	"webhook-telegram-bot/internal/storage"

	"github.com/gorilla/mux"
)

type Server struct {
	router *mux.Router
}

func New() (*Server, error) {
	ctx := context.Background()

	// Initialize centralized storage
	storageInstance, err := storage.NewStorage(ctx)
	if err != nil {
		return nil, fmt.Errorf("Failed to initialize storage: %w", err)
	}

	// Set up router
	router := mux.NewRouter()

	// Health check endpoint
	router.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("OK")); err != nil {
			log.Printf("Failed to write response: %v", err)
		}
	}).Methods("GET")

	// Setup GitHub service
	githubTelegramSvc, err := telegramGithub.NewGitHubTelegramService(storageInstance)
	if err != nil {
		return nil, fmt.Errorf("Failed to initialize GitHub Telegram service: %w", err)
	}
	if githubTelegramSvc != nil {
		githubSvc := github.NewGitHubService(githubTelegramSvc)

		// GitHub webhook endpoint
		githubHandler := handlers.NewGitHubHandler(githubTelegramSvc, githubSvc)
		router.HandleFunc("/github/{chatID}", githubHandler.HandleWebhook).Methods("POST")

		// GitHub Telegram bot webhook endpoint
		router.HandleFunc("/telegram/webhook/github", githubTelegramSvc.WebhookHandler()).Methods("POST")
	}

	// Setup GitLab service
	gitlabTelegramSvc, err := telegramGitlab.NewGitLabTelegramService(storageInstance)
	if err != nil {
		return nil, fmt.Errorf("Failed to initialize GitLab Telegram service: %w", err)
	}
	if gitlabTelegramSvc != nil {
		gitlabSvc := gitlab.NewGitLabService(gitlabTelegramSvc)

		// GitLab webhook endpoint
		gitlabHandler := handlers.NewGitLabHandler(gitlabTelegramSvc, gitlabSvc)
		router.HandleFunc("/gitlab/{chatID}", gitlabHandler.HandleWebhook).Methods("POST")

		// GitLab Telegram bot webhook endpoint
		router.HandleFunc("/telegram/webhook/gitlab", gitlabTelegramSvc.WebhookHandler()).Methods("POST")
	}

	// Initialize function for Telegram bots
	initBots := func() error {
		if githubTelegramSvc != nil {
			if err := githubTelegramSvc.InitBot(); err != nil {
				return fmt.Errorf("Failed to init GitHub Telegram bot: %v", err)
			}
		}
		if gitlabTelegramSvc != nil {
			if err := gitlabTelegramSvc.InitBot(); err != nil {
				return fmt.Errorf("Failed to init GitLab Telegram bot: %v", err)
			}
		}
		return nil
	}

	if config.Global.IsLambda {
		// In AWS Lambda, init bots once after deploy (otherwise this runs on every cold start)
		router.HandleFunc("/init", func(w http.ResponseWriter, r *http.Request) {
			// Validate secret key from header
			secretKey := r.Header.Get("secret-key")
			if secretKey != config.Global.SecretKey {
				log.Printf("Invalid secret key provided for init endpoint")
				w.WriteHeader(http.StatusUnauthorized)
				if _, writeErr := w.Write([]byte("Unauthorized: Invalid secret key")); writeErr != nil {
					log.Printf("Failed to write response: %v", writeErr)
				}
				return
			}

			if err := initBots(); err != nil {
				log.Printf("%v", err)
				w.WriteHeader(http.StatusInternalServerError)
				if _, writeErr := w.Write([]byte(err.Error())); writeErr != nil {
					log.Printf("Failed to write response: %v", writeErr)
				}
				return
			}

			w.WriteHeader(http.StatusOK)
			if _, err := w.Write([]byte("Telegram bots successfully initialized.")); err != nil {
				log.Printf("Failed to write response: %v", err)
			}
		}).Methods("GET")
	} else {
		// On local development, init bots on app start
		if err := initBots(); err != nil {
			return nil, err
		}
	}

	if githubTelegramSvc != nil {
		go githubTelegramSvc.StartBot(ctx)
	}
	if gitlabTelegramSvc != nil {
		go gitlabTelegramSvc.StartBot(ctx)
	}

	return &Server{
		router: router,
	}, nil
}

func (s *Server) Router() *mux.Router {
	return s.router
}

func (s *Server) ListenAndServe() error {
	return http.ListenAndServe(config.Global.ListenAddress, s.router)
}
