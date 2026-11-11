package modulos

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"botchat/internal/envio"
)

// Enviador es lo que la cola usa para escribir (en la app es el servicio de envío; en las pruebas, uno falso).
type Enviador interface {
	Texto(ctx context.Context, chat, texto, citaID string, menciones []string) error
	Medio(ctx context.Context, chat string, a envio.Adjunto, citaID string) error
	Conectado() bool
}

// Qué hacer con una iniciativa que cae en la pausa nocturna.
const (
	NocheDiferir   = "diferir"   // se manda al terminar la pausa (por defecto)
	NocheDescartar = "descartar" // se tira
)

// Resultados que quedan en el registro (códigos cortos, sin textos).
const (
	EnCola          = "en_cola"
	ResEnviado      = "enviado"
	ResFallido      = "fallido"
	ResDiferido     = "diferido"
	ResDescartado   = "descartado:" // + motivo
	MotMaestro      = "maestro"     // el interruptor general "Iniciativa propia" está apagado
	MotModulo       = "modulo"      // el módulo está apagado o no existe
	MotNoIniciativa = "no_es_iniciativa"
	MotSeguro       = "seguro"      // modo seguro
	MotNoMolestar   = "no_molestar" // la persona pidió que no le escriban
	MotNoche        = "noche"       // pausa nocturna y el módulo pidió descartar
	MotTope         = "tope"        // sin fichas y la espera sería demasiado larga
	MotSinConexion  = "sin_conexion"
	MotColaLlena    = "cola_llena"
)

const maxCola = 500

// Iniciativa es un mensaje que un módulo quiere mandar sin que nadie se lo haya pedido.
type Iniciativa struct {
	Modulo                 string   // id del módulo que la manda
	Que                    string   // código corto de qué es ("bienvenida", "difusion"…); va al registro, nunca el texto
	Chat                   string   // a dónde (privado o grupo)
	Personas               []string // a quién le llega (para No molestar); vacío = la persona del chat si es privado
	Texto                  string
	CitaID                 string
	Menciones              []string
	Medio                  *envio.Adjunto
	Noche                  string        // NocheDiferir (por defecto) o NocheDescartar
	RetrasoMin, RetrasoMax time.Duration // espera al azar antes de mandarla (0 = la general)
	EsperaMax              time.Duration // lo máximo que puede esperar una ficha (0 = la general)
}

type item struct {
	ini      Iniciativa
	listo    time.Time
	entro    time.Time
	diferida bool
}

// cubeta es una cubeta de fichas: cada envío gasta una ficha y esa ficha vuelve cuando pasa la ventana (una hora o un día).
type cubeta struct{ gastos []time.Time }

func (b *cubeta) podar(ahora time.Time, ventana time.Duration) {
	i := 0
	for i < len(b.gastos) && !b.gastos[i].After(ahora.Add(-ventana)) {
		i++
	}
	b.gastos = b.gastos[i:]
}

// espera dice cuánto falta para tener una ficha (0 = ya hay). tope <= 0 significa sin límite.
func (b *cubeta) espera(ahora time.Time, tope int, ventana time.Duration) time.Duration {
	if tope <= 0 {
		return 0
	}
	b.podar(ahora, ventana)
	if len(b.gastos) < tope {
		return 0
	}
	return b.gastos[len(b.gastos)-tope].Add(ventana).Sub(ahora)
}

type fichas struct{ hora, dia cubeta }

// Cola es la ÚNICA vía por la que un módulo escribe por iniciativa propia.
type Cola struct {
	g *Gestor

	mu      sync.Mutex
	items   []*item
	cargado bool
	global  fichas
	porMod  map[string]*fichas
	noAntes time.Time
	fallos  int
	senal   chan struct{}
}

func nuevaCola(g *Gestor) *Cola {
	return &Cola{g: g, porMod: map[string]*fichas{}, senal: make(chan struct{}, 1)}
}

func (c *Cola) despertar() {
	select {
	case c.senal <- struct{}{}:
	default:
	}
}

// Pendientes dice cuántas iniciativas esperan salir.
func (c *Cola) Pendientes() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.items)
}

// bloqueo revisa lo que impide mandar de golpe (se mira al encolar y otra vez justo antes de enviar). "" = nada lo impide.
func (c *Cola) bloqueo(i Iniciativa) string {
	g := c.g
	if !g.Maestro() {
		return MotMaestro
	}
	m, ok := Obtener(i.Modulo)
	if !ok || !g.Activo(i.Modulo) {
		return MotModulo
	}
	if !m.Iniciativa() {
		return MotNoIniciativa
	}
	if g.EnModoSeguro() {
		return MotSeguro
	}
	for _, p := range c.personas(i) {
		if g.Alm.EstaNoMolestar(g.BotID, p) {
			return MotNoMolestar
		}
	}
	return ""
}

// personas son las claves a las que llega la iniciativa: las dichas, o la del chat si es privado.
func (c *Cola) personas(i Iniciativa) []string {
	if len(i.Personas) > 0 {
		return i.Personas
	}
	if strings.HasSuffix(i.Chat, "@g.us") {
		return nil
	}
	return []string{i.Chat}
}

