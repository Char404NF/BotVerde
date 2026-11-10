package com.botchat.app

import android.graphics.Paint
import com.google.i18n.phonenumbers.PhoneNumberUtil
import java.text.Collator
import java.text.Normalizer
import java.util.Locale

/** Un país de la lista: nombre en español, código ISO y código telefónico. */
data class Pais(val iso: String, val nombre: String, val codigo: Int, val bandera: String) {
    val codigoTexto: String get() = "+$codigo"
}

object Paises {
    private val acentos = Regex("\\p{Mn}+")
    private val es = Locale.forLanguageTag("es")

    /** Bandera como emoji a partir del código ISO (letras regionales). */
    fun emojiBandera(iso: String): String {
        val u = iso.uppercase()
        if (u.length != 2 || u.any { it !in 'A'..'Z' }) return ""
        val a = 0x1F1E6 + (u[0] - 'A')
        val b = 0x1F1E6 + (u[1] - 'A')
        return String(Character.toChars(a)) + String(Character.toChars(b))
    }

    private val banderaCache = java.util.concurrent.ConcurrentHashMap<String, String>()
    private val pincel by lazy { Paint() }

    /** Bandera de un país (emoji); si el teléfono no sabe dibujarla, las dos letras del país. Vacío si el código no es válido. */
    fun banderaDe(iso: String): String {
        if (iso.length != 2) return ""
        return banderaCache.getOrPut(iso.uppercase()) {
            val emoji = emojiBandera(iso)
            if (emoji.isNotEmpty() && runCatching { pincel.hasGlyph(emoji) }.getOrDefault(false)) emoji else iso.uppercase()
        }
    }

    fun normaliza(s: String): String =
        Normalizer.normalize(s, Normalizer.Form.NFD).replace(acentos, "").lowercase(es)

    /** Todos los países de libphonenumber, ordenados por nombre en español (con acentos y ñ). */
    val todos: List<Pais> by lazy {
        val util = PhoneNumberUtil.getInstance()
        val paint = Paint()
        val lista = util.supportedRegions.mapNotNull { iso ->
            val codigo = util.getCountryCodeForRegion(iso)
            val nombre = Locale.Builder().setRegion(iso).build().getDisplayCountry(es).trim()
            if (codigo <= 0 || nombre.isEmpty() || nombre.equals(iso, ignoreCase = true)) return@mapNotNull null
            val emoji = emojiBandera(iso)
            // Si el teléfono no sabe dibujar la bandera, se muestran las dos letras del país.
            val bandera = if (emoji.isNotEmpty() && paint.hasGlyph(emoji)) emoji else iso.uppercase()
            Pais(iso.uppercase(), nombre.replaceFirstChar { it.uppercase(es) }, codigo, bandera)
        }
        val collator = Collator.getInstance(es)
        lista.sortedWith { a, b -> collator.compare(a.nombre, b.nombre) }
    }

    fun porIso(iso: String): Pais = todos.firstOrNull { it.iso == iso } ?: todos.first { it.iso == "MX" }

    fun buscar(consulta: String): List<Pais> {
        val q = normaliza(consulta.trim())
        if (q.isEmpty()) return todos
        val digitos = q.filter { it.isDigit() }
        return todos.filter {
            normaliza(it.nombre).contains(q) ||
                (digitos.isNotEmpty() && it.codigo.toString().startsWith(digitos)) ||
                it.iso.lowercase() == q
        }
    }
}
