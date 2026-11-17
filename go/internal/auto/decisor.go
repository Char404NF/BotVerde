package auto

import (
	"context"
	"regexp"
	"strings"
	"time"

	"botchat/internal/almacen"
	"botchat/internal/flujo"
	"botchat/internal/motor"
)

// DecisorConfig son las piezas del decisor de mensajes automáticos.
type DecisorConfig struct {
	BotID int64
	Alm   *almacen.Almacen
	// Base es el decisor de menús y reglas: manda primero. Lo que no resuelve queda para los mensajes automáticos.
	Base  motor.Decisor
	Ahora func() time.Time
	// Zona es la zona horaria del bot (la del teléfono).
	Zona func() *time.Location
	// Nombre da el nombre con el que se ve a esa persona en ese chat.
	Nombre func(chat, sender string) string
	// Persona lleva el JID de quien escribe a su clave canónica.
	Persona func(string) string
	// Conocido dice si es un contacto (agenda, rango o aprobada).
	Conocido func(persona, chat string) bool
	// Log es opcional.
	Log func(string)
}

// Decisor envuelve al decisor de menús: si éste no tiene nada que decir, ve si toca una bienvenida, una ausencia o un recibido, y
// cuando un menú termina, el cierre. Implementa motor.Decisor, motor.Admisor, motor.AdmisorMedios y motor.ConDiferidas.
type Decisor struct{ c DecisorConfig }

// Nuevo crea el decisor.
func Nuevo(c DecisorConfig) *Decisor {
	if c.Ahora == nil {
		c.Ahora = time.Now
	}
	if c.Zona == nil {
		c.Zona = func() *time.Location { return time.Local }
	}
	if c.Persona == nil {
		c.Persona = func(p string) string { return p }
	}
	if c.Nombre == nil {
		c.Nombre = func(chat, sender string) string { return "" }
	}
	if c.Log == nil {
		c.Log = func(string) {}
	}
	return &Decisor{c: c}
}

func (d *Decisor) activo() bool { return d.c.Alm.AjusteBot(d.c.BotID, "auto_activo") != "0" }

// zonaDefecto es la zona de los mensajes que no traen una: la de Ajustes si hay una válida, si no la del teléfono.
func (d *Decisor) zonaDefecto() *time.Location {
	if z := strings.TrimSpace(d.c.Alm.AjusteBot(d.c.BotID, "auto_zona")); z != "" {
		if l, err := time.LoadLocation(z); err == nil {
			return l
		}
	}
	return d.c.Zona()
}

// AdmiteEnGrupo pasa la pregunta a la base.
func (d *Decisor) AdmiteEnGrupo(msg motor.Mensaje) bool {
	ad, ok := d.c.Base.(motor.Admisor)
	return ok && ad.AdmiteEnGrupo(msg)
}

// EsperaTexto: un menú está esperando texto.
func (d *Decisor) EsperaTexto(msg motor.Mensaje) bool {
	ad, ok := d.c.Base.(motor.Admisor)
	return ok && ad.EsperaTexto(msg)
}

// Diferidas pasa la fila de espera de los grupos a la base.
func (d *Decisor) Diferidas(ahora time.Time) []motor.Diferida {
	if cd, ok := d.c.Base.(motor.ConDiferidas); ok {
		return cd.Diferidas(ahora)
	}
	return nil
}

// AdmiteSinTexto: una imagen, audio o archivo sin menú esperando puede recibir un mensaje automático si hay alguno activo que
// cuente con eso: solo "recibido", "bienvenida" y "ausencia" contestan a lo que no es texto.
func (d *Decisor) AdmiteSinTexto(msg motor.Mensaje) bool {
	if msg.EsMio || !d.activo() {
		return false
	}
	autos, err := d.c.Alm.ListarAutos(d.c.BotID, "")
	if err != nil {
		return false
	}
	for _, a := range autos {
		if a.Activo && (a.Tipo == almacen.AutoRecibido || a.Tipo == almacen.AutoBienvenida || a.Tipo == almacen.AutoAusencia) {
			return true
		}
	}
	return false
}

