# Clima por CEP com Observabilidade

Sistema em Go com dois serviços que consultam o clima de uma cidade a partir do CEP, com tracing distribuído usando OpenTelemetry e Zipkin.

- **Serviço A** (porta 8080): recebe o CEP via POST, valida e encaminha para o Serviço B.
- **Serviço B** (porta 8181): busca a cidade no ViaCEP, a temperatura na WeatherAPI e faz as conversões.
- **OTEL Collector**: recebe os traces dos serviços e envia para o Zipkin.
- **Zipkin** (porta 9411): mostra os traces.

## Como rodar

É preciso ter Docker e uma chave da [WeatherAPI](https://www.weatherapi.com/).

Crie o arquivo `.env` a partir do exemplo e coloque a sua chave:

```bash
cp .env.example .env
```

Suba os containers:

```bash
docker compose up --build
```

## Como fazer a requisição

```bash
curl -X POST http://localhost:8080/ -H "Content-Type: application/json" -d '{"cep":"29902555"}'
```

No PowerShell:

```powershell
Invoke-RestMethod -Method Post -Uri http://localhost:8080/ -ContentType "application/json" -Body '{"cep":"29902555"}'
```

### Respostas

| Status | Corpo | Quando |
|---|---|---|
| 200 | `{"city":"Linhares","temp_C":28.5,"temp_F":83.3,"temp_K":301.5}` | CEP encontrado (a temperatura varia) |
| 422 | `invalid zipcode` | CEP sem 8 dígitos ou que não é string |
| 404 | `can not find zipcode` | CEP com formato correto, mas não encontrado |

O Kelvin usa a fórmula do enunciado: `K = C + 273`.

## Como ver os traces no Zipkin

1. Faça uma requisição.
2. Acesse http://localhost:9411.
3. Clique em **Run Query** e abra o trace. Os spans podem levar alguns segundos para aparecer.

O trace mostra estes spans:

```
servico-a: recebe cep
└── servico-a: chama servico-b
    └── servico-b: busca clima
        ├── busca de cep
        └── busca de temperatura
```

## Testes

```bash
go test ./...
```
