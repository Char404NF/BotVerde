# Estudio de GOWA (go-whatsapp-web-multidevice)

Estudio de solo lectura, 2026-11-09. Nada de su código se copió a este proyecto (licencia MIT, `LICENCE.txt`).

- **Qué es:** servidor REST/MCP en Go (Fiber + SQLite) sobre whatsmeow, hecho para correr en un servidor con varias cuentas. Repo clonado en `~/estudio/gowa`, commit `8d6e058` (2026-11-10), 357 archivos `.go`.
- **whatsmeow:** GOWA fija `v0.0.0-20261105212532-307a2edf117d` (`src/go.mod`); nosotros `f1b798561919` (9-nov). Son casi la misma versión, así que lo que hace GOWA con whatsmeow aplica a nuestro módulo.
- **Rutas:** todas salen de `src/` en el repo (por ejemplo `infrastructure/whatsapp/device_manager.go`). Los números de línea son del commit `8d6e058`.
- **Diferencia de fondo:** GOWA es un servidor con muchas cuentas, webhooks y Chatwoot. Nosotros somos una sola cuenta dentro de un teléfono. Muchas de sus piezas no nos sirven.

---

## 1. Estado de cada cuenta y reconexión

**Cómo representa el estado**
- Cada cuenta es un `DeviceInstance` (`infrastructure/whatsapp/device_instance.go`) dentro de un `DeviceManager` (`device_manager.go`, mapa de "ranuras" por id).
- Estados posibles (`domains/device/device.go`): `disconnected`, `connecting`, `connected`, `logged_in`.
- `connecting` está declarado pero **no encontré ningún lugar que lo asigne**: nunca se usa.
- **No existe un estado "esperando vinculación".** Una cuenta sin sesión aparece como `disconnected`. El proceso de vincular (QR o passkey) se guarda aparte en `SetPasskeyChallenge` y compañía (`device_instance.go`).
- **El estado se calcula, no se recibe.** `UpdateStateFromClient` (`device_instance.go:166`) lo deduce de dos banderas de whatsmeow: `IsLoggedIn()` → `logged_in`; si no, `IsConnected()` → `connected`; si no, `disconnected`. Se recalcula al final de cada evento en `handler` (`event_handler.go:22`, última línea `instance.UpdateStateFromClient()`).
- `Status` (`usecase/app.go`) responde `(conectado, con_sesión)` leyendo las mismas banderas.

**Cómo se reconecta**
1. **Dentro de whatsmeow:** GOWA solo pone `client.EnableAutoReconnect = true` (`device_manager.go`, `EnsureClient`, línea 814). La reconexión es de la biblioteca (`autoReconnect` en whatsmeow `client.go:649`): espera `2 s × número de errores` entre intentos y no se rinde. Es lo que vimos en nuestro registro (reintentos a los 2 y 4 s, y volvió a las 19:10:02).
2. **Al arrancar el proceso:** `SetAutoConnectAfterBooting` (`ui/rest/helpers/common.go:13`) espera 2 s, lista las cuentas y llama `Reconnect` en cada una (`usecase/app.go:330`: `Disconnect()` y `Connect()`).
3. **Vigilante cada 5 minutos:** `SetAutoReconnectChecking` (`ui/rest/helpers/common.go:29`) llama `Connect()` si el cliente no está conectado. Solo vigila el cliente "global" heredado, no cada cuenta nueva.
4. **Registro de cuentas:** `LoadExistingDevices` (`device_manager.go:578`) une las ranuras guardadas (tabla `devices`) con las sesiones de whatsmeow, y evita adoptar sesiones ajenas.

**Casos especiales**
- `LoggedOut` (`handleLoggedOut`, `event_handler.go:246`): desconecta, avisa por webhook `device.logged_out`, borra las claves de la sesión pero **conserva la ranura y el historial** (`keepSlotLogout`, `device_manager.go:458`).
- `StreamReplaced` (`handleStreamReplaced`, `event_handler.go:341`): si otro proceso usa la misma sesión, deja solo esa cuenta desconectada y **no reconecta** (whatsmeow tampoco). Hay que reconectar a mano.
- `TemporaryBan` y `NotifyAccountReachoutTimelock`: se reenvían por webhook como eventos de dispositivo (`event_device.go`).
- Registro más limpio: `filteredLogger.Errorf` (`logger.go:29`) baja a "debug" el error `Error reading from websocket ... EOF`, porque es normal en una reconexión.
- Presencia: `presence_pulse.go` pone la cuenta "en línea" por ratos si se configura. Opcional.

