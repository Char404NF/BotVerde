# Proyecto: app Android nativa con bot de WhatsApp

## Proyecto
- App Android nativa (Kotlin + Jetpack Compose) que se vincula a WhatsApp como dispositivo vinculado y funciona toda en el teléfono: sin servidor, sin web y sin servicios de Google (nada de Firebase/FCM/Play Services).
- Conexión con whatsmeow (Go, módulo oficial go.mau.fi/whatsmeow), empaquetada con "gomobile bind" a un .aar que usa la app Kotlin con un servicio en primer plano.
- Se descartó Baileys (exige Node 20+, nodejs-mobile solo trae Node 18) y Cobalt (exige Java 25).
- Etapas: 1) vincular con código de 8 dígitos y responder "hola"; 2) guardar mensajes en SQLite y verlos por persona/grupo; 3) moderación de grupos (solo si el número es admin); 4) programador de mensajes; 5) estados (experimental).
- UI muy simple y visual; opciones técnicas escondidas en "Ajustes avanzados".
- Riesgo: whatsmeow es no oficial y puede causar baneo; usar número secundario y pocos envíos.

## Reglas
1. Responde siempre en español, breve y directo. Charly trabaja solo desde el celular: los comandos que me des deben ir en un solo bloque para copiar.
2. NO toques nada fuera del espacio nuevo. Este servidor tiene otros servicios (correo, Nextcloud, radio, bots de WhatsApp y Telegram, panel web). No los detengas, no reinicies el servidor, no borres ni modifiques sus archivos y no uses docker.
3. Verifica todo antes de usarlo: versiones, URLs y nombres de paquetes. Nada de inventar. Si algo no existe, busca la alternativa correcta y avisa.
4. Usa instalaciones sin root dentro del espacio nuevo siempre que sea posible. Solo usa root para paquetes del sistema y para crear el usuario.

## Entorno (probado el 2026-11-09)
Compilación en Linux x86_64 con JDK 17, Android SDK, Go y gomobile.
El entorno se carga solo al entrar como wabot (`~/.wabot_env`, enlazado desde `~/.bashrc` y `~/.profile`).

| Pieza | Versión | Ruta |
|---|---|---|
| JDK | OpenJDK 17.0.20.1 (paquete del sistema) | `/usr/lib/jvm/java-17-openjdk-amd64` (`JAVA_HOME`) |
| Node (del sistema, no se usa en la app) | 20.20.2 | `/usr/bin/node` |
| Go | 1.27.3 | `~/herramientas/go` (`GOROOT`), `GOPATH=~/go` |
| gomobile / gobind | x/mobile v0.0.0-20261011034528-197770444867 | `~/go/bin` |
| Gradle | 9.9.0 (distribución) y wrapper 9.9.0 en este proyecto | `~/herramientas/gradle-9.9.0`; `gradlew` en `wabot-app/` |
| Android cmdline-tools | 22.0 (build 15859902) | `~/android-sdk/cmdline-tools/latest` |
| platform-tools | 37.0.1 | `~/android-sdk/platform-tools` |
| Plataformas | android-36 y android-37.0 | `~/android-sdk/platforms` |
| build-tools | 36.0.0 | `~/android-sdk/build-tools` |
| NDK | 28.2.13676358 (r28c) | `ANDROID_NDK_HOME=~/android-sdk/ndk/28.2.13676358` |

`ANDROID_HOME=~/android-sdk`. Licencias del SDK aceptadas. `~/.gradle/gradle.properties`: daemon desactivado, máx. 2 workers, sin paralelo, `-Xmx2g`.

### Cómo compilar
- Siempre con `nice -n 10`: `nice -n 10 ./gradlew assembleDebug --no-daemon`.
- Archivo `.aar` de Go: `nice -n 10 gomobile bind -target=android/arm64 -androidapi 26 -o app/libs/wa.aar .` (solo arm64 para ir más rápido; con las 4 arquitecturas tardó casi 7 min con whatsmeow y pesa 54 MB).

### Combinación probada (compiló un APK debug)
- AGP `com.android.application` 9.5.1 (trae Kotlin integrado) + plugin `org.jetbrains.kotlin.plugin.compose` 2.4.21.
- Compose BOM 2026.09.00, `androidx.activity:activity-compose` 1.13.0, `androidx.core:core-ktx` disponible hasta 1.19.1.
- `compileSdk = 37` (Compose 1.12 lo exige; con 36 falla), `targetSdk` 36, `minSdk` 26, Java 17.
- `.aar` incluido con `implementation(files("libs/xxx.aar"))`.

### Trampas conocidas
- `go mod tidy` borra `golang.org/x/mobile` si el código no lo importa y `gomobile bind` falla con "missing golang.org/x/mobile dependency". Solución: `go get -tool golang.org/x/mobile/cmd/gobind golang.org/x/mobile/cmd/gomobile`.
- Controlador SQLite en Go puro: `modernc.org/sqlite` (v1.62.0 probado). En whatsmeow se abre con `sqlstore.New(ctx, "sqlite", "file:RUTA?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)", log)`. No usar `mattn/go-sqlite3` (necesita cgo).
- whatsmeow se usa en la rama principal (pseudo-versión, probada `v0.0.0-20261109152745-f1b798561919`); no hay etiquetas de versión.

