package com.botchat.app.medios

import android.net.Uri
import androidx.media3.common.C
import androidx.media3.common.PlaybackException
import androidx.media3.common.util.UnstableApi
import androidx.media3.datasource.BaseDataSource
import androidx.media3.datasource.DataSource
import androidx.media3.datasource.DataSourceException
import androidx.media3.datasource.DataSpec
import com.botchat.app.seguridad.ArchivoCifrado
import com.botchat.app.seguridad.FuenteArchivo
import java.io.File
import java.io.IOException

/**
 * Origen de datos de Media3 que lee un archivo cifrado y entrega el contenido descifrado en flujo, con acceso aleatorio
 * (saltar a la mitad de un video descifra solo los bloques de 64 KB que hacen falta). Nada se escribe en claro en disco.
 */
@UnstableApi
class FuenteCifrada(private val clave: ByteArray) : BaseDataSource(/* isNetwork = */ false) {
    private var archivo: ArchivoCifrado? = null
    private var uri: Uri? = null
    private var pos = 0L
    private var restante = 0L
    private var abierto = false

    override fun open(dataSpec: DataSpec): Long {
        uri = dataSpec.uri
        transferInitializing(dataSpec)
        val ruta = dataSpec.uri.path ?: throw IOException("Falta la ruta del archivo")
        try {
            archivo = ArchivoCifrado(FuenteArchivo(File(ruta)), clave)
        } catch (e: IOException) {
            throw DataSourceException(e, PlaybackException.ERROR_CODE_IO_FILE_NOT_FOUND)
        }
        val total = archivo!!.tam
        if (dataSpec.position > total) throw DataSourceException(PlaybackException.ERROR_CODE_IO_READ_POSITION_OUT_OF_RANGE)
        pos = dataSpec.position
        restante = if (dataSpec.length == C.LENGTH_UNSET.toLong()) total - pos else minOf(dataSpec.length, total - pos)
        abierto = true
        transferStarted(dataSpec)
        return restante
    }

    override fun read(buffer: ByteArray, offset: Int, length: Int): Int {
        if (length == 0) return 0
        if (restante == 0L) return C.RESULT_END_OF_INPUT
        val n = try {
            archivo!!.leer(pos, buffer, offset, minOf(length.toLong(), restante).toInt())
        } catch (e: IOException) {
            throw DataSourceException(e, PlaybackException.ERROR_CODE_IO_UNSPECIFIED)
        }
        if (n <= 0) return C.RESULT_END_OF_INPUT
        pos += n
        restante -= n
        bytesTransferred(n)
        return n
    }

    override fun getUri(): Uri? = uri

    override fun close() {
        uri = null
        archivo?.close()
        archivo = null
        if (abierto) {
            abierto = false
            transferEnded()
        }
    }

    class Fabrica(private val clave: ByteArray) : DataSource.Factory {
        override fun createDataSource(): DataSource = FuenteCifrada(clave)
    }
}
