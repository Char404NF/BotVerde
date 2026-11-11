package modulos

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"botchat/internal/almacen"
	"botchat/internal/comandos"
	"botchat/internal/identidad"
)

// Gestor es el marco de módulos de UN bot: sabe cuáles están activos, guarda su configuración cifrada, registra sus comandos mientras
// están activos, les reparte los eventos, lleva la cola de iniciativas y el modo seguro.
type Gestor struct {
	Alm   *almacen.Almacen
	BotID int64
	Cmd   *comandos.Despachador // nil = sin comandos (por ejemplo para editar sin conexión)
	// Enviador manda las iniciativas (nil = no se puede enviar: se editan ajustes y nada más).
	Enviador  Enviador
	Ahora     func() time.Time
	Aleatorio func(n int64) int64
	Zona      func() *time.Location
	Log       func(string)
	// Cambio avisa a la pantalla ("modulos", "iniciativas").
	Cambio func(ev string)
	// Aviso crea un aviso local (notificación de Android) para el dueño.
	Aviso func(titulo, detalle string)
	Cola  *Cola
	// Acc son las acciones de grupo y de WhatsApp que algunos módulos necesitan (nil = no disponibles, por ejemplo sin conexión).
	Acc Acciones
	// SinPausaArchivos quita la pausa de 1 a 3 s entre archivos de un mismo mensaje (las pruebas).
	SinPausaArchivos bool

	// h es el estado en memoria de los módulos H (contadores de lo que está en la cola, errores seguidos, usos por hora…).
	h estadoH

	mu     sync.Mutex
	cache  map[string]*almacen.ModuloFila
	vivo   bool
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NuevoGestor crea el gestor de un bot. Hay que llamar a Iniciar para que corran la cola y las alarmas.
func NuevoGestor(alm *almacen.Almacen, botID int64, cmd *comandos.Despachador) *Gestor {
	g := &Gestor{Alm: alm, BotID: botID, Cmd: cmd, Ahora: time.Now, Aleatorio: rand.Int63n, Log: func(string) {}, cache: map[string]*almacen.ModuloFila{}}
	g.Cola = nuevaCola(g)
	return g
}

func (g *Gestor) ahora() time.Time {
	if g.Ahora != nil {
		return g.Ahora()
	}
	return time.Now()
}

func (g *Gestor) cambio(ev string) {
	if g.Cambio != nil {
		g.Cambio(ev)
	}
}

func (g *Gestor) log(s string) {
	if g.Log != nil {
		g.Log(s)
	}
}

// ---------- vida ----------

// Iniciar vuelve a encender los módulos que estaban activos (registra sus comandos y llama a Activar) y arranca la cola, las alarmas
// de cada minuto y la limpieza del registro.
func (g *Gestor) Iniciar(ctx context.Context) {
	g.mu.Lock()
	if g.vivo {
		g.mu.Unlock()
		return
	}
	g.vivo = true
	g.ctx, g.cancel = context.WithCancel(ctx)
	g.mu.Unlock()
	for _, m := range Lista() {
		asegurarCasillas(m)
		if g.Activo(m.ID()) {
			if err := g.arrancar(m); err != nil {
				g.log("No se pudo encender el módulo " + m.ID() + ": " + err.Error())
			}
		}
	}
	g.wg.Add(2)
	go func() { defer g.wg.Done(); g.Cola.correr(g.ctx) }()
	go func() {
		defer g.wg.Done()
		g.alarmas(g.ctx)
	}()
}

// Detener para la cola y las alarmas y llama a Desactivar de cada módulo encendido (sin apagarlos en la base).
func (g *Gestor) Detener() {
	g.mu.Lock()
	if !g.vivo {
		g.mu.Unlock()
		return
	}
	g.vivo = false
	g.cancel()
	g.mu.Unlock()
	g.wg.Wait()
	for _, m := range Lista() {
		if g.Activo(m.ID()) {
			g.parar(m)
		}
	}
}

func (g *Gestor) estaVivo() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.vivo
}

