package com.botchat.app.pantallas

import android.app.AlarmManager
import android.content.Intent
import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.assertIsEnabled
import androidx.compose.ui.test.assertIsNotEnabled
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.compose.ui.test.onAllNodesWithTag
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performScrollTo
import androidx.compose.ui.test.performTextClearance
import androidx.compose.ui.test.performTextInput
import androidx.core.app.NotificationCompat
import androidx.test.core.app.ApplicationProvider
import com.botchat.app.AlarmaAvisosReceiver
import com.botchat.app.Alarmas
import com.botchat.app.ArranqueReceiver
import com.botchat.app.ui.BotChatTema
import com.botchat.app.ui.EditorRecordatorio
import com.botchat.app.ui.PantallaAvisos
import org.json.JSONArray
import org.json.JSONObject
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.Shadows.shadowOf
import org.robolectric.shadows.ShadowAlarmManager
import org.robolectric.annotation.Config

@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34], application = android.app.Application::class, qualifiers = "w411dp-h1600dp-xxhdpi")
class AvisosUiTest {
    @get:Rule val regla = createComposeRule()
    private lateinit var motor: MotorFalso
    private var bot = 0L
    private val ctx get() = ApplicationProvider.getApplicationContext<android.app.Application>()
    private fun am() = ctx.getSystemService(AlarmManager::class.java)

    @Before fun antes() {
        motor = Apoyo.reinicia(); bot = Apoyo.creaBot()
        motor.ajustesPerm = JSONObject(motor.ajustesPerm).put("alertas_activas", "1").put("alerta_fragmento", "0").put("resumen_activo", "0").put("resumen_hora", "20:00").put("avisos_exactos", "1").toString()
        ShadowAlarmManager.setCanScheduleExactAlarms(true)
    }
    @After fun despues() { ShadowAlarmManager.setCanScheduleExactAlarms(true) }
    private fun llamo(p: String) = motor.llamadas.toList().any { it.startsWith(p) }
    private fun hay(tag: String) = regla.onAllNodesWithTag(tag, useUnmergedTree = true).fetchSemanticsNodes().isNotEmpty()
    private fun pantalla() = regla.setContent { BotChatTema { PantallaAvisos(bot, {}) } }

    // ---------- estado de las alarmas ----------

    @Test fun conElPermisoExactoDiceQueSuenaALaHora() {
        pantalla()
        regla.esperaEtiqueta("alarmas_exactas")
        assertFalse(hay("alarmas_retraso") || hay("pedir_alarmas_exactas"))
    }

    @Test fun sinElPermisoExactoAvisaDeRetrasosPorAhorroDeBateriaYOfreceElBoton() {
        ShadowAlarmManager.setCanScheduleExactAlarms(false)
        pantalla()
        regla.esperaEtiqueta("alarmas_retraso")
        regla.esperaTexto("Puede haber retrasos por el ahorro de batería", substring = true)
        regla.onNodeWithTag("pedir_alarmas_exactas", useUnmergedTree = true).performClick()
        val i = shadowOf(ctx).nextStartedActivity
        assertNotNull("abre la pantalla del sistema", i)
        assertEquals(android.provider.Settings.ACTION_REQUEST_SCHEDULE_EXACT_ALARM, i.action)
        assertEquals("package:" + ctx.packageName, i.dataString)
    }

    // ---------- palabras clave ----------