// Encolar mete una iniciativa a la cola. Devuelve EnCola, o "descartado:<motivo>" si no se aceptó (también queda en el registro).
// Solo falla (error) si la iniciativa está mal armada.
func (c *Cola) Encolar(i Iniciativa) (string, error) {
	if i.Modulo == "" || i.Chat == "" {
		return "", errors.New("falta el módulo o el chat")
	}
	if strings.TrimSpace(i.Texto) == "" && i.Medio == nil {
		return "", errors.New("la iniciativa no tiene nada que enviar")
	}
	if i.Que == "" {
		i.Que = "mensaje"
	}
	now := c.g.ahora()
	if mot := c.bloqueo(i); mot != "" {
		c.registrar(i, ResDescartado+mot)
		return ResDescartado + mot, nil
	}
	c.mu.Lock()
	if len(c.items) >= maxCola {
		c.mu.Unlock()
		c.registrar(i, ResDescartado+MotColaLlena)
		return ResDescartado + MotColaLlena, nil
	}
	c.items = append(c.items, &item{ini: i, entro: now, listo: now.Add(c.retraso(i))})
	c.mu.Unlock()
	c.despertar()
	return EnCola, nil
}

// retraso es la espera al azar antes de mandar (entre el mínimo y el máximo, de la iniciativa o los generales).
func (c *Cola) retraso(i Iniciativa) time.Duration {
	mn, mx := i.RetrasoMin, i.RetrasoMax
	if mn <= 0 && mx <= 0 {
		mn = time.Duration(c.g.Alm.AjusteIniInt(c.g.BotID, "ini_retraso_min")) * time.Second
		mx = time.Duration(c.g.Alm.AjusteIniInt(c.g.BotID, "ini_retraso_max")) * time.Second
	}
	if mx < mn {
		mx = mn
	}
	d := mn
	if span := int64(mx - mn); span > 0 {
		d += time.Duration(c.g.Aleatorio(span))
	}
	return d
}

func (c *Cola) registrar(i Iniciativa, resultado string) {
	c.g.Alm.RegistrarModulo(c.g.BotID, i.Modulo, i.Que, resultado, i.Chat, c.g.ahora())
}

// Vaciar descarta todo lo que espera (modulo "" = todos los módulos) con ese motivo. Devuelve cuántas.
func (c *Cola) Vaciar(modulo, motivo string) int {
	c.mu.Lock()
	var quitar []*item
	nuevo := c.items[:0]
	for _, it := range c.items {
		if modulo == "" || it.ini.Modulo == modulo {
			quitar = append(quitar, it)
		} else {
			nuevo = append(nuevo, it)
		}
	}
	c.items = nuevo
	c.mu.Unlock()
	for _, it := range quitar {
		c.registrar(it.ini, ResDescartado+motivo)
	}
	return len(quitar)
}

// cargarFichas recarga, la primera vez, lo gastado en el último día desde el registro (así un reinicio no regala fichas).
func (c *Cola) cargarFichas(ahora time.Time) {
	if c.cargado {
		return
	}
	c.cargado = true
	for _, e := range c.g.Alm.EnviosRecientesModulos(c.g.BotID, ahora.Add(-24*time.Hour)) {
		t := time.Unix(e.TS, 0)
		c.global.dia.gastos = append(c.global.dia.gastos, t)
		f := c.fichasDe(e.Modulo)
		f.dia.gastos = append(f.dia.gastos, t)
		if t.After(ahora.Add(-time.Hour)) {
			c.global.hora.gastos = append(c.global.hora.gastos, t)
			f.hora.gastos = append(f.hora.gastos, t)
		}
	}
}

func (c *Cola) fichasDe(modulo string) *fichas {
	f := c.porMod[modulo]
	if f == nil {
		f = &fichas{}
		c.porMod[modulo] = f
	}
	return f
}

// Paso procesa una iniciativa lista, si la hay. Devuelve si hizo algo y, si no, cuánto conviene esperar (-1 = hasta que llegue algo).
// Lo llama el trabajador de la cola; las pruebas lo llaman a mano con un reloj falso.
func (c *Cola) Paso(ctx context.Context) (bool, time.Duration) {
	now := c.g.ahora()
	c.mu.Lock()
	c.cargarFichas(now)
	if len(c.items) == 0 {
		c.mu.Unlock()
		return false, -1
	}
	idx := -1
	min := c.items[0].listo
	for i, it := range c.items {
		if it.listo.Before(min) {
			min = it.listo
		}
		if !it.listo.After(now) && (idx < 0 || it.listo.Before(c.items[idx].listo)) {
			idx = i
		}
	}
	if idx < 0 {
		c.mu.Unlock()
		return false, min.Sub(now)
	}
	it := c.items[idx]
	c.items = append(c.items[:idx], c.items[idx+1:]...)
	c.mu.Unlock()
	c.procesar(ctx, it, now)
	return true, 0
}

