package com.botchat.app.datos

/**
 * Solo para medir: "carga anterior" reproduce la forma de abrir un chat de la 0.4.0 (páginas de 60, nombres uno por uno
 * en el módulo Go y recarga completa de lo ya cargado con cada cambio) para comparar antes y después en el mismo teléfono.
 * Nunca se queda activo: lo enciende y apaga la medición de Avanzado → Medir rendimiento.
 */
object ModoCarga {
    @Volatile var anterior = false
        private set

    suspend fun fijar(si: Boolean) {
        anterior = si
        runCatching { Repo.cargaAnterior(si) }
    }
}