// Decidir implementa motor.Decisor con la prioridad: menú > regla > bienvenida/ausencia > recibido; y el cierre tras un menú.
func (d *Decisor) Decidir(ctx context.Context, ultimo motor.Mensaje, lote []motor.Mensaje) (motor.Decision, error) {
	dec, err := d.c.Base.Decidir(ctx, ultimo, lote)
	if err != nil || !d.activo() {
		return dec, err
	}
	ev := d.evento(ultimo, lote, d.c.Ahora())
	// El Saludo de fábrica tiene la prioridad más baja: si hay bienvenida, ausencia o recibido que le toquen, salen ellos y no "hola".
	if dec.Sistema && !dec.PasarAHumano {
		salidas := d.elegir(almacen.AutoBienvenida, ev, false)
		salidas = append(salidas, d.elegir(almacen.AutoAusencia, ev, false)...)
		if len(salidas) == 0 {
			salidas = d.elegir(almacen.AutoRecibido, ev, false)
		}
		if len(salidas) > 0 {
			return motor.Decision{Respuestas: salidas, Coincidio: "mensaje automático"}, nil
		}
		return dec, nil
	}
	// Un menú terminó bien: sale el cierre detrás de su último mensaje.
	if dec.Termino {
		if s := d.elegir(almacen.AutoCierre, ev, false); len(s) > 0 {
			dec.Respuestas = append(dec.Respuestas, s...)
		}
		return dec, nil
	}
	// Algo se ocupó del mensaje (menú, regla, comando, una persona) o hay un motivo para callar: los automáticos no se meten.
	if dec.PasarAHumano || dec.Coincidio != "" || len(dec.Respuestas) > 0 {
		return dec, nil
	}
	if dec.Omitir != "" && dec.Omitir != almacen.MotNoTexto {
		return dec, nil
	}
	salidas := d.elegir(almacen.AutoBienvenida, ev, false)
	salidas = append(salidas, d.elegir(almacen.AutoAusencia, ev, false)...)
	if len(salidas) == 0 {
		salidas = d.elegir(almacen.AutoRecibido, ev, false)
	}
	if len(salidas) == 0 {
		return dec, nil
	}
	return motor.Decision{Respuestas: salidas, Coincidio: "mensaje automático"}, nil
}

// evento junta lo que importa de un mensaje recibido.
type evento struct {
	Chat, Remitente string
	Persona         string
	EsGrupo         bool
	Primer          time.Time // hora del primer mensaje del bloque
	Ahora           time.Time
}

func (d *Decisor) evento(ultimo motor.Mensaje, lote []motor.Mensaje, ahora time.Time) evento {
	e := evento{Chat: ultimo.Chat, Remitente: ultimo.Remitente, EsGrupo: ultimo.EsGrupo, Ahora: ahora, Primer: ultimo.TS}
	if ultimo.EsGrupo {
		e.Persona = d.c.Persona(ultimo.Remitente)
	} else {
		e.Persona = d.c.Persona(ultimo.Chat)
	}
	for _, x := range lote {
		if !x.TS.IsZero() && (e.Primer.IsZero() || x.TS.Before(e.Primer)) {
			e.Primer = x.TS
		}
	}
	if e.Primer.IsZero() {
		e.Primer = ahora
	}
	return e
}

func (d *Decisor) persona(e evento) Persona {
	return Persona{Clave: e.Persona, Chat: e.Chat, EsGrupo: e.EsGrupo, Canon: d.c.Persona, Conocida: func() bool {
		return d.c.Conocido != nil && d.c.Conocido(e.Persona, e.Chat)
	}}
}

