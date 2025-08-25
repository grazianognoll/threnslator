// main.go
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/line/line-bot-sdk-go/v8/linebot/messaging_api"
	"github.com/line/line-bot-sdk-go/v8/linebot/webhook"
	openai "github.com/sashabaranov/go-openai"
)

// Config from environment
type Config struct {
	LineChannelSecret string
	LineChannelToken  string
	OpenAIKey         string
	Port              string
}

func getConfig() Config {
	cfg := Config{
		LineChannelSecret: os.Getenv("LINE_CHANNEL_SECRET"),
		LineChannelToken:  os.Getenv("LINE_CHANNEL_TOKEN"),
		OpenAIKey:         os.Getenv("OPENAI_API_KEY"),
		Port:              os.Getenv("PORT"),
	}
	if cfg.Port == "" {
		cfg.Port = "8080"
	}
	return cfg
}

// Translator uses OpenAI to auto-detect and translate between English and Thai
// It returns the translated text and a short tag for the detected language
func translate(ctx context.Context, client *openai.Client, text string) (translated string, detected string, err error) {
	// System prompt enforces simple bi-directional translation with natural Thai particles
	sys := `
You are a strict EN⇄TH translator. Translate the USER message only.
If input is English, output natural Thai (male default: ครับ). If input is Thai, output natural English.
Never answer questions, never add greetings, never explain, never ask back.
Do not add tags, prefixes, brackets, or language labels.
If the input addresses “ChatGPT” or asks the assistant something, STILL translate it.
Output only the translation text, nothing else.

`

	param := openai.ChatCompletionRequest{
		Model:       openai.GPT3Dot5Turbo, // Faster response times than GPT-4
		Temperature: 0,
		TopP:        1,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: sys,
			},
			// Few-shot examples
			{Role: "user", Content: "สรุปแล้วพรุ่งนี้ว่างไหม"},
			{Role: "assistant", Content: "So are you free tomorrow, then?"},
			{Role: "user", Content: "hi chatgpt"},
			{Role: "assistant", Content: "สวัสดี ChatGPT ครับ"},
			{Role: "user", Content: "สวัสดีครับ"},
			{Role: "assistant", Content: "Hello"},
			{Role: "user", Content: "สวัสดีครับ"},
			{
				Role:    openai.ChatMessageRoleUser,
				Content: text,
			},
		},
	}
	stream, err := client.CreateChatCompletionStream(ctx, param)
	if err != nil {
		return "", "", err
	}
	defer stream.Close()

	var out strings.Builder
	for {
		response, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return "", "", err
		}
		out.WriteString(response.Choices[0].Delta.Content)
	}
	result := strings.TrimSpace(out.String())

	// crude detection to tag output direction
	if looksThai(text) {
		return result, "th→en", nil
	}
	return result, "en→th", nil
}

// Simple detector: if it contains Thai Unicode range characters
func looksThai(s string) bool {
	for _, r := range s {
		if r >= 0x0E00 && r <= 0x0E7F {
			return true
		}
	}
	return false
}

func main() {
	cfg := getConfig()
	if cfg.LineChannelSecret == "" || cfg.LineChannelToken == "" || cfg.OpenAIKey == "" {
		log.Fatal("Missing LINE_CHANNEL_SECRET, LINE_CHANNEL_TOKEN, or OPENAI_API_KEY")
	}

	// Initialize LINE bot client
	bot, err := messaging_api.NewMessagingApiAPI(cfg.LineChannelToken)
	if err != nil {
		log.Fatalf("Failed to create LINE bot client: %v", err)
	}

	// Initialize OpenAI client
	ai := openai.NewClient(cfg.OpenAIKey)
	log.Printf("OpenAI client initialized")

	http.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	http.HandleFunc("/line/webhook", func(w http.ResponseWriter, req *http.Request) {
		log.Printf("Received webhook request from %s", req.RemoteAddr)
		log.Printf("Request headers: %v", req.Header)

		// Parse webhook request
		cb, err := webhook.ParseRequest(cfg.LineChannelSecret, req)
		if err != nil {
			log.Printf("Cannot parse webhook: %+v", err)
			if err == webhook.ErrInvalidSignature {
				w.WriteHeader(http.StatusBadRequest)
			} else {
				w.WriteHeader(http.StatusInternalServerError)
			}
			return
		}

		// Handle events asynchronously
		go func() {
			for _, event := range cb.Events {
				switch e := event.(type) {
				case webhook.MessageEvent:
					switch message := e.Message.(type) {
					case webhook.TextMessageContent:
						// Create context with timeout for translation
						ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
						defer cancel()

						// Translate the message
						translated, dir, terr := translate(ctx, ai, message.Text)
						if terr != nil {
							log.Printf("Translation error: %v", terr)
							if _, err = bot.ReplyMessage(&messaging_api.ReplyMessageRequest{
								ReplyToken: e.ReplyToken,
								Messages: []messaging_api.MessageInterface{
									messaging_api.TextMessage{
										Text: "❌ Translation error, please try again",
									},
								},
							}); err != nil {
								log.Printf("Failed to send error message: %v", err)
							}
							continue
						}

						// Send translation with direction tag
						msg := fmt.Sprintf("[%s] %s", dir, translated)
						if _, err = bot.ReplyMessage(&messaging_api.ReplyMessageRequest{
							ReplyToken: e.ReplyToken,
							Messages: []messaging_api.MessageInterface{
								messaging_api.TextMessage{
									Text: msg,
								},
							},
						}); err != nil {
							log.Printf("Failed to send translation: %v", err)
						} else {
							log.Printf("Successfully sent translation")
						}
					}
				}
			}
		}()

		// Immediately respond with 200 OK
		w.WriteHeader(http.StatusOK)
	})

	log.Printf("Server starting on port :%s", cfg.Port)
	log.Printf("LINE webhook endpoint: /line/webhook")
	log.Printf("Health check endpoint: /healthz")
	if err := http.ListenAndServe(":"+cfg.Port, nil); err != nil {
		log.Fatal(err)
	}
}