---

## 2. Envío de mensajes

**Lo que hay**
- **Envío directo, sin cola.** `SendText` (`usecase/send.go:372`) valida el número, arma el mensaje y llama `wrapSendMessage` (`send.go:264`) → `client.SendMessage`. Si falla, devuelve el error. Guarda el mensaje enviado en la base de forma asíncrona (15 s de plazo).
- **Sin reintentos y sin pausas** en el envío normal. Tampoco vi límite de ritmo por contacto ni global. Lo único que traduce es el error 463 de "reach-out timelock" (`normalizeSendError`, `send.go:362`).
- **Validación del destino:** `ValidateJidWithLogin` (`pkg/utils/whatsapp.go:1116`): exige estar conectado, resuelve LID → teléfono y, si `WhatsappAccountValidation` está activo, consulta `IsOnWhatsApp` antes de enviar.
- **El número para vincular** (`validations/app_validation.go:12`) solo se valida con la expresión `^\+?[0-9]{1,15}$`; la limpieza fina la hace whatsmeow en `PairPhone`. No tiene lógica especial para México.

**Dónde sí hay cola: los mensajes programados** (`usecase/schedule.go`, tabla `scheduled_sends`)
- Un trabajador revisa cada 5 s (`Start`, línea 378). `processDue` (línea 429) reclama trabajos con un **arrendamiento de 2 minutos** (`ClaimNextScheduledSend`).
- Si la cuenta está desconectada, reintenta en 30 s **sin gastar intentos** (`processJob`, línea 488).
- Fallos de envío: `retry` (línea 624) espera `15 s × 2^intentos` con tope de 5 minutos; máximo 10 intentos (`scheduleMaxAttempts`), después se salta esa ocurrencia.
- **Evita duplicados:** antes de enviar, fija el id del mensaje de WhatsApp (`pinOccurrenceMessageID`, línea 519; columna `pending_message_id`). Si el proceso muere a mitad de un envío, `recoverInterrupted` (línea 461) reenvía con el **mismo id**.
- Soporta una vez, diario, semanal y mensual; zona horaria y fecha de fin (`validations.ParseScheduleOptions`).
- **No vi pausa entre trabajos vencidos:** `processDue` los reclama y envía seguidos. Para nosotros haría falta añadirla.

**Respuesta automática** (`infrastructure/whatsapp/auto_reply.go:17`, equivale a nuestro "hola")
- Un solo texto fijo (`WHATSAPP_AUTO_REPLY`). Solo chats privados (`s.whatsapp.net` o `lid`), nunca grupos, difusiones, estados ni mensajes propios. Exige texto real.
- **No tiene límite por contacto ni filtro de antigüedad:** contesta a todo mensaje de texto, incluso a los acumulados al reconectar. Nuestro módulo es más estricto.
- Responde a `evt.Info.Sender`; nosotros a `Info.Chat` (en chats privados dan lo mismo).

---

## 3. Eventos y webhooks

**Cómo se reparten**
- Cada cliente tiene un solo manejador (`eventHandler`, `client_events.go:12`) que busca a qué cuenta pertenece y llama `handler` (`event_handler.go:22`). Un `switch` por tipo despacha: mensajes, recibos, presencia, grupos, canales, llamadas, historial, estado de la app y eventos de sesión.
- Antes de enviar a webhooks, el mensaje se guarda: `handleMessage` (`event_message_handler.go:20`) → `CreateMessage` → imagen → marcar leído → respuesta automática → webhook.
- **El webhook va en una goroutine aparte** con 30 s de plazo (`handleWebhookForward`, `event_message_handler.go:211`), para no bloquear el bucle de eventos.