func (g *Gestor) alarmas(ctx context.Context) {
	t := time.NewTicker(time.Minute)
	defer t.Stop()
	dia := time.Time{}
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		now := g.ahora()
		if now.Sub(dia) > 24*time.Hour {
			dia = now
			g.Alm.LimpiarRegistroModulos(g.BotID, g.Alm.AjusteBotInt(g.BotID, "registro_dias"), now)
		}
		g.Evento(Evento{Tipo: EvTick, Hora: now})
	}
}

// ---------- módulos: estado ----------

func (g *Gestor) fila(id string) *almacen.ModuloFila {
	g.mu.Lock()
	f, ok := g.cache[id]
	g.mu.Unlock()
	if ok {
		return f
	}
	f, _ = g.Alm.ObtenerModulo(g.BotID, id)
	if f == nil {
		f = &almacen.ModuloFila{ID: id}
	}
	g.mu.Lock()
	g.cache[id] = f
	g.mu.Unlock()
	return f
}

func (g *Gestor) olvidar(id string) {
	g.mu.Lock()
	delete(g.cache, id)
	g.mu.Unlock()
}

// Activo dice si el módulo está encendido (aunque esté en pausa por el modo seguro).
func (g *Gestor) Activo(id string) bool { return g.fila(id).Activo }

// Pausado dice si el módulo está encendido pero detenido por el modo seguro.
func (g *Gestor) Pausado(id string) bool {
	m, ok := Obtener(id)
	return ok && m.Iniciativa() && g.Activo(id) && g.EnModoSeguro()
}

// Config devuelve la configuración del módulo (lo guardado completado con los valores por defecto).
func (g *Gestor) Config(id string) Config {
	m, ok := Obtener(id)
	if !ok {
		return Config{}
	}
	return Config(mezclarConfig(m, g.fila(id).Config))
}

// Info es la tarjeta de un módulo para la pantalla.
type Info struct {
	ID          string `json:"id"`
	Nombre      string `json:"nombre"`
	Descripcion string `json:"descripcion"`
	Icono       string `json:"icono"`
	Riesgo      string `json:"riesgo"`
	Iniciativa  bool   `json:"iniciativa"`
	Activo      bool   `json:"activo"`
	Pausado     bool   `json:"pausado"`
	// Codigo: apagado | activo | pausado_seguro | esperando_maestro.
	Codigo     string  `json:"codigo"`
	Estado     string  `json:"estado"` // texto en español para la tarjeta
	ConEsquema bool    `json:"conEsquema"`
	TopeHora   int     `json:"topeHora"`
	TopeDia    int     `json:"topeDia"`
	DefHora    int     `json:"defHora"`
	DefDia     int     `json:"defDia"`
	ActivadoTS int64   `json:"activadoTs"`
	Esquema    []Campo `json:"esquema,omitempty"`
	Paneles    []Panel `json:"paneles,omitempty"`
}

func (g *Gestor) info(m Modulo, conEsquema bool) Info {
	f := g.fila(m.ID())
	dh, dd := m.Limites()
	th, td := g.TopesModulo(m.ID())
	x := Info{ID: m.ID(), Nombre: m.Nombre(), Descripcion: m.Descripcion(), Icono: m.Icono(), Riesgo: m.Riesgo(), Iniciativa: m.Iniciativa(),
		Activo: f.Activo, ConEsquema: len(m.Esquema()) > 0, TopeHora: th, TopeDia: td, DefHora: dh, DefDia: dd, ActivadoTS: f.ActivadoTS}
	switch {
	case !f.Activo:
		x.Codigo, x.Estado = "apagado", "Apagado"
	case m.Iniciativa() && g.EnModoSeguro():
		x.Pausado, x.Codigo, x.Estado = true, "pausado_seguro", "Pausado por modo seguro"
	case m.Iniciativa() && !g.Maestro():
		x.Codigo, x.Estado = "esperando_maestro", "Encendido, pero la Iniciativa propia está apagada"
	default:
		x.Codigo, x.Estado = "activo", "Encendido"
		func() {
			defer func() { _ = recover() }()
			if t := strings.TrimSpace(m.Estado(g)); t != "" {
				x.Estado = t
			}
		}()
	}
	if conEsquema {
		x.Esquema = m.Esquema()
	}
	return x
}

