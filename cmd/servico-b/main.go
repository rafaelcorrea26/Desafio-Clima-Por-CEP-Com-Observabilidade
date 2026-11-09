package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rafaelcorrea26/Desafio-Clima-Por-CEP-Com-Observabilidade/internal/tracing"
	"github.com/rafaelcorrea26/Desafio-Clima-Por-CEP-Com-Observabilidade/internal/web"
	"github.com/spf13/viper"
	"go.opentelemetry.io/otel"
)

// load env vars cfg
func init() {
	viper.AutomaticEnv()
}

func main() {
	if viper.GetString("WEATHER_API_KEY") == "" {
		log.Fatal("WEATHER_API_KEY nao informada")
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	shutdown, err := tracing.InitProvider(viper.GetString("OTEL_SERVICE_NAME"), viper.GetString("OTEL_EXPORTER_OTLP_ENDPOINT"))
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		// Contexto novo porque o ctx ja esta cancelado quando chega aqui
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()
		if err := shutdown(shutdownCtx); err != nil {
			log.Println("failed to shutdown TracerProvider:", err)
		}
	}()

	tracer := otel.Tracer("microservice-tracer")

	server := web.NewServicoB(
		viper.GetString("VIACEP_URL"),
		viper.GetString("WEATHER_API_URL"),
		viper.GetString("WEATHER_API_KEY"),
		tracer,
	)
	router := server.CreateServer()

	go func() {
		log.Println("Starting server on port", viper.GetString("HTTP_PORT"))
		if err := http.ListenAndServe(viper.GetString("HTTP_PORT"), router); err != nil {
			log.Fatal(err)
		}
	}()

	<-ctx.Done()
	log.Println("Shutting down...")
}