**Eventos que entrega** (`docs/webhook-payload.md`): `message`, `message.reaction`, `message.revoked`, `message.edited`, `message.ack`, `message.deleted`, `chat_presence`, `presence`, `group.participants`, `group.info`, `group.joined`, `label.*`, `newsletter.*`, `call.offer`, `device.logged_out`, `device.temporary_ban`, `device.reachout_timelock`.
- **Mensajes:** no reenvía mensajes de protocolo internos (solo `REVOKE` y `MESSAGE_EDIT`) ni de `broadcast`/estados.
- **Recibos** (`handleReceipt`, línea 357): solo entrega y leído. `shouldForwardReceipt` (`event_receipt.go:106`) descarta los recibos de otros dispositivos del contacto para no duplicar.
- **Grupos** (`event_group.go`): solo reenvía si hubo cambios reales (entró, salió, ascendió, degradó, nombre, tema, bloqueo).
- **Llamadas** (`handleCallOffer`, `event_call.go:16`): puede rechazarla sola (`WhatsappAutoRejectCall`), la guarda como registro y avisa por webhook.
- **Mensajes que no se pudieron descifrar** se registran con una advertencia y no producen webhook (`handleUndecryptableMessage`).

**Cómo entrega** (`webhook.go:19`, `submitWebhook`)
- POST JSON firmado con **HMAC-SHA256** en `X-Hub-Signature-256`. Plazo de 10 s por intento.
- **5 intentos** con espera 1, 2, 4 y 8 s, respetando el plazo del contexto.
- Varias URLs: éxito si al menos una responde (`forwardToWebhooks`, `webhook_forward.go:478`).
- Filtros: lista de eventos permitidos global y **por cuenta** (`isEventWhitelistedForDevice`, línea 401) y lista de JID a ignorar. La URL, el secreto y los eventos por cuenta se guardan en la tabla `devices`.
- **Cola con memoria solo para Chatwoot:** `chatwoot_forward_queue` + `StartChatwootForwardRetryWorker` (línea 1933), con espera creciente: 1 minuto que se duplica hasta un tope de 1 hora (`chatwootForwardRetryDelay`, línea 1613).
- **Para los webhooks normales no encontré cola persistente:** si los 5 intentos fallan, el evento se registra en el log y se pierde.

---

## 4. Historial: tablas y sincronización

**Tres bases separadas**
1. **Sesión de whatsmeow** (`storages/whatsapp.db`): claves, contactos, identidades. Se abre en `database.go` (`initDatabase`).
2. **Claves opcionales** en otra base (`DBKeysURI`): `configureKeysStore` (`device_manager.go:967`) y `key_cache.go` redirigen identidades, sesiones y claves a ese archivo. Los tokens de privacidad se quedan en la base principal.
3. **Mensajes y chats propios** (`chatstorage.db`): `infrastructure/chatstorage/sqlite_repository.go`, migraciones en `getMigrations` (línea 3123). Se aplican de a una con la tabla `schema_info` y no se reordenan.

**Esquema principal** (todo lleva `device_id`, así cada cuenta ve solo lo suyo)

| Tabla | Clave | Para qué |
|---|---|---|
| `chats` | `(jid, device_id)` | nombre, última hora, expiración, archivado |
| `messages` | `(id, chat_jid, device_id)` | texto, remitente, `is_from_me`, datos del archivo adjunto, metadatos de llamada y de respuesta |
| `message_reactions` | `(message_id, reactor_jid, device_id)` | reacciones |
| `message_edits` | con clave foránea a `messages`, `ON DELETE CASCADE` | historial de ediciones |
| `poll_definitions` | `(device_id, chat_jid, poll_message_id)` | encuestas |
| `devices` | `device_id` | registro de cuentas, JID, webhook propio |
| `scheduled_sends` | `id` | cola de programados (arrendamiento, intentos, id fijado) |
| `chatwoot_*` | varios | solo para Chatwoot |

- **Ajustes de SQLite** (`pkg/sqlite/sqlite_purego.go:14`, versión Go puro con `modernc`): WAL, `busy_timeout` 30 s, `synchronous(1)` y claves foráneas activas. La versión con cgo usa `mattn`. Un comentario en `wrapSendMessage` (`usecase/send.go:264`) advierte que las sincronizaciones de historial retienen el escritor de SQLite un buen rato.
- **No usa `temp_store`.** Corre en servidores con `/tmp`; en Android hace falta (el arreglo 0.1.1 nuestro).