// elegir da las salidas del primer mensaje activo de ese tipo que le toca a esta persona ahora (por orden). `probar` es el simulador:
// no mira la repetición ni el tope.
func (d *Decisor) elegir(tipo string, e evento, probar bool) []motor.Salida {
	autos, err := d.c.Alm.ListarAutos(d.c.BotID, tipo)
	if err != nil {
		return nil
	}
	for i := range autos {
		m := autos[i]
		if !m.Activo {
			continue
		}
		cfg := LeerConfig(m.Tipo, m.Config)
		if ok, _ := d.leToca(m, cfg, e, probar); !ok {
			continue
		}
		if s := d.salidasDe(m, cfg, e, probar); len(s) > 0 {
			return s
		}
	}
	return nil
}

// leToca dice si ese mensaje le corresponde a esa persona ahora y, si no, por qué (para el simulador).
func (d *Decisor) leToca(m almacen.AutoMensaje, cfg Config, e evento, ignorarRepeticion bool) (bool, string) {
	if !cfg.Quien.Aplica(d.persona(e)) {
		return false, "no es para esta persona"
	}
	if !cfg.Cuando.Activo(e.Ahora, d.zonaDefecto()) {
		return false, "no es el momento"
	}
	ult := d.c.Alm.UltimaRepeticion(d.c.BotID, m.ID, e.Persona)
	if m.Tipo == almacen.AutoBienvenida {
		var ant int64
		if e.EsGrupo {
			ant = d.c.Alm.UltimoMensajeAnterior(d.c.BotID, e.Chat, e.Remitente, e.Primer.Unix())
		} else {
			ant = d.c.Alm.UltimoMensajeAnterior(d.c.BotID, e.Chat, "", e.Primer.Unix())
		}
		primera := ant == 0
		silencio := cfg.SilencioDias > 0 && ant > 0 && e.Primer.Sub(time.Unix(ant, 0)) >= time.Duration(cfg.SilencioDias)*24*time.Hour
		if !primera && !silencio {
			return false, "ya había escrito"
		}
		if !ignorarRepeticion && ult > 0 {
			// ya se le dio la bienvenida: solo vuelve a tocar tras el silencio configurado (nunca si es solo "primera vez")
			if cfg.SilencioDias == 0 || e.Ahora.Sub(time.Unix(ult, 0)) < time.Duration(cfg.SilencioDias)*24*time.Hour {
				return false, "ya se le dio la bienvenida"
			}
		}
		return true, ""
	}
	if !ignorarRepeticion {
		if cada := cfg.Repeticion.Duracion(); cada > 0 && ult > 0 && e.Ahora.Sub(time.Unix(ult, 0)) < cada {
			return false, "ya se le mandó hace poco"
		}
	}
	return true, ""
}

// salidasDe arma lo que se manda de un mensaje: el texto con las variables puestas y hasta 3 archivos. Aplica el tope por hora.
func (d *Decisor) salidasDe(m almacen.AutoMensaje, cfg Config, e evento, probar bool) []motor.Salida {
	soloTexto := false
	if tope := d.c.Alm.AjusteBotInt(d.c.BotID, "auto_tope_hora"); tope > 0 && !probar && !d.c.Alm.SinLimites(d.c.BotID) {
		usados := d.c.Alm.ContarAutoEnvios(d.c.BotID, e.Ahora.Add(-time.Hour))
		if usados >= tope*3 {
			// muy por encima del tope: no se manda nada (se anota para que se vea)
			d.c.Alm.RegistrarAutoEnvio(d.c.BotID, almacen.AutoEnvio{TS: e.Ahora.Unix(), AutoID: m.ID, Tipo: m.Tipo, Chat: e.Chat, Resultado: almacen.AutoTopeDuro, Motivo: "tope_por_hora"})
			return nil
		}
		soloTexto = usados >= tope && len(m.Adjuntos) > 0
	}
	texto := d.Render(m.Texto, cfg, e.Chat, e.Persona, e.Ahora)
	var medios []motor.Medio
	if !soloTexto {
		for _, a := range m.Adjuntos {
			medios = append(medios, motor.Medio{ID: a.ID, Tipo: a.Tipo, Mime: a.Mime, Nombre: a.Nombre})
		}
	}
	salidas := motor.ArmarSalidas(texto, medios)
	if len(salidas) == 0 {
		return nil
	}
	info := &motor.AutoInfo{ID: m.ID, Tipo: m.Tipo, Persona: e.Persona, Chat: e.Chat, SoloTxt: soloTexto}
	for i := range salidas {
		salidas[i].Auto = info
		salidas[i].Etiqueta = m.Tipo
	}
	return salidas
}