    @Test fun agregarCambiarElegirChatsYBorrarPalabras() {
        motor.chatsPersonales = Apoyo.arreglo(Apoyo.chatJson("a@s.whatsapp.net", "María"))
        pantalla()
        regla.esperaEtiqueta("sin_palabras")
        regla.onNodeWithTag("agregar_palabra", useUnmergedTree = true).performScrollTo().assertIsNotEnabled()
        regla.onNodeWithTag("palabra_nueva", useUnmergedTree = true).performScrollTo().performTextInput("urgente")
        regla.onNodeWithTag("agregar_palabra", useUnmergedTree = true).assertIsEnabled().performClick()
        regla.espera { llamo("crearPalabra:urgente:[]") }
        regla.esperaEtiqueta("palabra_1")
        regla.esperaTexto("En todos los chats")
        // Elegir chats
        regla.onNodeWithText("“urgente”").performClick()
        regla.esperaTexto("Dónde avisar por", substring = true)
        regla.esperaEtiqueta("palabra_chat_a@s.whatsapp.net")
        regla.onNodeWithTag("palabra_chat_a@s.whatsapp.net", useUnmergedTree = true).performClick()
        regla.onNodeWithTag("guardar_chats_palabra", useUnmergedTree = true).performClick()
        regla.espera { llamo("editarPalabra:1:[\"a@s.whatsapp.net\"]:true") }
        regla.esperaTexto("En 1 chat")
        // Apagar y borrar
        regla.onNodeWithTag("palabra_activa_1", useUnmergedTree = true).performScrollTo().performClick()
        regla.espera { llamo("editarPalabra:1:[\"a@s.whatsapp.net\"]:false") }
        regla.onNodeWithTag("borrar_palabra_1", useUnmergedTree = true).performClick()
        regla.espera { llamo("borrarPalabra:1") }
        regla.esperaEtiqueta("sin_palabras")
    }

    @Test fun elFragmentoDelMensajeEstaApagadoPorDefectoYSeCambia() {
        pantalla()
        regla.esperaTexto("Incluir un fragmento del mensaje")
        regla.onNodeWithTag("sw_Incluir un fragmento del mensaje", useUnmergedTree = true).performScrollTo().performClick()
        regla.espera { llamo("ajustePerm:alerta_fragmento:1") }
        regla.onNodeWithTag("sw_Avisarme por palabra clave", useUnmergedTree = true).performScrollTo().performClick()
        regla.espera { llamo("ajustePerm:alertas_activas:0") }
    }

    // ---------- resumen diario ----------

    @Test fun elResumenDiarioSeEnciendeEligeHoraYSePuedeVer() {
        pantalla()
        regla.esperaTexto("Resumen diario")
        assertFalse(hay("hora_resumen"))
        regla.onNodeWithTag("sw_Resumen diario", useUnmergedTree = true).performScrollTo().performClick()
        regla.espera { llamo("ajustePerm:resumen_activo:1") }
        regla.esperaEtiqueta("hora_resumen")
        regla.onNodeWithTag("hora_resumen", useUnmergedTree = true).performScrollTo().performClick()
        regla.esperaEtiqueta("selector_hora_resumen")
        regla.onNodeWithTag("guardar_hora_resumen", useUnmergedTree = true).performClick()
        regla.espera { llamo("ajustePerm:resumen_hora:20:00") }
        regla.onNodeWithTag("ver_resumen", useUnmergedTree = true).performScrollTo().performClick()
        regla.esperaEtiqueta("texto_resumen")
        regla.esperaTexto("Chats pendientes: 1", substring = true)
    }

    // ---------- recordatorios ----------

    @Test fun laListaAbreElEditorParaCrearYParaCambiar() {
        val futuro = System.currentTimeMillis() / 1000 + 86400
        motor.recordatoriosLista += JSONObject().put("id", 3).put("texto", "Pagar la luz").put("proximaTs", futuro).put("repetir", "semanal").put("estado", "pendiente").put("creadoTs", 1).put("avisadoTs", 0)
        var abierto = -1L
        regla.setContent { BotChatTema { PantallaAvisos(bot, {}, { abierto = it }) } }
        regla.esperaEtiqueta("recordatorio_3")
        regla.esperaTexto("Cada semana", substring = true)
        regla.onNodeWithTag("recordatorio_3", useUnmergedTree = true).performScrollTo().performClick()
        regla.espera { abierto == 3L }
        regla.onNodeWithTag("nuevo_recordatorio", useUnmergedTree = true).performScrollTo().performClick()
        regla.espera { abierto == 0L }
        regla.onNodeWithTag("borrar_recordatorio_3", useUnmergedTree = true).performScrollTo().performClick()
        regla.espera { llamo("borrarRecordatorio:3") }
        regla.esperaEtiqueta("sin_recordatorios")
    }

