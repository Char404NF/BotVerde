package almacen

import (
	"context"
	"fmt"
	"time"
)

// migraciones: cada una se aplica en su propia transacción y solo se agregan al final; nunca se reordenan ni se cambian.
var migraciones = [][]string{
	// 1: chats y mensajes, con bot_id en las claves.
	{
		`CREATE TABLE chats (
			jid TEXT NOT NULL,
			bot_id INTEGER NOT NULL,
			es_grupo INTEGER NOT NULL DEFAULT 0,
			nombre_guardado TEXT NOT NULL DEFAULT '',
			nombre_publico TEXT NOT NULL DEFAULT '',
			nombre_negocio TEXT NOT NULL DEFAULT '',
			nombre_mostrado TEXT NOT NULL DEFAULT '',
			telefono_e164 TEXT NOT NULL DEFAULT '',
			telefono_mostrado TEXT NOT NULL DEFAULT '',
			pais_iso TEXT NOT NULL DEFAULT '',
			foto_ruta TEXT NOT NULL DEFAULT '',
			foto_id TEXT NOT NULL DEFAULT '',
			foto_ts INTEGER NOT NULL DEFAULT 0,
			ultimo_ts INTEGER NOT NULL DEFAULT 0,
			busqueda TEXT NOT NULL DEFAULT '',
			creado_ts INTEGER NOT NULL DEFAULT 0,
			PRIMARY KEY (jid, bot_id)
		)`,
		`CREATE TABLE messages (
			id TEXT NOT NULL,
			chat_jid TEXT NOT NULL,
			bot_id INTEGER NOT NULL,
			remitente_jid TEXT NOT NULL DEFAULT '',
			remitente_nombre TEXT NOT NULL DEFAULT '',
			es_mio INTEGER NOT NULL DEFAULT 0,
			tipo TEXT NOT NULL DEFAULT 'texto',
			texto_cifrado BLOB,
			ts INTEGER NOT NULL,
			borrado INTEGER NOT NULL DEFAULT 0,
			editado INTEGER NOT NULL DEFAULT 0,
			archivo_ruta TEXT NOT NULL DEFAULT '',
			archivo_bytes INTEGER NOT NULL DEFAULT 0,
			archivo_mime TEXT NOT NULL DEFAULT '',
			archivo_estado TEXT NOT NULL DEFAULT 'ninguno',
			miniatura_ruta TEXT NOT NULL DEFAULT '',
			archivo_nombre_cifrado BLOB,
			archivo_info_cifrado BLOB,
			ancho INTEGER NOT NULL DEFAULT 0,
			alto INTEGER NOT NULL DEFAULT 0,
			duracion_s INTEGER NOT NULL DEFAULT 0,
			nota_voz INTEGER NOT NULL DEFAULT 0,
			PRIMARY KEY (id, chat_jid, bot_id),
			FOREIGN KEY (chat_jid, bot_id) REFERENCES chats(jid, bot_id) ON DELETE CASCADE
		)`,
		// historial de un chat, de lo nuevo a lo viejo
		`CREATE INDEX idx_mensajes_chat_ts ON messages(bot_id, chat_jid, ts, id)`,
		// sumar tamaños de archivos por chat y filtrar por estado
		`CREATE INDEX idx_mensajes_archivos ON messages(bot_id, chat_jid, archivo_estado, archivo_bytes)`,
		// sumar por tipo y por antigüedad en toda la cuenta
		`CREATE INDEX idx_mensajes_estado_tipo ON messages(bot_id, archivo_estado, tipo, ts)`,
		// lista de chats por pestaña, de lo nuevo a lo viejo
		`CREATE INDEX idx_chats_lista ON chats(bot_id, es_grupo, ultimo_ts)`,
	},
	// 2: estadísticas diarias (solo datos sin cifrar: quién, cuándo, tipo y tamaño; nunca el texto).
	// `rowid_max` marca hasta dónde llegan los mensajes anteriores a esta migración; el relleno cuenta solo esos.
	{
		`CREATE TABLE daily_stats (
			bot_id INTEGER NOT NULL,
			chat TEXT NOT NULL,
			sender TEXT NOT NULL,
			day TEXT NOT NULL,
			hora INTEGER NOT NULL,
			tipo TEXT NOT NULL,
			count INTEGER NOT NULL,
			bytes INTEGER NOT NULL,
			PRIMARY KEY (bot_id, chat, sender, day, hora, tipo)
		) WITHOUT ROWID`,
		`CREATE INDEX idx_stats_dia ON daily_stats(bot_id, day)`,
		`CREATE TABLE stats_info (campo TEXT PRIMARY KEY, valor INTEGER NOT NULL)`,
		`INSERT INTO stats_info(campo, valor) SELECT 'rowid_max', COALESCE(MAX(rowid),0) FROM messages`,
		`INSERT INTO stats_info(campo, valor) VALUES('relleno', 0)`,
	},
	// 3: mensajes eliminados por su autor o por un admin: el contenido se conserva y solo se anota cuándo y quién (sin cifrar).
	{
		`ALTER TABLE messages ADD COLUMN eliminado_en INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE messages ADD COLUMN eliminado_por TEXT NOT NULL DEFAULT ''`,
		`CREATE INDEX idx_mensajes_eliminados ON messages(bot_id, chat_jid, borrado, ts)`,
	},
	// 4: escribir desde la app: estado de envío, citas, menciones, reenviados, acuses, reacciones y estado por chat.
	{
		`ALTER TABLE messages ADD COLUMN estado_envio TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE messages ADD COLUMN cita_id TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE messages ADD COLUMN cita_autor TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE messages ADD COLUMN cita_texto_cifrado BLOB`,
		`ALTER TABLE messages ADD COLUMN menciones_cifrado BLOB`,
		`ALTER TABLE messages ADD COLUMN reenviado INTEGER NOT NULL DEFAULT 0`,
		`CREATE TABLE message_receipts (
			bot_id INTEGER NOT NULL,
			chat_jid TEXT NOT NULL,
			msg_id TEXT NOT NULL,
			participante TEXT NOT NULL,
			tipo TEXT NOT NULL,
			ts INTEGER NOT NULL,
			PRIMARY KEY (bot_id, chat_jid, msg_id, participante, tipo)
		) WITHOUT ROWID`,
		`CREATE TABLE reacciones (
			bot_id INTEGER NOT NULL,
			chat_jid TEXT NOT NULL,
			msg_id TEXT NOT NULL,
			sender TEXT NOT NULL,
			emoji TEXT NOT NULL,
			ts INTEGER NOT NULL,
			PRIMARY KEY (bot_id, chat_jid, msg_id, sender)
		) WITHOUT ROWID`,
		`CREATE TABLE chat_estado (
			bot_id INTEGER NOT NULL,
			chat_jid TEXT NOT NULL,
			leido_enviado_ts INTEGER NOT NULL DEFAULT 0,
			silenciado INTEGER NOT NULL DEFAULT 0,
			bot_pausa_hasta INTEGER NOT NULL DEFAULT 0,
			bot_activo INTEGER NOT NULL DEFAULT 1,
			borrador_cifrado BLOB,
			PRIMARY KEY (bot_id, chat_jid)
		) WITHOUT ROWID`,
	},
	// 5: "ver una vez" que llega completo: se guarda cifrado, se abre una sola vez y luego se borra (o caduca).
	{
		`ALTER TABLE messages ADD COLUMN ver_estado TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE messages ADD COLUMN ver_tipo TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE messages ADD COLUMN ver_expira INTEGER NOT NULL DEFAULT 0`,
	},
	// 6: motor de respuestas: mensajes ya vistos (para no contestar dos veces) y registro de actividad sin textos ni nombres.
	{
		`CREATE TABLE dedupe (
			bot_id INTEGER NOT NULL,
			chat_jid TEXT NOT NULL,
			msg_id TEXT NOT NULL,
			ts INTEGER NOT NULL,
			PRIMARY KEY (bot_id, chat_jid, msg_id)
		) WITHOUT ROWID`,
		`CREATE INDEX idx_dedupe_ts ON dedupe(ts)`,
		`CREATE TABLE actividad (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			bot_id INTEGER NOT NULL,
			ts INTEGER NOT NULL,
			evento TEXT NOT NULL,
			motivo TEXT NOT NULL DEFAULT '',
			chat TEXT NOT NULL DEFAULT '',
			detalle TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE INDEX idx_actividad_ts ON actividad(bot_id, ts)`,
		`ALTER TABLE chat_estado ADD COLUMN pausa_humano INTEGER NOT NULL DEFAULT 0`,
	},
	// 7: automatizaciones (menús y respuestas), sus versiones, el seguimiento de cada conversación y los avisos locales.
	{
		`CREATE TABLE menus (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			bot_id INTEGER NOT NULL,
			activo INTEGER NOT NULL DEFAULT 1,
			sistema INTEGER NOT NULL DEFAULT 0,
			version INTEGER NOT NULL DEFAULT 1,
			orden INTEGER NOT NULL DEFAULT 0,
			creado INTEGER NOT NULL,
			actualizado INTEGER NOT NULL,
			def_cifrado BLOB
		)`,
		`CREATE TABLE menu_versiones (
			menu_id INTEGER NOT NULL,
			version INTEGER NOT NULL,
			ts INTEGER NOT NULL,
			def_cifrado BLOB,
			PRIMARY KEY (menu_id, version)
		) WITHOUT ROWID`,
		`CREATE TABLE flow_sessions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			bot_id INTEGER NOT NULL,
			chat TEXT NOT NULL,
			sender TEXT NOT NULL,
			menu_id INTEGER NOT NULL,
			menu_version INTEGER NOT NULL,
			paso TEXT NOT NULL,
			espera_dato INTEGER NOT NULL DEFAULT 0,
			invalidas INTEGER NOT NULL DEFAULT 0,
			cortado INTEGER NOT NULL DEFAULT 0,
			inicio_ts INTEGER NOT NULL,
			ultima_ts INTEGER NOT NULL,
			hasta_ts INTEGER NOT NULL DEFAULT 0,
			estado TEXT NOT NULL DEFAULT 'activo',
			prueba INTEGER NOT NULL DEFAULT 0
		)`,
		`CREATE INDEX idx_sesiones_persona ON flow_sessions(bot_id, chat, sender, estado)`,
		`CREATE INDEX idx_sesiones_menu ON flow_sessions(bot_id, menu_id, inicio_ts)`,
		`CREATE TABLE flow_events (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			session_id INTEGER NOT NULL,
			ts INTEGER NOT NULL,
			paso TEXT NOT NULL,
			opcion TEXT NOT NULL DEFAULT '',
			resultado TEXT NOT NULL
		)`,
		`CREATE INDEX idx_eventos_sesion ON flow_events(session_id)`,
		`CREATE TABLE flow_vars (
			session_id INTEGER NOT NULL,
			nombre TEXT NOT NULL,
			valor_cifrado BLOB,
			PRIMARY KEY (session_id, nombre)
		) WITHOUT ROWID`,
		`CREATE TABLE menu_stats (
			bot_id INTEGER NOT NULL,
			day TEXT NOT NULL,
			menu_id INTEGER NOT NULL,
			paso TEXT NOT NULL,
			opcion TEXT NOT NULL,
			prueba INTEGER NOT NULL DEFAULT 0,
			cuenta INTEGER NOT NULL,
			PRIMARY KEY (bot_id, day, menu_id, paso, opcion, prueba)
		) WITHOUT ROWID`,
		`CREATE TABLE avisos (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			bot_id INTEGER NOT NULL,
			ts INTEGER NOT NULL,
			tipo TEXT NOT NULL,
			chat TEXT NOT NULL DEFAULT '',
			titulo TEXT NOT NULL,
			detalle_cifrado BLOB,
			visto INTEGER NOT NULL DEFAULT 0
		)`,
	},
	// 8: nombres de personas en los mensajes de un chat (un salto al índice por persona, sin recorrer el chat)
	{
		`CREATE INDEX idx_mensajes_remitente ON messages(bot_id, chat_jid, remitente_jid, ts) WHERE remitente_nombre<>''`,
	},
	// 9 (v0.5.0, bloque 0): contador de no leídos con COUNT sobre un índice parcial (solo lo que no es mío) y la señal de que
	// intervine a mano en una conversación de menú (el bot no le contesta "opción no válida" al texto libre).
	{
		`CREATE INDEX idx_mensajes_noleidos ON messages(bot_id, chat_jid, ts) WHERE es_mio=0`,
		`ALTER TABLE flow_sessions ADD COLUMN intervencion INTEGER NOT NULL DEFAULT 0`,
	},
}

func (a *Almacen) migrar(ctx context.Context) error {
	if _, err := a.db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_info (version INTEGER PRIMARY KEY, aplicada_ts INTEGER NOT NULL)`); err != nil {
		return err
	}
	var actual int
	if err := a.db.QueryRowContext(ctx, `SELECT COALESCE(MAX(version),0) FROM schema_info`).Scan(&actual); err != nil {
		return err
	}
	if actual > len(migraciones) {
		return fmt.Errorf("la base es de una versión más nueva (%d) que esta app (%d)", actual, len(migraciones))
	}
	for i := actual; i < len(migraciones); i++ {
		tx, err := a.db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		for _, s := range migraciones[i] {
			if _, err := tx.ExecContext(ctx, s); err != nil {
				_ = tx.Rollback()
				return fmt.Errorf("migración %d: %w", i+1, err)
			}
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO schema_info(version, aplicada_ts) VALUES(?,?)`, i+1, time.Now().Unix()); err != nil {
			_ = tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

// Version devuelve la versión del esquema aplicada.
func (a *Almacen) Version() int {
	var v int
	_ = a.db.QueryRow(`SELECT COALESCE(MAX(version),0) FROM schema_info`).Scan(&v)
	return v
}
