package com.botchat.app.pantallas

import androidx.compose.ui.test.junit4.ComposeContentTestRule
import androidx.compose.ui.test.onRoot
import androidx.compose.ui.test.printToString

/**
 * Espera (hasta 20 s) a que algo se cumpla. El reloj de las pruebas de Compose corre más rápido que el real, así que se
 * duerme un poco en cada vuelta para dar tiempo a los hilos de fondo (los datos llegan por ellos, igual que en la app).
 */
fun ComposeContentTestRule.espera(condicion: () -> Boolean) {
    waitUntil(20_000) { condicion() || run { waitForIdle(); Thread.sleep(25); false } }
}

fun ComposeContentTestRule.esperaTexto(texto: String, substring: Boolean = false) {
    try {
        espera { onAllNodesWithTextSafe(texto, substring) > 0 }
    } catch (e: Throwable) {
        throw AssertionError("No apareció «$texto». Pantalla:\n" + onRoot(useUnmergedTree = true).printToString(40), e)
    }
}

fun ComposeContentTestRule.onAllNodesWithTextSafe(texto: String, substring: Boolean = false): Int =
    onAllNodes(androidx.compose.ui.test.hasText(texto, substring = substring)).fetchSemanticsNodes().size

fun ComposeContentTestRule.esperaEtiqueta(etiqueta: String) {
    espera { onAllNodes(androidx.compose.ui.test.hasTestTag(etiqueta), useUnmergedTree = true).fetchSemanticsNodes().isNotEmpty() }
}
