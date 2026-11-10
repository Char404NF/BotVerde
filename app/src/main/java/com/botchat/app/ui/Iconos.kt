package com.botchat.app.ui

import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material.icons.filled.Add
import androidx.compose.material.icons.filled.Check
import androidx.compose.material.icons.filled.Close
import androidx.compose.material.icons.filled.Delete
import androidx.compose.material.icons.filled.Info
import androidx.compose.material.icons.filled.Lock
import androidx.compose.material.icons.filled.MoreVert
import androidx.compose.material.icons.filled.PlayArrow
import androidx.compose.material.icons.filled.Refresh
import androidx.compose.material.icons.filled.Search
import androidx.compose.material.icons.filled.Settings
import androidx.compose.material.icons.filled.Share
import androidx.compose.material.icons.filled.Warning
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.PathFillType
import androidx.compose.ui.graphics.SolidColor
import androidx.compose.ui.graphics.StrokeCap
import androidx.compose.ui.graphics.StrokeJoin
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.graphics.vector.path
import androidx.compose.ui.unit.dp

/** Iconos: los básicos de Material y unos pocos dibujados aquí con formas simples (para no cargar la colección completa). */
object Ico {
    val Atras = Icons.AutoMirrored.Filled.ArrowBack
    val Buscar = Icons.Filled.Search
    val Cerrar = Icons.Filled.Close
    val Mas = Icons.Filled.MoreVert
    val Ajustes = Icons.Filled.Settings
    val Borrar = Icons.Filled.Delete
    val Listo = Icons.Filled.Check
    val Candado = Icons.Filled.Lock
    val Info = Icons.Filled.Info
    val Reproducir = Icons.Filled.PlayArrow
    val Compartir = Icons.Filled.Share
    val Aviso = Icons.Filled.Warning
    val Actualizar = Icons.Filled.Refresh
    val Agregar = Icons.Filled.Add

    private fun icono(nombre: String, bloque: androidx.compose.ui.graphics.vector.ImageVector.Builder.() -> Unit) =
        ImageVector.Builder(nombre, 24.dp, 24.dp, 24f, 24f).apply(bloque).build()

    private fun androidx.compose.ui.graphics.vector.ImageVector.Builder.relleno(par: androidx.compose.ui.graphics.vector.PathBuilder.() -> Unit) =
        path(fill = SolidColor(Color.Black), pathFillType = PathFillType.EvenOdd, pathBuilder = par)

    private fun androidx.compose.ui.graphics.vector.ImageVector.Builder.trazo(ancho: Float = 2f, par: androidx.compose.ui.graphics.vector.PathBuilder.() -> Unit) =
        path(fill = null, stroke = SolidColor(Color.Black), strokeLineWidth = ancho, strokeLineCap = StrokeCap.Round, strokeLineJoin = StrokeJoin.Round, pathBuilder = par)

    /** Marco con una montaña y un sol. */
    val Foto = icono("foto") {
        trazo { moveTo(5f, 4f); lineTo(19f, 4f); curveTo(20.1f, 4f, 21f, 4.9f, 21f, 6f); lineTo(21f, 18f); curveTo(21f, 19.1f, 20.1f, 20f, 19f, 20f); lineTo(5f, 20f); curveTo(3.9f, 20f, 3f, 19.1f, 3f, 18f); lineTo(3f, 6f); curveTo(3f, 4.9f, 3.9f, 4f, 5f, 4f); close() }
        trazo { moveTo(3.5f, 17f); lineTo(9f, 11.5f); lineTo(13f, 15.5f); lineTo(16f, 12.5f); lineTo(20.5f, 17f) }
        relleno { moveTo(16f, 7.2f); curveTo(16.9f, 7.2f, 17.6f, 7.9f, 17.6f, 8.8f); curveTo(17.6f, 9.7f, 16.9f, 10.4f, 16f, 10.4f); curveTo(15.1f, 10.4f, 14.4f, 9.7f, 14.4f, 8.8f); curveTo(14.4f, 7.9f, 15.1f, 7.2f, 16f, 7.2f); close() }
    }