### Prueba real hecha (2026-11-09)
- `gomobile bind` de un paquete mínimo: OK (`.aar` de 6 MB con 4 arquitecturas).
- Proyecto Android mínimo con Compose usando ese `.aar` → APK debug OK (18 MB).
- `go.mau.fi/whatsmeow` + `modernc.org/sqlite`: `go get` OK, prueba en el servidor OK (crea el esquema y el cliente) y `gomobile bind` a Android OK.
- Las pruebas se borraron; el entorno y las cachés (`~/.gradle`, `~/.cache`, `~/go`) se conservan.

## Estado: ETAPA 1 (2026-11-09)
Hecho: asistente "Crear bot" de 3 pasos (nombre, país y número, confirmar y vincular con código de 8 dígitos), pantalla principal (círculo rojo/verde, Iniciar/Detener, Ajustes avanzados con Registro, Copiar registro y Eliminar bot), ayuda de batería, servicio en primer plano (`specialUse`), el bot responde "hola" a textos privados (máx. una vez por contacto cada 60 s).
Sin probar contra WhatsApp real (lo prueba Charly con su número secundario) ni en un teléfono real.

### Estructura
- `go/wabot/` módulo Go (`wabot.go` API, `responder.go` reglas de respuesta, pruebas `*_test.go`). Genera `app/libs/wabot.aar` (Java: `wabot.Wabot`, `wabot.Listener`).
- `app/src/main/java/com/botchat/app/`: `Paises.kt` (245 países, nombres en español, bandera con respaldo de letras), `Telefono.kt` (formato/validación con libphonenumber 9.0.43, sin Google), `Datos.kt` (SQLite; tabla `bots`, lista para varios), `BotState.kt` (estado y registro), `BotService.kt` (servicio), `AppVm.kt` (navegación del asistente), `Ui.kt` y `MainActivity.kt` (Compose).
- Paquete `com.botchat.app`, nombre "Bot Chat" (sin la palabra WhatsApp ni su logo).

### Comandos (como wabot)
- Pruebas Go: `cd ~/wabot-app/go && nice -n 10 go test ./...`
- Regenerar el .aar (tras cambiar Go): `cd ~/wabot-app/go && nice -n 10 gomobile bind -target=android/arm64 -androidapi 26 -trimpath -ldflags="-s -w" -o ../app/libs/wabot.aar ./wabot`
- Pruebas Kotlin: `cd ~/wabot-app && nice -n 10 ./gradlew testDebugUnitTest --no-daemon`
- APK: `cd ~/wabot-app && nice -n 10 ./gradlew assembleDebug --no-daemon` y copiar `app/build/outputs/apk/debug/app-debug.apk` a `salida/`.

### Decisiones y puntos a vigilar
- Número para vincular: `PairPhone` quita lo que no sea dígito, exige más de 6 dígitos y que no empiece con 0 (leído del código fuente). México se envía como `52` + 10 dígitos. Las fuentes públicas no coinciden sobre si WhatsApp quiere `521`: hay un interruptor "Formato antiguo de México (521…)" en Opciones avanzadas del paso 3. Si el teléfono dice que el código es incorrecto, probar con el interruptor activado. Otros países usan E.164 de libphonenumber (Argentina, +54 9…, no tiene tratamiento especial).
- `PairPhone` exige conectar primero y esperar el evento QR; el nombre del cliente debe ser "Navegador (Sistema)": se usa "Chrome (Linux)".
- Los mensajes acumulados con el bot apagado (más de 2 min) no se responden. Se ignoran grupos, estados, canales, propios y editados. Se espera 1–3 s antes de responder.
- Servicio `specialUse` (dataSync se corta a las 6 h en Android 15). Sin Firebase/Play Services: `./gradlew :app:dependencies` no muestra gms ni firebase y el dex no contiene `com/google/android/gms`.
- La sesión de WhatsApp (`filesDir/bots/<id>/sesion.db`) queda excluida de copias en la nube y de transferencia a otro equipo.
- El APK debug se firma con `~/.android/debug.keystore` de este servidor: para instalar actualizaciones encima hay que seguir usando esa misma llave.

## Estado: ETAPA 2 (2026-11-09) — versión 0.2.0 (versionCode 3)
Hecho: el bot guarda mensajes y archivos cifrados, se ven con Material 3 (lista de chats, chat, visor de fotos/videos, audios, PDF/documentos, archivos compartidos), administración de espacio, PIN + huella + FLAG_SECURE, Avanzado y "Medir rendimiento". El bot SOLO RESPONDE ("hola"); no envía nada por su cuenta (sin programados, estados ni botones).
No verificado aquí: WhatsApp real, teléfono real, rendimiento real, Keystore, PDF y reproducción Media3 en ejecución (no hay emulador/KVM). Lo cubren pruebas Go, pruebas Kotlin JVM y pruebas Robolectric con un `Motor` falso.