// Lista devuelve las tarjetas de todos los módulos registrados.
func (g *Gestor) Lista() []Info {
	ms := Lista()
	out := make([]Info, 0, len(ms))
	for _, m := range ms {
		out = append(out, g.info(m, false))
	}
	return out
}

// InfoDe devuelve la tarjeta (con su esquema) de un módulo.
func (g *Gestor) InfoDe(id string) (Info, error) {
	m, ok := Obtener(id)
	if !ok {
		return Info{}, ErrNoExiste
	}
	x := g.info(m, true)
	x.Paneles = g.PanelesDe(id)
	return x, nil
}

// ---------- activar y desactivar ----------

func nombreComando(c comandos.Comando) string { return c.Nombre }

// comandosDe junta los comandos fijos del módulo y los que arma con el gestor.
func (g *Gestor) comandosDe(m Modulo) []comandos.Comando {
	cs := append([]comandos.Comando(nil), m.Comandos()...)
	if c, ok := m.(ComandosDeGestor); ok {
		cs = append(cs, c.ComandosG(g)...)
	}
	return cs
}

// arrancar registra los comandos del módulo y llama a su Activar. Si algo falla deja todo como estaba.
func (g *Gestor) arrancar(m Modulo) error {
	var puestos []string
	if g.Cmd != nil {
		for _, c := range g.comandosDe(m) {
			cc := c
			id := m.ID()
			inner := c.Ejecutar
			cc.Ejecutar = func(cx *comandos.Contexto) (string, error) {
				if !g.Activo(id) {
					return "", errors.New("ese comando no está disponible ahora")
				}
				return inner(cx)
			}
			if err := g.Cmd.Registrar(cc); err != nil {
				for _, n := range puestos {
					g.Cmd.Quitar(n)
				}
				return err
			}
			puestos = append(puestos, nombreComando(c))
		}
	}
	if err := m.Activar(g); err != nil {
		if g.Cmd != nil {
			for _, n := range puestos {
				g.Cmd.Quitar(n)
			}
		}
		return err
	}
	return nil
}

// parar quita los comandos y llama a Desactivar.
func (g *Gestor) parar(m Modulo) {
	if g.Cmd != nil {
		for _, c := range g.comandosDe(m) {
			g.Cmd.Quitar(c.Nombre)
		}
	}
	func() {
		defer func() { _ = recover() }()
		if err := m.Desactivar(g); err != nil {
			g.log("El módulo " + m.ID() + " avisó un problema al apagarse: " + err.Error())
		}
	}()
}

// Activar enciende un módulo. Quien llama (la pantalla) es quien muestra el aviso de riesgo antes.
func (g *Gestor) Activar(id string) error {
	m, ok := Obtener(id)
	if !ok {
		return ErrNoExiste
	}
	asegurarCasillas(m)
	if g.Activo(id) {
		return nil
	}
	if g.estaVivo() {
		if err := g.arrancar(m); err != nil {
			return err
		}
	}
	if f := g.fila(id); f.Config == "" {
		b, _ := json.Marshal(configPorDefecto(m))
		if err := g.Alm.GuardarModuloConfig(g.BotID, id, string(b)); err != nil {
			return err
		}
	}
	if err := g.Alm.FijarModuloActivo(g.BotID, id, true, g.ahora()); err != nil {
		if g.estaVivo() {
			g.parar(m)
		}
		return err
	}
	g.olvidar(id)
	g.cambio("modulos")
	return nil
}

