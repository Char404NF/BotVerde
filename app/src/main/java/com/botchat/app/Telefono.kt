package com.botchat.app

import com.google.i18n.phonenumbers.NumberParseException
import com.google.i18n.phonenumbers.PhoneNumberUtil
import com.google.i18n.phonenumbers.PhoneNumberUtil.PhoneNumberFormat
import com.google.i18n.phonenumbers.PhoneNumberUtil.ValidationResult

/**
 * Números de teléfono: formato mientras se escribe, validación de largo por país y número final
 * (código de país + número, solo dígitos) como lo pide la vinculación.
 */
object Telefono {
    private val util: PhoneNumberUtil by lazy { PhoneNumberUtil.getInstance() }

    fun soloDigitos(s: String): String = s.filter { it in '0'..'9' }

    /**
     * México: el número nacional tiene 10 dígitos. Si la persona escribe "52", "521", "044" o "045" por delante,
     * se quitan solos.
     */
    fun limpiaMx(d0: String): String {
        val d = soloDigitos(d0)
        return when {
            d.length == 13 && (d.startsWith("044") || d.startsWith("045")) -> d.drop(3)
            d.length == 13 && d.startsWith("521") -> d.drop(3)
            d.length == 12 && d.startsWith("52") -> d.drop(2)
            d.length == 11 && d.startsWith("1") -> d.drop(1)
            else -> d
        }
    }

    /** Lo que se guarda como número del país: para México ya sin prefijos. */
    fun nacional(iso: String, digitos: String): String =
        if (iso == "MX") limpiaMx(digitos).take(10) else soloDigitos(digitos).take(15)

    /** Máximo de dígitos que deja escribir el campo. */
    fun maxDigitos(iso: String): Int = if (iso == "MX") 13 else 15

    /** Da formato al número mientras se escribe (por ejemplo 55 1234 5678). */
    fun formatea(iso: String, digitos: String): String {
        val f = util.getAsYouTypeFormatter(iso)
        var out = ""
        for (c in nacional(iso, digitos)) out = f.inputDigit(c)
        return out
    }

    /** Devuelve el mensaje de error en español, o null si el largo es correcto. */
    fun valida(iso: String, digitos: String): String? {
        val d = nacional(iso, digitos)
        if (d.isEmpty()) return "Escribe tu número de teléfono."
        if (iso == "MX") {
            return when {
                d.length < 10 -> "El número es muy corto: faltan ${10 - d.length} dígitos."
                d[0] == '0' -> "El número no debe empezar con 0."
                else -> null
            }
        }
        val num = try {
            util.parse(d, iso)
        } catch (e: NumberParseException) {
            return "El número no es válido."
        }
        return when (util.isPossibleNumberWithReason(num)) {
            ValidationResult.IS_POSSIBLE -> null
            ValidationResult.IS_POSSIBLE_LOCAL_ONLY -> "Falta el código de área."
            ValidationResult.TOO_SHORT -> "El número es muy corto."
            ValidationResult.TOO_LONG -> "El número es muy largo."
            ValidationResult.INVALID_LENGTH -> "El número no tiene el largo correcto para este país."
            else -> "El número no es válido."
        }
    }

    /**
     * Número tal como lo pide la vinculación con código: código de país + número, solo dígitos,
     * sin "+" ni espacios. Ejemplo México: 525512345678. Con [mx1] se usa el formato antiguo 5215512345678.
     */
    fun paraVincular(iso: String, digitos: String, mx1: Boolean): String {
        val d = nacional(iso, digitos)
        if (iso == "MX") return (if (mx1) "521" else "52") + d
        return try {
            util.format(util.parse(d, iso), PhoneNumberFormat.E164).removePrefix("+")
        } catch (e: NumberParseException) {
            Paises.porIso(iso).codigo.toString() + d
        }
    }

    /** Para mostrar: +52 55 1234 5678. */
    fun internacional(iso: String, digitos: String): String {
        val d = nacional(iso, digitos)
        if (iso == "MX") return "+52 " + formatea("MX", d)
        return try {
            util.format(util.parse(d, iso), PhoneNumberFormat.INTERNATIONAL)
        } catch (e: NumberParseException) {
            "+" + Paises.porIso(iso).codigo + " " + d
        }
    }
}
