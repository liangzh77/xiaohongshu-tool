package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"xiaohongshu-tool/internal/storage"
	"xiaohongshu-tool/internal/web"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	addr := flag.String("addr", ":8080", "HTTP listen address")
	dbPath := flag.String("db", "data/xhs.db", "SQLite database path")
	collectorCmd := flag.String("collector-command", "go run ./cmd/xhs-native-collector", "base collector command")
	limit := flag.Int("limit", 20, "default list limit")
	llmBaseURL := flag.String("llm-base-url", getenvDefault("XHS_LLM_BASE_URL", "https://api.openai.com/v1"), "OpenAI-compatible base URL")
	llmModel := flag.String("llm-model", os.Getenv("XHS_LLM_MODEL"), "LLM model")
	keyName := flag.String("key-name", getenvDefault("XHS_LLM_KEY_NAME", "OPENAI_API_KEY"), "key name in key distribution service")
	keyDistBaseURL := flag.String("key-dist-base-url", os.Getenv("XHS_KEY_DIST_BASE_URL"), "key distribution service base URL")
	flag.Parse()

	db, err := storage.Open(*dbPath)
	if err != nil {
		return err
	}
	defer db.Close()
	if err := db.Migrate(context.Background()); err != nil {
		return err
	}

	handler := web.NewServer(web.Config{
		DB:             db,
		DefaultLimit:   *limit,
		CollectorCmd:   *collectorCmd,
		LLMBaseURL:     *llmBaseURL,
		LLMModel:       *llmModel,
		KeyName:        *keyName,
		KeyDistBaseURL: *keyDistBaseURL,
	}).Handler()

	fmt.Printf("xhs-web listening: http://localhost%s\n", *addr)
	return http.ListenAndServe(*addr, handler)
}

func getenvDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
