// Package asistente es «Crea tu bot» (v0.6.0, E9): cinco pasos (dónde responde, bienvenida, menú, reglas, utilidades) que se guardan
// como borrador cifrado y se aplican de golpe. Solo CREA cosas nuevas y marcadas, y recuerda qué creó para poder deshacerlo; nunca
// edita lo que ya hizo el usuario. Todo lo que arma es RESPUESTA a un mensaje: no escribe primero (salvo el módulo H1 de reglas al entrar
// a un grupo, que el usuario enciende a propósito y con aviso).
package asistente

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"botchat/internal/almacen"
	"botchat/internal/auto"
	"botchat/internal/flujo"
)

// Pasos del asistente (6 = resumen final).
const (
	PasoDonde = 1 + iota
	PasoBienvenida
	PasoMenu
	PasoReglas
	PasoUtilidades
	PasoFinal
)

// IDBienvenidaModulo es el módulo H1 (bienvenida a quien entra a un grupo).
const IDBienvenidaModulo = "bienvenida"

// Opcion es una tarjeta del menú: lo que ve la persona y lo que se le contesta.
type Opcion struct {
	Texto     string `json:"texto"`
	Respuesta string `json:"respuesta"`
}

// Borrador es todo lo que el usuario fue eligiendo.
type Borrador struct {
	V        int   `json:"v"`
	Paso     int   `json:"paso"`
	Saltados []int `json:"saltados,omitempty"`

	// 1) Dónde responde
	Privados       bool     `json:"privados"`
	Grupos         bool     `json:"grupos"`
	GruposElegidos []string `json:"gruposElegidos,omitempty"`

	// 2) Bienvenida
	Bienvenida struct {
		Activa   bool                 `json:"activa"`
		Texto    string               `json:"texto"`
		Adjuntos []almacen.AdjuntoRef `json:"adjuntos,omitempty"`
		Cuando   string               `json:"cuando"` // primera | diaria
	} `json:"bienvenida"`

	// 3) Menú
	Menu struct {
		Activo   bool     `json:"activo"`
		Titulo   string   `json:"titulo"`
		Opciones []Opcion `json:"opciones"`
	} `json:"menu"`

	// 4) Reglas (la lista vive en las reglas generales de siempre: aquí solo se elige a quién se mandan)
	Reglas struct {
		AlEntrar bool `json:"alEntrar"` // enciende el módulo de bienvenida a quien entra a un grupo (iniciativa)
		AvisoOk  bool `json:"avisoOk"`  // «entiendo que el bot escribe solo al entrar alguien»
	} `json:"reglas"`

	// 5) Utilidades
	Util struct {
		Silenciar     bool `json:"silenciar"`
		Advertencias  bool `json:"advertencias"`
		Etiquetas     bool `json:"etiquetas"`
		Recordatorios bool `json:"recordatorios"`
		Notas         bool `json:"notas"`
	} `json:"util"`

	OtroNumero string `json:"otroNumero,omitempty"` // para «Probar mi bot» (solo se guarda aquí, cifrado)

	Creado *Creado `json:"creado,omitempty"`
}

// Creado recuerda lo que Aplicar creó o cambió, para Deshacer.
type Creado struct {
	TS         int64             `json:"ts"`
	Autos      []int64           `json:"autos,omitempty"`
	Menu       int64             `json:"menu,omitempty"`
	Etiquetas  []int64           `json:"etiquetas,omitempty"`
	Ajustes    map[string]string `json:"ajustes,omitempty"` // valor anterior de cada ajuste que se tocó
	Modulo     bool              `json:"modulo,omitempty"`  // el asistente encendió el módulo de bienvenida de grupo
	ModuloChat []string          `json:"moduloChat,omitempty"`
	ModuloPrev string            `json:"moduloPrev,omitempty"` // configuración que tenía el módulo antes
	Avisos     []string          `json:"avisos,omitempty"`
}

// Modulos es lo que el asistente necesita del gestor de módulos.
type Modulos interface {
	Activo(id string) bool
	Activar(id string) error
	Desactivar(id string) error
	GuardarConfig(id, texto string) error
	ConfigJSON(id string) (string, error)
}

