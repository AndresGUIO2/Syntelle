package main

import (
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"syntelle/api/internal/accounts"
	"syntelle/api/internal/conversation"
	"syntelle/api/internal/httpapi"
	"syntelle/api/internal/speech"
	"syntelle/api/internal/streaming"
)

func main() {
	if err := httpapi.ValidateDeployment(); err != nil {
		log.Fatal(err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	processorURL := os.Getenv("PROCESSOR_URL")
	if processorURL == "" {
		processorURL = "http://127.0.0.1:8000"
	}
	address := os.Getenv("LISTEN_ADDR")
	if address == "" {
		address = "127.0.0.1:8080"
	}
	var speechServiceInstance *speech.Service
	owner := os.Getenv("SYNTELLE_LOCAL_OWNER") == "1"
	accountMode := os.Getenv("SYNTELLE_ACCOUNTS") == "1"
	streamMode := os.Getenv("SYNTELLE_STREAMING") == "1"
	var conversations *conversation.Service
	var ownerStore *conversation.Store
	if owner || accountMode || streamMode {
		var err error
		ownerStore, err = conversation.Open(ctx, os.Getenv("SYNTELLE_DATABASE_URL"))
		if err != nil {
			log.Fatal(err)
		}
		defer ownerStore.Pool.Close()
		if err = ownerStore.Recover(ctx); err != nil {
			log.Fatal("Conversation recovery failed.")
		}
	}
	if (owner || streamMode || os.Getenv("SYNTELLE_SPEECH_TRIAL") == "1") && os.Getenv("AZURE_SPEECH_KEY") != "" && os.Getenv("AZURE_SPEECH_REGION") != "" {
		provider, err := speech.NewAzureProvider(os.Getenv("AZURE_SPEECH_KEY"), os.Getenv("AZURE_SPEECH_REGION"))
		if err != nil {
			log.Fatal(err)
		}
		ledger, err := speech.OpenTrialLedger(ctx, os.Getenv("SYNTELLE_DATABASE_URL"))
		if err != nil {
			log.Fatal(err)
		}
		defer ledger.Close()
		var activeLedger speech.Ledger = ledger
		if owner || streamMode {
			activeLedger = speech.NewOwnerLedger(ownerStore)
		}
		speechServiceInstance = speech.NewService(provider, activeLedger, os.Getenv("AZURE_SPEECH_REGION"))
		if path := os.Getenv("SYNTELLE_VOICE_CAPABILITIES"); path != "" {
			if err := speechServiceInstance.LoadVoiceCapabilities(path); err != nil {
				log.Fatal(err)
			}
		}
	}
	if owner || streamMode {
		var model conversation.Model
		chatProvider := os.Getenv("SYNTELLE_CHAT_PROVIDER")
		if chatProvider == "" {
			chatProvider = "gemini"
		}
		switch chatProvider {
		case "gemini":
			if os.Getenv("GEMINI_API_KEY") != "" {
				var err error
				model, err = conversation.NewGemini(os.Getenv("GEMINI_API_KEY"), os.Getenv("GEMINI_CHAT_MODEL"))
				if err != nil {
					log.Fatal(err)
				}
				model.(*conversation.Gemini).WithDirection()
			}
		case "foundry":
			if os.Getenv("AZURE_OPENAI_ENDPOINT") != "" && os.Getenv("AZURE_OPENAI_DEPLOYMENT") != "" && os.Getenv("AZURE_OPENAI_API_KEY") != "" {
				var err error
				model, err = conversation.NewResponses(os.Getenv("AZURE_OPENAI_ENDPOINT"), os.Getenv("AZURE_OPENAI_DEPLOYMENT"), os.Getenv("AZURE_OPENAI_API_KEY"))
				if err != nil {
					log.Fatal(err)
				}
			}
		default:
			log.Fatal("Choose the explicit gemini or foundry conversation provider.")
		}
		var voice conversation.Speech
		var recognition conversation.Recognition
		if speechServiceInstance != nil {
			voice = speech.NewConversationSpeech(speechServiceInstance)
			recognition = speech.NewAzureRecognition(os.Getenv("AZURE_SPEECH_KEY"), os.Getenv("AZURE_SPEECH_REGION"))
		}
		conversations = conversation.New(ownerStore, model, voice, recognition)
		if path := os.Getenv("SYNTELLE_PERFORMANCE_PROFILE"); path != "" {
			profile, err := conversation.LoadPerformanceProfile(path, os.Getenv("SYNTELLE_PACKAGE_HASH"))
			if err != nil {
				log.Fatal("Character performance profile failed validation: ", err)
			}
			conversations.Performance = profile
		}
	}
	var legacyConversations *conversation.Service
	if owner {
		legacyConversations = conversations
	}
	var handler http.Handler = newApplicationHandler(processorURL, &http.Client{Timeout: 15 * time.Second}, speechServiceInstance, legacyConversations)
	var productAccounts *accounts.Service
	var streams *streaming.Service
	if accountMode || streamMode {
		var err error
		productAccounts, err = accounts.Open(ctx, ownerStore.Pool, accounts.ConfigFromEnv())
		if err != nil {
			log.Fatal("Account service configuration failed: ", err)
		}
		handler = productAccounts.Handler(handler)
	}
	if streamMode {
		maximum, _ := strconv.Atoi(os.Getenv("SYNTELLE_RENDER_CAPACITY"))
		admission, err := streaming.AdmissionConfigFromEnv()
		if err != nil {
			log.Fatal("Stream admission configuration failed: ", err)
		}
		disableTimeLimits, err := streaming.TimeLimitsDisabledFromEnv()
		if err != nil {
			log.Fatal("Stream time configuration failed: ", err)
		}
		streams, err = streaming.Open(ctx, ownerStore.Pool, conversations, productAccounts.Resolve, streaming.Config{
			WorkerToken:       os.Getenv("SYNTELLE_RENDER_WORKER_TOKEN"),
			MaximumSessions:   maximum,
			PackageHash:       os.Getenv("SYNTELLE_PACKAGE_HASH"),
			ValidatePrincipal: productAccounts.ValidatePrincipal,
			Admission:         admission,
			DisableTimeLimits: disableTimeLimits,
		})
		if err != nil {
			log.Fatal("Stream service configuration failed: ", err)
		}
		productAccounts.OnRevoked = streams.Revoke
		handler = streams.Handler(handler)
	}
	if streamMode || accountMode {
		handler = &productHandler{Handler: handler, streams: streams, conversations: conversations}
	}
	handler = httpapi.DeploymentGuard(handler)
	server := &http.Server{
		Addr: address, Handler: handler,
		ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 20 * time.Second,
		WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second,
		BaseContext: func(_ net.Listener) context.Context { return ctx },
	}
	log.Printf("Syntelle API listening on %s", address)
	if err := httpapi.Run(ctx, server, 5*time.Second); err != nil {
		log.Fatal(err)
	}
}

func newApplicationHandler(processorURL string, client *http.Client, voice *speech.Service, conversations *conversation.Service) http.Handler {
	var handler http.Handler = speech.Handler(httpapi.NewHandler(processorURL, client), voice)
	if conversations != nil {
		handler = conversations.Handler(handler)
	}
	handler = httpapi.LocalOnly(handler)
	// Keep Shutdown visible to the server after composing HTTP middleware.
	if conversations != nil {
		handler = &ownerHandler{Handler: handler, conversations: conversations}
	}
	return handler
}

type ownerHandler struct {
	http.Handler
	conversations *conversation.Service
}

func (h *ownerHandler) Shutdown(ctx context.Context) error { return h.conversations.Shutdown(ctx) }

type productHandler struct {
	http.Handler
	streams       *streaming.Service
	conversations *conversation.Service
}

func (h *productHandler) Shutdown(ctx context.Context) error {
	// Stop media/control connections before draining provider work; both share
	// the existing five-second server shutdown deadline.
	var streamError error
	if h.streams != nil {
		streamError = h.streams.Shutdown(ctx)
	}
	var conversationError error
	if h.conversations != nil {
		conversationError = h.conversations.Shutdown(ctx)
	}
	return errors.Join(streamError, conversationError)
}