// Desactivar apaga un módulo en el acto: quita sus comandos y descarta lo que tenía esperando en la cola. Su configuración se conserva.
func (g *Gestor) Desactivar(id string) error {
	m, ok := Obtener(id)
	if !ok {
		return ErrNoExiste
	}
	if !g.Activo(id) {
		return nil
	}
	if err := g.Alm.FijarModuloActivo(g.BotID, id, false, g.ahora()); err != nil {
		return err
	}
	g.olvidar(id)
	g.Cola.Vaciar(id, MotModulo)
	if g.estaVivo() {
		g.parar(m)
	}
	g.cambio("modulos")
	return nil
}

// ---------- configuración ----------

// ConfigJSON devuelve la configuración completa del módulo.
func (g *Gestor) ConfigJSON(id string) (string, error) {
	if _, ok := Obtener(id); !ok {
		return "", ErrNoExiste
	}
	b, err := json.Marshal(g.Config(id))
	return string(b), err
}

// GuardarConfig valida y guarda (cifrada) la configuración de un módulo. Devuelve el error en español si algo no vale.
func (g *Gestor) GuardarConfig(id, texto string) error {
	m, ok := Obtener(id)
	if !ok {
		return ErrNoExiste
	}
	completa, err := validarConfig(m, texto)
	if err != nil {
		return err
	}
	b, _ := json.Marshal(completa)
	if err := g.Alm.GuardarModuloConfig(g.BotID, id, string(b)); err != nil {
		return err
	}
	g.olvidar(id)
	if cc, ok := m.(ConfigCambiada); ok && g.Activo(id) && g.estaVivo() {
		func() {
			defer func() { _ = recover() }()
			cc.ConfigCambiada(g)
		}()
	}
	g.cambio("modulos")
	return nil
}

// FijarTopesModulo guarda los topes propios del módulo (0 = los que trae el módulo).
func (g *Gestor) FijarTopesModulo(id string, hora, dia int) error {
	if _, ok := Obtener(id); !ok {
		return ErrNoExiste
	}
	if hora < 0 || hora > 1000 || dia < 0 || dia > 10000 {
		return errors.New("los topes deben estar entre 0 y 1000 por hora y 10000 por día")
	}
	if err := g.Alm.FijarTopesModulo(g.BotID, id, hora, dia); err != nil {
		return err
	}
	g.olvidar(id)
	g.cambio("modulos")
	return nil
}

// TopesModulo son los topes por hora y por día de un módulo: los suyos si los fijó, o los que trae el módulo (0 = sin tope).
func (g *Gestor) TopesModulo(id string) (hora, dia int) {
	f := g.fila(id)
	hora, dia = f.TopeHora, f.TopeDia
	if m, ok := Obtener(id); ok {
		dh, dd := m.Limites()
		if hora <= 0 {
			hora = dh
		}
		if dia <= 0 {
			dia = dd
		}
	}
	return hora, dia
}

// ---------- ajustes generales de iniciativa ----------

// Maestro dice si el interruptor general "Iniciativa propia" está encendido (nace APAGADO).
func (g *Gestor) Maestro() bool { return g.Alm.AjusteIni(g.BotID, "ini_maestro") == "1" }

// FijarMaestro enciende o apaga el interruptor general. Al apagarlo se descarta de golpe todo lo que esperaba en la cola.
func (g *Gestor) FijarMaestro(si bool) error {
	v := "0"
	if si {
		v = "1"
	}
	if err := g.Alm.FijarAjusteIni(g.BotID, "ini_maestro", v); err != nil {
		return err
	}
	if !si {
		g.Cola.Vaciar("", MotMaestro)
	}
	g.cambio("modulos")
	return nil
}

// TopesGlobales son los topes de iniciativas por hora y por día entre todos los módulos.
func (g *Gestor) TopesGlobales() (hora, dia int) {
	return g.Alm.AjusteIniInt(g.BotID, "ini_tope_hora"), g.Alm.AjusteIniInt(g.BotID, "ini_tope_dia")
}

var reHoraFija = regexp.MustCompile(`^([01][0-9]|2[0-3]):[0-5][0-9]$`)