    /** Pantalla con un triángulo de reproducir. */
    val Video = icono("video") {
        trazo { moveTo(5f, 5f); lineTo(14f, 5f); curveTo(15.1f, 5f, 16f, 5.9f, 16f, 7f); lineTo(16f, 17f); curveTo(16f, 18.1f, 15.1f, 19f, 14f, 19f); lineTo(5f, 19f); curveTo(3.9f, 19f, 3f, 18.1f, 3f, 17f); lineTo(3f, 7f); curveTo(3f, 5.9f, 3.9f, 5f, 5f, 5f); close() }
        relleno { moveTo(17.5f, 10f); lineTo(21f, 7.5f); lineTo(21f, 16.5f); lineTo(17.5f, 14f); close() }
    }

    /** Micrófono. */
    val Audio = icono("audio") {
        trazo { moveTo(12f, 3f); curveTo(10.3f, 3f, 9f, 4.3f, 9f, 6f); lineTo(9f, 11f); curveTo(9f, 12.7f, 10.3f, 14f, 12f, 14f); curveTo(13.7f, 14f, 15f, 12.7f, 15f, 11f); lineTo(15f, 6f); curveTo(15f, 4.3f, 13.7f, 3f, 12f, 3f); close() }
        trazo { moveTo(5.5f, 11f); curveTo(5.5f, 14.6f, 8.4f, 17.5f, 12f, 17.5f); curveTo(15.6f, 17.5f, 18.5f, 14.6f, 18.5f, 11f) }
        trazo { moveTo(12f, 17.5f); lineTo(12f, 21f); moveTo(8.5f, 21f); lineTo(15.5f, 21f) }
    }

    /** Hoja con la esquina doblada y renglones. */
    val Documento = icono("documento") {
        trazo { moveTo(6f, 3f); lineTo(14f, 3f); lineTo(19f, 8f); lineTo(19f, 19f); curveTo(19f, 20.1f, 18.1f, 21f, 17f, 21f); lineTo(6f, 21f); curveTo(4.9f, 21f, 4f, 20.1f, 4f, 19f); lineTo(4f, 5f); curveTo(4f, 3.9f, 4.9f, 3f, 6f, 3f); close() }
        trazo { moveTo(14f, 3f); lineTo(14f, 8f); lineTo(19f, 8f) }
        trazo { moveTo(7.5f, 13f); lineTo(15.5f, 13f); moveTo(7.5f, 17f); lineTo(13f, 17f) }
    }

    /** Flecha hacia abajo sobre una bandeja. */
    val Descargar = icono("descargar") {
        trazo { moveTo(12f, 4f); lineTo(12f, 15f); moveTo(7.5f, 10.5f); lineTo(12f, 15f); lineTo(16.5f, 10.5f) }
        trazo { moveTo(4.5f, 15f); lineTo(4.5f, 19f); lineTo(19.5f, 19f); lineTo(19.5f, 15f) }
    }

    /** Dos barras. */
    val Pausa = icono("pausa") {
        relleno { moveTo(7f, 5f); lineTo(10.5f, 5f); lineTo(10.5f, 19f); lineTo(7f, 19f); close() }
        relleno { moveTo(13.5f, 5f); lineTo(17f, 5f); lineTo(17f, 19f); lineTo(13.5f, 19f); close() }
    }

