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
	// 10 (v0.5.0, bloque A): rangos y permisos. roles (el Dueño es una fila fija), persona_rol (con caducidad), persona_alias
	// (número y LID de la misma persona → clave canónica), registro_acciones (sin textos), solicitudes (desconocidos),
	// codigos_confirmacion (un solo uso), grupos_politica y bot_ajustes (prefijo, política de desconocidos, topes…).
	{
		`CREATE TABLE roles (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			bot_id INTEGER NOT NULL,
			nombre TEXT NOT NULL,
			color TEXT NOT NULL DEFAULT '#2EC4CB',
			posicion INTEGER NOT NULL,
			plantilla TEXT NOT NULL DEFAULT '',
			es_dueno INTEGER NOT NULL DEFAULT 0,
			casillas TEXT NOT NULL DEFAULT '{}',
			menus_modo TEXT NOT NULL DEFAULT 'todos',
			menus TEXT NOT NULL DEFAULT '[]',
			limite_min INTEGER NOT NULL DEFAULT 10,
			creado_ts INTEGER NOT NULL DEFAULT 0,
			actualizado_ts INTEGER NOT NULL DEFAULT 0
		)`,
		`CREATE INDEX idx_roles_bot ON roles(bot_id, posicion)`,
		`CREATE TABLE persona_rol (
			bot_id INTEGER NOT NULL,
			persona TEXT NOT NULL,
			rol_id INTEGER NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
			caduca_en INTEGER NOT NULL DEFAULT 0,
			asignado_por TEXT NOT NULL DEFAULT '',
			asignado_ts INTEGER NOT NULL DEFAULT 0,
			nota_cifrada BLOB,
			PRIMARY KEY (bot_id, persona)
		)`,
		`CREATE INDEX idx_persona_rol_rol ON persona_rol(rol_id)`,
		`CREATE INDEX idx_persona_rol_caduca ON persona_rol(bot_id, caduca_en) WHERE caduca_en>0`,
		`CREATE TABLE persona_alias (
			bot_id INTEGER NOT NULL,
			alias TEXT NOT NULL,
			canonica TEXT NOT NULL,
			ts INTEGER NOT NULL DEFAULT 0,
			PRIMARY KEY (bot_id, alias)
		) WITHOUT ROWID`,
		`CREATE INDEX idx_persona_alias_canonica ON persona_alias(bot_id, canonica)`,
		`CREATE TABLE registro_acciones (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			bot_id INTEGER NOT NULL,
			ts INTEGER NOT NULL,
			quien TEXT NOT NULL,
			accion TEXT NOT NULL,
			objetivo TEXT NOT NULL DEFAULT '',
			chat TEXT NOT NULL DEFAULT '',
			resultado TEXT NOT NULL,
			detalle TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE INDEX idx_registro_acciones ON registro_acciones(bot_id, id)`,
		`CREATE TABLE solicitudes (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			bot_id INTEGER NOT NULL,
			persona TEXT NOT NULL,
			nombre_cifrado BLOB,
			estado TEXT NOT NULL DEFAULT 'pendiente',
			primera_ts INTEGER NOT NULL,
			ultima_ts INTEGER NOT NULL,
			mensajes INTEGER NOT NULL DEFAULT 1,
			avisada_ts INTEGER NOT NULL DEFAULT 0,
			decidida_ts INTEGER NOT NULL DEFAULT 0,
			UNIQUE (bot_id, persona)
		)`,
		`CREATE INDEX idx_solicitudes_estado ON solicitudes(bot_id, estado, ultima_ts)`,
		`CREATE TABLE codigos_confirmacion (
			bot_id INTEGER NOT NULL,
			codigo TEXT NOT NULL,
			persona TEXT NOT NULL,
			comando TEXT NOT NULL,
			args_cifrados BLOB,
			chat TEXT NOT NULL DEFAULT '',
			cita_id TEXT NOT NULL DEFAULT '',
			creado_ts INTEGER NOT NULL,
			expira_ts INTEGER NOT NULL,
			estado TEXT NOT NULL DEFAULT 'pendiente',
			PRIMARY KEY (bot_id, codigo)
		) WITHOUT ROWID`,
		`CREATE TABLE grupos_politica (
			bot_id INTEGER NOT NULL,
			chat_jid TEXT NOT NULL,
			modo TEXT NOT NULL DEFAULT 'todos',
			lista TEXT NOT NULL DEFAULT '[]',
			actualizado_ts INTEGER NOT NULL DEFAULT 0,
			PRIMARY KEY (bot_id, chat_jid)
		) WITHOUT ROWID`,
		`CREATE TABLE bot_ajustes (
			bot_id INTEGER NOT NULL,
			clave TEXT NOT NULL,
			valor TEXT NOT NULL,
			PRIMARY KEY (bot_id, clave)
		) WITHOUT ROWID`,
	},
	// 11 (v0.5.0, etapa H0): marco de módulos. `modulos` (activo + configuración cifrada con el cifrado de columna + topes propios),
	// `modulos_registro` (qué iniciativa salió o por qué no; SIN textos), `no_molestar` (lista global) y los ajustes de iniciativa
	// (maestro, topes, pausa nocturna, modo seguro) que viven en bot_ajustes con prefijo `ini_`.
	{
		`CREATE TABLE modulos (
			bot_id INTEGER NOT NULL,
			id TEXT NOT NULL,
			activo INTEGER NOT NULL DEFAULT 0,
			config BLOB,
			tope_hora INTEGER NOT NULL DEFAULT 0,
			tope_dia INTEGER NOT NULL DEFAULT 0,
			activado_ts INTEGER NOT NULL DEFAULT 0,
			PRIMARY KEY (bot_id, id)
		) WITHOUT ROWID`,
		`CREATE TABLE modulos_registro (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			bot_id INTEGER NOT NULL,
			modulo TEXT NOT NULL,
			que TEXT NOT NULL,
			ts INTEGER NOT NULL,
			resultado TEXT NOT NULL,
			chat TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE INDEX idx_modulos_registro ON modulos_registro(bot_id, modulo, ts)`,
		`CREATE TABLE no_molestar (
			bot_id INTEGER NOT NULL,
			persona TEXT NOT NULL,
			desde_ts INTEGER NOT NULL,
			motivo TEXT NOT NULL DEFAULT '',
			confirmado INTEGER NOT NULL DEFAULT 0,
			PRIMARY KEY (bot_id, persona)
		) WITHOUT ROWID`,
	},
	// 12 (v0.5.0, etapa B): personas, chat y avisos para mí. persona_silencio (el bot calla con esa persona; hasta_ts 0 = siempre),
	// advertencias y notas_persona (motivo y texto cifrados), etiquetas locales y chat_etiqueta, estado_trabajo (pendiente/resuelto) en
	// chat_estado, recordatorios (texto cifrado), alertas_palabra (palabra cifrada) y la persona en los avisos. Bloquear en el bot NO
	// tiene tabla propia: es el estado "bloqueada" de solicitudes (una sola fuente de verdad). Los ajustes (tope de advertencias,
	// resumen diario…) viven en bot_ajustes.
	{
		`CREATE TABLE persona_silencio (
			bot_id INTEGER NOT NULL,
			persona TEXT NOT NULL,
			hasta_ts INTEGER NOT NULL DEFAULT 0,
			motivo TEXT NOT NULL DEFAULT '',
			creado_ts INTEGER NOT NULL DEFAULT 0,
			por TEXT NOT NULL DEFAULT '',
			PRIMARY KEY (bot_id, persona)
		) WITHOUT ROWID`,
		`CREATE TABLE advertencias (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			bot_id INTEGER NOT NULL,
			persona TEXT NOT NULL,
			chat TEXT NOT NULL DEFAULT '',
			motivo_cifrado BLOB,
			ts INTEGER NOT NULL,
			por TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE INDEX idx_advertencias ON advertencias(bot_id, persona, ts)`,
		`CREATE TABLE notas_persona (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			bot_id INTEGER NOT NULL,
			persona TEXT NOT NULL,
			texto_cifrado BLOB NOT NULL,
			ts INTEGER NOT NULL,
			por TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE INDEX idx_notas_persona ON notas_persona(bot_id, persona, ts)`,
		`CREATE TABLE etiquetas (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			bot_id INTEGER NOT NULL,
			nombre TEXT NOT NULL,
			color TEXT NOT NULL DEFAULT '#2EC4CB',
			posicion INTEGER NOT NULL DEFAULT 0,
			creado_ts INTEGER NOT NULL DEFAULT 0
		)`,
		`CREATE TABLE chat_etiqueta (
			bot_id INTEGER NOT NULL,
			chat_jid TEXT NOT NULL,
			etiqueta_id INTEGER NOT NULL REFERENCES etiquetas(id) ON DELETE CASCADE,
			PRIMARY KEY (bot_id, chat_jid, etiqueta_id)
		) WITHOUT ROWID`,
		`CREATE INDEX idx_chat_etiqueta ON chat_etiqueta(bot_id, etiqueta_id)`,
		`ALTER TABLE chat_estado ADD COLUMN estado_trabajo TEXT NOT NULL DEFAULT ''`,
		`CREATE TABLE recordatorios (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			bot_id INTEGER NOT NULL,
			texto_cifrado BLOB NOT NULL,
			proxima_ts INTEGER NOT NULL,
			repetir TEXT NOT NULL DEFAULT '',
			estado TEXT NOT NULL DEFAULT 'pendiente',
			creado_ts INTEGER NOT NULL DEFAULT 0,
			avisado_ts INTEGER NOT NULL DEFAULT 0
		)`,
		`CREATE INDEX idx_recordatorios ON recordatorios(bot_id, estado, proxima_ts)`,
		`CREATE TABLE alertas_palabra (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			bot_id INTEGER NOT NULL,
			palabra_cifrada BLOB NOT NULL,
			clave TEXT NOT NULL DEFAULT '',
			chats TEXT NOT NULL DEFAULT '[]',
			activa INTEGER NOT NULL DEFAULT 1,
			creado_ts INTEGER NOT NULL DEFAULT 0
		)`,
		`ALTER TABLE avisos ADD COLUMN persona TEXT NOT NULL DEFAULT ''`,
		`CREATE INDEX idx_stats_sender ON daily_stats(bot_id, sender)`,
	},
	// 13 (v0.5.0, etapa C): menús ampliados. flow_sessions guarda cuánto dura cada conversación (privado y grupo), la pila de menús
	// para "atrás", la fase de un formulario y los errores seguidos; msg_bot_sesion liga cada mensaje del bot en un grupo con su
	// conversación y su dueña; flow_cola es la fila de espera de un grupo lleno; flow_vencido evita repetir "esa conversación terminó";
	// flow_noentendio guarda (cifrado) lo que el menú no entendió; formularios guarda (cifrado) los formularios completos.
	{
		`ALTER TABLE flow_sessions ADD COLUMN expira_seg INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE flow_sessions ADD COLUMN pila TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE flow_sessions ADD COLUMN fase TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE flow_sessions ADD COLUMN campo INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE flow_sessions ADD COLUMN reint INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE flow_sessions ADD COLUMN cierre TEXT NOT NULL DEFAULT ''`,
		`CREATE INDEX idx_sesiones_chat ON flow_sessions(bot_id, chat, prueba, estado)`,
		`CREATE TABLE msg_bot_sesion (
			bot_id INTEGER NOT NULL,
			chat TEXT NOT NULL,
			msg_id TEXT NOT NULL,
			sesion_id INTEGER NOT NULL,
			persona TEXT NOT NULL,
			menu_id INTEGER NOT NULL,
			ts INTEGER NOT NULL,
			PRIMARY KEY (bot_id, chat, msg_id)
		) WITHOUT ROWID`,
		`CREATE INDEX idx_msg_bot_ts ON msg_bot_sesion(bot_id, ts)`,
		`CREATE TABLE flow_cola (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			bot_id INTEGER NOT NULL,
			chat TEXT NOT NULL,
			persona TEXT NOT NULL,
			mencion_wa TEXT NOT NULL DEFAULT '',
			msg_id TEXT NOT NULL DEFAULT '',
			menu_id INTEGER NOT NULL,
			ts INTEGER NOT NULL
		)`,
		`CREATE INDEX idx_flow_cola ON flow_cola(bot_id, chat, ts)`,
		`CREATE TABLE flow_vencido (
			bot_id INTEGER NOT NULL,
			chat TEXT NOT NULL,
			persona TEXT NOT NULL,
			menu_id INTEGER NOT NULL,
			ts INTEGER NOT NULL,
			PRIMARY KEY (bot_id, chat, persona, menu_id)
		) WITHOUT ROWID`,
		`CREATE TABLE flow_noentendio (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			bot_id INTEGER NOT NULL,
			menu_id INTEGER NOT NULL,
			paso TEXT NOT NULL DEFAULT '',
			ts INTEGER NOT NULL,
			texto_cifrado BLOB
		)`,
		`CREATE INDEX idx_noentendio ON flow_noentendio(bot_id, menu_id, ts)`,
		`CREATE TABLE formularios (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			bot_id INTEGER NOT NULL,
			menu_id INTEGER NOT NULL,
			paso TEXT NOT NULL DEFAULT '',
			titulo TEXT NOT NULL DEFAULT '',
			sesion_id INTEGER NOT NULL DEFAULT 0,
			chat TEXT NOT NULL DEFAULT '',
			persona TEXT NOT NULL DEFAULT '',
			ts INTEGER NOT NULL,
			nombre_cifrado BLOB,
			datos_cifrado BLOB NOT NULL
		)`,
		`CREATE INDEX idx_formularios ON formularios(bot_id, menu_id, ts)`,
	},
	// 14 (v0.5.0, etapa D): mensajes automáticos. auto_mensajes guarda cada mensaje (texto, configuración y adjuntos CIFRADOS) de los
	// tipos bienvenida, ausencia, recibido, cierre y atajo; auto_repeticion recuerda cuándo se le mandó cada uno a cada persona (como la
	// tabla de duplicados); auto_registro anota cada envío automático SIN textos (resultado, archivos que salieron y cuáles fallaron);
	// messages.automatico etiqueta los mensajes que mandó una automatización y el índice parcial permite filtrarlos.
	{
		`CREATE TABLE auto_mensajes (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			bot_id INTEGER NOT NULL,
			tipo TEXT NOT NULL,
			nombre TEXT NOT NULL DEFAULT '',
			texto_cifrado BLOB,
			config_cifrado BLOB,
			adjuntos_cifrado BLOB,
			activo INTEGER NOT NULL DEFAULT 1,
			orden INTEGER NOT NULL DEFAULT 0,
			creado_ts INTEGER NOT NULL DEFAULT 0,
			editado_ts INTEGER NOT NULL DEFAULT 0
		)`,
		`CREATE INDEX idx_auto_mensajes ON auto_mensajes(bot_id, tipo, orden)`,
		`CREATE TABLE auto_repeticion (
			bot_id INTEGER NOT NULL,
			auto_id INTEGER NOT NULL,
			persona TEXT NOT NULL,
			ultimo_ts INTEGER NOT NULL,
			PRIMARY KEY (bot_id, auto_id, persona)
		) WITHOUT ROWID`,
		`CREATE TABLE auto_registro (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			bot_id INTEGER NOT NULL,
			ts INTEGER NOT NULL,
			auto_id INTEGER NOT NULL,
			tipo TEXT NOT NULL,
			chat TEXT NOT NULL DEFAULT '',
			resultado TEXT NOT NULL DEFAULT '',
			adj_ok INTEGER NOT NULL DEFAULT 0,
			adj_fallo INTEGER NOT NULL DEFAULT 0,
			motivo TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE INDEX idx_auto_registro ON auto_registro(bot_id, ts)`,
		`CREATE INDEX idx_auto_registro_auto ON auto_registro(bot_id, auto_id, ts)`,
		`ALTER TABLE messages ADD COLUMN automatico TEXT NOT NULL DEFAULT ''`,
		`CREATE INDEX idx_mensajes_auto ON messages(bot_id, chat_jid, ts) WHERE automatico <> ''`,
	},
	// 15: grupos (bloque E de la v0.5.0): filtros por grupo (enlaces, palabras, repetidos, flood) con su acción y excepciones, y las
	// reglas del grupo (texto que escribo yo; WhatsApp no tiene un campo de reglas aparte de la descripción). La configuración de cada
	// filtro y las reglas van cifradas. El ranking y los inactivos salen de daily_stats (ya existe).
	{
		`CREATE TABLE grupo_filtros (
			bot_id INTEGER NOT NULL,
			chat_jid TEXT NOT NULL,
			filtro TEXT NOT NULL,
			activo INTEGER NOT NULL DEFAULT 0,
			accion TEXT NOT NULL DEFAULT 'avisarme',
			config_cifrado BLOB,
			editado_ts INTEGER NOT NULL DEFAULT 0,
			PRIMARY KEY (bot_id, chat_jid, filtro)
		) WITHOUT ROWID`,
		`CREATE TABLE grupo_reglas (
			bot_id INTEGER NOT NULL,
			chat_jid TEXT NOT NULL,
			texto_cifrado BLOB,
			editado_ts INTEGER NOT NULL DEFAULT 0,
			PRIMARY KEY (bot_id, chat_jid)
		) WITHOUT ROWID`,
	},
	// 16 (v0.5.0, etapa F): utilidades por comando. util_items guarda las NOTAS y los PENDIENTES (tareas) PROPIOS de cada persona
	// (texto cifrado). Van aparte de notas_persona porque esas son notas del personal SOBRE alguien (las ve la ficha); estas son de
	// quien las escribe y nadie más las ve.
	{
		`CREATE TABLE util_items (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			bot_id INTEGER NOT NULL,
			persona TEXT NOT NULL,
			tipo TEXT NOT NULL,
			texto_cifrado BLOB NOT NULL,
			hecho INTEGER NOT NULL DEFAULT 0,
			ts INTEGER NOT NULL,
			hecho_ts INTEGER NOT NULL DEFAULT 0
		)`,
		`CREATE INDEX idx_util_items ON util_items(bot_id, persona, tipo, id)`,
	},
	// 17 (v0.5.0, etapa G): Laboratorio. poll_sessions guarda cada menú interactivo que salió (encuesta, botones, lista o mensaje
	// interactivo): id del mensaje, chat, persona, el MessageSecret y las opciones (los dos CIFRADOS). poll_votos guarda el último
	// voto de cada votante en cada encuesta (cifrado) para detectar duplicados, cambios y retiros. lab_resultados es la matriz de
	// pruebas del Laboratorio (llegó, se ve, se puede tocar, respondió, versiones de WhatsApp); sin textos de personas.
	{
		`CREATE TABLE poll_sessions (
			bot_id INTEGER NOT NULL,
			id TEXT NOT NULL,
			chat TEXT NOT NULL,
			persona TEXT NOT NULL DEFAULT '',
			tipo TEXT NOT NULL,
			origen TEXT NOT NULL DEFAULT 'menu',
			prueba_id INTEGER NOT NULL DEFAULT 0,
			ts INTEGER NOT NULL,
			expira_ts INTEGER NOT NULL,
			estado TEXT NOT NULL DEFAULT 'activa',
			secreto_cifrado BLOB,
			opciones_cifrado BLOB,
			PRIMARY KEY (bot_id, id)
		) WITHOUT ROWID`,
		`CREATE INDEX idx_poll_sessions_chat ON poll_sessions(bot_id, chat, persona, estado)`,
		`CREATE TABLE poll_votos (
			bot_id INTEGER NOT NULL,
			poll_id TEXT NOT NULL,
			votante TEXT NOT NULL,
			seleccion_cifrado BLOB,
			ts INTEGER NOT NULL,
			PRIMARY KEY (bot_id, poll_id, votante)
		) WITHOUT ROWID`,
		`CREATE TABLE lab_resultados (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			bot_id INTEGER NOT NULL,
			ts INTEGER NOT NULL,
			tipo TEXT NOT NULL,
			variante TEXT NOT NULL DEFAULT '',
			chat TEXT NOT NULL,
			msg_id TEXT NOT NULL DEFAULT '',
			enviado INTEGER NOT NULL DEFAULT -1,
			visible INTEGER NOT NULL DEFAULT -1,
			tocable INTEGER NOT NULL DEFAULT -1,
			respondio INTEGER NOT NULL DEFAULT -1,
			version_wa TEXT NOT NULL DEFAULT '',
			version_cliente TEXT NOT NULL DEFAULT '',
			error TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE INDEX idx_lab_resultados ON lab_resultados(bot_id, ts)`,
	},
	// 18 (v0.5.0, etapa H parte 1): módulos opcionales H1–H5. mod_listas (suscritos, "quieren entrar"…), mod_agenda (programados y
	// difusiones; contenido CIFRADO), mod_dest (destinatarios de una difusión), mod_pendientes (verificaciones de bienvenida),
	// mod_chats (a qué grupos aplica un módulo) y mod_conectores (conectores HTTP; configuración con llaves CIFRADA).
	{
		`CREATE TABLE mod_listas (
			bot_id INTEGER NOT NULL,
			lista TEXT NOT NULL,
			persona TEXT NOT NULL,
			desde_ts INTEGER NOT NULL,
			origen TEXT NOT NULL DEFAULT '',
			PRIMARY KEY (bot_id, lista, persona)
		) WITHOUT ROWID`,
		`CREATE INDEX idx_mod_listas_persona ON mod_listas(bot_id, persona)`,
		`CREATE TABLE mod_agenda (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			bot_id INTEGER NOT NULL,
			modulo TEXT NOT NULL,
			nombre TEXT NOT NULL DEFAULT '',
			destino TEXT NOT NULL DEFAULT '',
			contenido BLOB,
			proximo_ts INTEGER NOT NULL DEFAULT 0,
			repeticion TEXT NOT NULL DEFAULT '',
			estado TEXT NOT NULL DEFAULT 'activo',
			creado_ts INTEGER NOT NULL DEFAULT 0,
			ultimo_ts INTEGER NOT NULL DEFAULT 0,
			ultimo_res TEXT NOT NULL DEFAULT '',
			veces INTEGER NOT NULL DEFAULT 0
		)`,
		`CREATE INDEX idx_mod_agenda ON mod_agenda(bot_id, modulo, estado, proximo_ts)`,
		`CREATE TABLE mod_dest (
			bot_id INTEGER NOT NULL,
			agenda_id INTEGER NOT NULL,
			persona TEXT NOT NULL,
			estado TEXT NOT NULL DEFAULT 'pendiente',
			ts INTEGER NOT NULL DEFAULT 0,
			PRIMARY KEY (bot_id, agenda_id, persona)
		) WITHOUT ROWID`,
		`CREATE TABLE mod_pendientes (
			bot_id INTEGER NOT NULL,
			modulo TEXT NOT NULL,
			chat TEXT NOT NULL,
			persona TEXT NOT NULL,
			desde_ts INTEGER NOT NULL,
			vence_ts INTEGER NOT NULL,
			esperada TEXT NOT NULL DEFAULT '',
			intentos INTEGER NOT NULL DEFAULT 0,
			PRIMARY KEY (bot_id, modulo, chat, persona)
		) WITHOUT ROWID`,
		`CREATE TABLE mod_chats (
			bot_id INTEGER NOT NULL,
			modulo TEXT NOT NULL,
			chat TEXT NOT NULL,
			PRIMARY KEY (bot_id, modulo, chat)
		) WITHOUT ROWID`,
		`CREATE TABLE mod_conectores (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			bot_id INTEGER NOT NULL,
			nombre TEXT NOT NULL,
			config BLOB,
			UNIQUE (bot_id, nombre)
		)`,
	},
	// 19 · v0.5.0 etapa H (parte 2): datos propios de los módulos H6–H9 (personalidad de la IA por chat, marcador y preguntas de los
	// juegos). El valor va CIFRADO; num es un contador simple (puntos).
	{
		`CREATE TABLE mod_datos (
			bot_id INTEGER NOT NULL,
			modulo TEXT NOT NULL,
			coleccion TEXT NOT NULL,
			clave TEXT NOT NULL,
			num INTEGER NOT NULL DEFAULT 0,
			ts INTEGER NOT NULL DEFAULT 0,
			valor BLOB,
			PRIMARY KEY (bot_id, modulo, coleccion, clave)
		) WITHOUT ROWID`,
	},
	// 20 · v0.6.0 etapa E1: auditoría y base para multimedia/UI. chat_estado: interruptor de texto por chat (-1 = usar el global),
	// retención por chat (-1 = usar la global) y fijado local. messages: marca de "contenido olvidado" (0 = normal, 1 = se quitó el texto,
	// 2 = revisado sin nada que olvidar), "del historial", huella del archivo (dedupe), portada de video, motivo del borrado del archivo
	// y bandera de enlaces. mensaje_ediciones guarda las versiones anteriores de un mensaje editado (texto cifrado con AAD texto_ed).
	// Quien actualiza queda con la retención "pendiente de elegir": no se purga nada hasta que se elija.
	{
		`ALTER TABLE chat_estado ADD COLUMN audit_texto INTEGER NOT NULL DEFAULT -1`,
		`ALTER TABLE chat_estado ADD COLUMN audit_ret_dias INTEGER NOT NULL DEFAULT -1`,
		`ALTER TABLE chat_estado ADD COLUMN fijado INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE messages ADD COLUMN sha256 BLOB`,
		`ALTER TABLE messages ADD COLUMN portada_ruta TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE messages ADD COLUMN archivo_borrado_ts INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE messages ADD COLUMN archivo_borrado_motivo TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE messages ADD COLUMN enlaces INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE messages ADD COLUMN contenido_olvidado INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE messages ADD COLUMN del_historial INTEGER NOT NULL DEFAULT 0`,
		`CREATE TABLE mensaje_ediciones (
			bot_id INTEGER NOT NULL,
			chat_jid TEXT NOT NULL,
			msg_id TEXT NOT NULL,
			n INTEGER NOT NULL,
			ts INTEGER NOT NULL,
			texto_cifrado BLOB,
			PRIMARY KEY (bot_id, chat_jid, msg_id, n),
			FOREIGN KEY (msg_id, chat_jid, bot_id) REFERENCES messages(id, chat_jid, bot_id) ON DELETE CASCADE
		) WITHOUT ROWID`,
		`CREATE INDEX idx_mensajes_sha ON messages(bot_id, sha256) WHERE sha256 IS NOT NULL AND archivo_estado='descargado'`,
		`CREATE INDEX idx_mensajes_viejos ON messages(bot_id, archivo_estado, ts) WHERE archivo_estado='descargado'`,
		`CREATE INDEX idx_mensajes_autor_tipo ON messages(bot_id, chat_jid, remitente_jid, tipo, ts)`,
		`CREATE INDEX idx_mensajes_autor_global ON messages(bot_id, remitente_jid, ts)`,
		`CREATE INDEX idx_mensajes_olvidar ON messages(bot_id, ts) WHERE contenido_olvidado=0`,
		`INSERT OR IGNORE INTO bot_ajustes(bot_id, clave, valor) SELECT DISTINCT bot_id, 'audit_ret_pendiente', '1' FROM messages`,
	},
	// 21 · v0.6.0 etapa E3: etiquetas y miembros de grupo. etiquetas.emoji; persona_etiqueta (persona canónica, FK con cascada);
	// msg_marca (marca de mensaje con nota privada cifrada, AAD nota_marca; etiqueta_id 0 = solo nota, por eso sin FK: se limpia por
	// código); grupo_miembros (quién está en cada grupo: rol en el grupo, desde/hasta, contador de mensajes y las banderas silenciado/
	// advertido que llenará E5); grupo_cambios (entradas, salidas, expulsiones, ascensos); persona_foto (foto de perfil de quien no
	// tiene chat privado). Sin columnas de texto de usuario salvo la nota de la marca.
	{
		`ALTER TABLE etiquetas ADD COLUMN emoji TEXT NOT NULL DEFAULT ''`,
		`CREATE TABLE persona_etiqueta (
			bot_id INTEGER NOT NULL,
			persona TEXT NOT NULL,
			etiqueta_id INTEGER NOT NULL REFERENCES etiquetas(id) ON DELETE CASCADE,
			PRIMARY KEY (bot_id, persona, etiqueta_id)
		) WITHOUT ROWID`,
		`CREATE INDEX idx_persona_etiqueta ON persona_etiqueta(bot_id, etiqueta_id)`,
		`CREATE TABLE msg_marca (
			bot_id INTEGER NOT NULL,
			chat_jid TEXT NOT NULL,
			msg_id TEXT NOT NULL,
			etiqueta_id INTEGER NOT NULL DEFAULT 0,
			nota_cifrado BLOB,
			ts INTEGER NOT NULL DEFAULT 0,
			PRIMARY KEY (bot_id, chat_jid, msg_id, etiqueta_id)
		) WITHOUT ROWID`,
		`CREATE INDEX idx_msg_marca_etiqueta ON msg_marca(bot_id, etiqueta_id, ts)`,
		`CREATE TABLE grupo_miembros (
			bot_id INTEGER NOT NULL,
			grupo TEXT NOT NULL,
			persona TEXT NOT NULL,
			jid TEXT NOT NULL DEFAULT '',
			rol TEXT NOT NULL DEFAULT 'miembro',
			desde_ts INTEGER NOT NULL DEFAULT 0,
			hasta_ts INTEGER NOT NULL DEFAULT 0,
			msgs INTEGER NOT NULL DEFAULT 0,
			silenciado INTEGER NOT NULL DEFAULT 0,
			advertido INTEGER NOT NULL DEFAULT 0,
			PRIMARY KEY (bot_id, grupo, persona)
		) WITHOUT ROWID`,
		`CREATE INDEX idx_grupo_miembros_persona ON grupo_miembros(bot_id, persona)`,
		`CREATE TABLE grupo_cambios (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			bot_id INTEGER NOT NULL,
			grupo TEXT NOT NULL,
			ts INTEGER NOT NULL,
			tipo TEXT NOT NULL,
			persona TEXT NOT NULL DEFAULT '',
			por TEXT NOT NULL DEFAULT '',
			via TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE INDEX idx_grupo_cambios ON grupo_cambios(bot_id, grupo, ts)`,
		`CREATE INDEX idx_grupo_cambios_persona ON grupo_cambios(bot_id, persona, ts)`,
		`CREATE TABLE persona_foto (
			bot_id INTEGER NOT NULL,
			persona TEXT NOT NULL,
			ruta TEXT NOT NULL DEFAULT '',
			foto_id TEXT NOT NULL DEFAULT '',
			ts INTEGER NOT NULL DEFAULT 0,
			PRIMARY KEY (bot_id, persona)
		) WITHOUT ROWID`,
	},
	// 22 · v0.6.0 etapa E5: silenciar en grupos y advertencias con sanciones. silencio_grupo (un silencio de una persona en uno,
	// varios o todos los grupos, con vencimiento; el texto editable de ese silencio va cifrado con AAD texto_silencio) y sus grupos;
	// silencio_conteo (intentos borrados por día lógico y persona/grupo, avisos mandados y la sanción ya aplicada; con persona ''
	// lleva el tope global de avisos del día y con grupo 'na:...' los avisos de "no soy admin" de ese día); silencio_textos (los
	// textos editables de los avisos, cifrados con AAD texto_silencio); sancion_pendiente (expulsiones que esperan mi confirmación,
	// del silencio o de la escalera de advertencias). Advertencias: adv_tipos (Spam, Insultos, Enlaces, Otro se siembran desde código;
	// los nombres van en claro), adv_escalera (pasos por tipo; el texto del aviso cifrado con AAD aviso_esc), advertencias gana
	// tipo_id, caduca_ts (0 = no caduca: así quedan las que ya existían) y regla_id; grupo_regla_items (reglas del grupo como
	// lista; título y texto cifrados con AAD titulo_regla y texto_regla; grupo '' = reglas globales).
	{
		`CREATE TABLE silencio_grupo (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			bot_id INTEGER NOT NULL,
			persona TEXT NOT NULL,
			alcance TEXT NOT NULL DEFAULT 'uno',
			hasta_ts INTEGER NOT NULL DEFAULT 0,
			creado_ts INTEGER NOT NULL DEFAULT 0,
			por TEXT NOT NULL DEFAULT '',
			origen TEXT NOT NULL DEFAULT 'manual',
			aviso_ts INTEGER NOT NULL DEFAULT 0,
			avisar INTEGER NOT NULL DEFAULT 1,
			texto_cifrado BLOB,
			activo INTEGER NOT NULL DEFAULT 1,
			fin_ts INTEGER NOT NULL DEFAULT 0,
			fin_motivo TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE INDEX idx_silencio_persona ON silencio_grupo(bot_id, persona, activo, hasta_ts)`,
		`CREATE TABLE silencio_grupo_chats (
			silencio_id INTEGER NOT NULL REFERENCES silencio_grupo(id) ON DELETE CASCADE,
			grupo TEXT NOT NULL,
			PRIMARY KEY (silencio_id, grupo)
		) WITHOUT ROWID`,
		`CREATE TABLE silencio_conteo (
			bot_id INTEGER NOT NULL,
			persona TEXT NOT NULL,
			grupo TEXT NOT NULL,
			dia_logico TEXT NOT NULL,
			n INTEGER NOT NULL DEFAULT 0,
			avisos INTEGER NOT NULL DEFAULT 0,
			ultimo_aviso_ts INTEGER NOT NULL DEFAULT 0,
			sancion_ts INTEGER NOT NULL DEFAULT 0,
			PRIMARY KEY (bot_id, persona, grupo, dia_logico)
		) WITHOUT ROWID`,
		`CREATE TABLE silencio_textos (
			bot_id INTEGER NOT NULL,
			texto_id TEXT NOT NULL,
			texto_cifrado BLOB,
			PRIMARY KEY (bot_id, texto_id)
		) WITHOUT ROWID`,
		`CREATE TABLE sancion_pendiente (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			bot_id INTEGER NOT NULL,
			origen TEXT NOT NULL,
			persona TEXT NOT NULL,
			grupo TEXT NOT NULL DEFAULT '',
			accion TEXT NOT NULL DEFAULT 'expulsar',
			aviso_id INTEGER NOT NULL DEFAULT 0,
			creado_ts INTEGER NOT NULL,
			estado TEXT NOT NULL DEFAULT 'pendiente',
			resuelto_ts INTEGER NOT NULL DEFAULT 0,
			resultado TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE INDEX idx_sancion_pendiente ON sancion_pendiente(bot_id, estado, persona)`,
		`CREATE TABLE adv_tipos (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			bot_id INTEGER NOT NULL,
			nombre TEXT NOT NULL,
			emoji TEXT NOT NULL DEFAULT '',
			color TEXT NOT NULL DEFAULT '',
			posicion INTEGER NOT NULL DEFAULT 0,
			caducidad_dias INTEGER NOT NULL DEFAULT -1,
			regla_id INTEGER NOT NULL DEFAULT 0,
			activo INTEGER NOT NULL DEFAULT 1
		)`,
		`CREATE INDEX idx_adv_tipos ON adv_tipos(bot_id, posicion)`,
		`CREATE TABLE adv_escalera (
			tipo_id INTEGER NOT NULL REFERENCES adv_tipos(id) ON DELETE CASCADE,
			nivel INTEGER NOT NULL,
			accion TEXT NOT NULL,
			minutos INTEGER NOT NULL DEFAULT 0,
			alcance TEXT NOT NULL DEFAULT 'grupo',
			aviso_cifrado BLOB,
			PRIMARY KEY (tipo_id, nivel)
		) WITHOUT ROWID`,
		`ALTER TABLE advertencias ADD COLUMN tipo_id INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE advertencias ADD COLUMN caduca_ts INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE advertencias ADD COLUMN regla_id INTEGER NOT NULL DEFAULT 0`,
		`CREATE INDEX idx_adv_vigentes ON advertencias(bot_id, persona, tipo_id, caduca_ts)`,
		`CREATE TABLE grupo_regla_items (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			bot_id INTEGER NOT NULL,
			grupo TEXT NOT NULL DEFAULT '',
			pos INTEGER NOT NULL DEFAULT 0,
			titulo_cifrado BLOB,
			texto_cifrado BLOB
		)`,
		`CREATE INDEX idx_regla_items ON grupo_regla_items(bot_id, grupo, pos)`,
	},
	// Descargas de video (módulo opcional): el historial de lo que se pidió. Enlace, título, quién lo pidió y miniatura van cifrados.
	{
		`CREATE TABLE descargas_video (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			bot_id INTEGER NOT NULL,
			ts INTEGER NOT NULL,
			persona TEXT NOT NULL DEFAULT '',
			chat TEXT NOT NULL DEFAULT '',
			es_grupo INTEGER NOT NULL DEFAULT 0,
			nombre_cifrado BLOB,
			url_cifrado BLOB,
			titulo_cifrado BLOB,
			subidor_cifrado BLOB,
			mini_cifrada BLOB,
			extractor TEXT NOT NULL DEFAULT '',
			duracion INTEGER NOT NULL DEFAULT 0,
			peso INTEGER NOT NULL DEFAULT 0,
			formato TEXT NOT NULL DEFAULT '',
			calidad TEXT NOT NULL DEFAULT '',
			partes INTEGER NOT NULL DEFAULT 1,
			entrega TEXT NOT NULL DEFAULT '',
			estado TEXT NOT NULL DEFAULT 'en_curso',
			error TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE INDEX idx_descargas_video ON descargas_video(bot_id, ts DESC)`,
		`CREATE INDEX idx_descargas_video_persona ON descargas_video(bot_id, persona, ts)`,
	},
	// Administrar grupos: baneados (no pueden volver; si entran, el bot los saca) y horario de cierre automático por grupo.
	{
		`CREATE TABLE grupo_baneados (
			bot_id INTEGER NOT NULL,
			grupo TEXT NOT NULL,
			persona TEXT NOT NULL,
			ts INTEGER NOT NULL,
			por TEXT NOT NULL DEFAULT '',
			motivo TEXT NOT NULL DEFAULT '',
			PRIMARY KEY (bot_id, grupo, persona)
		) WITHOUT ROWID`,
		`CREATE TABLE grupo_horario (
			bot_id INTEGER NOT NULL,
			grupo TEXT NOT NULL,
			activo INTEGER NOT NULL DEFAULT 0,
			cierra TEXT NOT NULL DEFAULT '22:00',
			abre TEXT NOT NULL DEFAULT '07:00',
			dias INTEGER NOT NULL DEFAULT 127,
			ultimo TEXT NOT NULL DEFAULT '',
			PRIMARY KEY (bot_id, grupo)
		) WITHOUT ROWID`,
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