// FijarAjuste valida y guarda un ajuste de iniciativa: maestro, tope_hora, tope_dia, noche_on, noche_desde, noche_hasta,
// retraso_min, retraso_max, errores, espera_max, baja_texto.
func (g *Gestor) FijarAjuste(clave, valor string) error {
	valor = strings.TrimSpace(valor)
	clave = "ini_" + strings.TrimPrefix(clave, "ini_")
	num := func(min, max int) error {
		n, err := strconv.Atoi(valor)
		if err != nil || n < min || n > max {
			return fmt.Errorf("el valor debe estar entre %d y %d", min, max)
		}
		return nil
	}
	switch clave {
	case "ini_maestro":
		return g.FijarMaestro(valor == "1" || valor == "true")
	case "ini_tope_hora":
		if err := num(1, 1000); err != nil {
			return err
		}
	case "ini_tope_dia":
		if err := num(1, 10000); err != nil {
			return err
		}
	case "ini_noche_on":
		if valor != "0" && valor != "1" {
			return errors.New("el valor debe ser 0 o 1")
		}
	case "ini_noche_desde", "ini_noche_hasta":
		if !reHoraFija.MatchString(valor) {
			return errors.New("la hora debe verse como 23:00")
		}
	case "ini_retraso_min":
		if err := num(0, 600); err != nil {
			return err
		}
		if n, _ := strconv.Atoi(valor); n > g.Alm.AjusteIniInt(g.BotID, "ini_retraso_max") {
			_ = g.Alm.FijarAjusteIni(g.BotID, "ini_retraso_max", valor) // el máximo sube con el mínimo
		}
	case "ini_retraso_max":
		if err := num(0, 3600); err != nil {
			return err
		}
		if n, _ := strconv.Atoi(valor); n < g.Alm.AjusteIniInt(g.BotID, "ini_retraso_min") {
			_ = g.Alm.FijarAjusteIni(g.BotID, "ini_retraso_min", valor) // el mínimo baja con el máximo
		}
	case "ini_errores":
		if err := num(1, 50); err != nil {
			return err
		}
	case "ini_espera_max":
		if err := num(1, 1440); err != nil {
			return err
		}
	case "ini_baja_texto":
		if n := len([]rune(valor)); n < 1 || n > 300 {
			return errors.New("el texto debe tener entre 1 y 300 letras")
		}
	default:
		return errors.New("ajuste desconocido")
	}
	if err := g.Alm.FijarAjusteIni(g.BotID, clave, valor); err != nil {
		return err
	}
	g.cambio("modulos")
	return nil
}

// AjustesJSON devuelve los ajustes de iniciativa para la pantalla (sin el estado interno del modo seguro).
func (g *Gestor) AjustesJSON() string {
	m := g.Alm.AjustesIni(g.BotID)
	delete(m, "ini_seguro")
	out := map[string]string{}
	for k, v := range m {
		out[strings.TrimPrefix(k, "ini_")] = v
	}
	b, _ := json.Marshal(out)
	return string(b)
}

// ---------- pausa nocturna ----------

func minutosDe(hhmm string) int {
	if !reHoraFija.MatchString(hhmm) {
		return 0
	}
	h, _ := strconv.Atoi(hhmm[:2])
	m, _ := strconv.Atoi(hhmm[3:])
	return h*60 + m
}

func (g *Gestor) zona() *time.Location {
	if g.Zona != nil {
		if z := g.Zona(); z != nil {
			return z
		}
	}
	return time.Local
}