    @Test fun crearUnRecordatorioConAtajoYRepeticion() {
        var volvio = false
        regla.setContent { BotChatTema { EditorRecordatorio(bot, 0, { volvio = true }) } }
        regla.esperaEtiqueta("texto_recordatorio")
        regla.onNodeWithTag("guardar_recordatorio", useUnmergedTree = true).assertIsNotEnabled() // falta texto y hora
        regla.onNodeWithTag("texto_recordatorio", useUnmergedTree = true).performTextInput("Llamar al banco")
        regla.onNodeWithTag("guardar_recordatorio", useUnmergedTree = true).assertIsNotEnabled()
        regla.onNodeWithTag("atajo_1h", useUnmergedTree = true).performClick()
        regla.onNodeWithTag("repite_diaria", useUnmergedTree = true).performClick()
        regla.onNodeWithTag("guardar_recordatorio", useUnmergedTree = true).assertIsEnabled().performClick()
        regla.espera { motor.llamadas.any { it.startsWith("crearRecordatorio:Llamar al banco:") && it.endsWith(":diaria") } }
        val ts = motor.llamadas.first { it.startsWith("crearRecordatorio:") }.split(":")[2].toLong()
        assertTrue("dentro de ~1 hora", ts - System.currentTimeMillis() / 1000 in 3000..3700)
        regla.espera { volvio }
    }

    @Test fun cambiarUnRecordatorioConservaSuHoraYGuardaLosCambios() {
        val futuro = System.currentTimeMillis() / 1000 + 86400
        motor.recordatoriosLista += JSONObject().put("id", 3).put("texto", "Pagar la luz").put("proximaTs", futuro).put("repetir", "").put("estado", "pendiente").put("creadoTs", 1).put("avisadoTs", 0)
        var volvio = false
        regla.setContent { BotChatTema { EditorRecordatorio(bot, 3, { volvio = true }) } }
        regla.esperaTexto("Pagar la luz")
        regla.onNodeWithTag("texto_recordatorio", useUnmergedTree = true).performTextClearance()
        regla.onNodeWithTag("texto_recordatorio", useUnmergedTree = true).performTextInput("Pagar la luz hoy")
        regla.onNodeWithTag("repite_semanal", useUnmergedTree = true).performClick()
        regla.onNodeWithTag("guardar_recordatorio", useUnmergedTree = true).performClick()
        regla.espera { motor.llamadas.any { it.startsWith("editarRecordatorio:3:Pagar la luz hoy:$futuro:semanal") } }
        regla.espera { volvio }
    }

    @Test fun unaHoraPasadaNoSePuedeGuardar() {
        val pasado = System.currentTimeMillis() / 1000 - 3600
        motor.recordatoriosLista += JSONObject().put("id", 4).put("texto", "Viejo").put("proximaTs", pasado).put("repetir", "").put("estado", "pendiente").put("creadoTs", 1).put("avisadoTs", 0)
        regla.setContent { BotChatTema { EditorRecordatorio(bot, 4, {}) } }
        regla.esperaTexto("Esa hora ya pasó.")
        regla.onNodeWithTag("guardar_recordatorio", useUnmergedTree = true).assertIsNotEnabled()
    }

    // ---------- alarmas ----------