### Módulos nuevos
- Go `go/internal/`: `cifrado` (archivos y columnas), `identidad` (teléfonos, nombre_mostrado), `almacen` (messages.db, migraciones, consultas, espacio, reparar), `ingesta` (eventos → filas; política de descarga), `medios` (cola de descargas), `fotos` (fotos de perfil). API para Kotlin en `go/wabot/api.go` (`Start(raiz, botID, clave, Listener)`, `ListarChats/Mensajes/Archivos`, `EspacioTotal/PorTipo/PorChat`, `Estimar*/Borrar*`, `Reparar`, `Descargar`, `FotoGrande`, `ConfigurarDescargas`, `EstablecerRed`…) y `Observador.OnCambio(json)`.
- Kotlin: `seguridad/` (FormatoCifrado, ClaveDatos, Pin, Huella), `datos/` (Motor = interfaz sobre Go, Repo, Ajustes, Modelos), `medios/` (Temporales, CacheImagenes, FuenteCifrada para Media3, Exportar, ReproductorAudio), `ui/` (Compose). Pruebas de pantalla en `app/src/test/.../pantallas/`.

### Formato de cifrado (igual en Go y Kotlin; fixtures en `pruebas_cruzadas/`)
- Llave de datos: 32 bytes aleatorios, envueltos con una llave AES-GCM de AndroidKeyStore (alias `botchat_envolver_v1`, sin autenticación de usuario) en `filesDir/clave.bin`. Se pasa a Go como `[]byte` y solo vive en memoria.
- Archivos `.enc` (BCF1): encabezado de 20 bytes ("BCF1", versión 1, log2 del bloque = 16, 2 reservados, nonce base de 12). Bloques de 64 KB de texto claro, cada uno AES-256-GCM (texto cifrado + etiqueta de 16). Nonce = base + índice de bloque (96 bits big-endian con acarreo). AAD = encabezado(20) ‖ índice (8, BE) ‖ marca de último (1). Siempre hay ≥ 1 bloque (archivo vacío = un bloque vacío). El tamaño en claro se deduce del tamaño del archivo; permite acceso aleatorio (seek de video).
- Columnas de texto: versión(1) ‖ nonce(12) ‖ cifrado+etiqueta, AAD = nombre de la columna; vacío = NULL.

### Esquema (`filesDir/bots/<id>/mensajes.db`, SQLite modernc, WAL; sesión de WhatsApp aparte en `sesion.db`)
- Migraciones en `almacen/migraciones.go` (cada una en su transacción, tabla `schema_info`). Tablas: `chats` (clave jid+bot_id), `mensajes` (id+chat_jid+bot_id, FK con cascada), `archivos`/adjuntos con estado (`ninguno`, `descargando`, `descargado`, `fallido`, `omitido`), `ajustes`. Fechas en segundos Unix. Rutas relativas: `media/<bot>/<hash16(chat)>/<id>.enc`, `.thumb.enc`, `avatars/<bot>/<hash>.enc` y `_full.enc`.
- Texto, nombres y rutas de adjuntos van cifrados por columna. Nunca se muestra un JID/LID.

### Decisiones
- Nombre mostrado: agenda → nombre de perfil → negocio → número con formato; grupos = asunto; solo LID sin número → "Número no disponible". México: se oculta el 521 solo al mostrar. Al conocerse la relación LID↔teléfono se fusionan los chats.
- Ver una vez: solo se guarda un registro `ver_una_vez` con texto fijo, estado `omitido`, sin miniatura, llaves ni rutas.
- Descargas: fotos/audios/documentos ≤ 50 MB automático; videos solo con Wi‑Fi (salvo ajuste); mínimo 500 MB libres; 2 simultáneas y las > 8 MB de una en una; reintentos 10 s / 1 min / 5 min; 403/404/410 = `fallido` definitivo.
- Seguridad: ver "Etapa 2.5" (el bloqueo ya es opcional y viene apagado). PIN con PBKDF2 y esperas crecientes por intentos fallidos. Copias descifradas temporales en `cache/tmp_descifrado/` se borran al bloquear y al abrir la app. "Guardar copia" va a Documents/BotChat/<chat>/ (MediaStore, Android 10+).
- Logs: nunca llevan texto de mensajes, nombres ni nombres de archivos; los números van enmascarados.
- Sin Google: ni Firebase ni Play Services (verificado en el APK). Lint: `UnsafeOptInUsageError` desactivado a propósito (Media3 UnstableApi, versión fija).

### Límites conocidos
- La búsqueda solo funciona por nombre del chat y número (el texto de los mensajes va cifrado por columna y no se indexa).
- whatsmeow solo descarga a memoria: un archivo de ~50 MB sube la memoria de golpe; por eso las grandes van de una en una.
- Rendimiento sin medir en teléfono real. Objetivos: foto ~5 MB < 200 ms, primer cuadro de video de ~50 MB < 1 s, desplazamiento de 200 chats fluido (≤ 5 % de cuadros caídos). Se mide en Avanzado → Medir rendimiento.
- Solo arm64; APK debug firmado con la llave de este servidor (misma que la etapa 1, actualiza sin perder la sesión).