    /** Disco de almacenamiento. */
    val Espacio = icono("espacio") {
        trazo { moveTo(12f, 3f); curveTo(7.6f, 3f, 4.5f, 4.5f, 4.5f, 6.3f); lineTo(4.5f, 17.7f); curveTo(4.5f, 19.5f, 7.6f, 21f, 12f, 21f); curveTo(16.4f, 21f, 19.5f, 19.5f, 19.5f, 17.7f); lineTo(19.5f, 6.3f); curveTo(19.5f, 4.5f, 16.4f, 3f, 12f, 3f); close() }
        trazo { moveTo(4.5f, 6.3f); curveTo(4.5f, 8.1f, 7.6f, 9.6f, 12f, 9.6f); curveTo(16.4f, 9.6f, 19.5f, 8.1f, 19.5f, 6.3f) }
        trazo { moveTo(4.5f, 12f); curveTo(4.5f, 13.8f, 7.6f, 15.3f, 12f, 15.3f); curveTo(16.4f, 15.3f, 19.5f, 13.8f, 19.5f, 12f) }
    }

    /** Carpeta de archivos compartidos. */
    val Carpeta = icono("carpeta") {
        trazo { moveTo(4f, 6f); lineTo(9.5f, 6f); lineTo(11.5f, 8.5f); lineTo(20f, 8.5f); curveTo(20.6f, 8.5f, 21f, 8.9f, 21f, 9.5f); lineTo(21f, 18f); curveTo(21f, 18.6f, 20.6f, 19f, 20f, 19f); lineTo(4f, 19f); curveTo(3.4f, 19f, 3f, 18.6f, 3f, 18f); lineTo(3f, 7f); curveTo(3f, 6.4f, 3.4f, 6f, 4f, 6f); close() }
    }

    /** Huella: tres arcos concéntricos. */
    val Huella = icono("huella") {
        trazo { moveTo(7f, 17f); curveTo(7f, 12f, 8.5f, 8f, 12f, 8f); curveTo(15.5f, 8f, 17f, 12f, 17f, 17f) }
        trazo { moveTo(4f, 15f); curveTo(4f, 8.5f, 7.5f, 4.5f, 12f, 4.5f); curveTo(16.5f, 4.5f, 20f, 8.5f, 20f, 15f) }
        trazo { moveTo(10f, 19.5f); curveTo(10.8f, 17f, 11f, 14f, 12f, 12f); curveTo(13f, 14f, 13.2f, 17f, 14f, 19.5f) }
    }

    /** Altavoz. */
    val Volumen = icono("volumen") {
        relleno { moveTo(4f, 9.5f); lineTo(8f, 9.5f); lineTo(13f, 5f); lineTo(13f, 19f); lineTo(8f, 14.5f); lineTo(4f, 14.5f); close() }
        trazo { moveTo(16f, 9f); curveTo(17.4f, 10.4f, 17.4f, 13.6f, 16f, 15f); moveTo(18.5f, 6.5f); curveTo(21.3f, 9.3f, 21.3f, 14.7f, 18.5f, 17.5f) }
    }

    /** Teléfono con un cuadro (copia). */
    val Copiar = icono("copiar") {
        trazo { moveTo(9f, 9f); lineTo(19f, 9f); lineTo(19f, 20f); lineTo(9f, 20f); close() }
        trazo { moveTo(5f, 15f); lineTo(5f, 4f); lineTo(15f, 4f) }
    }

    /** Nube con rayo cruzado: sin descargar. */
    val Guardar = icono("guardar") {
        trazo { moveTo(5f, 4f); lineTo(16f, 4f); lineTo(20f, 8f); lineTo(20f, 20f); lineTo(5f, 20f); close() }
        trazo { moveTo(8f, 4f); lineTo(8f, 9f); lineTo(15f, 9f); lineTo(15f, 4f) }
        trazo { moveTo(8f, 20f); lineTo(8f, 14f); lineTo(17f, 14f); lineTo(17f, 20f) }
    }

    /** Casilla de seleccionar varios. */
    val Seleccionar = icono("seleccionar") {
        trazo { moveTo(4f, 4f); lineTo(20f, 4f); lineTo(20f, 20f); lineTo(4f, 20f); close() }
        trazo { moveTo(8f, 12f); lineTo(11f, 15f); lineTo(16.5f, 9f) }
    }
}
