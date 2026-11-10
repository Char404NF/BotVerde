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