## Estado: ETAPA 2.5 (2026-11-10) — versión 0.2.1 (versionCode 4)
Hecho: bloqueo opcional, ícono nuevo, paleta y sistema de diseño, rediseño con barra superior y navegación inferior, animaciones, bienvenida y subida a GitHub. No se tocó la lógica del bot, el cifrado ni la base de mensajes.
APK: `salida/BotChat-etapa2-5-debug.apk` (misma llave de firma que las etapas 1 y 2: actualiza encima y conserva la sesión).
No verificado sin teléfono: cómo se ve el ícono en el lanzador (adaptable y temático), fluidez real de las animaciones y la huella (BiometricPrompt).

### Bloqueo opcional (Ajustes → Seguridad)
- Por defecto NO hay bloqueo: la app nunca exige PIN ni huella. `Ajustes.bloqueo` = `NINGUNO` | `PIN` | `HUELLA`. Quien venía de la 0.2.0 con PIN conserva su bloqueo (PIN o huella); sin PIN queda en `NINGUNO`.
- Tiempo de bloqueo (solo con bloqueo activo): enseguida (0), 1 min (por defecto) o 5 min. Girar la pantalla no cuenta como salir.
- Huella: Android 11+ con huella registrada usa el bloqueo del teléfono como respaldo (BIOMETRIC_STRONG | DEVICE_CREDENTIAL) y no pide PIN propio; si no, se crea un PIN de respaldo.
- Quitar el bloqueo pide el PIN actual (o la huella si no hay PIN propio). "Olvidé mi PIN": confirma con el bloqueo del teléfono y lo quita; si el teléfono no tiene bloqueo, doble confirmación y aviso de que el PIN es solo una protección ligera.
- Olvidar el PIN NO deja sin acceso a los mensajes: la llave de datos vive en el Keystore sin depender del PIN (`setUserAuthenticationRequired(false)`); el PIN solo decide quién abre la app. Solo se pierde el acceso con "borrar datos" o desinstalar.
- "Evitar capturas de pantalla" (FLAG_SECURE) es un interruptor aparte, apagado por defecto.
- Pruebas: `SeguridadUiTest`, `NavegacionUiTest`.

