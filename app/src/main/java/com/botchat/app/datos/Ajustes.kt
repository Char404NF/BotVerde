package com.botchat.app.datos

import android.content.Context
import android.content.SharedPreferences
import com.botchat.app.seguridad.ControlIntentos
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import org.json.JSONObject
import wabot.Wabot

/** Ajustes de la app (descargas, privacidad, bloqueo). Se guardan en el teléfono. */
class Ajustes private constructor(ctx: Context) {
    private val p: SharedPreferences = ctx.getSharedPreferences("ajustes", Context.MODE_PRIVATE)
    private val seg: SharedPreferences = ctx.getSharedPreferences("seguridad", Context.MODE_PRIVATE)

    val intentos = ControlIntentos({ seg.getString(it, null) }, { k, v -> seg.edit().apply { if (v == null) remove(k) else putString(k, v) }.apply() })

    private val _secure = MutableStateFlow(p.getBoolean("secure", true))
    /** Bloquea capturas de pantalla y la vista previa en apps recientes (activado por defecto). */
    val secure: StateFlow<Boolean> = _secure
    fun fijarSecure(v: Boolean) { p.edit().putBoolean("secure", v).apply(); _secure.value = v }

    private val _huella = MutableStateFlow(p.getBoolean("huella", false))
    val huella: StateFlow<Boolean> = _huella
    fun fijarHuella(v: Boolean) { p.edit().putBoolean("huella", v).apply(); _huella.value = v }

    private val _pin = MutableStateFlow(intentos.tienePin())
    val tienePin: StateFlow<Boolean> = _pin
    fun fijarPin(pin: String) { intentos.fijar(pin); _pin.value = true }
    fun quitarPin() { intentos.quitar(); fijarHuella(false); _pin.value = false }
    fun refrescarPin() { _pin.value = intentos.tienePin() }

    private val _omitioPin = MutableStateFlow(p.getBoolean("omitio_pin", false))
    val omitioPin: StateFlow<Boolean> = _omitioPin
    fun omitirPin() { p.edit().putBoolean("omitio_pin", true).apply(); _omitioPin.value = true }

    // ---- descargas automáticas: cada tipo, por Wi-Fi y por datos móviles ----
    data class Regla(val wifi: Boolean, val datos: Boolean)
    data class Descargas(val foto: Regla, val audio: Regla, val documento: Regla, val video: Regla, val videoMaxMb: Int, val pisoLibreMb: Int)

    private val _descargas = MutableStateFlow(leerDescargas())
    val descargas: StateFlow<Descargas> = _descargas

    private fun leerDescargas() = Descargas(
        foto = Regla(p.getBoolean("foto_wifi", true), p.getBoolean("foto_datos", true)),
        audio = Regla(p.getBoolean("audio_wifi", true), p.getBoolean("audio_datos", true)),
        documento = Regla(p.getBoolean("doc_wifi", true), p.getBoolean("doc_datos", true)),
        video = Regla(p.getBoolean("video_wifi", true), p.getBoolean("video_datos", false)), // videos con datos: apagado por defecto
        videoMaxMb = p.getInt("video_max_mb", 50),
        pisoLibreMb = p.getInt("piso_mb", 500),
    )

    fun cambiarDescargas(f: (Descargas) -> Descargas) {
        val d = f(_descargas.value)
        p.edit()
            .putBoolean("foto_wifi", d.foto.wifi).putBoolean("foto_datos", d.foto.datos)
            .putBoolean("audio_wifi", d.audio.wifi).putBoolean("audio_datos", d.audio.datos)
            .putBoolean("doc_wifi", d.documento.wifi).putBoolean("doc_datos", d.documento.datos)
            .putBoolean("video_wifi", d.video.wifi).putBoolean("video_datos", d.video.datos)
            .putInt("video_max_mb", d.videoMaxMb).putInt("piso_mb", d.pisoLibreMb).apply()
        _descargas.value = d
        aplicarAGo()
    }

    /** Pasa las reglas al módulo Go (se llama al iniciar el servicio y cada vez que cambian). */
    fun aplicarAGo() {
        runCatching { Wabot.configurarDescargas(jsonDescargas(_descargas.value)) }
    }

    companion object {
        private const val MB = 1024L * 1024L

        fun jsonDescargas(d: Descargas): String {
            fun r(x: Regla) = JSONObject().put("wifi", x.wifi).put("datos", x.datos)
            return JSONObject().put("foto", r(d.foto)).put("audio", r(d.audio)).put("documento", r(d.documento)).put("video", r(d.video))
                .put("videoMaxBytes", d.videoMaxMb * MB).put("pisoLibre", d.pisoLibreMb * MB).toString()
        }

        @Volatile private var inst: Ajustes? = null

        /** Solo para pruebas: olvida la instancia para empezar con ajustes limpios. */
        fun reiniciarParaPruebas() { inst = null }

        fun get(ctx: Context): Ajustes = inst ?: synchronized(this) { inst ?: Ajustes(ctx.applicationContext).also { inst = it } }
    }
}