**Cómo sincroniza el historial al vincular**
- El teléfono envía eventos `HistorySync` tras la vinculación. GOWA **no cambia la configuración de historial de whatsmeow**: no busqué ni encontré `RequireFullSync` ni `HistorySyncConfig` en su código, así que usa lo que whatsmeow pide por defecto.
- `handleHistorySync` (`history_sync.go:22`) **primero vuelca todo el bloque a un archivo JSON** en `storages/history-*.json` y después lo procesa. En un teléfono eso ocuparía almacenamiento sin necesidad.
- `processConversationMessages` (línea 87): para cada conversación, normaliza el JID (LID → teléfono), decide el nombre del chat con el almacén de contactos (`conversationChatName`) y guarda los mensajes **en lotes** (`StoreMessagesBatch`). Un mensaje sin id o sin remitente se salta.
- `processPushNames` (línea 348): crea o actualiza chats con los nombres que mandó WhatsApp. Los chats creados así quedan con hora cero para que no suban en la lista.
- Tipos que procesa: `INITIAL_BOOTSTRAP`, `RECENT`, `ON_DEMAND` y `PUSH_NAME`. Los demás se ignoran.
- No encontré un tope de días o de mensajes al sincronizar. El límite de días (`ChatwootDaysLimitImportMessages`, 3) solo aplica a lo que se importa a Chatwoot.

**Nombres de chat:** `ChatDisplayNameResolver` (`chat_display_name.go`) elige primero el nombre guardado si es significativo; si es solo un número o JID, usa el nombre completo del contacto, luego el apodo (`PushName`), luego el nombre de negocio, y al final el número.

---

## 5. Permisos de administrador en grupos

**Hallazgo principal: GOWA no comprueba antes si la cuenta es admin.**
- `ManageParticipant` (`usecase/group.go:123`) valida solo que existan el grupo y los participantes, convierte los números y llama directo `client.UpdateGroupParticipants` (agregar, quitar, ascender, degradar). Si no eres admin, **responde WhatsApp con un error** y GOWA lo devuelve.
- **Ojo con el mapeo de resultados:** en esa función solo se marca `error` cuando el participante trae código 403 con solicitud de ingreso (`participant.Error == 403 && participant.AddRequest != nil`). Cualquier otro caso se informa como `success`. No verifiqué qué códigos devuelve WhatsApp para un "quitar" sin permisos, pero el código no los distingue.
- **Borrar mensajes de otros** (`RevokeMessage`, `usecase/message.go:330`): busca al autor del mensaje guardado; si no es nuestro, arma un "borrado de admin" con `BuildRevoke`. El comentario del código dice que requiere ser admin; la verificación la hace el servidor de WhatsApp.
- **Dónde sí consulta admins: al recibir.** `IsGroupAdmin` (`webhook_forward.go:1298`) pide la información del grupo (plazo 10 s) y comprueba si el usuario es `IsAdmin` o `IsSuperAdmin`, comparando JID, LID y número (un fallo de consulta cuenta como "no admin"). Se usa en `groupAdminRevoke` (`chatstorage/sqlite_repository.go:2485`) para decidir si un borrado hecho por un admin ajeno es legítimo antes de borrar el mensaje de la base.
- La lista de participantes con banderas de admin está en `GetGroupParticipants` (`usecase/group.go:168`), pero solo se muestra; no se usa para decidir.

---

## 6. Contactos duplicados

- El aviso **`637 duplicate contacts found in PutAllContactNames` viene de whatsmeow, no de GOWA**: `store/sqlstore/store.go:681-693` (hay uno parecido en `PutManyRedactedPhones`, línea 723).
- Qué hace whatsmeow: invierte la lista, **se queda con la última entrada de cada JID** (la más nueva), borra las repetidas y guarda por lotes en una sola transacción. Luego vacía su caché de contactos. Es solo un aviso: los datos quedan bien.
- En GOWA **no hay código propio** para duplicados de contactos. No lo busca ni lo corrige. Lo único parecido son:
  - un cerrojo por contacto al crear contactos y conversaciones en Chatwoot (`getContactMutex`, `webhook_forward.go:157`);
  - la normalización LID → teléfono (`NormalizeJIDFromLID`, `jid_utils.go:12`), que evita guardar la misma persona dos veces con dos identificadores.
- Conclusión: nuestro aviso es inofensivo y se puede ignorar o bajar de nivel en el registro.

---

## 7. Qué hace distinto nuestro módulo (`go/wabot`)