// enPausaNocturna dice si `t` cae dentro de la pausa nocturna y, si es así, cuándo termina.
func (g *Gestor) enPausaNocturna(t time.Time) (bool, time.Time) {
	if g.Alm.AjusteIni(g.BotID, "ini_noche_on") != "1" {
		return false, time.Time{}
	}
	desde, hasta := minutosDe(g.Alm.AjusteIni(g.BotID, "ini_noche_desde")), minutosDe(g.Alm.AjusteIni(g.BotID, "ini_noche_hasta"))
	if desde == hasta {
		return false, time.Time{}
	}
	l := t.In(g.zona())
	m := l.Hour()*60 + l.Minute()
	fin := func(dias int) time.Time {
		return time.Date(l.Year(), l.Month(), l.Day()+dias, hasta/60, hasta%60, 0, 0, l.Location())
	}
	if desde < hasta {
		if m >= desde && m < hasta {
			return true, fin(0)
		}
		return false, time.Time{}
	}
	switch {
	case m >= desde:
		return true, fin(1)
	case m < hasta:
		return true, fin(0)
	}
	return false, time.Time{}
}

// ---------- modo seguro ----------

// EnModoSeguro dice si los módulos de iniciativa están pausados por el modo seguro.
func (g *Gestor) EnModoSeguro() bool { return g.Alm.AjusteIni(g.BotID, "ini_seguro") != "" }

// EstadoSeguro devuelve si está activo, desde cuándo (segundos) y por qué ("errores", "desconexion:sesion_cerrada"…).
func (g *Gestor) EstadoSeguro() (activo bool, ts int64, motivo string) {
	v := g.Alm.AjusteIni(g.BotID, "ini_seguro")
	if v == "" {
		return false, 0, ""
	}
	a, b, _ := strings.Cut(v, "|")
	ts, _ = strconv.ParseInt(a, 10, 64)
	return true, ts, b
}

// textoMotivo traduce el motivo del modo seguro a palabras simples.
func textoMotivo(motivo string) string {
	switch {
	case motivo == "errores":
		return "fallaron varios envíos seguidos"
	case strings.HasPrefix(motivo, "desconexion"):
		return "WhatsApp cortó la conexión (" + strings.TrimPrefix(strings.TrimPrefix(motivo, "desconexion"), ":") + ")"
	}
	return motivo
}

// ModoSeguro pausa los módulos de iniciativa (no los apaga: siguen encendidos y quedan "pausados por modo seguro" hasta que se
// reanuden a mano), descarta lo que esperaba en la cola y avisa por una notificación local. Se llama solo.
func (g *Gestor) ModoSeguro(motivo string) {
	if g.EnModoSeguro() {
		return
	}
	now := g.ahora()
	if err := g.Alm.FijarAjusteIni(g.BotID, "ini_seguro", fmt.Sprintf("%d|%s", now.Unix(), motivo)); err != nil {
		return
	}
	g.Cola.Vaciar("", MotSeguro)
	g.Alm.RegistrarModulo(g.BotID, "sistema", "modo_seguro", motivo, "", now)
	for _, m := range Lista() {
		if p, ok := m.(ConPausa); ok && m.Iniciativa() && g.Activo(m.ID()) {
			func() { defer func() { _ = recover() }(); p.Pausar(g) }()
		}
	}
	g.log("Modo seguro: los módulos de iniciativa quedaron en pausa.")
	if g.Aviso != nil {
		g.Aviso("Modo seguro activado", "Los módulos que escriben solos quedaron en pausa porque "+textoMotivo(motivo)+". Revisa y reanúdalos en Ajustes → Iniciativa propia.")
	}
	g.cambio("modulos")
}

// Reanudar saca del modo seguro: los módulos de iniciativa vuelven a trabajar.
func (g *Gestor) Reanudar() error {
	if !g.EnModoSeguro() {
		return nil
	}
	if err := g.Alm.FijarAjusteIni(g.BotID, "ini_seguro", ""); err != nil {
		return err
	}
	g.Cola.reiniciarErrores()
	g.Alm.RegistrarModulo(g.BotID, "sistema", "modo_seguro", "reanudado", "", g.ahora())
	for _, m := range Lista() {
		if p, ok := m.(ConPausa); ok && m.Iniciativa() && g.Activo(m.ID()) {
			func() { defer func() { _ = recover() }(); p.Reanudar(g) }()
		}
	}
	g.cambio("modulos")
	return nil
}