// Nuevo es el borrador de arranque, con ejemplos que el usuario puede cambiar.
func Nuevo() *Borrador {
	b := &Borrador{V: 1, Paso: PasoDonde, Privados: true}
	b.Bienvenida.Activa = true
	b.Bienvenida.Cuando = "primera"
	b.Bienvenida.Texto = "¡Hola {nombre}! Gracias por escribirnos. En un momento te atendemos. Escribe *menu* para ver las opciones."
	b.Menu.Activo = true
	b.Menu.Titulo = "¿En qué te ayudo?"
	b.Menu.Opciones = []Opcion{
		{"Horarios", "Atendemos de lunes a viernes de 9:00 a 18:00. (Cambia este texto por tu horario.)"},
		{"Ubicación", "Estamos en: escribe aquí tu dirección."},
		{"Precios", "Pídenos tu cotización y te respondemos en cuanto podamos."},
	}
	b.Util.Silenciar, b.Util.Advertencias, b.Util.Etiquetas, b.Util.Recordatorios, b.Util.Notas = true, true, true, true, true
	return b
}

// RepoBotBase es el proyecto del bot (código abierto). El botón «GitHub» del saludo lleva aquí.
const RepoBotBase = "https://github.com/Char404NF/BotVerde"

// NuevoBase es el borrador del «bot base»: listo para usar, con un saludo y tres opciones (GitHub, Menú y Ayuda). Todo se puede cambiar
// en los pasos del asistente, como cualquier otro borrador.
func NuevoBase() *Borrador {
	b := Nuevo()
	b.Grupos = true
	b.Bienvenida.Texto = "¡Hola {nombre}! 👋 Soy tu bot. Toca una opción para empezar."
	b.Menu.Titulo = "¿Qué quieres hacer?"
	b.Menu.Opciones = []Opcion{
		{"GitHub", "Este proyecto es de código abierto 💚\n" + RepoBotBase},
		{"Menú", "Esto puedes hacer:\n• !ayuda — te explico cada comando\n• !reglas — las reglas del grupo\n• !grupo — datos del grupo\n\nLos administradores y el dueño tienen más comandos: escribe !ayuda para ver los tuyos."},
		{"Ayuda", "Escribe *!ayuda* para ver lo que puedes hacer, o *!ayuda* y el nombre de un comando (por ejemplo *!ayuda silenciar*) y te lo explico con un ejemplo. Si te pierdes, escribe *menu*."},
	}
	return b
}

// Leer carga el borrador guardado (o uno nuevo si no hay).
func Leer(a *almacen.Almacen, botID int64) *Borrador {
	if raw := a.LeerBorradorAsistente(botID); len(raw) > 0 {
		b := &Borrador{}
		if json.Unmarshal(raw, b) == nil && b.V > 0 {
			b.Normalizar()
			return b
		}
	}
	return Nuevo()
}

// Guardar guarda el borrador (cifrado).
func Guardar(a *almacen.Almacen, botID int64, b *Borrador) error {
	b.Normalizar()
	raw, err := json.Marshal(b)
	if err != nil {
		return err
	}
	return a.GuardarBorradorAsistente(botID, raw)
}

// Descartar borra el borrador (no deshace lo ya aplicado).
func Descartar(a *almacen.Almacen, botID int64) error { return a.GuardarBorradorAsistente(botID, nil) }

// Normalizar limpia lo que el usuario escribió y acota tamaños.
func (b *Borrador) Normalizar() {
	if b.V == 0 {
		b.V = 1
	}
	if b.Paso < PasoDonde {
		b.Paso = PasoDonde
	}
	if b.Paso > PasoFinal {
		b.Paso = PasoFinal
	}
	if b.Bienvenida.Cuando != "diaria" {
		b.Bienvenida.Cuando = "primera"
	}
	b.Bienvenida.Texto = strings.TrimSpace(b.Bienvenida.Texto)
	if len([]rune(b.Bienvenida.Texto)) > 900 {
		b.Bienvenida.Texto = string([]rune(b.Bienvenida.Texto)[:900])
	}
	if len(b.Bienvenida.Adjuntos) > 3 {
		b.Bienvenida.Adjuntos = b.Bienvenida.Adjuntos[:3]
	}
	b.Menu.Titulo = strings.TrimSpace(b.Menu.Titulo)
	var ops []Opcion
	for _, o := range b.Menu.Opciones {
		o.Texto, o.Respuesta = strings.TrimSpace(o.Texto), strings.TrimSpace(o.Respuesta)
		if o.Texto == "" {
			continue
		}
		if len([]rune(o.Texto)) > 60 {
			o.Texto = string([]rune(o.Texto)[:60])
		}
		if len([]rune(o.Respuesta)) > 900 {
			o.Respuesta = string([]rune(o.Respuesta)[:900])
		}
		ops = append(ops, o)
		if len(ops) == 10 {
			break
		}
	}
	b.Menu.Opciones = ops
	seen := map[string]bool{}
	var gs []string
	for _, g := range b.GruposElegidos {
		if strings.HasSuffix(g, "@g.us") && !seen[g] && len(gs) < 200 {
			seen[g] = true
			gs = append(gs, g)
		}
	}
	b.GruposElegidos = gs
}

