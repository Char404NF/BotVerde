package wabot

import (
	"botchat/internal/permisos"
	"encoding/json"
	"errors"
	"strings"

	"go.mau.fi/whatsmeow/types"

	"botchat/internal/almacen"
	"botchat/internal/asistente"
	"botchat/internal/avatar"
	"botchat/internal/modulos"
)

// API del asistente «Crea tu bot» (v0.6.0, E9). El borrador vive cifrado en bot_ajustes; Aplicar crea todo de golpe y Deshacer lo quita.

type estadoAsistente struct {
	Borrador   *asistente.Borrador `json:"borrador"`
	Problemas  []string            `json:"problemas"`
	Visto      bool                `json:"visto"`
	Aplicado   bool                `json:"aplicado"`
	NumeroPrb  string              `json:"numeroPrueba"`   // el otro número ya configurado ("" = no hay)
	NumeroOrig string              `json:"numeroPruebaDe"` // asistente | laboratorio
}

// AsistenteEstado devuelve (JSON) el borrador (o uno nuevo con ejemplos), lo que le falta, si ya se aplicó y qué otro número hay para probar.
func AsistenteEstado(botID int64) (string, error) {
	a, err := almacenAbierto()
	if err != nil {
		return "", err
	}
	b := asistente.Leer(a, botID)
	if len(a.LeerBorradorAsistente(botID)) == 0 {
		b.DesdeAjustes(a, botID) // primera vez: los interruptores del paso 5 empiezan como están hoy
	}
	e := estadoAsistente{Borrador: b, Problemas: b.Problemas(), Visto: a.AjusteBot(botID, "asistente_visto") == "1", Aplicado: b.Creado != nil}
	if e.Problemas == nil {
		e.Problemas = []string{}
	}
	switch {
	case b.OtroNumero != "":
		e.NumeroPrb, e.NumeroOrig = b.OtroNumero, "asistente"
	}
	return js(e, nil)
}

// AsistenteBase deja guardado el borrador del «bot base» (listo para usar) y devuelve el estado como AsistenteEstado. Lo ya aplicado se conserva.
func AsistenteBase(botID int64) (string, error) {
	a, err := almacenAbierto()
	if err != nil {
		return "", err
	}
	b := asistente.NuevoBase()
	if viejo := asistente.Leer(a, botID); viejo.Creado != nil {
		b.Creado = viejo.Creado
	}
	b.DesdeAjustes(a, botID)
	// La bienvenida lleva la cara feliz del avatar (la imagen viaja dentro de la app).
	if len(b.Bienvenida.Adjuntos) == 0 {
		if id, mime, nombre, ok := avatar.Nuevo(a, botID).Imagen(avatar.Bienvenida); ok {
			b.Bienvenida.Adjuntos = []almacen.AdjuntoRef{{ID: id, Tipo: almacen.TipoImagen, Mime: mime, Nombre: nombre}}
		}
	}
	if err := asistente.Guardar(a, botID, b); err != nil {
		return "", err
	}
	// El rango Admin de fábrica (si el bot todavía no lo tiene), para que el bot base ya traiga sus tres niveles: Dueño, Admin y todos los demás.
	if p, err := permisos.Nuevo(a, botID); err == nil {
		_, _ = p.AsegurarAdmin()
		p.CompletarUsuario() // el rango de todos puede usar stickers, descargas, IA, notas y juegos
	}
	// Módulos de uso diario (solo responden a comandos y traen sus topes). Los que ya estaban encendidos se dejan como están.
	if g, err := gestorDe(botID); err == nil {
		for _, id := range []string{modulos.IDExtras, modulos.IDJuegos} {
			if !g.Activo(id) {
				_ = g.Activar(id)
			}
		}
	}
	return AsistenteEstado(botID)
}

