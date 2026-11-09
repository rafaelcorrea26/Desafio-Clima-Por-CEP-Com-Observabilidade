package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"net/url"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

type ViaCEP struct {
	Cep        string `json:"cep"`
	Localidade string `json:"localidade"`
	Uf         string `json:"uf"`
}

type WeatherAPI struct {
	Current struct {
		TempC float64 `json:"temp_c"`
	} `json:"current"`
}

type Clima struct {
	City  string  `json:"city"`
	TempC float64 `json:"temp_C"`
	TempF float64 `json:"temp_F"`
	TempK float64 `json:"temp_K"`
}

var ErrCepNaoEncontrado = errors.New("cep nao encontrado")

type ServicoB struct {
	ViaCepURL     string
	WeatherAPIURL string
	WeatherAPIKey string
	OTELTracer    trace.Tracer
	Client        *http.Client
}

func NewServicoB(viaCepURL, weatherAPIURL, weatherAPIKey string, tracer trace.Tracer) *ServicoB {
	return &ServicoB{
		ViaCepURL:     viaCepURL,
		WeatherAPIURL: weatherAPIURL,
		WeatherAPIKey: weatherAPIKey,
		OTELTracer:    tracer,
		Client:        &http.Client{Timeout: 5 * time.Second},
	}
}

func (s *ServicoB) CreateServer() *chi.Mux {
	router := chi.NewRouter()

	router.Use(middleware.RequestID)
	router.Use(middleware.Recoverer)
	router.Use(middleware.Logger)
	router.Use(middleware.Timeout(60 * time.Second))
	router.Get("/{cep}", s.HandleRequest)
	return router
}

func (s *ServicoB) HandleRequest(w http.ResponseWriter, r *http.Request) {
	carrier := propagation.HeaderCarrier(r.Header)
	ctx := r.Context()
	ctx = otel.GetTextMapPropagator().Extract(ctx, carrier)

	ctx, span := s.OTELTracer.Start(ctx, "servico-b: busca clima")
	defer span.End()

	cepParam := chi.URLParam(r, "cep")
	if !CepValido(cepParam) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		w.Write([]byte("invalid zipcode"))
		return
	}

	cep, err := s.BuscaCep(ctx, cepParam)
	if errors.Is(err, ErrCepNaoEncontrado) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("can not find zipcode"))
		return
	}
	if err != nil {
		log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	tempC, err := s.BuscaTemperatura(ctx, cep.Localidade)
	if err != nil {
		log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(ConverteTemperatura(cep.Localidade, tempC))
}

func (s *ServicoB) BuscaCep(ctx context.Context, cep string) (*ViaCEP, error) {
	ctx, span := s.OTELTracer.Start(ctx, "busca de cep")
	defer span.End()

	req, err := http.NewRequestWithContext(ctx, "GET", s.ViaCepURL+cep+"/json/", nil)
	if err != nil {
		return nil, err
	}
	resp, err := s.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("viacep retornou status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var c ViaCEP
	err = json.Unmarshal(body, &c)
	if err != nil {
		return nil, err
	}
	// Quando o CEP nao existe o ViaCEP responde 200 com {"erro": "true"}
	if c.Localidade == "" {
		return nil, ErrCepNaoEncontrado
	}
	return &c, nil
}

func (s *ServicoB) BuscaTemperatura(ctx context.Context, cidade string) (float64, error) {
	ctx, span := s.OTELTracer.Start(ctx, "busca de temperatura")
	defer span.End()

	// QueryEscape por causa de espaco e acento, ex: São Paulo
	endereco := s.WeatherAPIURL + "?key=" + s.WeatherAPIKey + "&q=" + url.QueryEscape(cidade+",Brazil")
	req, err := http.NewRequestWithContext(ctx, "GET", endereco, nil)
	if err != nil {
		return 0, errors.New("erro ao montar a chamada da weatherapi")
	}
	resp, err := s.Client.Do(req)
	if err != nil {
		// O erro do client traz a URL com a chave, por isso nao devolvo ele
		return 0, errors.New("erro ao chamar a weatherapi")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("weatherapi retornou status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}
	var weather WeatherAPI
	err = json.Unmarshal(body, &weather)
	if err != nil {
		return 0, err
	}
	return weather.Current.TempC, nil
}

func ConverteTemperatura(cidade string, celsius float64) Clima {
	return Clima{
		City:  cidade,
		TempC: celsius,
		TempF: arredonda(celsius*1.8 + 32),
		TempK: arredonda(celsius + 273),
	}
}

// Sem isso o JSON pode sair com valores como 83.30000000000001
func arredonda(valor float64) float64 {
	return math.Round(valor*100) / 100
}