func (b *Borrador) saltado(p int) bool {
	for _, x := range b.Saltados {
		if x == p {
			return true
		}
	}
	return false
}

// Qué secciones se aplican de verdad: una sección saltada (o apagada) no crea ni cambia nada.
func (b *Borrador) conBienvenida() bool { return b.Bienvenida.Activa && !b.saltado(PasoBienvenida) }
func (b *Borrador) conMenu() bool       { return b.Menu.Activo && !b.saltado(PasoMenu) }
func (b *Borrador) conReglas() bool     { return b.Reglas.AlEntrar && !b.saltado(PasoReglas) }
func (b *Borrador) conUtil() bool       { return !b.saltado(PasoUtilidades) }

// DesdeAjustes pone los interruptores del paso 5 como están hoy (para que «Siguiente» sin tocar nada no cambie nada).
func (b *Borrador) DesdeAjustes(a *almacen.Almacen, botID int64) {
	b.Util.Silenciar = a.AjusteBot(botID, "sil_activo") != "0"
	b.Util.Advertencias = a.AjusteBot(botID, "adv_tope") != "0"
	b.Util.Notas = a.AjusteBot(botID, "util_notas") != "0"
	b.Util.Recordatorios = a.AjusteBot(botID, "util_recordatorios") != "0"
	b.Util.Etiquetas = true
}

// Problemas dice en español simple qué le falta al borrador para poder aplicarse ("" lista vacía = listo).
func (b *Borrador) Problemas() []string {
	var p []string
	if (b.conBienvenida() || b.conMenu()) && !b.Privados && !(b.Grupos && len(b.GruposElegidos) > 0) {
		p = append(p, "Elige dónde responde: chats privados o al menos un grupo.")
	}
	if b.conBienvenida() && b.Bienvenida.Texto == "" && len(b.Bienvenida.Adjuntos) == 0 {
		p = append(p, "Escribe el mensaje de bienvenida o apágala.")
	}
	if b.conMenu() && len(b.Menu.Opciones) == 0 {
		p = append(p, "El menú necesita al menos una opción, o apágalo.")
	}
	if b.conMenu() {
		for _, o := range b.Menu.Opciones {
			if o.Respuesta == "" {
				p = append(p, fmt.Sprintf("La opción «%s» no tiene respuesta.", o.Texto))
			}
		}
	}
	if b.conReglas() && !b.Reglas.AvisoOk {
		p = append(p, "Para mandar las reglas a quien entra a un grupo, marca que entiendes que el bot escribe solo en ese momento.")
	}
	if b.conReglas() && !(b.Grupos && len(b.GruposElegidos) > 0) {
		p = append(p, "Para mandar las reglas a quien entra, elige al menos un grupo en el paso 1.")
	}
	return p
}

func (b *Borrador) quien() auto.Quien {
	q := auto.Quien{Personas: "todos", Privados: b.Privados, Grupos: b.Grupos && len(b.GruposElegidos) > 0}
	if q.Grupos {
		q.GruposElegidos = append([]string(nil), b.GruposElegidos...)
	}
	return q
}

// DefMenu arma la automatización del menú con las tarjetas del borrador.
func (b *Borrador) DefMenu() *flujo.Def {
	d := &flujo.Def{V: 1, Nombre: "Mi menú (asistente)", Inicio: "menu"}
	titulo := b.Menu.Titulo
	if titulo == "" {
		titulo = "¿En qué te ayudo?"
	}
	m := flujo.Paso{ID: "menu", Tipo: flujo.PasoMenu, Titulo: titulo}
	for i, o := range b.Menu.Opciones {
		id := fmt.Sprintf("r%d", i+1)
		m.Opciones = append(m.Opciones, flujo.Opcion{Texto: o.Texto, Destino: id})
		d.Pasos = append(d.Pasos, flujo.Paso{ID: id, Tipo: flujo.PasoRespuesta, Texto: o.Respuesta})
	}
	d.Pasos = append([]flujo.Paso{m}, d.Pasos...)
	d.Disparadores = flujo.Disparadores{Palabras: []string{"menu", "menú", "opciones"}, Comando: ".menu"}
	d.Alcance = flujo.Alcance{Privados: b.Privados}
	if b.Grupos && len(b.GruposElegidos) > 0 {
		d.Alcance.Grupos = append([]string(nil), b.GruposElegidos...)
	}
	d.Normalizar()
	return d
}

