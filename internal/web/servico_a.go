package web

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

type CepInput struct {
	Cep string `json:"cep"`
}

type ServicoA struct {
	ServicoBURL string
	OTELTracer  trace.Tracer
	Client      *http.Client
}

func NewServicoA(servicoBURL string, tracer trace.Tracer) *ServicoA {
	return &ServicoA{
		ServicoBURL: servicoBURL,
		OTELTracer:  tracer,
		Client:      &http.Client{Timeout: 15 * time.Second},
	}
}

func (s *ServicoA) CreateServer() *chi.Mux {
	router := chi.NewRouter()

	router.Use(middleware.RequestID)
	router.Use(middleware.Recoverer)
	router.Use(middleware.Logger)
	router.Use(middleware.Timeout(60 * time.Second))
	router.Post("/", s.HandleRequest)
	return router
}

func (s *ServicoA) HandleRequest(w http.ResponseWriter, r *http.Request) {
	carrier := propagation.HeaderCarrier(r.Header)
	ctx := r.Context()
	ctx = otel.GetTextMapPropagator().Extract(ctx, carrier)

	ctx, span := s.OTELTracer.Start(ctx, "servico-a: recebe cep")
	defer span.End()

	// Se o cep vier como numero o Decode retorna erro, por isso tambem cai no 422
	var input CepInput
	err := json.NewDecoder(r.Body).Decode(&input)
	if err != nil || !CepValido(input.Cep) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		w.Write([]byte("invalid zipcode"))
		return
	}

	ctx, spanServicoB := s.OTELTracer.Start(ctx, "servico-a: chama servico-b")
	defer spanServicoB.End()

	req, err := http.NewRequestWithContext(ctx, "GET", s.ServicoBURL+"/"+input.Cep, nil)
	if err != nil {
		log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(req.Header))

	resp, err := s.Client.Do(req)
	if err != nil {
		log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
	w.WriteHeader(resp.StatusCode)
	w.Write(body)
}
