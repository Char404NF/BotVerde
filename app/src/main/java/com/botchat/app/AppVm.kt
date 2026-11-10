package com.botchat.app

import android.app.Application
import android.content.Context
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.lifecycle.AndroidViewModel
import androidx.lifecycle.viewModelScope
import kotlinx.coroutines.launch

enum class Pantalla { NOMBRE, NUMERO, PAIS, CONFIRMAR, CODIGO, PRINCIPAL, AYUDA }

class AppVm(app: Application) : AndroidViewModel(app) {
    private val datos = Datos.get(app)

    var bot by mutableStateOf<Bot?>(null); private set
    var pantalla by mutableStateOf(Pantalla.NOMBRE); private set
    var nombre by mutableStateOf(""); private set
    var iso by mutableStateOf("MX"); private set
    var digitos by mutableStateOf(""); private set
    var mx1 by mutableStateOf(false)
    var errorNombre by mutableStateOf<String?>(null); private set
    var errorNumero by mutableStateOf<String?>(null); private set
    var eliminando by mutableStateOf(false); private set
    var aviso by mutableStateOf<String?>(null)

    init {
        recarga()
        viewModelScope.launch { BotState.estado.collect { if (it == "vinculado") alVincular() } }
        viewModelScope.launch { BotState.version.collect { if (it > 0) recarga() } }
        viewModelScope.launch {
            BotState.aviso.collect { if (it != null) { aviso = it; BotState.aviso.value = null } }
        }
    }

    /** Lee el bot guardado y decide qué pantalla mostrar. */
    private fun recarga() {
        eliminando = false
        bot = datos.primero()
        val b = bot
        if (b == null) {
            nombre = ""; digitos = ""; iso = "MX"; mx1 = false
            pantalla = Pantalla.NOMBRE
        } else {
            nombre = b.nombre; iso = b.iso; digitos = b.numero; mx1 = b.mx1
            pantalla = if (b.vinculado) Pantalla.PRINCIPAL else Pantalla.CONFIRMAR
        }
    }

    private fun alVincular() {
        if (pantalla == Pantalla.CODIGO || pantalla == Pantalla.CONFIRMAR) {
            bot = datos.primero()
            pantalla = Pantalla.PRINCIPAL
        }
    }

    // ---------- paso 1 ----------
    fun cambiaNombre(t: String) { nombre = t.take(30); errorNombre = null }

    fun siguienteNombre() {
        val n = nombre.trim()
        errorNombre = when {
            n.length < 2 -> "El nombre es muy corto: escribe al menos 2 letras."
            n.length > 30 -> "El nombre es muy largo: máximo 30 letras."
            else -> null
        }
        if (errorNombre == null) { nombre = n; pantalla = Pantalla.NUMERO }
    }

    // ---------- paso 2 ----------
    fun cambiaDigitos(t: String) {
        digitos = Telefono.nacional(iso, Telefono.soloDigitos(t).take(Telefono.maxDigitos(iso)))
        errorNumero = null
    }

    fun abrePaises() { pantalla = Pantalla.PAIS }

    fun eligePais(p: Pais) {
        iso = p.iso
        digitos = Telefono.nacional(iso, digitos)
        errorNumero = null
        pantalla = Pantalla.NUMERO
    }

    fun siguienteNumero() {
        errorNumero = Telefono.valida(iso, digitos)
        if (errorNumero == null) pantalla = Pantalla.CONFIRMAR
    }

    // ---------- paso 3 ----------
    fun resumenNumero(): String = Telefono.internacional(iso, digitos)

    /** "Sí, vincular": guarda el bot (si no existía) y pide el código. */
    fun vincular(ctx: Context) {
        var b = bot
        if (b != null && b.mx1 != mx1) { // cambió el formato de México: se rehace el registro
            datos.eliminar(b.id); b = null
        }
        if (b == null) {
            val id = datos.crear(
                nombre.trim(), iso, Paises.porIso(iso).codigo, Telefono.nacional(iso, digitos),
                Telefono.paraVincular(iso, digitos, mx1), mx1,
            )
            if (id < 0) return
            b = datos.porId(id) ?: return
        }
        bot = b
        BotState.codigo.value = null
        pantalla = Pantalla.CODIGO
        BotService.manda(ctx, BotService.ACC_VINCULAR, b.id)
    }

    /** "Corregir": vuelve al paso del número; si había un bot sin vincular guardado, se descarta. */
    fun corregir(ctx: Context) {
        val b = bot
        if (b != null && !b.vinculado) {
            BotService.manda(ctx, BotService.ACC_DETENER, b.id)
            datos.eliminar(b.id)
            runCatching { datos.carpetaSesion(b.id).deleteRecursively() }
            bot = null
        }
        pantalla = Pantalla.NUMERO
    }

    fun atras(ctx: Context) {
        when (pantalla) {
            Pantalla.NUMERO -> pantalla = Pantalla.NOMBRE
            Pantalla.PAIS -> pantalla = Pantalla.NUMERO
            Pantalla.CONFIRMAR -> corregir(ctx)
            Pantalla.CODIGO -> {
                bot?.let { BotService.manda(ctx, BotService.ACC_DETENER, it.id) }
                pantalla = Pantalla.CONFIRMAR
            }
            Pantalla.AYUDA -> pantalla = Pantalla.PRINCIPAL
            else -> {}
        }
    }

    val puedeVolver: Boolean get() = pantalla !in listOf(Pantalla.NOMBRE, Pantalla.PRINCIPAL)

    // ---------- pantalla principal ----------
    fun abreAyuda() { pantalla = Pantalla.AYUDA }

    fun iniciaODetiene(ctx: Context) {
        val b = bot ?: return
        val accion = if (BotState.corriendo.value) BotService.ACC_DETENER else BotService.ACC_INICIAR
        BotService.manda(ctx, accion, b.id)
    }

    fun eliminar(ctx: Context) {
        val b = bot ?: return
        eliminando = true
        BotService.manda(ctx, BotService.ACC_ELIMINAR, b.id)
    }
}