func fijaAjuste(a *almacen.Almacen, botID int64, c *Creado, clave, valor string) error {
	if c.Ajustes == nil {
		c.Ajustes = map[string]string{}
	}
	if _, ya := c.Ajustes[clave]; !ya {
		c.Ajustes[clave] = a.AjusteBot(botID, clave)
	}
	return a.FijarAjusteBot(botID, clave, valor)
}

func si(v bool) string {
	if v {
		return "1"
	}
	return "0"
}

// Aplicar crea todo de golpe. Si ya había algo aplicado antes, lo deshace primero (solo lo que el asistente creó). Si algo falla,
// deshace lo que alcanzó a crear y devuelve el error: no queda a medias.
func Aplicar(a *almacen.Almacen, botID int64, b *Borrador, mods Modulos) (*Creado, error) {
	b.Normalizar()
	if p := b.Problemas(); len(p) > 0 {
		return nil, errors.New(p[0])
	}
	if b.Creado != nil {
		if err := Deshacer(a, botID, b, mods); err != nil {
			return nil, err
		}
	}
	c := &Creado{TS: time.Now().Unix()}
	fallo := func(err error) (*Creado, error) {
		_ = deshacerCreado(a, botID, c, mods)
		return nil, err
	}
	// Bienvenida: «primera vez» es el mensaje de bienvenida; «una vez al día» es un «recibido» que se repite cada día.
	if b.conBienvenida() {
		tipo := almacen.AutoBienvenida
		if b.Bienvenida.Cuando == "diaria" {
			tipo = almacen.AutoRecibido
		}
		cfg := auto.ConfigPorDefecto(tipo)
		cfg.Quien = b.quien()
		if tipo == almacen.AutoRecibido {
			cfg.Repeticion = auto.Repeticion{Cada: 1, Unidad: "dias"}
		}
		m := &almacen.AutoMensaje{Tipo: tipo, Nombre: "Bienvenida (asistente)", Texto: b.Bienvenida.Texto, Adjuntos: b.Bienvenida.Adjuntos, Activo: true}
		if err := auto.Validar(m, &cfg); err != nil {
			return fallo(err)
		}
		m.Config = cfg.JSON()
		for i, ad := range m.Adjuntos {
			if !a.ExisteAdjunto(botID, ad.ID) {
				return fallo(fmt.Errorf("el archivo %d de la bienvenida ya no está guardado; quítalo y vuelve a agregarlo", i+1))
			}
		}
		id, err := a.CrearAuto(botID, m)
		if err != nil {
			return fallo(err)
		}
		c.Autos = append(c.Autos, id)
	}
	if b.conMenu() {
		d := b.DefMenu()
		for _, v := range d.Validar() {
			if v.Grave {
				return fallo(errors.New("El menú tiene un problema: " + v.Texto))
			}
		}
		id, err := a.CrearMenu(botID, d.JSON(), true, false)
		if err != nil {
			return fallo(err)
		}
		c.Menu = id
	}
	// Reglas al entrar a un grupo: enciende H1 solo si el usuario lo eligió y no lo tenía ya.
	if b.conReglas() {
		if mods == nil {
			return fallo(errors.New("los módulos no están disponibles todavía"))
		}
		if mods.Activo(IDBienvenidaModulo) {
			c.Avisos = append(c.Avisos, "El módulo de bienvenida a grupos ya estaba encendido: lo dejé como lo tenías.")
		} else {
			prev, _ := mods.ConfigJSON(IDBienvenidaModulo)
			c.ModuloPrev = prev
			cfg, _ := json.Marshal(map[string]any{"mensaje": "¡Hola {nombre}! 👋 Bienvenido a *{grupo}*.\n\n{reglas}", "grupos_modo": "elegidos"})
			if err := mods.GuardarConfig(IDBienvenidaModulo, string(cfg)); err != nil {
				return fallo(err)
			}
			for _, g := range b.GruposElegidos {
				if err := a.AgregarChatMod(botID, IDBienvenidaModulo, g); err != nil {
					return fallo(err)
				}
				c.ModuloChat = append(c.ModuloChat, g)
			}
			if err := mods.Activar(IDBienvenidaModulo); err != nil {
				return fallo(err)
			}
			c.Modulo = true
		}
	}
	// Utilidades: solo ajustes que ya existen, guardando el valor anterior.
	if b.conUtil() {
		if err := aplicarUtil(a, botID, b, c); err != nil {
			return fallo(err)
		}
	}
	b.Creado = c
	b.Paso = PasoFinal
	if err := Guardar(a, botID, b); err != nil {
		return fallo(err)
	}
	return c, nil
}

