package almacen

import (
	"database/sql"
	"encoding/base64"
	"errors"

	"botchat/internal/cifrado"
)

// Borrador del asistente «Crea tu bot» (v0.6.0, E9). Vive en bot_ajustes (sin migración): la clave NO está en ajustesBotDefecto, así que
// no sale en AjustesBot; el valor es el JSON cifrado por columna (AAD "asistente_borrador") en base64.
const claveBorradorAsistente = "asistente_borrador"

// GuardarBorradorAsistente guarda el borrador cifrado. Un borrador vacío lo borra.
func (a *Almacen) GuardarBorradorAsistente(botID int64, json []byte) error {
	if len(json) == 0 {
		_, err := a.db.Exec(`DELETE FROM bot_ajustes WHERE bot_id=? AND clave=?`, botID, claveBorradorAsistente)
		return err
	}
	if len(json) > 256<<10 {
		return errors.New("el borrador es demasiado grande")
	}
	sel, err := cifrado.SellarTexto(a.clave, "asistente_borrador", json)
	if err != nil {
		return err
	}
	_, err = a.db.Exec(`INSERT INTO bot_ajustes(bot_id, clave, valor) VALUES(?,?,?) ON CONFLICT(bot_id,clave) DO UPDATE SET valor=excluded.valor`,
		botID, claveBorradorAsistente, base64.StdEncoding.EncodeToString(sel))
	return err
}

// LeerBorradorAsistente devuelve el borrador guardado (nil si no hay o no se puede abrir).
func (a *Almacen) LeerBorradorAsistente(botID int64) []byte {
	var v string
	if err := a.db.QueryRow(`SELECT valor FROM bot_ajustes WHERE bot_id=? AND clave=?`, botID, claveBorradorAsistente).Scan(&v); err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return nil
	}
	b, err := base64.StdEncoding.DecodeString(v)
	if err != nil {
		return nil
	}
	t, err := cifrado.AbrirTexto(a.clave, "asistente_borrador", b)
	if err != nil {
		return nil
	}
	return t
}

// ObtenerEtiquetaPorID dice si existe la etiqueta (para el Deshacer del asistente).
func (a *Almacen) ExisteEtiqueta(botID, id int64) bool {
	var x int
	return a.db.QueryRow(`SELECT 1 FROM etiquetas WHERE bot_id=? AND id=?`, botID, id).Scan(&x) == nil
}

// BorradorCrudoAsistente devuelve lo que hay en disco (cifrado, en base64); sirve para comprobar que nada va en claro.
func (a *Almacen) BorradorCrudoAsistente(botID int64) string {
	var v string
	_ = a.db.QueryRow(`SELECT valor FROM bot_ajustes WHERE bot_id=? AND clave=?`, botID, claveBorradorAsistente).Scan(&v)
	return v
}