| Tema | GOWA | Nuestro módulo |
|---|---|---|
| Cuentas | varias, con ranuras y registro (`DeviceManager`) | una (`estado` global) |
| Estado | calculado de las banderas de whatsmeow, 4 valores (uno sin uso) | **textos enviados por callback** (`sin_vincular`, `esperando_codigo`, `vinculado`, `conectando`, `conectado`, `desconectado`, `sesion_cerrada`, `error: …`): más detalle |
| `Start` | `EnsureClient` crea el cliente de forma perezosa, sin conectar; la conexión sale de otra función (`Reconnect` / `Login`) | abre la base, crea el cliente y **conecta solo** si ya hay sesión |
| `Pair` | `LoginWithCode` (`usecase/app.go:161`): bloqueo por cuenta, validación del número, conecta y llama `PairPhone`; **devuelve el código en la respuesta** | **callback** `OnPairingCode`, espera el evento QR (20 s) antes de pedirlo; formatea `ABCD-EFGH` |
| `Stop` | `Disconnect` (varios sitios); la cuenta sigue registrada | desconecta, cierra la base y vuelve a `detenido` |
| `IsConnected` | `DeviceInstance.IsConnected/IsLoggedIn` | una sola función: conectado **y** con sesión |
| `Logout` | `LogoutDeviceKeepSlot` (`device_manager.go:424`): desvincula en WhatsApp (si falla, sigue) y borra filas de sesión, **conserva la ranura** | desvincula en WhatsApp; si falla borra la sesión local y avisa; la app **borra el bot** |
| Reconexión | la de whatsmeow + conexión al arrancar + vigilante de 5 min | solo la de whatsmeow |
| Respuesta | texto fijo configurable, sin límites | "hola": sin grupos, sin propios, sin editados, **máx. 1 por contacto por 60 s, ignora mensajes de más de 2 min, pausa de 1 a 3 s** |
| `LoggedOut` remoto | limpia la sesión y avisa; conserva la ranura | solo cambia el estado a `sesion_cerrada`; **no marca el bot como desvinculado en la base de la app** |
| Historial | guarda mensajes, chats, reacciones y ediciones | **todavía no guarda nada** (etapa 2) |
| Registro de eventos | logrus | callback `OnLog`, con números enmascarados y sin texto de mensajes |
| SQLite | `modernc` solo en la compilación `purego` (WAL, 30 s, claves foráneas); `mattn` con cgo | `modernc`; añade `temp_store(2)` por Android |

---

## Recomendaciones para la etapa 2

### Conviene adoptar

1. **Una base propia de mensajes separada de la sesión de whatsmeow.** Es lo que hace GOWA (`chatstorage.db` aparte de `whatsapp.db`): si algo falla en una, la otra sigue intacta.
   - Dónde se ve: `infrastructure/chatstorage/sqlite_repository.go` (`getMigrations`, línea 3123), `infrastructure/whatsapp/database.go` (`initDatabase`).
2. **Esquema con `device_id` y claves compuestas** `chats(jid, device_id)` y `messages(id, chat_jid, device_id)`. Aunque hoy sea un solo bot, nos deja tener varios después (ya diseñamos `bots` así).
   - Dónde: `getMigrations`, migraciones 1 y 2.
3. **Migraciones numeradas y solo hacia adelante** con una tabla de versión (`schema_info`, `runMigration`). Se aplican de a una en una transacción.
   - Dónde: `sqlite_repository.go`, `getSchemaVersion` y `runMigration` (alrededor de las líneas 3075-3120).
4. **Guardar mensajes en lotes** (`StoreMessagesBatch`) y usar WAL con `busy_timeout` de 30 s.
   - Dónde: `sqlite_repository.go:433`, `pkg/sqlite/sqlite_purego.go:14`.
5. **Normalizar LID → teléfono antes de guardar** (`NormalizeJIDFromLID`), con la regla "si falla, usar el JID original". Es clave para agrupar "por persona".
   - Dónde: `infrastructure/whatsapp/jid_utils.go:12`. La guía `infrastructure/whatsapp/AGENTS.md` explica la regla.
6. **Resolver el nombre de la persona con la jerarquía de GOWA:** nombre guardado significativo → nombre del contacto → apodo → nombre de negocio → número.
   - Dónde: `infrastructure/whatsapp/chat_display_name.go` (`ChatDisplayNameResolver.Resolve`) y `history_sync.go` (`conversationChatName`, línea 431).
