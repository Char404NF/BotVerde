# Bot Chat

App Android nativa (Kotlin, Jetpack Compose y Material 3) con un bot de mensajería que **solo responde**: conecta un número secundario, contesta a quien le escribe y guarda los mensajes y archivos **cifrados en tu teléfono**.

## Aviso importante

- Es una **herramienta no oficial**. No está afiliada, respaldada ni patrocinada por WhatsApp ni por Meta.
- Se usa **bajo tu propia responsabilidad**. WhatsApp puede **bloquear la cuenta** que se use con herramientas como esta; usa un número secundario.
- No usa el nombre ni el logo de WhatsApp como marca.

## Privacidad

- **No recolecta datos ni usa servidores propios.** No hay cuentas, análisis ni publicidad.
- Todo se guarda solo en el teléfono: mensajes y archivos cifrados con AES-256-GCM; la llave vive en el Keystore de Android.
- Sin servicios de Google (ni Firebase ni Play Services).
- Bloqueo opcional con PIN o huella, y opción para evitar capturas de pantalla.

## Cómo compilar

Requisitos: JDK 17, Android SDK (plataforma 37 y build-tools), Go y `gomobile`.

```bash
# 1. Módulo Go → biblioteca para Android (genera app/libs/wabot.aar)
cd go
gomobile bind -target=android/arm64 -androidapi 26 -trimpath -ldflags="-s -w" -o ../app/libs/wabot.aar ./wabot
go test ./...

# 2. Pruebas y APK de depuración
cd ..
./gradlew testDebugUnitTest
./gradlew assembleDebug   # app/build/outputs/apk/debug/app-debug.apk
```

Solo arm64. El APK de depuración se firma con tu propia llave de depuración: para actualizar encima de una instalación previa hay que usar siempre la misma.

## Estructura

- `go/` módulo Go (conexión, almacén cifrado, ingesta, descargas).
- `app/` aplicación Android.
- `pruebas_cruzadas/` archivos de prueba cifrados que Go y Kotlin deben leer igual.
- `CLAUDE.md` notas de diseño, formato de cifrado y decisiones.

Sin licencia por ahora: todos los derechos reservados.
