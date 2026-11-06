package app

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"webhook-telegram-bot/internal/config"

	"github.com/gorilla/mux"
)

const (
	alphaTokenEnv = "TEST_ALPHA_TELEGRAM_BOT_TOKEN"
	betaTokenEnv  = "TEST_BETA_TELEGRAM_BOT_TOKEN"
)

// fakeBot is a bot that serves one route and records its initialization.
type fakeBot struct {
	token   string
	route   string
	initErr error
	inits   int
	started chan struct{} // When set, StartBot closes it and blocks until the context is canceled
}

func (b *fakeBot) Mount(router *mux.Router) {
	router.HandleFunc(b.route, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

func (b *fakeBot) InitBot() error {
	b.inits++
	return b.initErr
}

func (b *fakeBot) StartBot(ctx context.Context) {
	if b.started != nil {
		close(b.started)
		<-ctx.Done()
	}
}

// newFakeConnector creates a connector that returns the given bot and records the token it received.
func newFakeConnector(tokenEnv string, bot *fakeBot) Connector {
	return Connector{
		Name:     tokenEnv,
		TokenEnv: tokenEnv,
		New: func(_ context.Context, deps Deps) (Bot, error) {
			bot.token = deps.Token
			return bot, nil
		},
	}
}

func serve(router *mux.Router, path string, secretKey string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodGet, path, nil)
	if secretKey != "" {
		request.Header.Set("secret-key", secretKey)
	}
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return response
}

func TestStartBots(t *testing.T) {
	ctx := context.Background()

	t.Run("creates only the connectors that have a token", func(t *testing.T) {
		t.Setenv(alphaTokenEnv, "alpha-token")
		t.Setenv(betaTokenEnv, "")
		alpha := &fakeBot{route: "/alpha"}
		beta := &fakeBot{route: "/beta"}

		router, err := startBots(ctx, &config.Config{}, []Connector{
			newFakeConnector(alphaTokenEnv, alpha),
			newFakeConnector(betaTokenEnv, beta),
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if alpha.token != "alpha-token" {
			t.Errorf("alpha token = %q, want %q", alpha.token, "alpha-token")
		}
		if alpha.inits != 1 {
			t.Errorf("alpha inits = %d, want 1", alpha.inits)
		}
		if code := serve(router, "/alpha", "").Code; code != http.StatusOK {
			t.Errorf("/alpha status = %d, want %d", code, http.StatusOK)
		}
		if beta.inits != 0 {
			t.Errorf("beta inits = %d, want 0", beta.inits)
		}
		if code := serve(router, "/beta", "").Code; code != http.StatusNotFound {
			t.Errorf("/beta status = %d, want %d", code, http.StatusNotFound)
		}
	})

	t.Run("without any token names the expected variables", func(t *testing.T) {
		t.Setenv(alphaTokenEnv, "")
		t.Setenv(betaTokenEnv, "")

		// The connection string is an invalid URL, so this fails if the storage opens before the token check.
		_, err := startBots(ctx, &config.Config{StorageConnectionStringBase: "dynamodb://%"}, []Connector{
			newFakeConnector(alphaTokenEnv, &fakeBot{}),
			newFakeConnector(betaTokenEnv, &fakeBot{}),
		})
		if err == nil || !strings.Contains(err.Error(), alphaTokenEnv+" or "+betaTokenEnv) {
			t.Errorf("error = %v, want one naming both variables", err)
		}
	})

	t.Run("returns the connector error", func(t *testing.T) {
		t.Setenv(alphaTokenEnv, "alpha-token")
		connectorErr := errors.New("connector failed")
		connector := Connector{
			Name:     "Alpha",
			TokenEnv: alphaTokenEnv,
			New: func(_ context.Context, _ Deps) (Bot, error) {
				return nil, connectorErr
			},
		}

		_, err := startBots(ctx, &config.Config{}, []Connector{connector})
		if !errors.Is(err, connectorErr) {
			t.Errorf("error = %v, want %v", err, connectorErr)
		}
	})

	t.Run("returns the bot init error", func(t *testing.T) {
		t.Setenv(alphaTokenEnv, "alpha-token")
		initErr := errors.New("init failed")

		_, err := startBots(ctx, &config.Config{}, []Connector{
			newFakeConnector(alphaTokenEnv, &fakeBot{route: "/alpha", initErr: initErr}),
		})
		if !errors.Is(err, initErr) {
			t.Errorf("error = %v, want %v", err, initErr)
		}
	})
}

func TestStartBotsRunsEveryEnabledBot(t *testing.T) {
	for _, tt := range []struct {
		name     string
		isLambda bool
	}{
		{name: "long-lived process", isLambda: false},
		{name: "Lambda", isLambda: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(alphaTokenEnv, "alpha-token")
			t.Setenv(betaTokenEnv, "beta-token")
			ctx, cancel := context.WithCancel(context.Background())
			t.Cleanup(cancel)
			alpha := &fakeBot{route: "/alpha", started: make(chan struct{})}
			beta := &fakeBot{route: "/beta", started: make(chan struct{})}
			connectors := []Connector{
				newFakeConnector(alphaTokenEnv, alpha),
				newFakeConnector(betaTokenEnv, beta),
			}

			// A bot blocks in StartBot, so this times out unless bots start in the background.
			routers := make(chan *mux.Router, 1)
			go func() {
				router, err := startBots(ctx, &config.Config{SecretKey: "secret", IsLambda: tt.isLambda}, connectors)
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				routers <- router
			}()
			var router *mux.Router
			select {
			case router = <-routers:
			case <-time.After(5 * time.Second):
				t.Fatal("startBots did not return")
			}
			if router == nil {
				t.FailNow()
			}

			if tt.isLambda {
				if code := serve(router, "/init", "secret").Code; code != http.StatusOK {
					t.Errorf("/init status = %d, want %d", code, http.StatusOK)
				}
			}

			for _, bot := range []*fakeBot{alpha, beta} {
				select {
				case <-bot.started:
				case <-time.After(5 * time.Second):
					t.Errorf("%s bot did not start", bot.route)
				}
				if bot.inits != 1 {
					t.Errorf("%s bot inits = %d, want 1", bot.route, bot.inits)
				}
				if code := serve(router, bot.route, "").Code; code != http.StatusOK {
					t.Errorf("%s status = %d, want %d", bot.route, code, http.StatusOK)
				}
			}
		})
	}
}

func TestStartBotsServiceEndpoints(t *testing.T) {
	ctx := context.Background()
	t.Setenv(alphaTokenEnv, "alpha-token")

	t.Run("serves health and no init endpoint outside Lambda", func(t *testing.T) {
		router, err := startBots(ctx, &config.Config{SecretKey: "secret"}, []Connector{
			newFakeConnector(alphaTokenEnv, &fakeBot{route: "/alpha"}),
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		response := serve(router, "/health", "")
		if response.Code != http.StatusOK || response.Body.String() != "OK" {
			t.Errorf("/health = %d %q, want %d %q", response.Code, response.Body.String(), http.StatusOK, "OK")
		}
		if code := serve(router, "/init", "secret").Code; code != http.StatusNotFound {
			t.Errorf("/init status = %d, want %d", code, http.StatusNotFound)
		}
	})

	t.Run("initializes bots only through the authorized init endpoint on Lambda", func(t *testing.T) {
		alpha := &fakeBot{route: "/alpha"}
		router, err := startBots(ctx, &config.Config{SecretKey: "secret", IsLambda: true}, []Connector{
			newFakeConnector(alphaTokenEnv, alpha),
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if alpha.inits != 0 {
			t.Errorf("inits after start = %d, want 0", alpha.inits)
		}

		if code := serve(router, "/init", "wrong").Code; code != http.StatusUnauthorized {
			t.Errorf("/init with a wrong key status = %d, want %d", code, http.StatusUnauthorized)
		}
		if alpha.inits != 0 {
			t.Errorf("inits after a wrong key = %d, want 0", alpha.inits)
		}

		if code := serve(router, "/init", "secret").Code; code != http.StatusOK {
			t.Errorf("/init status = %d, want %d", code, http.StatusOK)
		}
		if alpha.inits != 1 {
			t.Errorf("inits after init = %d, want 1", alpha.inits)
		}
	})
}
