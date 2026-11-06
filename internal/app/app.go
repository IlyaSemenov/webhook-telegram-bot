package app

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"webhook-telegram-bot/internal/config"
	"webhook-telegram-bot/internal/storage"
	"webhook-telegram-bot/internal/telegram"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	muxadapter "github.com/awslabs/aws-lambda-go-api-proxy/gorillamux"
	"github.com/gorilla/mux"
)

// Connector describes a bot that a binary can run.
type Connector struct {
	Name     string // Source name for error messages
	TokenEnv string // Environment variable with the bot token; the connector is disabled without it
	New      func(ctx context.Context, deps Deps) (Bot, error)
}

// Deps holds what a connector needs to create its bot.
type Deps struct {
	Token       string
	Settings    telegram.Settings
	Store       *storage.Store
	ChatStorage *storage.ChatStorage
}

// Bot is the runtime contract of a created connector.
type Bot interface {
	Mount(router *mux.Router)
	InitBot() error
	StartBot(ctx context.Context)
}

// Run starts the connectors that have a token and serves them until the process stops.
func Run(connectors ...Connector) {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	router, err := startBots(context.Background(), cfg, connectors)
	if err != nil {
		log.Fatalf("Failed to start bots: %v", err)
	}

	if cfg.IsLambda {
		adapter := muxadapter.NewV2(router)
		lambda.Start(func(ctx context.Context, req events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
			return adapter.ProxyWithContext(ctx, req)
		})
		return
	}

	if err := http.ListenAndServe(cfg.ListenAddress, router); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

// startBots creates the enabled bots, starts them, and returns the router that serves them.
func startBots(ctx context.Context, cfg *config.Config, connectors []Connector) (*mux.Router, error) {
	bots, err := createBots(ctx, cfg, connectors)
	if err != nil {
		return nil, err
	}

	router := createRouter(cfg, bots)

	// In AWS Lambda, bots are initialized by the /init endpoint
	if !cfg.IsLambda {
		if err := initBots(bots); err != nil {
			return nil, err
		}
	}

	for _, bot := range bots {
		go bot.StartBot(ctx)
	}

	return router, nil
}

// createBots creates a bot for each connector that has a token.
func createBots(ctx context.Context, cfg *config.Config, connectors []Connector) ([]Bot, error) {
	tokens := make(map[string]string)
	tokenEnvs := make([]string, 0, len(connectors))
	for _, connector := range connectors {
		tokenEnvs = append(tokenEnvs, connector.TokenEnv)
		if token := os.Getenv(connector.TokenEnv); token != "" {
			tokens[connector.TokenEnv] = token
		}
	}
	if len(tokens) == 0 {
		return nil, fmt.Errorf("You must provide at least one of the environment variables: %s.", strings.Join(tokenEnvs, " or "))
	}

	store := storage.NewStore(cfg.StorageConnectionStringBase)
	chatStorage, err := storage.NewChatStorage(ctx, store)
	if err != nil {
		return nil, fmt.Errorf("Failed to initialize chat storage: %w", err)
	}

	settings := telegram.Settings{
		BaseURL:   cfg.BaseURL,
		SecretKey: cfg.SecretKey,
	}

	bots := make([]Bot, 0, len(tokens))
	for _, connector := range connectors {
		token, enabled := tokens[connector.TokenEnv]
		if !enabled {
			continue
		}

		bot, err := connector.New(ctx, Deps{
			Token:       token,
			Settings:    settings,
			Store:       store,
			ChatStorage: chatStorage,
		})
		if err != nil {
			return nil, fmt.Errorf("Failed to initialize %s Telegram service: %w", connector.Name, err)
		}
		bots = append(bots, bot)
	}

	return bots, nil
}

// createRouter creates the router with the service endpoints and the routes of the bots.
func createRouter(cfg *config.Config, bots []Bot) *mux.Router {
	router := mux.NewRouter()

	// Health check endpoint
	router.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("OK")); err != nil {
			log.Printf("Failed to write response: %v", err)
		}
	}).Methods("GET")

	for _, bot := range bots {
		bot.Mount(router)
	}

	if cfg.IsLambda {
		// In AWS Lambda, init bots once after deploy (otherwise this runs on every cold start)
		router.HandleFunc("/init", func(w http.ResponseWriter, r *http.Request) {
			// Validate secret key from header
			secretKey := r.Header.Get("secret-key")
			if secretKey != cfg.SecretKey {
				log.Printf("Invalid secret key provided for init endpoint")
				w.WriteHeader(http.StatusUnauthorized)
				if _, writeErr := w.Write([]byte("Unauthorized: Invalid secret key")); writeErr != nil {
					log.Printf("Failed to write response: %v", writeErr)
				}
				return
			}

			if err := initBots(bots); err != nil {
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
	}

	return router
}

// initBots initializes the Telegram side of each bot, such as its webhook and commands.
func initBots(bots []Bot) error {
	for _, bot := range bots {
		if err := bot.InitBot(); err != nil {
			return fmt.Errorf("Failed to init Telegram bot: %w", err)
		}
	}
	return nil
}
