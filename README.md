# wuzapi-webhook-gateway

Gateway stateless y liviano entre [WuzAPI](https://github.com/asternic/wuzapi) y n8n. Evita ejecuciones innecesarias descartando eventos que no sean mensajes privados entrantes.

```text
WhatsApp -> WuzAPI -> Gateway -> n8n -> workflows
```

Características:

- Go y librería estándar, sin dependencias externas;
- política default-deny;
- HMAC SHA-256 sobre body raw;
- body reenviado sin transformación;
- soporte para chats privados `@s.whatsapp.net` y `@lid`;
- imagen `scratch`, binario estático y usuario non-root;
- sin base de datos, Redis, colas, almacenamiento o sesiones.

## Política de admisión

Un evento llega a n8n únicamente cuando:

1. JSON es válido y `type == "Message"`.
2. Existen `event.Info.Chat`, `Sender`, `IsFromMe`, `IsGroup`, `ID` y `Timestamp`.
3. `IsFromMe == false` e `IsGroup == false`.
4. Chat no es `status@broadcast`.
5. Chat no termina en `@g.us`, `@broadcast` o `@newsletter`.
6. Chat contiene identificador numérico y termina en `@s.whatsapp.net` o `@lid`.

Contenido no participa en decisión: texto, audio, imagen y documento privados pasan. Body recibido por n8n conserva exactamente bytes enviados por WuzAPI.

Eventos descartados responden HTTP `200` con reason no sensible. n8n `2xx` produce `200`; cualquier otro status, redirect, timeout o error de red produce `502`, permitiendo retry de WuzAPI.

## Despliegue rápido con imagen publicada

Copie `docker-compose.image.yml` y `.env.example`, luego configure:

```bash
cp .env.example .env
```

```env
N8N_WEBHOOK_URL=https://n8n.example.com/webhook/wuzapi
WUZAPI_HMAC_SECRET=replace-with-a-secure-random-secret
REQUIRE_HMAC=true
N8N_FORWARD_TIMEOUT=8s
MAX_BODY_BYTES=1048576
LOG_LEVEL=info
```

Inicie versión estable explícita:

```bash
IMAGE_TAG=1.0.0 docker compose -f docker-compose.image.yml up -d
```

No se publica tag `latest`. Revise [Releases](https://github.com/cristianmgb/wuzapi-webhook-gateway/releases) para escoger versión.

## Construcción desde código

```bash
docker compose up -d --build
```

O directamente:

```bash
docker build -t wuzapi-webhook-gateway:local .
```

Dockerfile usa Go `1.26.6`, runtime `scratch`, certificados CA y usuario `65532:65532`.

## Variables

| Variable | Default | Requisito |
|---|---:|---|
| `PORT` | `8080` | entero entre 1 y 65535 |
| `N8N_WEBHOOK_URL` | ninguno | URL `http` o `https` obligatoria |
| `WUZAPI_HMAC_SECRET` | vacío | obligatorio si `REQUIRE_HMAC=true` |
| `REQUIRE_HMAC` | `true` | booleano |
| `N8N_FORWARD_TIMEOUT` | `8s` | duración Go positiva |
| `MAX_BODY_BYTES` | `1048576` | entero positivo; 1 MiB |
| `LOG_LEVEL` | `info` | `debug`, `info`, `warn` o `error` |

Nunca versione `.env`, tokens o secretos.

## Endpoints

- `POST /webhook`
- `GET /health`: responde `{"status":"ok"}` sin llamadas externas.

## Ejecución local

```bash
export N8N_WEBHOOK_URL='http://localhost:5678/webhook-test/wuzapi'
export WUZAPI_HMAC_SECRET='una-clave-aleatoria-de-al-menos-32-caracteres'
go run ./cmd/server
```

```bash
go test ./...
go test -race ./...
go vet ./...
```

## Dokploy

Puede desplegar desde repositorio usando `docker-compose.yml`, o desde GHCR usando `docker-compose.image.yml`.

Configuración recomendada:

- dominio: dominio propio del gateway;
- path: `/`;
- container port: `8080`;
- HTTPS: habilitado;
- healthcheck HTTP: `GET /health`, esperado `200`;
- no publicar `ports`; ambos Compose usan `expose`;
- límites iniciales: `64 MB` RAM y `0.25 CPU`.

## Configuración WuzAPI 3

Confirme endpoints con OpenAPI de su instalación antes de cambiar producción. En WuzAPI 3 estándar, endpoints de instancia usan header `Token`.

Configure HMAC:

```bash
curl -fsS -X POST "$WUZAPI_BASE_URL/session/hmac/config" \
  -H "Token: $WUZAPI_INSTANCE_TOKEN" \
  -H 'Content-Type: application/json' \
  --data-binary "{\"hmac_key\":\"$WUZAPI_HMAC_SECRET\"}"

curl -fsS "$WUZAPI_BASE_URL/session/hmac/config" \
  -H "Token: $WUZAPI_INSTANCE_TOKEN"
```

Después de probar gateway, configure webhook y único evento `Message`:

```bash
curl -fsS -X PUT "$WUZAPI_BASE_URL/webhook" \
  -H "Token: $WUZAPI_INSTANCE_TOKEN" \
  -H 'Content-Type: application/json' \
  --data-binary "{\"webhook\":\"$GATEWAY_PUBLIC_URL/webhook\",\"events\":[\"Message\"],\"Active\":true}"

curl -fsS "$WUZAPI_BASE_URL/webhook" \
  -H "Token: $WUZAPI_INSTANCE_TOKEN"
```

Gateway acepta firma hex pura y `sha256=<hex>` en `x-hmac-signature`. Comparación usa `hmac.Equal`.

## Activación segura

1. Despliegue gateway y confirme `/health`.
2. Use webhook n8n de prueba.
3. Pruebe firma válida e inválida.
4. Confirme que privado llega una vez y body permanece idéntico.
5. Confirme que status, grupo, from-me y eventos distintos no llegan.
6. Simule n8n caído y confirme `502`.
7. Configure WuzAPI con `events=["Message"]` y URL pública del gateway.
8. Mantenga filtro defensivo en n8n durante periodo de observación.

## Logs

Logs JSON incluyen acción, reason, status, duración y hash parcial de ID. Nunca incluyen body, contenido, teléfono, JID, token, secreto, firma o URL n8n.

## Rollback

Guarde URL anterior antes del cambio. Para rollback, restaure esa URL mediante `PUT /webhook`, mantenga `events=["Message"]` y compruebe configuración con `GET /webhook`. Gateway no conserva estado.

## Compatibilidad

Filtro implementa contrato WuzAPI 3 observado: `type` y `event.Info`. No acepta automáticamente formatos de otros proveedores como Evolution. Consulte [observaciones del payload](docs/payload-observations.md).

## Licencia

[MIT](LICENSE)