// Deshacer quita lo que el asistente creó y devuelve los ajustes a como estaban. Las reglas del grupo no se tocan (son del usuario).
func Deshacer(a *almacen.Almacen, botID int64, b *Borrador, mods Modulos) error {
	if b.Creado == nil {
		return nil
	}
	if err := deshacerCreado(a, botID, b.Creado, mods); err != nil {
		return err
	}
	b.Creado = nil
	return Guardar(a, botID, b)
}

func deshacerCreado(a *almacen.Almacen, botID int64, c *Creado, mods Modulos) error {
	var primero error
	nota := func(err error) {
		if err != nil && primero == nil {
			primero = err
		}
	}
	for _, id := range c.Autos {
		nota(a.BorrarAuto(botID, id))
	}
	c.Autos = nil
	if c.Menu != 0 {
		nota(a.BorrarMenu(botID, c.Menu))
		c.Menu = 0
	}
	for _, id := range c.Etiquetas {
		if a.ExisteEtiqueta(botID, id) {
			nota(a.BorrarEtiqueta(botID, id))
		}
	}
	c.Etiquetas = nil
	if c.Modulo && mods != nil {
		nota(mods.Desactivar(IDBienvenidaModulo))
		for _, g := range c.ModuloChat {
			nota(a.QuitarChatMod(botID, IDBienvenidaModulo, g))
		}
		if strings.TrimSpace(c.ModuloPrev) != "" {
			_ = mods.GuardarConfig(IDBienvenidaModulo, c.ModuloPrev)
		}
	}
	c.Modulo, c.ModuloChat = false, nil
	for k, v := range c.Ajustes {
		nota(a.FijarAjusteBot(botID, k, v))
	}
	c.Ajustes = nil
	return primero
}

// TextoDePrueba es lo que «Probar mi bot» manda: la bienvenida con las variables puestas.
func TextoDePrueba(b *Borrador, nombre string) string {
	t := b.Bienvenida.Texto
	if t == "" {
		t = "Hola, esta es una prueba de mi bot."
	}
	if strings.TrimSpace(nombre) == "" {
		nombre = ""
	}
	r := strings.NewReplacer("{nombre}", nombre, "{Nombre}", nombre)
	return strings.TrimSpace(r.Replace(t))
}

// aplicarUtil cambia solo ajustes que ya existen (guardando el valor anterior) y siembra tres etiquetas de ejemplo si no hay ninguna.
func aplicarUtil(a *almacen.Almacen, botID int64, b *Borrador, c *Creado) error {
	u := b.Util
	adv := "0"
	if u.Advertencias {
		adv = a.AjusteBot(botID, "adv_tope")
		if adv == "0" {
			adv = "3"
		}
	}
	for _, par := range [][2]string{{"sil_activo", si(u.Silenciar)}, {"adv_tope", adv}, {"util_notas", si(u.Notas)}, {"util_recordatorios", si(u.Recordatorios)}} {
		if err := fijaAjuste(a, botID, c, par[0], par[1]); err != nil {
			return err
		}
	}
	if u.Etiquetas {
		if ex, _ := a.ListarEtiquetas(botID); len(ex) == 0 {
			for _, e := range []struct{ n, c string }{{"Cliente", "#2E9E5B"}, {"Importante", "#D64545"}, {"Pendiente", "#E0A100"}} {
				if id, err := a.CrearEtiqueta(botID, e.n, e.c); err == nil {
					c.Etiquetas = append(c.Etiquetas, id)
				}
			}
		}
	}
	return nil
}