// diferir vuelve a poner la iniciativa en la cola para más tarde.
func (c *Cola) diferir(it *item, hasta time.Time) {
	it.listo = hasta
	c.mu.Lock()
	c.items = append(c.items, it)
	c.mu.Unlock()
}

func (c *Cola) procesar(ctx context.Context, it *item, now time.Time) {
	g := c.g
	i := it.ini
	// 1) Lo que se puede haber apagado mientras esperaba: maestro, módulo, modo seguro, No molestar.
	if mot := c.bloqueo(i); mot != "" {
		c.registrar(i, ResDescartado+mot)
		return
	}
	// 2) Pausa nocturna.
	if dentro, fin := g.enPausaNocturna(now); dentro {
		if i.Noche == NocheDescartar {
			c.registrar(i, ResDescartado+MotNoche)
			return
		}
		if !it.diferida {
			it.diferida = true
			c.registrar(i, ResDiferido)
		}
		c.diferir(it, fin.Add(c.retraso(i)))
		return
	}
	espMax := i.EsperaMax
	if espMax <= 0 {
		espMax = time.Duration(g.Alm.AjusteIniInt(g.BotID, "ini_espera_max")) * time.Minute
	}
	// 3) Conexión.
	if g.Enviador == nil || !g.Enviador.Conectado() {
		if now.Sub(it.entro) >= espMax {
			c.registrar(i, ResDescartado+MotSinConexion)
			return
		}
		c.diferir(it, now.Add(30*time.Second))
		return
	}
	// 4) Fichas: global y del módulo, por hora y por día.
	topeH, topeD := g.TopesGlobales()
	mh, md := g.TopesModulo(i.Modulo)
	c.mu.Lock()
	f := c.fichasDe(i.Modulo)
	var espera time.Duration
	for _, e := range []time.Duration{
		c.global.hora.espera(now, topeH, time.Hour), c.global.dia.espera(now, topeD, 24*time.Hour),
		f.hora.espera(now, mh, time.Hour), f.dia.espera(now, md, 24*time.Hour),
	} {
		if e > espera {
			espera = e
		}
	}
	if espera == 0 && now.Before(c.noAntes) {
		espera = c.noAntes.Sub(now) // separación entre un envío y el siguiente
	}
	if espera > 0 {
		c.mu.Unlock()
		if now.Add(espera).Sub(it.entro) > espMax {
			c.registrar(i, ResDescartado+MotTope)
			return
		}
		c.diferir(it, now.Add(espera))
		return
	}
	c.global.hora.gastos = append(c.global.hora.gastos, now)
	c.global.dia.gastos = append(c.global.dia.gastos, now)
	f.hora.gastos = append(f.hora.gastos, now)
	f.dia.gastos = append(f.dia.gastos, now)
	c.mu.Unlock()
	// 5) Enviar.
	cx, cancel := context.WithTimeout(ctx, 90*time.Second)
	var err error
	if i.Medio != nil {
		err = g.Enviador.Medio(cx, i.Chat, *i.Medio, i.CitaID)
		if err == nil && strings.TrimSpace(i.Texto) != "" && i.Medio.Pie == "" {
			err = g.Enviador.Texto(cx, i.Chat, i.Texto, "", i.Menciones)
		}
	} else {
		err = g.Enviador.Texto(cx, i.Chat, i.Texto, i.CitaID, i.Menciones)
	}
	cancel()
	c.mu.Lock()
	c.noAntes = g.ahora().Add(c.retraso(i))
	if err == nil {
		c.fallos = 0
	} else {
		c.fallos++
	}
	fallos := c.fallos
	c.mu.Unlock()
	if err == nil {
		c.registrar(i, ResEnviado)
		return
	}
	c.registrar(i, ResFallido)
	g.Log(fmt.Sprintf("Una iniciativa del módulo %s no salió.", i.Modulo))
	if fallos >= g.Alm.AjusteIniInt(g.BotID, "ini_errores") {
		g.ModoSeguro("errores")
	}
}

// ErroresSeguidos son los envíos fallidos seguidos de la cola.
func (c *Cola) ErroresSeguidos() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.fallos
}

func (c *Cola) reiniciarErrores() {
	c.mu.Lock()
	c.fallos = 0
	c.mu.Unlock()
}

// correr es el trabajador: saca de la cola cuando le toca a cada iniciativa.
func (c *Cola) correr(ctx context.Context) {
	for {
		hecho, espera := c.Paso(ctx)
		if hecho {
			continue
		}
		var t <-chan time.Time
		var tm *time.Timer
		if espera >= 0 {
			if espera < 50*time.Millisecond {
				espera = 50 * time.Millisecond
			}
			tm = time.NewTimer(espera)
			t = tm.C
		}
		select {
		case <-ctx.Done():
			if tm != nil {
				tm.Stop()
			}
			return
		case <-c.senal:
		case <-t:
		}
		if tm != nil {
			tm.Stop()
		}
	}
}
