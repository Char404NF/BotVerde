package flujo

import (
	"bytes"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"botchat/internal/almacen"
	"botchat/internal/cifrado"
)

// Exportar e importar menús en un archivo cifrado.
//
// FORMATO (extensión .bcm):
//
//	"BCMX" | versión (1) | modo (0 = llave de este teléfono, 1 = contraseña)
//	modo 1: sal (16 bytes) | vueltas de PBKDF2-SHA256 (4 bytes, big-endian)
//	resto: contenido en el formato cifrado BCF1 (AES-256-GCM) de BotChat, con la llave de este teléfono (modo 0) o la derivada de la
//	contraseña (modo 1).
//
// Con contraseña el archivo se puede llevar a otro teléfono; con la llave del teléfono sirve de copia de seguridad en este mismo
// (otro teléfono no podría abrirlo). Importar solo lee datos: valida la versión y la forma, crea los menús APAGADOS y no ejecuta nada.

const (
	magiaExport     = "BCMX"
	versionExport   = 1
	vueltasPBKDF2   = 200_000
	maxMenusExport  = 50
	maxAdjuntoBytes = 8 << 20
	maxExportBytes  = 40 << 20
	minClaveExport  = 6
)

// ErrClaveExport se devuelve cuando la contraseña no abre el archivo.
var ErrClaveExport = errors.New("La contraseña no es correcta o el archivo está dañado.")

type adjuntoExp struct {
	ID     string `json:"id"`
	Tipo   string `json:"tipo"`
	Mime   string `json:"mime,omitempty"`
	Nombre string `json:"nombre,omitempty"`
	Datos  []byte `json:"datos"`
}

type menuExp struct {
	Def      *Def         `json:"def"`
	Activo   bool         `json:"activo"`
	Adjuntos []adjuntoExp `json:"adjuntos,omitempty"`
}

type contenidoExp struct {
	V        int       `json:"v"`
	Programa string    `json:"programa"`
	Creado   int64     `json:"creado"`
	Menus    []menuExp `json:"menus"`
}

// ResultadoImport dice qué se creó al importar.
type ResultadoImport struct {
	IDs      []int64  `json:"ids"`
	Nombres  []string `json:"nombres"`
	Avisos   int      `json:"avisos"`   // observaciones de validación (se importa igual, apagado)
	Omitidos int      `json:"omitidos"` // menús que no se pudieron leer
}

// ExportarMenus junta los menús pedidos (con sus archivos adjuntos) en un archivo cifrado. clave vacía = con la llave de este teléfono.
func ExportarMenus(alm *almacen.Almacen, botID int64, ids []int64, clave string) ([]byte, error) {
	if len(ids) == 0 {
		return nil, errors.New("Elige al menos un menú para exportar.")
	}
	if len(ids) > maxMenusExport {
		return nil, fmt.Errorf("Se pueden exportar hasta %d menús a la vez.", maxMenusExport)
	}
	if clave != "" && len([]rune(clave)) < minClaveExport {
		return nil, fmt.Errorf("La contraseña debe tener al menos %d caracteres.", minClaveExport)
	}
	c := contenidoExp{V: 1, Programa: "BotChat", Creado: time.Now().Unix()}
	total := 0
	for _, id := range ids {
		mn, err := alm.ObtenerMenu(botID, id)
		if err != nil || mn == nil {
			return nil, errors.New("Uno de los menús ya no existe.")
		}
		d, err := Parse(mn.Def)
		if err != nil || d.Propio != nil || mn.Sistema {
			continue // los comandos propios y el saludo de fábrica no se exportan aquí
		}
		d.Alcance.Chats, d.Alcance.Grupos = nil, nil // los chats de este teléfono no valen en otro
		me := menuExp{Def: d, Activo: mn.Activo}
		vistos := map[string]bool{}
		for _, p := range d.Pasos {
			for _, a := range p.Adjuntos {
				if vistos[a.ID] {
					continue
				}
				vistos[a.ID] = true
				b, err := alm.LeerAdjuntoMenu(botID, a.ID)
				if err != nil {
					continue
				}
				total += len(b)
				if total > maxExportBytes {
					return nil, errors.New("Los archivos adjuntos de esos menús pesan demasiado para exportarlos juntos.")
				}
				me.Adjuntos = append(me.Adjuntos, adjuntoExp{ID: a.ID, Tipo: a.Tipo, Mime: a.Mime, Nombre: a.Nombre, Datos: b})
			}
		}
		c.Menus = append(c.Menus, me)
	}
	if len(c.Menus) == 0 {
		return nil, errors.New("Esos menús no se pueden exportar (los comandos propios y el saludo de fábrica van aparte).")
	}
	plano, err := json.Marshal(c)
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	out.WriteString(magiaExport)
	out.WriteByte(versionExport)
	if clave == "" {
		out.WriteByte(0)
		sel, err := alm.SellarArchivo(plano)
		if err != nil {
			return nil, err
		}
		out.Write(sel)
		return out.Bytes(), nil
	}
	sal := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, sal); err != nil {
		return nil, err
	}
	k, err := pbkdf2.Key(sha256.New, clave, sal, vueltasPBKDF2, cifrado.TamClave)
	if err != nil {
		return nil, err
	}
	sel, err := cifrado.CifrarBytes(k, plano)
	if err != nil {
		return nil, err
	}
	out.WriteByte(1)
	out.Write(sal)
	var v [4]byte
	binary.BigEndian.PutUint32(v[:], vueltasPBKDF2)
	out.Write(v[:])
	out.Write(sel)
	return out.Bytes(), nil
}

