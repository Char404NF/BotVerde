# Archivos de referencia del formato cifrado (Go ↔ Kotlin)

Los generó Go (`ACTUALIZAR_FIXTURES=1 go test ./internal/cifrado`) y los comprueban **las dos** pruebas:
`go/internal/cifrado/archivo_test.go` y `app/src/test/.../CifradoCruzadoTest.kt`.
Cada lado cifra los mismos datos con la misma llave y el mismo nonce y debe producir **exactamente los mismos bytes**,
y además descifrar el archivo del otro. Así queda probado en las dos direcciones.

- Llave: 32 bytes, `byte i = i` (0x00, 0x01, … 0x1f).
- Nonce base: 12 bytes, `byte i = 0xA0 + i`.
- Texto en claro de `n<N>.bcf`: N bytes con `byte i = (i * 31 + 7) mod 256`.
- Tamaños: 0, 1, 65535, 65536 (un bloque exacto), 65537, 131072 y 200000 bytes.

Esta llave y este nonce son solo de prueba. No se usan en la app.
