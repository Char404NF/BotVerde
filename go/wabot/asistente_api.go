package wabot

import (
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"time"

	"go.mau.fi/whatsmeow/types"

	"botchat/internal/almacen"
	"botchat/internal/asistente"
)

// API del asistente «Crea tu bot» (v0.6.0, E9). El borrador vive cifrado en bot_ajustes; Aplicar crea todo de golpe y Deshacer lo quita.

var (
	pruebasAsistMu sync.Mutex
	pruebasAsist   []time.Time
)

const topePruebasAsistHora = 3

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
	case a.AjusteBot(botID, "lab_prueba_chat") != "": // solo se LEE el número que ya se había puesto para pruebas
		e.NumeroPrb, e.NumeroOrig = a.AjusteBot(botID, "lab_prueba_chat"), "laboratorio"
	}
	return js(e, nil)
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
	if viejo := asistente.Leer(a, botID); viejo.Creado != nil {
		b.Creado = viejo.Creado
	} else {
		b.Creado = nil
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

// AsistenteProbar manda la bienvenida del borrador al «otro número» (el de prueba). Es un mensaje manual con confirmación en la pantalla,
// con tope de 3 por hora; el número se guarda solo en el borrador cifrado. Si no hay otro número, devuelve un error que lo explica.
func AsistenteProbar(botID int64, numero string) (string, error) {
	a, err := almacenAbierto()
	if err != nil {
		return "", err
	}
	b := asistente.Leer(a, botID)
	if strings.TrimSpace(numero) == "" {
		numero = b.OtroNumero
	}
	if strings.TrimSpace(numero) == "" {
		numero = a.AjusteBot(botID, "lab_prueba_chat")
	}
	if strings.TrimSpace(numero) == "" {
		return "", errors.New("Para probar necesitas un segundo número (otro teléfono o WhatsApp). Escríbelo aquí y el bot le manda la bienvenida; desde ese número puedes contestar para ver el menú.")
	}
	jid, err := chatDePrueba(numero)
	if err != nil {
		return "", err
	}
	sv, err := servicioEnvio(botID)
	if err != nil {
		return "", err
	}
	mu.Lock()
	s := st
	mu.Unlock()
	if s != nil && s.cli != nil && s.esPropio(jid) {
		return "", errors.New("Ese es el número del propio bot. Usa otro número para la prueba.")
	}
	pruebasAsistMu.Lock()
	corte := time.Now().Add(-time.Hour)
	var vivos []time.Time
	for _, t := range pruebasAsist {
		if t.After(corte) {
			vivos = append(vivos, t)
		}
	}
	if len(vivos) >= topePruebasAsistHora {
		pruebasAsist = vivos
		pruebasAsistMu.Unlock()
		return "", errors.New("Ya mandaste 3 pruebas en la última hora. Espera un rato para no arriesgar tu número.")
	}
	pruebasAsist = append(vivos, time.Now())
	pruebasAsistMu.Unlock()
	if b.OtroNumero != strings.TrimSpace(numero) {
		b.OtroNumero = strings.TrimSpace(numero)
		_ = asistente.Guardar(a, botID, b)
	}
	ctx, cancel := ctxEnvio()
	defer cancel()
	r, err := sv.EnviarTexto(ctx, jid.String(), asistente.TextoDePrueba(b, "Ana"), "", nil)
	if err == nil {
		a.RegistrarAccion(botID, almacen.PersonaDueno, "asistente.probar", "", "", "ok", "")
	}
	return js(r, err)
}
