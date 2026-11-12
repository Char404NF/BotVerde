package com.botchat.app.ui

import androidx.compose.runtime.compositionLocalOf

/**
 * Modo de la app. En Simple se ocultan comandos propios, módulos, Laboratorio, límites e iniciativa y la actividad del bot; en Completo
 * se ve todo. El modo solo filtra lo que se muestra: no borra ni apaga nada. Por defecto (en pruebas sueltas) se ve todo.
 */
val LocalModoCompleto = compositionLocalOf { true }