    @Test fun programaLaAlarmaExactaOInexactaSegunElPermiso() {
        val cuando = System.currentTimeMillis() / 1000 + 600
        assertTrue(Alarmas.programar(ctx, cuando, exactas = true))
        assertEquals(1, shadowOf(am()).scheduledAlarms.size)
        val a = shadowOf(am()).peekNextScheduledAlarm()!!
        assertEquals(cuando * 1000, a.triggerAtMs)
        assertEquals(AlarmManager.RTC_WAKEUP, a.type)
        assertTrue("exacta y con el teléfono en reposo", a.isAllowWhileIdle && a.windowLengthMs == ShadowAlarmManager.WINDOW_EXACT)
        // Sin permiso o con el ajuste apagado: inexacta (sigue habiendo una sola alarma).
        ShadowAlarmManager.setCanScheduleExactAlarms(false)
        assertFalse(Alarmas.programar(ctx, cuando, exactas = true))
        assertEquals(1, shadowOf(am()).scheduledAlarms.size)
        assertTrue("inexacta pero que despierta aunque haya ahorro", shadowOf(am()).peekNextScheduledAlarm()!!.let { it.isAllowWhileIdle && it.windowLengthMs != ShadowAlarmManager.WINDOW_EXACT })
        ShadowAlarmManager.setCanScheduleExactAlarms(true)
        assertFalse(Alarmas.programar(ctx, cuando, exactas = false))
        // Sin nada pendiente se cancela.
        Alarmas.programar(ctx, -1)
        assertEquals(0, shadowOf(am()).scheduledAlarms.size)
    }

    @Test fun reprogramarUsaElProximoAvisoDelModuloYElAjuste() {
        motor.proximoAvisoTs = System.currentTimeMillis() / 1000 + 7200
        Alarmas.reprogramar(ctx)
        assertTrue(llamo("proximoAviso"))
        assertEquals(motor.proximoAvisoTs * 1000, shadowOf(am()).peekNextScheduledAlarm()!!.triggerAtMs)
        motor.proximoAvisoTs = 0
        Alarmas.reprogramar(ctx)
        assertNull(shadowOf(am()).peekNextScheduledAlarm())
    }

    @Test fun trasReiniciarElTelefonoSeVuelvenAProgramarLasAlarmas() {
        motor.proximoAvisoTs = System.currentTimeMillis() / 1000 + 3600
        val r = ArranqueReceiver()
        r.onReceive(ctx, Intent(Intent.ACTION_BOOT_COMPLETED))
        regla.espera { shadowOf(am()).peekNextScheduledAlarm() != null }
        assertEquals(motor.proximoAvisoTs * 1000, shadowOf(am()).peekNextScheduledAlarm()!!.triggerAtMs)
    }

    @Test fun elReceptorDeArranqueIgnoraCualquierOtraAccion() {
        motor.proximoAvisoTs = System.currentTimeMillis() / 1000 + 3600
        ArranqueReceiver().onReceive(ctx, Intent("otra.accion"))
        Thread.sleep(300)
        assertNull(shadowOf(am()).peekNextScheduledAlarm())
        assertFalse(llamo("proximoAviso"))
    }

    @Test fun alSonarLaAlarmaSeMuestranLosAvisosYSeProgramaLaSiguiente() {
        shadowOf(ctx).grantPermissions(android.Manifest.permission.POST_NOTIFICATIONS)
        val nm = ctx.getSystemService(android.app.NotificationManager::class.java)
        nm.cancelAll()
        val siguiente = System.currentTimeMillis() / 1000 + 86400
        motor.disparo = JSONObject().put("avisos", JSONArray().put(9)).put("siguiente", siguiente).toString()
        motor.aviso = """{"id":9,"ts":1,"tipo":"recordatorio","chat":"","titulo":"Recordatorio","detalle":"Llamar al banco"}"""
        AlarmaAvisosReceiver().onReceive(ctx, Intent(Alarmas.ACCION))
        regla.espera { shadowOf(nm).size() == 1 && shadowOf(am()).peekNextScheduledAlarm() != null }
        val n = shadowOf(nm).allNotifications.single()
        assertEquals(NotificationCompat.VISIBILITY_PRIVATE, n.visibility)
        assertEquals("Recordatorio", n.publicVersion.extras.getCharSequence("android.title").toString())
        assertNull("en pantalla bloqueada no se ve el texto", n.publicVersion.extras.getCharSequence("android.text"))
        assertEquals("Llamar al banco", n.extras.getCharSequence("android.text").toString())
        assertEquals(siguiente * 1000, shadowOf(am()).peekNextScheduledAlarm()!!.triggerAtMs)
    }
}
