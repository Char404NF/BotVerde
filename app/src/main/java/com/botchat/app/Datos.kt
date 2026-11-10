package com.botchat.app

import android.content.ContentValues
import android.content.Context
import android.database.sqlite.SQLiteDatabase
import android.database.sqlite.SQLiteOpenHelper
import java.io.File

/** Un bot guardado. La tabla admite varios, aunque en la etapa 1 solo se permite uno. */
data class Bot(
    val id: Long,
    val nombre: String,
    val iso: String,
    val codigo: Int,
    val numero: String,    // número nacional (México: 10 dígitos)
    val vincular: String,  // número con código de país, solo dígitos
    val mx1: Boolean,      // formato antiguo de México (521…)
    val vinculado: Boolean,
)

/** Base local de la app (SQLite de Android, sin servicios de Google). */
class Datos private constructor(private val ctx: Context) : SQLiteOpenHelper(ctx, "botchat.db", null, 1) {

    override fun onCreate(db: SQLiteDatabase) {
        db.execSQL(
            """CREATE TABLE bots (
                id INTEGER PRIMARY KEY AUTOINCREMENT,
                nombre TEXT NOT NULL,
                pais_iso TEXT NOT NULL,
                pais_codigo INTEGER NOT NULL,
                numero TEXT NOT NULL,
                numero_vincular TEXT NOT NULL,
                mx1 INTEGER NOT NULL DEFAULT 0,
                vinculado INTEGER NOT NULL DEFAULT 0,
                creado INTEGER NOT NULL
            )""",
        )
    }

    override fun onUpgrade(db: SQLiteDatabase, oldVersion: Int, newVersion: Int) {}

    private fun lee(c: android.database.Cursor) = Bot(
        id = c.getLong(0), nombre = c.getString(1), iso = c.getString(2), codigo = c.getInt(3),
        numero = c.getString(4), vincular = c.getString(5), mx1 = c.getInt(6) == 1, vinculado = c.getInt(7) == 1,
    )

    private val columnas = "id,nombre,pais_iso,pais_codigo,numero,numero_vincular,mx1,vinculado"

    @Synchronized
    fun lista(): List<Bot> = readableDatabase.rawQuery("SELECT $columnas FROM bots ORDER BY id", null).use { c ->
        buildList { while (c.moveToNext()) add(lee(c)) }
    }

    fun primero(): Bot? = lista().firstOrNull()

    fun porId(id: Long): Bot? = lista().firstOrNull { it.id == id }

    /** Etapa 1: solo se permite un bot. Devuelve el id nuevo o -1 si ya existe uno. */
    @Synchronized
    fun crear(nombre: String, iso: String, codigo: Int, numero: String, vincular: String, mx1: Boolean): Long {
        if (lista().isNotEmpty()) return -1
        val v = ContentValues().apply {
            put("nombre", nombre); put("pais_iso", iso); put("pais_codigo", codigo)
            put("numero", numero); put("numero_vincular", vincular)
            put("mx1", if (mx1) 1 else 0); put("vinculado", 0); put("creado", System.currentTimeMillis())
        }
        return writableDatabase.insert("bots", null, v)
    }

    @Synchronized
    fun marcarVinculado(id: Long, si: Boolean = true) {
        val v = ContentValues().apply { put("vinculado", if (si) 1 else 0) }
        writableDatabase.update("bots", v, "id=?", arrayOf(id.toString()))
    }

    @Synchronized
    fun eliminar(id: Long) {
        writableDatabase.delete("bots", "id=?", arrayOf(id.toString()))
    }

    /** Carpeta privada de la sesión de este bot (base de WhatsApp). */
    fun carpetaSesion(id: Long): File = File(ctx.filesDir, "bots/$id")

    companion object {
        @Volatile private var inst: Datos? = null
        fun get(ctx: Context): Datos = inst ?: synchronized(this) {
            inst ?: Datos(ctx.applicationContext).also { inst = it }
        }
    }
}