// ImportarMenus lee un archivo de ExportarMenus y crea los menús APAGADOS. Valida la versión y la forma; no ejecuta nada.
func ImportarMenus(alm *almacen.Almacen, botID int64, datos []byte, clave string) (*ResultadoImport, error) {
	if len(datos) < 6 || string(datos[:4]) != magiaExport {
		return nil, errors.New("Ese archivo no es una exportación de menús de Bot Chat.")
	}
	if datos[4] != versionExport {
		return nil, errors.New("Ese archivo es de una versión de Bot Chat que esta app no conoce.")
	}
	if len(datos) > maxExportBytes+(1<<20) {
		return nil, errors.New("El archivo es demasiado grande.")
	}
	var plano []byte
	var err error
	switch datos[5] {
	case 0:
		plano, err = alm.AbrirArchivoSellado(datos[6:])
		if err != nil {
			return nil, errors.New("Este archivo se hizo con la llave de otro teléfono. Para llevar menús a otro teléfono, expórtalos con contraseña.")
		}
	case 1:
		if len(datos) < 6+16+4 {
			return nil, errors.New("El archivo está dañado.")
		}
		if clave == "" {
			return nil, errors.New("Este archivo tiene contraseña: escríbela para abrirlo.")
		}
		sal := datos[6:22]
		vueltas := int(binary.BigEndian.Uint32(datos[22:26]))
		if vueltas < 10_000 || vueltas > 5_000_000 {
			return nil, errors.New("El archivo está dañado.")
		}
		k, err := pbkdf2.Key(sha256.New, clave, sal, vueltas, cifrado.TamClave)
		if err != nil {
			return nil, err
		}
		plano, err = cifrado.DescifrarBytes(k, datos[26:])
		if err != nil {
			return nil, ErrClaveExport
		}
	default:
		return nil, errors.New("Ese archivo no es una exportación de menús de Bot Chat.")
	}
	var c contenidoExp
	if err := json.Unmarshal(plano, &c); err != nil || c.V != 1 || c.Programa != "BotChat" {
		return nil, errors.New("El contenido del archivo no tiene la forma esperada.")
	}
	if len(c.Menus) == 0 || len(c.Menus) > maxMenusExport {
		return nil, errors.New("El archivo no trae menús válidos.")
	}
	res := &ResultadoImport{IDs: []int64{}, Nombres: []string{}}
	for _, me := range c.Menus {
		d := me.Def
		if d == nil || len(d.Pasos) == 0 || len(d.Pasos) > 300 || d.Propio != nil {
			res.Omitidos++
			continue
		}
		// los adjuntos se guardan de nuevo con ids nuevos; los que no vengan o sean de un tipo raro se quitan
		nuevos := map[string]AdjuntoMenu{}
		for _, a := range me.Adjuntos {
			if len(a.Datos) == 0 || len(a.Datos) > maxAdjuntoBytes || !tipoAdjuntoValido(a.Tipo) {
				continue
			}
			id, _, err := alm.GuardarAdjuntoMenu(botID, bytes.NewReader(a.Datos))
			if err != nil {
				continue
			}
			nuevos[a.ID] = AdjuntoMenu{ID: id, Tipo: a.Tipo, Mime: a.Mime, Nombre: a.Nombre}
		}
		for i := range d.Pasos {
			var ok []AdjuntoMenu
			for _, a := range d.Pasos[i].Adjuntos {
				if n, hay := nuevos[a.ID]; hay {
					ok = append(ok, n)
				}
			}
			d.Pasos[i].Adjuntos = ok
		}
		d.Alcance.Chats, d.Alcance.Grupos = nil, nil
		d.Normalizar()
		d.Nombre = limpiaNombre(d.Nombre)
		for _, av := range d.Validar() {
			_ = av
			res.Avisos++
		}
		id, err := alm.CrearMenu(botID, d.JSON(), false, false)
		if err != nil {
			res.Omitidos++
			continue
		}
		res.IDs = append(res.IDs, id)
		res.Nombres = append(res.Nombres, d.Nombre)
	}
	if len(res.IDs) == 0 {
		return nil, errors.New("El archivo no trae menús válidos.")
	}
	return res, nil
}

func tipoAdjuntoValido(t string) bool {
	switch t {
	case "imagen", "video", "audio", "documento":
		return true
	}
	return false
}

func limpiaNombre(n string) string {
	n = strings.TrimSpace(strings.Join(strings.Fields(n), " "))
	if r := []rune(n); len(r) > 60 {
		n = string(r[:60])
	}
	return n
}
