# Contrato de payload observado

Inspección read-only de ejecuciones WuzAPI 3 confirmó esta estructura:

```text
type
event.Info.Chat
event.Info.Sender
event.Info.IsFromMe
event.Info.IsGroup
event.Info.ID
event.Info.Timestamp
event.Info.Type
event.Message
```

`type` es discriminador de evento. Para mensajes válidos su valor es `Message`. `event.Info` contiene origen; no apareció `MessageSource` anidado. Se observaron chats privados `@lid`.

Contrato alternativo con `event` string y `data.Info` corresponde a otros proveedores y no se admite. Default-deny evita reenviar estructuras desconocidas.

Fixtures son sanitizados. No contienen teléfonos, nombres, tokens, IDs o mensajes reales. Fixtures de status, grupo y mensajes propios preservan forma verificada, cambiando solo campos necesarios para probar clasificación.

La especificación pública WuzAPI 3 confirma:

- autenticación de instancia mediante header `token`;
- `GET`/`POST`/`PUT /webhook`;
- actualización mediante campos `webhook`, `events` y `Active`;
- `POST`/`GET /session/hmac/config`;
- HMAC mínimo de 32 caracteres;
- firma en header `x-hmac-signature`.

Instalaciones y forks pueden diferir. Verifique OpenAPI de su instancia antes de modificar configuración productiva.