// AlAuto anota un mensaje automático que ya salió: cuándo se le mandó a esa persona (la repetición) y el registro sin textos.
func (d *Decisor) AlAuto(a *motor.AutoInfo) {
	if a == nil {
		return
	}
	d.c.Alm.MarcarRepeticion(d.c.BotID, a.ID, a.Persona, d.c.Ahora())
	res := almacen.AutoOK
	switch {
	case a.AdjFallo > 0:
		res = almacen.AutoParcial
	case a.SoloTxt:
		res = almacen.AutoSoloTexto
	}
	mot := a.Motivo
	if mot == "" && a.SoloTxt {
		mot = "tope_por_hora"
	}
	d.c.Alm.RegistrarAutoEnvio(d.c.BotID, almacen.AutoEnvio{TS: d.c.Ahora().Unix(), AutoID: a.ID, Tipo: a.Tipo, Chat: a.Chat, Resultado: res, AdjOK: a.AdjOK, AdjFallo: a.AdjFallo, Motivo: mot})
}

// ---------- variables ----------

var reVar = regexp.MustCompile(`\{([a-zA-Z0-9_áéíóúÁÉÍÓÚ]+)\}`)
var dias = []string{"domingo", "lunes", "martes", "miércoles", "jueves", "viernes", "sábado"}
var reEspacioAntes = regexp.MustCompile(`[ \t]+([,.;:!?])`)
var reEspacios = regexp.MustCompile(`[ \t]{2,}`)

// Render pone {nombre} {hora} {dia} {fecha} {horario} en un texto. Si no se sabe el nombre, la variable desaparece y se arregla el
// espacio que dejaba ("Hola , " → "Hola,"). Las demás llaves se dejan tal cual.
func (d *Decisor) Render(texto string, cfg Config, chat, persona string, ahora time.Time) string {
	return d.renderCon(texto, cfg, func() string { return d.c.Nombre(chat, persona) }, ahora)
}

func (d *Decisor) renderCon(texto string, cfg Config, nombre func() string, ahora time.Time) string {
	t := ahora.In(cfg.Cuando.ZonaDe(d.zonaDefecto()))
	sinNombre := false
	out := reVar.ReplaceAllStringFunc(texto, func(x string) string {
		switch strings.ToLower(x[1 : len(x)-1]) {
		case "nombre":
			n := strings.TrimSpace(nombre())
			if n == "" {
				sinNombre = true
			}
			return n
		case "hora":
			return t.Format("15:04")
		case "dia", "día":
			return dias[int(t.Weekday())]
		case "fecha":
			return t.Format("02/01/2006")
		case "horario":
			return d.describirHorario(cfg)
		}
		return x
	})
	if sinNombre {
		out = reEspacioAntes.ReplaceAllString(out, "$1")
		out = reEspacios.ReplaceAllString(out, " ")
	}
	return strings.TrimSpace(out)
}

// describirHorario es el horario de atención: el del propio mensaje si trae uno, si no el general de Ajustes → Menús.
func (d *Decisor) describirHorario(cfg Config) string {
	if len(cfg.Cuando.Horario) > 0 {
		return DescribirSemana(cfg.Cuando.Horario)
	}
	if j := strings.TrimSpace(d.c.Alm.AjusteBot(d.c.BotID, "menu_horario")); j != "" {
		var h flujo.Horario
		if jsonUnmarshal(j, &h) && (len(h.Dias) > 0 || h.Desde != "" || h.Hasta != "") {
			return flujo.DescribirHorario(h)
		}
	}
	return "todos los días"
}
