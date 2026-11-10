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
- Regenerar el .aar (tras cambiar Go): `cd ~/wabot-app/go && nice -n 10 gomobile bind -target=android/arm64 -androidapi 26 -o ../app/libs/wabot.aar ./wabot`
- Pruebas Kotlin: `cd ~/wabot-app && nice -n 10 ./gradlew testDebugUnitTest --no-daemon`
- APK: `cd ~/wabot-app && nice -n 10 ./gradlew assembleDebug --no-daemon` y copiar `app/build/outputs/apk/debug/app-debug.apk` a `salida/`.

### Decisiones y puntos a vigilar
- Número para vincular: `PairPhone` quita lo que no sea dígito, exige más de 6 dígitos y que no empiece con 0 (leído del código fuente). México se envía como `52` + 10 dígitos. Las fuentes públicas no coinciden sobre si WhatsApp quiere `521`: hay un interruptor "Formato antiguo de México (521…)" en Opciones avanzadas del paso 3. Si el teléfono dice que el código es incorrecto, probar con el interruptor activado. Otros países usan E.164 de libphonenumber (Argentina, +54 9…, no tiene tratamiento especial).
- `PairPhone` exige conectar primero y esperar el evento QR; el nombre del cliente debe ser "Navegador (Sistema)": se usa "Chrome (Linux)".
- Los mensajes acumulados con el bot apagado (más de 2 min) no se responden. Se ignoran grupos, estados, canales, propios y editados. Se espera 1–3 s antes de responder.
- Servicio `specialUse` (dataSync se corta a las 6 h en Android 15). Sin Firebase/Play Services: `./gradlew :app:dependencies` no muestra gms ni firebase y el dex no contiene `com/google/android/gms`.
- La sesión de WhatsApp (`filesDir/bots/<id>/sesion.db`) queda excluida de copias en la nube y de transferencia a otro equipo.
- El APK debug se firma con `~/.android/debug.keystore` de este servidor: para instalar actualizaciones encima hay que seguir usando esa misma llave.
