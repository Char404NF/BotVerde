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

    /**
     * Animaciones de pantallas (Avanzado): Natural, Reducidas (solo fundidos cortos) o Ninguna. Reemplaza al antiguo interruptor
     * "reducir_anim" de la 0.4.x: quien lo tenía encendido pasa a Reducidas (la migración se hace una sola vez al leer).
     */
    private fun animInicial(): com.botchat.app.ui.ModoAnim {
        p.getString("anim_modo", null)?.let { n -> runCatching { com.botchat.app.ui.ModoAnim.valueOf(n) }.getOrNull()?.let { return it } }
        val m = if (p.getBoolean("reducir_anim", false)) com.botchat.app.ui.ModoAnim.REDUCIDAS else com.botchat.app.ui.ModoAnim.NATURAL
        p.edit().putString("anim_modo", m.name).remove("reducir_anim").apply()
        return m
    }

    private val _animaciones = MutableStateFlow(animInicial())
    val animaciones: StateFlow<com.botchat.app.ui.ModoAnim> = _animaciones
    fun fijarAnimaciones(m: com.botchat.app.ui.ModoAnim) { p.edit().putString("anim_modo", m.name).apply(); _animaciones.value = m }

    // ---- funciones que se pueden apagar sin reinstalar (Avanzado → Funciones) ----
    private val _conservar = MutableStateFlow(p.getBoolean("conservar_elim", true))
    /** Conservar el contenido de los mensajes que otros eliminan (encendido por defecto). */
    val conservarEliminados: StateFlow<Boolean> = _conservar
    fun fijarConservarEliminados(v: Boolean) { p.edit().putBoolean("conservar_elim", v).apply(); _conservar.value = v; aplicarAGo() }

    private val _motorAuto = MutableStateFlow(p.getBoolean("motor_auto", true))
    /** Motor de automatización (respuestas automáticas) general. */
    val motorAutomatico: StateFlow<Boolean> = _motorAuto
    fun fijarMotorAutomatico(v: Boolean) { p.edit().putBoolean("motor_auto", v).apply(); _motorAuto.value = v; aplicarAGo() }

    private val _estiloLetra = MutableStateFlow(p.getBoolean("estilo_letra", false))
    /** Mostrar “Aa Estilo de letra” en los editores de mensajes (Avanzado; apagado por defecto). */
    val estiloLetra: StateFlow<Boolean> = _estiloLetra
    fun fijarEstiloLetra(v: Boolean) { p.edit().putBoolean("estilo_letra", v).apply(); _estiloLetra.value = v }

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

    private val _pausaBot = MutableStateFlow(p.getInt("pausa_bot_min", 0))
    /**
     * Minutos que el bot calla en un chat después de que escribo yo (a mano o desde el teléfono). Por defecto 0 (ninguno): si ya contesté
     * yo, solo se cancela la respuesta pendiente de ese mensaje. Opción oculta en Avanzado.
     */
    val pausaBotMin: StateFlow<Int> = _pausaBot
    fun fijarPausaBotMin(v: Int) { p.edit().putInt("pausa_bot_min", v).apply(); _pausaBot.value = v; aplicarAGo() }

    private val _firmaActiva = MutableStateFlow(p.getBoolean("firma_activa", false))
    /** Agregar una firma al final de las respuestas automáticas del bot (apagado por defecto; nunca se agrega a lo que escribo yo). */
    val firmaActiva: StateFlow<Boolean> = _firmaActiva
    fun fijarFirmaActiva(v: Boolean) { p.edit().putBoolean("firma_activa", v).apply(); _firmaActiva.value = v; aplicarAGo() }

    private val _firmaTexto = MutableStateFlow(p.getString("firma_texto", "") ?: "")
    val firmaTexto: StateFlow<String> = _firmaTexto
    fun fijarFirmaTexto(v: String) { p.edit().putString("firma_texto", v).apply(); _firmaTexto.value = v; aplicarAGo() }

    private val _antiguedad = MutableStateFlow(p.getInt("antiguedad_min", 10))
    /** Minutos: lo que llegó hace más que esto no se responde. */
    val antiguedadMin: StateFlow<Int> = _antiguedad
    fun fijarAntiguedadMin(v: Int) { p.edit().putInt("antiguedad_min", v).apply(); _antiguedad.value = v; aplicarAGo() }

    private val _expiraFlujo = MutableStateFlow(p.getInt("expira_flujo_min", 120))
    /** Minutos sin actividad para que una conversación dentro de un menú caduque. */
    val expiraFlujoMin: StateFlow<Int> = _expiraFlujo
    fun fijarExpiraFlujoMin(v: Int) { p.edit().putInt("expira_flujo_min", v).apply(); _expiraFlujo.value = v; aplicarAGo() }

    private val _detalleBloqueado = MutableStateFlow(p.getBoolean("detalle_bloqueado", false))
    /** Mostrar el detalle completo de los avisos de "hablar con una persona" también con el teléfono bloqueado (apagado por defecto). */
    val detalleBloqueado: StateFlow<Boolean> = _detalleBloqueado
    fun fijarDetalleBloqueado(v: Boolean) { p.edit().putBoolean("detalle_bloqueado", v).apply(); _detalleBloqueado.value = v }

    private val _verDias = MutableStateFlow(p.getInt("ver_una_vez_dias", 14))
    /** Días que un "ver una vez" espera a que lo abras antes de borrarse. */
    val verUnaVezDias: StateFlow<Int> = _verDias
    fun fijarVerUnaVezDias(v: Int) { p.edit().putInt("ver_una_vez_dias", v).apply(); _verDias.value = v; aplicarAGo() }

    private val _enLinea = MutableStateFlow(p.getBoolean("mostrar_en_linea", false))
    /** Mostrarse "en línea" mientras hay un chat abierto (apagado por defecto: puede dejar sin avisos al teléfono principal). */
    val mostrarEnLinea: StateFlow<Boolean> = _enLinea
    fun fijarMostrarEnLinea(v: Boolean) { p.edit().putBoolean("mostrar_en_linea", v).apply(); _enLinea.value = v }

    // ---- modo Simple / Completo (0.6.0) ----
    // Instalación nueva: Simple. Quien actualiza (ya había aceptado el aviso) empieza en Simple pero se revisan sus módulos: si tiene
    // alguno encendido pasa a Completo, y a todos se les muestra un aviso único con atajo a Completo.
    init {
        if (!p.contains("modo_completo")) {
            val actualiza = p.getBoolean("aviso_aceptado", false)
            p.edit().putBoolean("modo_completo", false).putBoolean("modo_aviso_pendiente", actualiza).putBoolean("modo_revisar_modulos", actualiza).apply()
        }
    }

    private val _completo = MutableStateFlow(p.getBoolean("modo_completo", false))
    /** true = modo Completo (todo a la vista); false = Simple (se ocultan comandos, módulos y límites). No borra nada. */
    val modoCompleto: StateFlow<Boolean> = _completo
    fun fijarModoCompleto(v: Boolean) { p.edit().putBoolean("modo_completo", v).apply(); _completo.value = v }

    private val _avisoModo = MutableStateFlow(p.getBoolean("modo_aviso_pendiente", false))
    /** Falta mostrar el aviso único "la app ahora tiene modo Simple" (solo quien actualizó). */
    val avisoModoPendiente: StateFlow<Boolean> = _avisoModo
    fun avisoModoVisto() { p.edit().putBoolean("modo_aviso_pendiente", false).apply(); _avisoModo.value = false }

    /** true si todavía hay que mirar si el usuario tiene módulos encendidos (una sola vez, al actualizar). */
    fun revisarModulosPendiente(): Boolean = p.getBoolean("modo_revisar_modulos", false)
    fun modulosRevisados() { p.edit().putBoolean("modo_revisar_modulos", false).apply() }

    // ---- silencios en grupos: lo que viene elegido al abrir «Silenciar» (0.6.0, E8) ----
    private val _silTiempo = MutableStateFlow(p.getInt("silencio_tiempo_def", 1440))
    /** Minutos del silencio de entrada (0 = siempre). */
    val silencioTiempoDefecto: StateFlow<Int> = _silTiempo
    fun fijarSilencioTiempoDefecto(min: Int) { p.edit().putInt("silencio_tiempo_def", min).apply(); _silTiempo.value = min }
    private val _silAlcance = MutableStateFlow(p.getString("silencio_alcance_def", "todos") ?: "todos")
    /** «uno» (un grupo) o «todos» (todos los que administro). */
    val silencioAlcanceDefecto: StateFlow<String> = _silAlcance
    fun fijarSilencioAlcanceDefecto(a: String) { p.edit().putString("silencio_alcance_def", a).apply(); _silAlcance.value = a }

    // ---- aviso de tope de almacenamiento al actualizar (0.6.0, E8): se muestra una vez si el uso ya pasa el tope ----
    private val _topeAviso = MutableStateFlow(p.getBoolean("tope_aviso_visto", false))
    val topeAvisoVisto: StateFlow<Boolean> = _topeAviso
    fun topeAvisoMarcado() { p.edit().putBoolean("tope_aviso_visto", true).apply(); _topeAviso.value = true }

    // ---- términos y condiciones (se aceptan una sola vez, al empezar) ----
    // La clave «aviso_aceptado» es la del aviso anterior: sigue marcándose para saber que la app ya se usó. Los términos nuevos llevan
    // su propia clave, así quien actualiza los ve y acepta una vez.
    private val _aviso = MutableStateFlow(p.getBoolean("terminos_v1", false))
    val avisoAceptado: StateFlow<Boolean> = _aviso
    fun aceptarAviso() { p.edit().putBoolean("terminos_v1", true).putBoolean("aviso_aceptado", true).apply(); _aviso.value = true }

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
    data class Descargas(
        val foto: Regla, val audio: Regla, val documento: Regla, val video: Regla, val videoMaxMb: Int, val pisoLibreMb: Int,
        // 0.6.0 (E8): sticker propio, interruptor por tipo, topes por tipo (en MB; 0 = sin tope), Wi-Fi para lo grande y batería baja.
        val sticker: Regla = Regla(true, true),
        val apagados: Set<String> = emptySet(),
        val topeArchivoMb: Map<String, Int> = emptyMap(),
        val topeTipoMb: Map<String, Int> = emptyMap(),
        val soloWifiGrandes: Boolean = true,
        val pausaBateria: Boolean = true,
        val bateriaMin: Int = 15,
    )

    private val _descargas = MutableStateFlow(leerDescargas())
    val descargas: StateFlow<Descargas> = _descargas

    private fun leerDescargas() = Descargas(
        foto = Regla(p.getBoolean("foto_wifi", true), p.getBoolean("foto_datos", true)),
        audio = Regla(p.getBoolean("audio_wifi", true), p.getBoolean("audio_datos", true)),
        documento = Regla(p.getBoolean("doc_wifi", true), p.getBoolean("doc_datos", true)),
        video = Regla(p.getBoolean("video_wifi", true), p.getBoolean("video_datos", false)), // videos con datos: apagado por defecto
        videoMaxMb = p.getInt("video_max_mb2", 0),
        pisoLibreMb = p.getInt("piso_mb", 500),
        sticker = Regla(p.getBoolean("sticker_wifi", true), p.getBoolean("sticker_datos", true)),
        apagados = TIPOS_DESCARGA.filter { p.getBoolean("apagado_$it", false) }.toSet(),
        topeArchivoMb = TIPOS_DESCARGA.associateWith { p.getInt("tope_archivo_mb_$it", 0) }.filterValues { it > 0 },
        topeTipoMb = TIPOS_DESCARGA.associateWith { p.getInt("tope_tipo_mb_$it", 0) }.filterValues { it > 0 },
        soloWifiGrandes = p.getBoolean("solo_wifi_grandes", true),
        pausaBateria = p.getBoolean("pausa_bateria", true),
        bateriaMin = p.getInt("bateria_min", 15),
    )

    fun cambiarDescargas(f: (Descargas) -> Descargas) {
        val d = f(_descargas.value)
        p.edit()
            .putBoolean("foto_wifi", d.foto.wifi).putBoolean("foto_datos", d.foto.datos)
            .putBoolean("audio_wifi", d.audio.wifi).putBoolean("audio_datos", d.audio.datos)
            .putBoolean("doc_wifi", d.documento.wifi).putBoolean("doc_datos", d.documento.datos)
            .putBoolean("video_wifi", d.video.wifi).putBoolean("video_datos", d.video.datos)
            .putInt("video_max_mb2", d.videoMaxMb).putInt("piso_mb", d.pisoLibreMb)
            .putBoolean("sticker_wifi", d.sticker.wifi).putBoolean("sticker_datos", d.sticker.datos)
            .putBoolean("solo_wifi_grandes", d.soloWifiGrandes).putBoolean("pausa_bateria", d.pausaBateria).putInt("bateria_min", d.bateriaMin)
            .apply {
                TIPOS_DESCARGA.forEach { t ->
                    putBoolean("apagado_$t", t in d.apagados)
                    putInt("tope_archivo_mb_$t", d.topeArchivoMb[t] ?: 0)
                    putInt("tope_tipo_mb_$t", d.topeTipoMb[t] ?: 0)
                }
            }.apply()
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
        runCatching { Wabot.configurarFlujo(_expiraFlujo.value.toLong()) }
        runCatching { Wabot.configurarFirma(if (_firmaActiva.value) _firmaTexto.value.trim() else "") }
    }

    companion object {
        private const val MB = 1024L * 1024L

        /** Las claves de tipo de la política de descarga de Go. */
        val TIPOS_DESCARGA = listOf("foto", "video", "audio", "documento", "sticker")

        fun jsonDescargas(d: Descargas): String {
            fun r(x: Regla, tipo: String) = JSONObject().put("wifi", x.wifi).put("datos", x.datos).put("apagada", tipo in d.apagados)
            fun mapa(m: Map<String, Int>) = JSONObject().also { o -> TIPOS_DESCARGA.forEach { t -> o.put(t, (m[t] ?: 0) * MB) } }
            return JSONObject().put("foto", r(d.foto, "foto")).put("audio", r(d.audio, "audio")).put("documento", r(d.documento, "documento")).put("video", r(d.video, "video"))
                .put("sticker", r(d.sticker, "sticker"))
                .put("videoMaxBytes", d.videoMaxMb * MB).put("pisoLibre", d.pisoLibreMb * MB)
                .put("topeArchivo", mapa(d.topeArchivoMb)).put("topeTipoTotal", mapa(d.topeTipoMb))
                .put("soloWifiGrandes", d.soloWifiGrandes).put("pausaBateria", d.pausaBateria).put("bateriaMin", d.bateriaMin).toString()
        }

        @Volatile private var inst: Ajustes? = null

        /** Solo para pruebas: olvida la instancia para empezar con ajustes limpios. */
        fun reiniciarParaPruebas() { inst = null }

        fun get(ctx: Context): Ajustes = inst ?: synchronized(this) { inst ?: Ajustes(ctx.applicationContext).also { inst = it } }
    }
}