// SeguroJSON es el estado del modo seguro para la pantalla.
func (g *Gestor) SeguroJSON() string {
	a, ts, mot := g.EstadoSeguro()
	b, _ := json.Marshal(map[string]any{"activo": a, "ts": ts, "motivo": mot, "texto": textoMotivo(mot), "errores": g.Cola.ErroresSeguidos()})
	return string(b)
}

// ---------- No molestar ----------

var reBaja = regexp.MustCompile(`[\p{P}\p{S}]+`)

// EsBaja dice si el texto, entero, pide que no le escriban: BAJA, STOP o NO MÁS (sin importar mayúsculas, acentos ni signos).
func EsBaja(texto string) bool {
	t := identidad.Normalizar(texto)
	t = strings.Join(strings.Fields(reBaja.ReplaceAllString(t, " ")), " ")
	switch t {
	case "baja", "stop", "no mas", "nomas":
		return true
	}
	return false
}

// Baja revisa un texto recibido en privado. Si pide la baja, mete a esa persona en la lista No molestar (de inmediato) y devuelve una
// función que da el texto de confirmación SOLO la primera vez (después, vacío).
func (g *Gestor) Baja(chat, texto string) (bool, func() string) {
	if !EsBaja(texto) || strings.HasSuffix(chat, "@g.us") {
		return false, nil
	}
	nuevo, _, err := g.Alm.AgregarNoMolestar(g.BotID, chat, "baja", g.ahora())
	if err != nil {
		return false, nil
	}
	if nuevo {
		g.Alm.RegistrarModulo(g.BotID, "sistema", "no_molestar", "agregado", chat, g.ahora())
		g.cambio("iniciativas")
	}
	return true, func() string {
		lista, _ := g.Alm.ListarNoMolestar(g.BotID)
		p := g.Alm.ResolverPersona(g.BotID, chat)
		for _, x := range lista {
			if x.Persona == p {
				if x.Confirmado {
					return ""
				}
				g.Alm.MarcarNoMolestarConfirmado(g.BotID, chat)
				return g.Alm.AjusteIni(g.BotID, "ini_baja_texto")
			}
		}
		return ""
	}
}

// AgregarNoMolestar mete a mano a una persona en la lista (sin mandarle confirmación).
func (g *Gestor) AgregarNoMolestar(persona, motivo string) error {
	persona = strings.TrimSpace(persona)
	if persona == "" {
		return errors.New("falta la persona")
	}
	if _, _, err := g.Alm.AgregarNoMolestar(g.BotID, persona, motivo, g.ahora()); err != nil {
		return err
	}
	g.Alm.MarcarNoMolestarConfirmado(g.BotID, persona) // al agregar yo, no se le escribe nada
	g.cambio("iniciativas")
	return nil
}

// QuitarNoMolestar saca a una persona de la lista.
func (g *Gestor) QuitarNoMolestar(persona string) error {
	if err := g.Alm.QuitarNoMolestar(g.BotID, persona); err != nil {
		return err
	}
	g.cambio("iniciativas")
	return nil
}

// ---------- eventos ----------

// Evento reparte un evento a los módulos encendidos (los de iniciativa en pausa por el modo seguro no lo reciben). Un módulo que
// falla no tumba a los demás.
func (g *Gestor) Evento(e Evento) {
	if e.Hora.IsZero() {
		e.Hora = g.ahora()
	}
	for _, m := range Lista() {
		if !g.Activo(m.ID()) || (m.Iniciativa() && g.EnModoSeguro()) {
			continue
		}
		func() {
			defer func() {
				if r := recover(); r != nil {
					g.log(fmt.Sprintf("El módulo %s falló con un evento: %v", m.ID(), r))
				}
			}()
			m.Evento(g, e)
		}()
	}
}

// Enviar es el atajo para que un módulo mande una iniciativa por la cola.
func (g *Gestor) Enviar(i Iniciativa) (string, error) { return g.Cola.Encolar(i) }
