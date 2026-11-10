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