// AsistenteGuardar guarda el borrador (JSON) con el paso en que va.
func AsistenteGuardar(botID int64, borradorJSON string) error {
	a, err := almacenAbierto()
	if err != nil {
		return err
	}
	b := &asistente.Borrador{}
	if err := json.Unmarshal([]byte(borradorJSON), b); err != nil {
		return errors.New("El borrador no es válido.")
	}
	// Lo que ya se aplicó es del servidor, no del teléfono: se conserva aunque Kotlin no lo mande.
	viejo := asistente.Leer(a, botID)
	if viejo.Creado != nil {
		b.Creado = viejo.Creado
	} else {
		b.Creado = nil
	}
	// «Fabrica» (el menú completo del Bot para todos) no viaja por el teléfono: sigue mientras no se cambien las opciones del paso del menú.
	if viejo.Menu.Fabrica && b.Menu.Titulo == viejo.Menu.Titulo && len(b.Menu.Opciones) == len(viejo.Menu.Opciones) {
		igual := true
		for i, o := range b.Menu.Opciones {
			if o != viejo.Menu.Opciones[i] {
				igual = false
			}
		}
		b.Menu.Fabrica = igual
	}
	return asistente.Guardar(a, botID, b)
}

// AsistenteVisto marca que el asistente ya se ofreció (para no insistir).
func AsistenteVisto(botID int64) error {
	a, err := almacenAbierto()
	if err != nil {
		return err
	}
	return a.FijarAjusteBot(botID, "asistente_visto", "1")
}

// AsistenteAplicar crea todo lo del borrador guardado. Devuelve (JSON) lo creado y los avisos.
func AsistenteAplicar(botID int64) (string, error) {
	a, err := almacenAbierto()
	if err != nil {
		return "", err
	}
	var mods asistente.Modulos
	if g, err := gestorDe(botID); err == nil {
		mods = g
	}
	b := asistente.Leer(a, botID)
	c, err := asistente.Aplicar(a, botID, b, mods)
	if err != nil {
		return "", err
	}
	a.RegistrarAccion(botID, almacen.PersonaDueno, "asistente.aplicar", "", "", "ok", "")
	_ = a.FijarAjusteBot(botID, "asistente_visto", "1")
	return js(c, nil)
}

// AsistenteDeshacer quita lo que creó el asistente (las reglas del grupo no se tocan).
func AsistenteDeshacer(botID int64) error {
	a, err := almacenAbierto()
	if err != nil {
		return err
	}
	var mods asistente.Modulos
	if g, err := gestorDe(botID); err == nil {
		mods = g
	}
	b := asistente.Leer(a, botID)
	if err := asistente.Deshacer(a, botID, b, mods); err != nil {
		return err
	}
	a.RegistrarAccion(botID, almacen.PersonaDueno, "asistente.deshacer", "", "", "ok", "")
	return nil
}

// AsistenteDescartar borra el borrador (no deshace lo ya aplicado).
func AsistenteDescartar(botID int64) error {
	a, err := almacenAbierto()
	if err != nil {
		return err
	}
	return asistente.Descartar(a, botID)
}

// chatDePrueba convierte lo que escribió el usuario en un chat privado. Solo números de persona (no grupos).
func chatDePrueba(numero string) (types.JID, error) {
	n := strings.TrimSpace(numero)
	if strings.HasSuffix(n, "@g.us") {
		return types.JID{}, errors.New("La prueba va a un número, no a un grupo.")
	}
	if i := strings.Index(n, "@"); i > 0 {
		n = n[:i]
	}
	var d strings.Builder
	for _, r := range n {
		if r >= '0' && r <= '9' {
			d.WriteRune(r)
		}
	}
	if len(d.String()) < 8 || len(d.String()) > 15 || strings.HasPrefix(d.String(), "0") {
		return types.JID{}, errors.New("Escribe el número completo con el código de país, solo dígitos (por ejemplo 5215512345678).")
	}
	return types.NewJID(d.String(), types.DefaultUserServer), nil
}
