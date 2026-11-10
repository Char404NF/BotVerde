package com.botchat.app.pantallas

import androidx.compose.ui.test.junit4.ComposeContentTestRule

/** Espera (hasta 20 s, por si el servidor está ocupado) a que aparezca un texto: los datos llegan por hilos de fondo, igual que en la app. */
fun ComposeContentTestRule.esperaTexto(texto: String, substring: Boolean = false) {
    waitUntil(20_000) { onAllNodesWithTextSafe(texto, substring) > 0 }
}

fun ComposeContentTestRule.onAllNodesWithTextSafe(texto: String, substring: Boolean = false): Int =
    onAllNodes(androidx.compose.ui.test.hasText(texto, substring = substring)).fetchSemanticsNodes().size