7. **Bajar a "debug" el aviso de cierre del websocket** (`EOF`) en nuestro `OnLog`, para que el registro no asuste.
   - Dónde: `infrastructure/whatsapp/logger.go:29` (`filteredLogger.Errorf`).
8. **Sincronización del historial por lotes y por tipo** (`INITIAL_BOOTSTRAP`, `RECENT`, `ON_DEMAND`, `PUSH_NAME`), con una excepción: ver abajo.
   - Dónde: `history_sync.go:61` (`processHistorySync`) y `processConversationMessages` (línea 87).
9. **Para la etapa 3 (moderación):** copiar la idea de `IsGroupAdmin` pero usarla **antes** de actuar. Consultar la información del grupo, comprobar `IsAdmin`/`IsSuperAdmin` por JID, LID y número, y tratar un fallo de consulta como "no admin".
   - Dónde: `infrastructure/whatsapp/webhook_forward.go:1298`. También tomar de `ManageParticipant` (`usecase/group.go:123`) la llamada a `UpdateGroupParticipants`, pero **leer el resultado de cada participante**, no solo el 403.
10. **Para la etapa 4 (programador):** tomar el diseño de `scheduled_sends`: arrendamiento, id de mensaje fijado antes de enviar, reintento con espera creciente (`15 s × 2^n`, tope 5 min, máx. 10) y "dispositivo sin conexión no gasta intentos". En Android, además, guardar la hora de la próxima ejecución para recuperarse tras un reinicio del teléfono.
    - Dónde: `usecase/schedule.go` (`processDue` 429, `processJob` 488, `deliver` 556, `retry` 624, `pinOccurrenceMessageID` 519, `recoverInterrupted` 461) y la tabla `scheduled_sends` en `getMigrations`.
11. **Qué hacer con `LoggedOut`:** además de avisar, marcar el bot como desvinculado en la base de la app (hoy no lo hacemos) para que la pantalla ofrezca vincular de nuevo.
    - Dónde se ve el patrón: `event_handler.go:246` (`handleLoggedOut`) y `device_manager.go:458` (`keepSlotLogout`).
12. **Vigilante de reconexión propio:** si el estado lleva varios minutos en `desconectado`, llamar `Connect()` (el patrón de `SetAutoReconnectChecking`, `ui/rest/helpers/common.go:29`), pero atado al servicio de Android.

### No conviene adoptar

1. **El volcado del historial a un JSON en disco** antes de procesarlo (`handleHistorySync`, `history_sync.go:22`). Ocupa almacenamiento del teléfono y no aporta nada.
2. **Webhooks, Chatwoot, MCP, REST, WebSocket y cola de Chatwoot** (`webhook.go`, `webhook_forward.go` en su mayoría, `infrastructure/chatwoot/`). Nuestra app no tiene servidor.
3. **El registro de varias cuentas con ranuras, alias y emparejamiento de JID** (`DeviceManager`, `LoadExistingDevices`). Es muy complejo (maneja varias sesiones para un mismo número). Nos basta con la tabla `bots`.
4. **La respuesta automática tal cual** (`auto_reply.go`): no tiene límite por contacto ni filtro de mensajes viejos. La nuestra es más segura.
5. **Enviar sin pausa ni límite** (`wrapSendMessage`, `usecase/send.go:264`) y **los programados seguidos sin espera** (`processDue`). Con un número secundario y riesgo de baneo, conviene una pausa entre envíos y un tope por hora.
6. **Marcar `success` en `ManageParticipant`** salvo en un caso (`usecase/group.go:123`). No repetir ese mapeo.
7. **Presencia automática "en línea"** (`presence_pulse.go`): gasta batería y datos y no la necesitamos.

### Cosas que no pude confirmar
- Qué códigos devuelve WhatsApp al quitar o ascender sin ser admin: no hay pruebas en el repo que lo muestren y nosotros no lo hemos probado.
- Si GOWA ajusta la cantidad de historial que pide al vincular: no encontré ninguna configuración, pero no revisé el código de whatsmeow (`appstate` y `DeviceProps`) para ver qué pide por defecto.
- Cuánto crece `chatstorage.db` con el uso real: no hay medición en el repo.
