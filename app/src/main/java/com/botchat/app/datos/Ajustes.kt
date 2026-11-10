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

    // ---- bloqueo opcional: por defecto la app NO pide PIN ni huella ----
    enum class Bloqueo { NINGUNO, PIN, HUELLA }

    /** Quien venía de la 0.2.0 con PIN (y huella) conserva su bloqueo; los demás empiezan sin bloqueo. */
    private fun bloqueoInicial(): Bloqueo = when (p.getString("bloqueo", null)) {
        "pin" -> Bloqueo.PIN
        "huella" -> Bloqueo.HUELLA
        "ninguno" -> Bloqueo.NINGUNO
        else -> if (!intentos.tienePin()) Bloqueo.NINGUNO else if (p.getBoolean("huella", false)) Bloqueo.HUELLA else Bloqueo.PIN
    }

    private val _bloqueo = MutableStateFlow(bloqueoInicial())
    val bloqueo: StateFlow<Bloqueo> = _bloqueo
    fun fijarBloqueo(b: Bloqueo) {
        p.edit().putString("bloqueo", b.name.lowercase()).putBoolean("huella", b == Bloqueo.HUELLA).apply()
        _bloqueo.value = b
        _huella.value = b == Bloqueo.HUELLA
    }

    private val _huella = MutableStateFlow(_bloqueo.value == Bloqueo.HUELLA)
    val huella: StateFlow<Boolean> = _huella

    private val _bloqueoSeg = MutableStateFlow(p.getInt("bloqueo_seg", 60))
    /** Segundos fuera de la app antes de volver a pedir el desbloqueo: 0 (inmediato), 60 o 300. */
    val bloqueoSeg: StateFlow<Int> = _bloqueoSeg
    fun fijarBloqueoSeg(s: Int) { p.edit().putInt("bloqueo_seg", s).apply(); _bloqueoSeg.value = s }

    /** Quita el bloqueo sin tocar nada más: los mensajes y su llave no dependen del PIN. */
    fun quitarBloqueo() { intentos.quitar(); _pin.value = false; fijarBloqueo(Bloqueo.NINGUNO) }

    private val _pin = MutableStateFlow(intentos.tienePin())
    val tienePin: StateFlow<Boolean> = _pin
    fun fijarPin(pin: String) { intentos.fijar(pin); _pin.value = true }
    fun refrescarPin() { _pin.value = intentos.tienePin() }

    private val _secure = MutableStateFlow(p.getBoolean("secure", false))
    /** Bloquea capturas de pantalla y la vista previa en apps recientes (apagado por defecto). */
    val secure: StateFlow<Boolean> = _secure
    fun fijarSecure(v: Boolean) { p.edit().putBoolean("secure", v).apply(); _secure.value = v }

    // ---- aspecto ----
    private val _modoTema = MutableStateFlow(runCatching { com.botchat.app.ui.ModoTema.valueOf(p.getString("tema", "SISTEMA")!!) }.getOrDefault(com.botchat.app.ui.ModoTema.SISTEMA))
    val modoTema: StateFlow<com.botchat.app.ui.ModoTema> = _modoTema
    fun fijarModoTema(m: com.botchat.app.ui.ModoTema) { p.edit().putString("tema", m.name).apply(); _modoTema.value = m }

    private val _dinamico = MutableStateFlow(p.getBoolean("color_dinamico", false))
    val colorDinamico: StateFlow<Boolean> = _dinamico
    fun fijarColorDinamico(v: Boolean) { p.edit().putBoolean("color_dinamico", v).apply(); _dinamico.value = v }

    private val _reducir = MutableStateFlow(p.getBoolean("reducir_anim", false))
    val reducirAnimaciones: StateFlow<Boolean> = _reducir
    fun fijarReducirAnimaciones(v: Boolean) { p.edit().putBoolean("reducir_anim", v).apply(); _reducir.value = v }

    // ---- funciones que se pueden apagar sin reinstalar (Avanzado → Funciones) ----
    private val _conservar = MutableStateFlow(p.getBoolean("conservar_elim", true))
    /** Conservar el contenido de los mensajes que otros eliminan (encendido por defecto). */
    val conservarEliminados: StateFlow<Boolean> = _conservar
    fun fijarConservarEliminados(v: Boolean) { p.edit().putBoolean("conservar_elim", v).apply(); _conservar.value = v; aplicarAGo() }

    private val _motorAuto = MutableStateFlow(p.getBoolean("motor_auto", true))
    /** Motor de automatización (respuestas automáticas) general. */
    val motorAutomatico: StateFlow<Boolean> = _motorAuto
    fun fijarMotorAutomatico(v: Boolean) { p.edit().putBoolean("motor_auto", v).apply(); _motorAuto.value = v; aplicarAGo() }

    private val _escribir = MutableStateFlow(p.getBoolean("escribir_app", true))
    /** Poder escribir mensajes a mano desde la app. */
    val escribirDesdeApp: StateFlow<Boolean> = _escribir
    fun fijarEscribirDesdeApp(v: Boolean) { p.edit().putBoolean("escribir_app", v).apply(); _escribir.value = v }

    private val _notifs = MutableStateFlow(p.getBoolean("notif_mensajes", false))
    /** Notificaciones de mensajes nuevos (apagado por defecto). */
    val notificacionesMensajes: StateFlow<Boolean> = _notifs
    fun fijarNotificacionesMensajes(v: Boolean) { p.edit().putBoolean("notif_mensajes", v).apply(); _notifs.value = v }

    private val _leido = MutableStateFlow(p.getBoolean("marcar_leido", true))
    /** Marcar como leído al abrir un chat (respeta la configuración de confirmaciones de lectura de la cuenta). */
    val marcarLeidoAlAbrir: StateFlow<Boolean> = _leido
    fun fijarMarcarLeidoAlAbrir(v: Boolean) { p.edit().putBoolean("marcar_leido", v).apply(); _leido.value = v }

    private val _pausaBot = MutableStateFlow(p.getInt("pausa_bot_min", 30))
    /** Minutos que el bot calla en un chat después de que escribo yo (a mano o desde el teléfono). */
    val pausaBotMin: StateFlow<Int> = _pausaBot
    fun fijarPausaBotMin(v: Int) { p.edit().putInt("pausa_bot_min", v).apply(); _pausaBot.value = v; aplicarAGo() }

    private val _antiguedad = MutableStateFlow(p.getInt("antiguedad_min", 10))
    /** Minutos: lo que llegó hace más que esto no se responde. */
    val antiguedadMin: StateFlow<Int> = _antiguedad
    fun fijarAntiguedadMin(v: Int) { p.edit().putInt("antiguedad_min", v).apply(); _antiguedad.value = v; aplicarAGo() }

    private val _verDias = MutableStateFlow(p.getInt("ver_una_vez_dias", 14))
    /** Días que un "ver una vez" espera a que lo abras antes de borrarse. */
    val verUnaVezDias: StateFlow<Int> = _verDias
    fun fijarVerUnaVezDias(v: Int) { p.edit().putInt("ver_una_vez_dias", v).apply(); _verDias.value = v; aplicarAGo() }

    private val _enLinea = MutableStateFlow(p.getBoolean("mostrar_en_linea", false))
    /** Mostrarse "en línea" mientras hay un chat abierto (apagado por defecto: puede dejar sin avisos al teléfono principal). */
    val mostrarEnLinea: StateFlow<Boolean> = _enLinea
    fun fijarMostrarEnLinea(v: Boolean) { p.edit().putBoolean("mostrar_en_linea", v).apply(); _enLinea.value = v }

    // ---- aviso de herramienta no oficial (se acepta una sola vez) ----
    private val _aviso = MutableStateFlow(p.getBoolean("aviso_aceptado", false))
    val avisoAceptado: StateFlow<Boolean> = _aviso
    fun aceptarAviso() { p.edit().putBoolean("aviso_aceptado", true).apply(); _aviso.value = true }

    // ---- "mensajes nuevos": hasta cuándo vio cada chat (solo en este teléfono; no toca la base de mensajes) ----
    private val vistos: SharedPreferences = ctx.getSharedPreferences("vistos", Context.MODE_PRIVATE)

    /** Momento (segundos) desde el que todo cuenta como ya visto: al actualizar no aparecen contadores viejos. */
    fun baseVistos(): Long {
        val b = vistos.getLong("_base", 0L)
        if (b > 0) return b
        val ahora = System.currentTimeMillis() / 1000
        vistos.edit().putLong("_base", ahora).apply()
        return ahora
    }
    fun visto(jid: String): Long = maxOf(vistos.getLong(jid, 0L), baseVistos())
    fun marcarVisto(jid: String, ts: Long = System.currentTimeMillis() / 1000) { vistos.edit().putLong(jid, ts).apply() }

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
        runCatching { Wabot.conservarEliminados(_conservar.value) }
        runCatching { Wabot.configurarEnvio(_pausaBot.value.toLong()) }
        runCatching { Wabot.configurarVerUnaVez(_verDias.value.toLong()) }
        runCatching { Wabot.configurarMotor(_motorAuto.value, _antiguedad.value.toLong()) }
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