### Ícono
- Vector con viewBox 1024 (turquesa #2EC4CB, ámbar #F59E28, fondo #151A38). Archivos en `res/drawable`: `ic_launcher_foreground` (adaptable, el fondo es el color `ic_launcher_background`), `ic_launcher_monochrome` (Android 13 temático), `ic_stat_bot` (notificación, blanco, 24 dp) e `ic_logo` (logo dentro de la app, `LogoBot()`). `LogoDibujado` (Compose) dibuja el mismo logo por partes para el splash.

### Paleta y sistema de diseño (`ui/Tema.kt`)
- Oscuro: fondo #0F1330, superficie #151A38, variante #1F2547, primario #2EC4CB (sobre #003739, contenedor #00494D / #A0F1F5), ámbar #F59E28 (sobre #4A2A00, contenedor #6A3F00), texto #E3E5F5, contorno #8A90B5. Claro: fondo #F6F8FF, superficie #FBFCFF, primario #00727A, contenedor #9EF0F6, ámbar #8A5100 / #FFDCBA, texto #151A38.
- Turquesa = acciones y "conectado"; ámbar solo como acento (avisos, "reconectando"). `LocalColoresApp` da esos dos colores de estado. Contraste mínimo AA verificado en pruebas (`TemaUiTest`).
- Tema: Sistema/Claro/Oscuro (Avanzado → Aspecto). Color dinámico del sistema: opción apagada por defecto en Avanzado.
- Espaciado en múltiplos de 4 dp (`Esp`), formas redondeadas (`Formas`), tipografía Material 3 con pesos propios, un solo conjunto de íconos (`Ico`).

### Navegación y pantallas
- Barra superior (logo + nombre + estado: Conectado / Reconectando / Desvinculado / Detenido) y navegación inferior Chats · Espacio · Ajustes en las pantallas principales; chats, visor, documentos y archivos compartidos van a pantalla completa. Ajustes: Seguridad, Descargas, Estado del bot y Avanzado (aspecto, mantenimiento, registro, eliminar bot).
- Primer arranque: bienvenida de 3 pantallas con aviso de herramienta no oficial y casilla "Entiendo y acepto" obligatoria antes de vincular. Si ya había un bot (actualización), solo el aviso, una vez.
- Contador de chats: mensajes recibidos desde que se abrió el chat (hasta "30+"), calculado en el teléfono con lo ya guardado; no toca la base de mensajes.

### Animaciones (solo APIs de Compose, 150–300 ms)
- Splash con el logo (< 1 s, no tapa toques; no sale con "Reducir animaciones"), transición entre pantallas y elemento compartido avatar/nombre, entrada escalonada de la lista (solo la primera vez), burbujas nuevas que se deslizan, miniaturas con desvanecido, punto de estado que pulsa o gira, escala al tocar chats, palomita animada al vincular. Respeta "quitar animaciones" de Android (escala 0) y el interruptor "Reducir animaciones" de Avanzado (`LocalReducirMovimiento`).

### GitHub
- Repositorio privado `Char404NF/BotVerde`, rama `main`, etiquetas `v0.2.0` y `v0.2.1`. No se suben APK (`salida/` está ignorada), bases de datos, archivos `.enc`, llaves ni `.env`.
- Excepción a la regla global "No subas código a GitHub" (confirmada por Charly el 2026-11-10): vale SOLO para este repositorio y SOLO con su deploy key. El `CLAUDE.md global` global no se modifica.
- Llave de despliegue: `~/.ssh/<llave>` (usuario `wabot`, sin contraseña, con permiso de escritura solo en BotVerde); alias SSH `github.com` en `~/.ssh/config`; la huella de github.com en `known_hosts` se verificó contra la documentación oficial de GitHub. Nunca copiar la llave privada fuera del servidor.
- Subir cambios: `su - wabot -c 'cd ~/wabot-app && git push'`; versión nueva: `git tag -a vX.Y.Z -m "..." && git push origin vX.Y.Z`. Antes de subir: `git ls-files` y buscar secretos (keystore, password, token, .pem) en archivos e historial.

## Estado: 0.3.0 "Info del chat" (2026-11-10) — versionCode 6
- **Estadísticas solo con datos sin cifrar.** Tabla `daily_stats(bot_id, chat, sender, day, hora, tipo, count, bytes)` en `messages.db` (migración 2; `hora` se añadió a la clave del plan original para poder graficar por hora). `sender` = JID del remitente o `"yo"` si lo envió el bot. Nunca se lee el texto de los mensajes. Se actualiza en la misma transacción que guarda cada mensaje (`GuardarMensajes`); `FusionarChat` fusiona contadores, `BorrarChat`/`EliminarBot` los borran.
- **Relleno de lo anterior:** la migración 2 anota `rowid_max` en `stats_info`; `RellenarEstadisticas` cuenta solo los mensajes con rowid ≤ ese tope (una vez, en segundo plano, al fijar la zona). Los mensajes nuevos ya cuentan por su cuenta, así que nunca se duplican. Mientras corre, las pantallas dicen "contando los mensajes anteriores…".
- **Zona horaria:** `time/tzdata` va dentro del programa; Kotlin llama `Repo.actualizarZona()` (al abrir y en `onStart`) → `Wabot.establecerZona(id)`. El día de cada mensaje nuevo usa la zona de ese momento; el relleno usa la zona actual (nota visible en Estadísticas e Info del chat). "Estadísticas desde [fecha]" = primer mensaje guardado.
- **Contactos y ubicaciones:** `Clasificar` guarda JSON versionado (`{"v":1,"contactos":[{nombre,tels}]}` / `{"v":1,lat,lon,nombre,dir,vivo}`) en `texto_cifrado`; los mensajes viejos (texto plano "Contacto: X") se siguen mostrando (`Json.contactos/ubicacion` devuelven null). `VistaPrevia` (Go) y `descripcionCorta` (Kotlin) arman el texto corto de la lista. Lo que no se reconoce se guarda como `otro` con "Mensaje especial" (tarjeta; ya no hay "[Mensaje no compatible…]").
- **Pantallas:** encabezado del chat → foto = foto grande, nombre/número = `info/{jid}` (`InfoChat.kt`: Resumen, Calendario con colores por actividad y lista del día, Multimedia por fecha con filtros). Ajustes → Estadísticas (`PantallaEstadisticas.kt`). Gráficas con Compose Canvas (`Graficos.kt`), sin librerías. Tarjetas de contacto (copiar/agregar a agenda), ubicación (intent `geo:`), mensaje especial y stickers animados (`medios/Stickers.kt`, ImageDecoder en Android 9+; en 8.x se ve el primer cuadro).
- Trampa: en Compose no uses `return@repeat` dentro de lambdas componibles (corrompió el slot table en una prueba: "Write to an invalid slot index"); usa if/else.
- Trampa: la palabra "clave" en el esquema SQL rompe `TestVerUnaVezNoDejaNadaEnDisco` (busca "clave" en el archivo de la base); por eso `stats_info` usa la columna `campo`.

## Estado: 0.4.0 "Todo en uno" (2026-11-10) — versionCode 7
Una sola entrega con cinco bloques (A–E). `PROGRESO.md` tiene la lista de ítems. Migraciones de `messages.db`: 3 (eliminados), 4 (envío/citas/acuses/reacciones/estado por chat), 5 (ver una vez), 6 (dedupe y actividad), 7 (automatizaciones). Siempre hacia adelante.

### Reglas que se mantienen
- **El bot automático SOLO RESPONDE**: nunca escribe primero ni hace nada programado. Lo que Charly escribe a mano desde la app es un cliente normal: sale por `envio.Servicio.EnviarTexto/EnviarMedio…` (sin retardos, con tope de 30 seguidos y luego 1/s), se registra como "manual" en la actividad y pausa al bot en ese chat (30 min, ajustable). Los mensajes que Charly escribe desde el teléfono (`IsFromMe` cuyo ID no salió de la app) pausan igual; los de la app o el motor se reconocen por su ID (`EsEnvioPropio`).
- Todo local, sin Google, sin servidor. Formato de cifrado y llave de datos sin cambios (columnas nuevas con AAD propio: `cita_texto`, `menciones`, `borrador`, `menu_def`, `flow_var`, `aviso`). No se toca la identificación del dispositivo.

### Bloque A — eliminados e Info táctil
- Eliminar "para todos" NO toca contenido ni archivo (`MarcarEliminado`): solo `eliminado_en` y `eliminado_por` (JID de quien lo eliminó si no fue el autor = admin). Ya existía `borrado`: los marcados antes siguen mostrando su contenido, con "Eliminado" sin hora. Interruptor Privacidad → "Conservar mensajes eliminados por otros" (encendido): apagado, al llegar la eliminación se sobrescribe el contenido y se borra el archivo (excepto en mensajes míos). "Olvidar este mensaje" borra contenido y archivo para siempre. "Ver una vez" eliminado por el remitente nunca se conserva.
- `FiltroLista` + `ListarFiltrados` (solo columnas sin cifrar; hora y día de la semana se filtran en Go con la zona del teléfono) alimentan UNA pantalla (`ListaFiltrada.kt`) usada desde las filas de tipos, las barras de hora y de semana, el calendario y "Eliminados". "Ir al mensaje" abre el chat con `?ir=<id>` (carga páginas hasta encontrarlo y lo resalta).

### Bloque B — chat de texto (`internal/envio`, `ui/Compositor.kt`, `ChatVm`)
- Cita manual (`ContextInfo` con `Participant` en la forma del grupo: LID si el grupo direcciona por LID; el autor de un mensaje propio sale como mi LID/teléfono), menciones con selector de miembros (`GetGroupInfo`, caché 10 min), reacciones, edición (`whatsmeow.EditWindow` = 20 min; la UI oculta Editar pasado el plazo, pero si WhatsApp rechaza se muestra su error), borrar para todos (se intenta siempre; error de WhatsApp a la vista), borrar para mí, reenviar (texto y multimedia descargada; máx. 5 chats; re-subida del archivo).
- Palomitas: `message_receipts` (entregado/leído/reproducido por participante) llenada por `events.Receipt`; en grupos la palomita azul aparece con que al menos una persona lo haya leído (límite conocido; Info del mensaje da el detalle exacto).
- Presencia ("escribiendo…", "en línea", "últ. vez"): solo llega si la otra persona lo permite y si se pidió con `SubscribePresence`. **`Available` solo se manda con un chat abierto y con "Mostrar en línea" encendido (apagado por defecto)**: en el código de whatsmeow no hay documentación sobre notificaciones del teléfono, pero estar "en línea" con un dispositivo vinculado puede hacer que el teléfono principal no avise de mensajes mientras tanto. Sin ese interruptor tampoco llegan los "escribiendo…" de los demás.
- "Marcar como leído al abrir": usa `MarkRead`, que ya manda `read-self` (nadie se entera) si la cuenta tiene desactivadas las confirmaciones de lectura.
- Borradores cifrados en `chat_estado.borrador_cifrado`. Notificaciones de mensajes (`Notificador`, apagadas por defecto, canal propio, versión pública "Mensaje nuevo", respuesta rápida con `RemoteInput`).

### Bloque C — adjuntos, voz y ver una vez
- Subida con `UploadReader` en flujo (archivo temporal cifrado por WhatsApp en `<datos>/tmp`); la copia local queda cifrada con el formato BCF1 y la copia en claro se borra siempre. Fotos: reducidas a 1600 px y JPEG; videos/documentos tal cual (máx. 100 MB; fotos 20 MB). No hay barra de progreso real de subida (whatsmeow no la expone): barra indeterminada.
- Nota de voz: `MediaRecorder` OGG/Opus (Android 10+). En Android 8–9 sale AAC como audio normal (no "nota de voz"). Mantener pulsado el micrófono; deslizar a la izquierda cancela; permiso RECORD_AUDIO se pide la primera vez.
- Ver una vez: si llega con datos de descarga (`ImageMessage`/`VideoMessage`/`AudioMessage` con `ViewOnce`) se baja al llegar (sin importar la política de descarga; no en el historial), se guarda cifrado y sin miniatura, y se abre UNA vez en pantalla con `FLAG_SECURE` (`ver_estado`: pendiente → abierto → visto/expirado). Al cerrar, salir de la app o morir el proceso, se borra (`BarrerVerUnaVez` al abrir la app cierra los "abiertos" y caduca a los 14 días, ajustable). Acuse "reproducido" solo si la cuenta tiene confirmaciones de lectura activas. **En la práctica WhatsApp suele NO entregar el contenido a dispositivos vinculados**: whatsmeow emite `UndecryptableMessage` con tipo `view_once` y solo se guarda la tarjeta "ábrelo en tu teléfono". No se intenta sortear.

### Bloque D — motor (`internal/motor`)
- El manejador de eventos solo guarda y encola (`estado.trabajo`); el motor se alimenta desde ese trabajador. Flujo: dedupe persistente (`dedupe`, 7 días) → antigüedad (10 min, ajustable) y sincronización offline (flag desde `OfflineSyncPreview/Completed`, capturado al llegar el evento) → interruptor global → pausas (`chat_estado`) → grupos (menciona / cita mía / empieza con "." + letra; interruptor Activo/Pausado por grupo) → repetido (5 s) → carril FIFO por chat (máx. 10, descarta lo más viejo) → ráfagas privadas (1,5 s, tope 6 s; el decisor ve todo el lote) → cola de salida única (0,8–2,5 s + 25 ms/letra, "escribiendo…", 20/min global, 6/min por chat, freno 5 min tras 5 fallos seguidos con aviso).
- Cada respuesta cita el mensaje original; en grupos menciona a quien la provocó. `envio.EnviarAuto` no usa el tope manual ni pausa al bot.
- `actividad` registra qué llegó, qué coincidió y qué se respondió/omitió y por qué, sin textos ni nombres (chat como código de 6 hex). Pantalla en Avanzado → Actividad del bot.

### Bloque E — automatizaciones (`internal/flujo`)
- Definición JSON (`Def`: pasos menú/respuesta/pedir/humano/fin, disparadores, alcance, horario) guardada cifrada (`menus.def_cifrado`) con versiones (`menu_versiones`). Las conversaciones (`flow_sessions`) terminan con la versión con la que empezaron; si su paso ya no existe vuelven al menú principal; si borran la automatización se cierran.
- Estado por (chat, remitente): en grupos cada persona su menú. Expira a las 2 h (ajustable 30–360 min). "0" vuelve al principal; 2 inválidas seguidas reenvían el menú y una tercera corta el ciclo (abandonado y en silencio). `pedir` guarda el texto libre cifrado en `flow_vars`; `flow_events` y `menu_stats` nunca llevan texto libre.
- Orden de evaluación: por `orden`, con el Saludo de fábrica (`sistema=1`, "cualquier mensaje", una vez por minuto por persona, cita el mensaje) siempre al final. Se siembra una sola vez por bot.
- Pasar a humano: la persona queda en `flow_sessions.estado=humano` 12 h (el bot calla con ella); en chats privados además se pausa el chat (`bot_pausa_hasta`, `pausa_humano`). Reactivar desde el chat cierra ambas cosas. Los botones de "En curso ahora" no mandan mensajes.
- Avisos locales (`avisos`): "Alguien pidió hablar con una persona" / "Nueva solicitud" con detalle cifrado; Android: visibilidad privada + versión pública neutra; interruptor en Avanzado para mostrar el detalle bloqueado (apagado).
- Simulador: mismo motor, chat falso `prueba:simulador@botchat`, sesiones y estadísticas marcadas `prueba`, sin enviar ni notificar.

### Límites conocidos / pendientes
- Adjuntos en respuestas automáticas, moderación, respaldo, perfiles, búsqueda por texto, IA, etiquetas, fijados y archivados: fuera de alcance (0.5+).
- Sin pruebas con tráfico real de WhatsApp: acuses, presencia, reenvío, menciones con LID, voz y ver una vez requieren probarse en el teléfono.
- No hay reordenar automatizaciones desde la pantalla (la API `ReordenarAutomatizaciones` existe); el orden es el de creación.
- El editor es una lista indentada (sin lienzo). Las estadísticas por menú cuentan a partir de 0.4.0.

## Estado: 0.4.1 "Rendimiento al abrir un chat" (2026-11-10) — versionCode 8

Parche solo de rendimiento (sin funciones nuevas). Causa: `vista()` pedía el nombre de cada autor de cita, mencionado y "eliminado por" con
1–2 consultas por mensaje, y `nombreDe` recorría el chat entero si la persona no tenía nombre (≈1 s por página de 60 con 20,000 mensajes); además
cada cambio releía todo lo ya cargado.

- **Go**: `internal/almacen/nombres.go` resuelve los nombres por lote (una consulta por chat para contactos y una sentencia con un salto al índice por
  persona) y los guarda en una caché pequeña que se invalida en `escribirChat` (contacto, push name, negocio), al llegar un mensaje con nombre de esa
  persona, y al borrar/fusionar chats. Migración 8: índice parcial `idx_mensajes_remitente`. `MensajesPorID` (lote de hasta 300 ids) y eventos nuevos
  `msg:<jid>:<id>` (`avisaMsg`) en acuses, reacciones, ediciones, eliminados, estado de envío y ver una vez. `ChatSintetico` crea el chat de prueba.
  `CargaAntigua(true)` reproduce la ruta de la 0.4.0 (solo para medir).
- **Kotlin**: `ChatVm` abre con 40 mensajes (la caché `CachePaginas` pinta al instante lo último visto), la mezcla (`MezclaMensajes`, con tests) refresca
  la primera página y pide por id, en un solo lote, los mensajes viejos ya cargados que cambiaron. Los renglones (`ElementosChat.kt`) y el texto con
  formato (`CacheTexto`) se calculan fuera del hilo principal. Presencia, leído, participantes y extras esperan `ENTRADA_MS` (fin de la animación).
  Miniaturas: la miniatura al instante; la imagen nítida se pide tras 140 ms y a ≤720 px. LazyColumn con `contentType`, sin fade de entrada.
- **Medir**: Avanzado → Medir rendimiento → "Medir abrir chat" (chat sintético de 20,000 mensajes que se borra solo; 3 aperturas; ms hasta el primer
  contenido y hasta el completo; cuadros perdidos). Tocar 5 veces el título muestra "Medir con carga anterior (0.4.0)" para ver antes y después.
- **Dos APK, mismo app**: `debug` y `rendimiento` (no depurable, R8, `proguard-rules.pro`, `baseline-prof.txt` escrito a mano). Mismo applicationId, mismo
  versionCode y la misma llave (`~/.android/debug.keystore`, SHA-256 699002c7…40e0): se instalan uno encima del otro en ambos sentidos sin desinstalar.
  Compilar: `./gradlew :app:assembleDebug` / `:app:assembleRendimiento` (el APK sale en `app/build/outputs/apk/<variante>/`). Tests contra la variante:
  `./gradlew -PpruebasRendimiento :app:testRendimientoUnitTest` (la propiedad solo añade `ui-test-manifest` para Robolectric; el APK no la lleva).
  Si se agrega una librería que use reflexión o JNI, revisar `proguard-rules.pro` y comprobar con `apkanalyzer dex packages` que sus clases siguen ahí.

## Estado: 0.5.0 "Todo en uno" (2026-11-11) — versionCode 9
Por defecto el bot SOLO RESPONDE. Todo lo que toma la iniciativa vive en módulos opcionales (Ajustes → Módulos), apagados, con interruptor propio, aviso de riesgo, límites y registro. Detalle completo, choques y pruebas de teléfono por bloque en `PROGRESO.md`.

- **Bloques**: 0 transiciones/no leídos/"tomar el control" · A rangos, permisos, comandos y confirmaciones con código · H0 marco de módulos · B personas, silencios, advertencias, notas, etiquetas, avisos · C menús ampliados (formularios, grupos, plantillas, .bcm) · D mensajes automáticos con adjuntos/atajos "/x" · E grupos (con y sin admin), filtros y app state · F notas/pendientes y `!sticker` · G Laboratorio (encuestas/botones) · H1–H9 módulos (bienvenida, difusión, programados, invitar, descargas/conectores, traductor, IA, llamadas, juegos).
- **Migraciones** solo hacia adelante: 9–19 (`go/internal/almacen/migraciones.go`). Textos de usuario siempre cifrados.
- **Reglas de identidad**: no se simula Business, no se cambia el tipo de dispositivo, no se inventan nodos `<biz>`. La librería añade `<biz>` sola a ButtonsMessage/ListMessage (InteractiveMessage no); se usa solo por defecto y en el Laboratorio.
- **Dueño** = la cuenta del teléfono; sus comandos solo desde "Mensajes guardados" y la app. Los demás rangos usan comandos como respuestas en su chat, con permisos, límites y confirmación por código de un solo uso.
- **Llaves**: las de IA se guardan con el Android Keystore y pasan a Go solo en memoria; nunca en base, repo ni registros.
- **Compilar**: igual que 0.4.1; APK de entrega = variante `rendimiento` en `salida/BotChat-0.5.0.apk`, misma llave (SHA-256 699002c7…40e0), applicationId y versionCode 9: actualizar encima, NUNCA desinstalar.

### Cómo añadir un módulo nuevo
1. Crear `go/internal/modulos/<id>.go` y, desde `init()`/`registro_h.go`, `Registrar(Nuevo(Def{ID, Nombre, Descripcion, Icono, Riesgo, Iniciativa, Casillas, Esquema, Config, TopeHora/TopeDia, Comandos, AlActivar/AlDesactivar, AlEvento, AlEstado}))`; importarlo/registrarlo en `wabot` al arrancar.
2. `AlEvento(g, e)` (EvMensaje, EvMiembroEntra, EvLlamada, EvTick) solo anota y encola; nunca envía directo.
3. Config con `g.Config("id").Texto/Int/Bool/Num`; secretos con campo `secreto` del esquema (Keystore), no en config.
4. Enviar SOLO con `g.Enviar(modulos.Iniciativa{Modulo, Que, Chat, Personas, Texto|Medio, Noche})`: respeta No molestar, modo seguro, maestro "Iniciativa propia" y topes. La cola es en memoria: si el módulo tiene agenda, guardarla (tabla propia) y reencolar al arrancar.
5. UI: ventana genérica por esquema/paneles descritos en Go; o `PantallasModulo.registrar("id"){...}` en Kotlin para una propia.
6. Casillas de permiso en el catálogo (`permisos.Registrar`), texto cifrado, nada de textos en `modulos_registro`. Pruebas siguiendo `modulos_test.go`.
