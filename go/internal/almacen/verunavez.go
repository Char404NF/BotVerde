package almacen

import (
	"errors"
	"io"
	"os"
	"time"

	"botchat/internal/cifrado"
)

// Estados de un "ver una vez" con contenido.
const (
	VerPendiente = "pendiente" // guardado, esperando que se abra
	VerAbierto   = "abierto"   // se está viendo ahora
	VerVisto     = "visto"     // ya se vio: el contenido se borró
	VerExpirado  = "expirado"  // no se abrió a tiempo: el contenido se borró
)

// ErrVerUnaVez se muestra cuando ya no hay nada que abrir.
var ErrVerUnaVez = errors.New("Este mensaje de “ver una vez” ya no está disponible.")

// AbrirVerUnaVez deja abrir el contenido una sola vez: pasa de pendiente a abierto y devuelve la ruta del archivo cifrado.
func (a *Almacen) AbrirVerUnaVez(botID int64, chat, id string, ahora time.Time) (ruta, tipo, mime string, err error) {
	m, err := a.ObtenerMensaje(botID, chat, id)
	if err != nil {
		return "", "", "", err
	}
	if m == nil || m.Tipo != TipoVerUnaVez || m.VerEstado != VerPendiente || m.ArchivoEstado != EstadoDescargado || m.ArchivoRuta == "" || m.Borrado {
		return "", "", "", ErrVerUnaVez
	}
	if m.VerExpira > 0 && m.VerExpira < ahora.Unix() {
		_ = a.cerrarVerUnaVez(m, VerExpirado)
		return "", "", "", ErrVerUnaVez
	}
	if abs, e := a.Abs(m.ArchivoRuta); e != nil {
		return "", "", "", ErrVerUnaVez
	} else if _, e := os.Stat(abs); e != nil {
		return "", "", "", ErrVerUnaVez
	}
	r, err := a.db.Exec(`UPDATE messages SET ver_estado=? WHERE bot_id=? AND chat_jid=? AND id=? AND ver_estado=?`, VerAbierto, botID, chat, id, VerPendiente)
	if err != nil {
		return "", "", "", err
	}
	if n, _ := r.RowsAffected(); n == 0 {
		return "", "", "", ErrVerUnaVez
	}
	a.avisaMsg(chat, id)
	return m.ArchivoRuta, m.VerTipo, m.ArchivoMime, nil
}

// CerrarVerUnaVez termina de ver el contenido: se borra para siempre y queda "Ya visto". Devuelve false si no estaba abierto.
func (a *Almacen) CerrarVerUnaVez(botID int64, chat, id string) (bool, error) {
	m, err := a.ObtenerMensaje(botID, chat, id)
	if err != nil || m == nil || m.Tipo != TipoVerUnaVez || m.VerEstado != VerAbierto {
		return false, err
	}
	return true, a.cerrarVerUnaVez(m, VerVisto)
}

func (a *Almacen) cerrarVerUnaVez(m *Mensaje, estado string) error {
	if err := a.olvidar(m); err != nil {
		return err
	}
	if _, err := a.db.Exec(`UPDATE messages SET ver_estado=? WHERE bot_id=? AND chat_jid=? AND id=?`, estado, m.BotID, m.ChatJID, m.ID); err != nil {
		return err
	}
	a.avisaMsg(m.ChatJID, m.ID)
	a.avisa("chats")
	a.avisa("espacio")
	return nil
}

// BarrerVerUnaVez cierra los que quedaron abiertos (la app se cerró mientras se veían) y borra los que caducaron.
func (a *Almacen) BarrerVerUnaVez(botID int64, ahora time.Time) (cerrados, caducados int, err error) {
	filas, err := a.db.Query(`SELECT id, chat_jid, ver_estado FROM messages WHERE bot_id=? AND tipo=? AND (ver_estado=? OR (ver_estado=? AND ver_expira>0 AND ver_expira<?))`,
		botID, TipoVerUnaVez, VerAbierto, VerPendiente, ahora.Unix())
	if err != nil {
		return 0, 0, err
	}
	type par struct{ id, chat, est string }
	var lista []par
	for filas.Next() {
		var p par
		if err := filas.Scan(&p.id, &p.chat, &p.est); err != nil {
			filas.Close()
			return 0, 0, err
		}
		lista = append(lista, p)
	}
	filas.Close()
	for _, p := range lista {
		m, err := a.ObtenerMensaje(botID, p.chat, p.id)
		if err != nil || m == nil {
			continue
		}
		if p.est == VerAbierto {
			if a.cerrarVerUnaVez(m, VerVisto) == nil {
				cerrados++
			}
		} else if a.cerrarVerUnaVez(m, VerExpirado) == nil {
			caducados++
		}
	}
	return cerrados, caducados, nil
}

// PromoverVerUnaVez completa un "ver una vez" que antes llegó sin contenido (aviso) y ahora llegó completo. Devuelve false si no había aviso.
func (a *Almacen) PromoverVerUnaVez(n *Mensaje) (bool, error) {
	info, err := a.ObtenerMensaje(n.BotID, n.ChatJID, n.ID)
	if err != nil || info == nil || info.Tipo != TipoVerUnaVez || info.VerEstado != "" {
		return false, err
	}
	sel, err := cifrado.SellarTexto(a.clave, "archivo_info", n.ArchivoInfo)
	if err != nil {
		return false, err
	}
	_, err = a.db.Exec(`UPDATE messages SET archivo_info_cifrado=?, archivo_mime=?, archivo_bytes=?, ancho=?, alto=?, duracion_s=?, nota_voz=?, archivo_estado=?, ver_estado=?, ver_tipo=?, ver_expira=?
		WHERE bot_id=? AND chat_jid=? AND id=?`, sel, n.ArchivoMime, n.ArchivoBytes, n.Ancho, n.Alto, n.DuracionS, b2i(n.NotaVoz), EstadoPendiente, VerPendiente, n.VerTipo, n.VerExpira, n.BotID, n.ChatJID, n.ID)
	if err == nil {
		a.avisaMsg(n.ChatJID, n.ID)
	}
	return err == nil, err
}

// Descifrado es un archivo guardado, abierto para leerlo ya descifrado.
type Descifrado struct {
	io.Reader
	Tam int64
	f   *os.File
}

// Close cierra el archivo.
func (d *Descifrado) Close() error { return d.f.Close() }

// AbrirDescifrado abre un archivo cifrado de la carpeta de datos para leer su contenido en claro (en flujo, sin cargarlo entero).
func (a *Almacen) AbrirDescifrado(rel string) (*Descifrado, error) {
	abs, err := a.Abs(rel)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(abs)
	if err != nil {
		return nil, err
	}
	st, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, err
	}
	l, err := cifrado.Abrir(f, st.Size(), a.clave)
	if err != nil {
		f.Close()
		return nil, err
	}
	return &Descifrado{Reader: io.NewSectionReader(l, 0, l.Tam()), Tam: l.Tam(), f: f}, nil
}
