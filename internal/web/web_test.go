package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go.opentelemetry.io/otel"
)

func TestServicoACepInvalido(t *testing.T) {
	servico := NewServicoA("http://nao-deve-ser-chamado", otel.Tracer("test"))
	router := servico.CreateServer()

	corpos := []string{`{"cep":"123"}`, `{"cep":"123456789"}`, `{"cep":"1234567a"}`, `{"cep":29902555}`, `{}`, `abc`}
	for _, corpo := range corpos {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("POST", "/", strings.NewReader(corpo))
		router.ServeHTTP(w, r)

		if w.Code != http.StatusUnprocessableEntity || w.Body.String() != "invalid zipcode" {
			t.Errorf("corpo %s: recebeu %d %q", corpo, w.Code, w.Body.String())
		}
	}
}

func TestServicoBCepNaoEncontrado(t *testing.T) {
	viaCep := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"erro":"true"}`))
	}))
	defer viaCep.Close()

	servico := NewServicoB(viaCep.URL+"/", "", "chave", otel.Tracer("test"))
	w := httptest.NewRecorder()
	servico.CreateServer().ServeHTTP(w, httptest.NewRequest("GET", "/99999999", nil))

	if w.Code != http.StatusNotFound || w.Body.String() != "can not find zipcode" {
		t.Errorf("recebeu %d %q", w.Code, w.Body.String())
	}
}

func TestServicoBSucesso(t *testing.T) {
	viaCep := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"cep":"01001-000","localidade":"São Paulo","uf":"SP"}`))
	}))
	defer viaCep.Close()

	var cidade string
	weather := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cidade = r.URL.Query().Get("q")
		w.Write([]byte(`{"current":{"temp_c":28.5}}`))
	}))
	defer weather.Close()

	servico := NewServicoB(viaCep.URL+"/", weather.URL, "chave", otel.Tracer("test"))
	w := httptest.NewRecorder()
	servico.CreateServer().ServeHTTP(w, httptest.NewRequest("GET", "/01001000", nil))

	esperado := `{"city":"São Paulo","temp_C":28.5,"temp_F":83.3,"temp_K":301.5}`
	if w.Code != http.StatusOK || strings.TrimSpace(w.Body.String()) != esperado {
		t.Errorf("recebeu %d %q", w.Code, w.Body.String())
	}
	if cidade != "São Paulo,Brazil" {
		t.Errorf("cidade enviada para a weatherapi: %q", cidade)
	}
}
