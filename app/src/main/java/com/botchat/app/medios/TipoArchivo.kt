package com.botchat.app.medios

/** Lo que se sabe de un documento por su nombre y su tipo (MIME), para mostrar una tarjeta clara de cualquier archivo. */
object TipoArchivo {
    /** Extensión en minúsculas, sin punto; vacía si no hay. */
    fun extension(nombre: String): String {
        val i = nombre.trim().lastIndexOf('.')
        return if (i in 1 until nombre.trim().length - 1) nombre.trim().substring(i + 1).lowercase().filter { it.isLetterOrDigit() }.take(8) else ""
    }

    private val PORMIME = mapOf(
        "application/pdf" to "pdf", "text/plain" to "txt", "text/csv" to "csv", "text/html" to "html", "application/zip" to "zip",
        "application/x-rar-compressed" to "rar", "application/vnd.rar" to "rar", "application/x-7z-compressed" to "7z", "application/json" to "json",
        "application/msword" to "doc", "application/vnd.ms-excel" to "xls", "application/vnd.ms-powerpoint" to "ppt",
        "application/vnd.openxmlformats-officedocument.wordprocessingml.document" to "docx",
        "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet" to "xlsx",
        "application/vnd.openxmlformats-officedocument.presentationml.presentation" to "pptx",
        "application/vnd.android.package-archive" to "apk", "application/xml" to "xml", "text/xml" to "xml",
        "application/rtf" to "rtf", "application/epub+zip" to "epub",
    )

    private val POREXT = PORMIME.entries.associate { it.value to it.key } + mapOf("jpg" to "image/jpeg", "png" to "image/png", "mp3" to "audio/mpeg", "mp4" to "video/mp4", "md" to "text/markdown", "odt" to "application/vnd.oasis.opendocument.text", "ods" to "application/vnd.oasis.opendocument.spreadsheet")

    private fun limpio(mime: String) = mime.substringBefore(';').trim().lowercase()

    /** Extensión que se muestra: la del nombre; si no tiene, la que corresponde al MIME. */
    fun extensionMostrada(nombre: String, mime: String): String = extension(nombre).ifEmpty { PORMIME[limpio(mime)] ?: "" }

    // MIME para abrir el archivo con otra app: el que trae; si falta o es genérico, el de la extensión; si no, "cualquiera".
    fun mimeEfectivo(nombre: String, mime: String): String {
        val m = limpio(mime)
        if (m.isNotEmpty() && m != "application/octet-stream") return m
        return POREXT[extension(nombre)] ?: "*/*"
    }

    /** Nombre que se muestra y con el que se guarda la copia: nunca vacío, y con extensión si se puede deducir. */
    fun nombreMostrable(nombre: String, mime: String): String {
        val n = nombre.trim()
        if (n.isNotEmpty()) return n
        val ext = PORMIME[limpio(mime)]
        return if (ext != null) "Documento.$ext" else "Documento"
    }

    enum class Familia { PDF, HOJA, TEXTO, PRESENTACION, COMPRIMIDO, IMAGEN, AUDIO, VIDEO, APP, OTRO }

    fun familia(nombre: String, mime: String): Familia {
        val e = extensionMostrada(nombre, mime)
        val m = limpio(mime)
        return when {
            e == "pdf" -> Familia.PDF
            e in setOf("xls", "xlsx", "csv", "ods", "numbers") -> Familia.HOJA
            e in setOf("doc", "docx", "txt", "rtf", "odt", "md", "pages", "epub", "html", "xml", "json") -> Familia.TEXTO
            e in setOf("ppt", "pptx", "odp", "key") -> Familia.PRESENTACION
            e in setOf("zip", "rar", "7z", "tar", "gz") -> Familia.COMPRIMIDO
            e in setOf("apk") -> Familia.APP
            m.startsWith("image/") -> Familia.IMAGEN
            m.startsWith("audio/") -> Familia.AUDIO
            m.startsWith("video/") -> Familia.VIDEO
            else -> Familia.OTRO
        }
    }

    /** Etiqueta corta para la tarjeta: "PDF", "XLSX"… (máximo 4 letras). */
    fun etiqueta(nombre: String, mime: String): String = extensionMostrada(nombre, mime).uppercase().take(4).ifEmpty { "ARCH" }
}
