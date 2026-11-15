// Package comandos es el despachador de comandos con prefijo ("!" por defecto, configurable). Cada comando se registra con su
// nombre, alias, casilla de permiso, ámbito (privado / grupo) y si pide confirmación con código. El bot solo RESPONDE: los
// comandos del Dueño solo valen desde "Mensajes guardados" o la app, y los demás rangos los usan en su chat o grupo.
package comandos

import (
	"botchat/internal/respuestas"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"botchat/internal/almacen"
	"botchat/internal/identidad"
	"botchat/internal/permisos"
)

// Ámbitos donde vale un comando.
const (
	Privado uint8 = 1
	Grupo   uint8 = 2
	Ambos         = Privado | Grupo
)

// Contexto es lo que recibe un comando al ejecutarse.
type Contexto struct {
	BotID     int64
	Persona   string // clave canónica de quien lo pidió ("yo" si es el Dueño)
	Nombre    string
	Chat      string
	EsGrupo   bool
	MensajeID string
	Menciones []string // personas mencionadas en el mensaje (claves guardadas)
	CitaAutor string   // autor del mensaje citado, si lo hay
	CitaID    string   // id del mensaje citado, si lo hay
	Args      []string // palabras después del comando
	Resto     string   // lo mismo, como texto
	Dueno     bool     // viene de Mensajes guardados o de la app
	Rol       *almacen.Rol
	D         *Despachador
	// Archivo: si el comando lo pone, la respuesta lleva ese archivo guardado (por ejemplo un sticker) además del texto.
	Archivo *Archivo
}

// Archivo es un archivo ya guardado (cifrado) en el teléfono que sale como respuesta de un comando. El motor lo manda y el sobrante
// se limpia solo con los adjuntos huérfanos.
type Archivo struct{ ID, Tipo, Mime, Nombre string }

// Comando es una orden registrable.
type Comando struct {
	Nombre    string
	Alias     []string
	Resumen   string // una línea para la ayuda
	Uso       string // ejemplo, por ejemplo "!asignar <persona> <rango> [24h|7d|permanente]"
	Casilla   string // permiso que pide ("" = cualquiera)
	Ambito    uint8  // Privado, Grupo o Ambos (0 = Ambos)
	Confirma  bool   // pide código de confirmación antes de ejecutar
	SoloDueno bool   // solo el Dueño desde Mensajes guardados o la app
	// Siempre: se atiende aunque el bot esté pausado en ese chat (por ejemplo !reanudar).
	Siempre bool
	// Validar (opcional) revisa los argumentos ANTES de pedir el código de confirmación (por ejemplo que se indicó a quién), para no
	// gastar un código en algo que de todos modos no se puede hacer.
	Validar  func(*Contexto) error
	Ejecutar func(*Contexto) (string, error)
}

// Resultado es lo que sale de ejecutar un texto.
type Resultado struct {
	Manejado bool     // el texto era un comando conocido (si no, que siga el motor de menús)
	Texto    string   // respuesta ("" = no contestar)
	Archivo  *Archivo // archivo que acompaña la respuesta (nil = ninguno)
}

// Despachador guarda los comandos y los ejecuta con permisos, límites y confirmaciones.
type Despachador struct {
	Alm   *almacen.Almacen
	BotID int64
	Perm  *permisos.Servicio
	Ahora func() time.Time
	// AlCodigo muestra el código de confirmación al dueño (notificación de Android y diálogo en la app). Nunca va al chat.
	AlCodigo func(c almacen.Confirmacion, descripcion string)
	// Responder manda un texto a un chat citando un mensaje (respuesta diferida, cuando el dueño confirma desde la app).
	Responder func(chat, citaID, texto string)
	// Extra da líneas de ayuda de comandos que no están registrados aquí (los comandos propios del dueño), según el rango de quien pregunta.
	Extra func(cx *Contexto) []string

	mu    sync.RWMutex
	mapa  map[string]*Comando
	orden []string
}

// Nuevo crea el despachador con los comandos de esta etapa (!ayuda, !rango, !asignar, !quitarrango, !confirmar).
func Nuevo(alm *almacen.Almacen, botID int64, perm *permisos.Servicio) *Despachador {
	d := &Despachador{Alm: alm, BotID: botID, Perm: perm, Ahora: time.Now, mapa: map[string]*Comando{}}
	d.registrarBase()
	return d
}

func clave(s string) string { return identidad.Normalizar(s) }

// Registrar agrega un comando (los módulos lo llaman al iniciarse). Falla si el nombre o un alias ya existe.
func (d *Despachador) Registrar(c Comando) error {
	if strings.TrimSpace(c.Nombre) == "" || c.Ejecutar == nil {
		return errors.New("el comando necesita nombre y función")
	}
	if c.Ambito == 0 {
		c.Ambito = Ambos
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	nombres := append([]string{c.Nombre}, c.Alias...)
	for _, n := range nombres {
		if _, ok := d.mapa[clave(n)]; ok {
			return fmt.Errorf("el comando %q ya existe", n)
		}
	}
	cp := c
	for _, n := range nombres {
		d.mapa[clave(n)] = &cp
	}
	d.orden = append(d.orden, clave(c.Nombre))
	return nil
}

// Quitar saca un comando (con sus alias).
func (d *Despachador) Quitar(nombre string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	c, ok := d.mapa[clave(nombre)]
	if !ok {
		return
	}
	for k, v := range d.mapa {
		if v == c {
			delete(d.mapa, k)
		}
	}
	n := d.orden[:0]
	for _, o := range d.orden {
		if o != clave(c.Nombre) {
			n = append(n, o)
		}
	}
	d.orden = n
}

func (d *Despachador) ahora() time.Time {
	if d.Ahora != nil {
		return d.Ahora()
	}
	return time.Now()
}

// Prefijo actual de los comandos.
func (d *Despachador) Prefijo() string { return d.Perm.Prefijo() }

// Parsear separa un texto con prefijo en (palabra, argumentos, resto). ok = false si no empieza con el prefijo y una letra.
func (d *Despachador) Parsear(texto string) (palabra string, args []string, resto string, ok bool) {
	t := strings.TrimSpace(texto)
	p := d.Prefijo()
	if !strings.HasPrefix(t, p) {
		return "", nil, "", false
	}
	t = t[len(p):]
	if t == "" {
		return "", nil, "", false
	}
	r := []rune(t)[0]
	if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r > 127) {
		return "", nil, "", false
	}
	campos := strings.Fields(t)
	palabra = campos[0]
	args = campos[1:]
	resto = strings.TrimSpace(strings.TrimPrefix(t, campos[0]))
	return palabra, args, resto, true
}

// EsComando dice si el texto empieza con el prefijo seguido de una letra (aunque el comando no exista).
func (d *Despachador) EsComando(texto string) bool {
	_, _, _, ok := d.Parsear(texto)
	return ok
}

// Existe dice si el texto es un comando registrado.
func (d *Despachador) Existe(texto string) bool {
	p, _, _, ok := d.Parsear(texto)
	if !ok {
		return false
	}
	d.mu.RLock()
	defer d.mu.RUnlock()
	_, hay := d.mapa[clave(p)]
	return hay
}

// SiemprePermitido dice si el texto es un comando que se atiende aunque el bot esté pausado en el chat (por ejemplo !reanudar).
func (d *Despachador) SiemprePermitido(texto string) bool {
	p, _, _, ok := d.Parsear(texto)
	if !ok {
		return false
	}
	d.mu.RLock()
	defer d.mu.RUnlock()
	c := d.mapa[clave(p)]
	return c != nil && c.Siempre
}

// Entrada es un mensaje que puede ser comando.
type Entrada struct {
	Persona   string
	Nombre    string
	Chat      string
	EsGrupo   bool
	MensajeID string
	Texto     string
	Menciones []string
	CitaAutor string
	CitaID    string
	Dueno     bool // llegó por Mensajes guardados o por la app
}

// Ejecutar interpreta el texto. Si no es un comando conocido y no viene del Dueño, devuelve Manejado=false para que siga el motor
// de menús. Aplica ámbito, permiso, límite por minuto y confirmación; cada intento queda en el registro de acciones (sin textos).
func (d *Despachador) Ejecutar(e Entrada) Resultado {
	r := d.ejecutar(e)
	if r.Texto != "" {
		r.Texto = respuestas.Aplicar(d.Alm, d.BotID, r.Texto) // «cambiar cualquier otra frase»: lo que el dueño reemplazó
	}
	return r
}

func (d *Despachador) ejecutar(e Entrada) Resultado {
	palabra, args, resto, ok := d.Parsear(e.Texto)
	if !ok {
		return Resultado{}
	}
	d.mu.RLock()
	cmd := d.mapa[clave(palabra)]
	d.mu.RUnlock()
	persona := d.Perm.Persona(e.Persona)
	if e.Dueno {
		persona = almacen.PersonaDueno
	}
	if cmd == nil {
		if e.Dueno {
			return Resultado{Manejado: true, Texto: d.R("cmd.desconocido", "prefijo", d.Prefijo())}
		}
		return Resultado{}
	}
	cx := &Contexto{BotID: d.BotID, Persona: persona, Nombre: e.Nombre, Chat: e.Chat, EsGrupo: e.EsGrupo, MensajeID: e.MensajeID, Menciones: e.Menciones,
		CitaAutor: e.CitaAutor, CitaID: e.CitaID, Args: args, Resto: resto, Dueno: persona == almacen.PersonaDueno && e.Dueno, D: d}
	cx.Rol = d.Perm.RolDe(persona)
	registro := func(res, det string) {
		d.Alm.RegistrarAccion(d.BotID, persona, "cmd."+cmd.Nombre, "", e.Chat, res, det)
	}
	if ok, avisar := d.Perm.Limitar(persona); !ok {
		registro("limite", "")
		if avisar {
			return Resultado{Manejado: true, Texto: d.R("cmd.muy_rapido")}
		}
		return Resultado{Manejado: true}
	}
	if cmd.SoloDueno && !cx.Dueno {
		registro("denegado", "solo_dueno")
		return Resultado{Manejado: true, Texto: d.R("cmd.solo_dueno")}
	}
	if !cx.Dueno {
		if (e.EsGrupo && cmd.Ambito&Grupo == 0) || (!e.EsGrupo && cmd.Ambito&Privado == 0) {
			registro("denegado", "ambito")
			if e.EsGrupo {
				return Resultado{Manejado: true, Texto: d.R("cmd.solo_privado")}
			}
			return Resultado{Manejado: true, Texto: d.R("cmd.solo_grupo")}
		}
		if cmd.Casilla != "" && !d.Perm.Puede(persona, cmd.Casilla, e.EsGrupo, e.Chat) {
			registro("denegado", "permiso")
			return Resultado{Manejado: true, Texto: d.R("cmd.sin_permiso")}
		}
	}
	if cmd.Confirma {
		if cmd.Validar != nil {
			if err := cmd.Validar(cx); err != nil {
				registro("error", "validar")
				return Resultado{Manejado: true, Texto: mensajeError(err)}
			}
		}
		// Al confirmar ya no hay menciones ni cita: la persona y el mensaje citado se guardan resueltos en los argumentos.
		if obj, resto, ok := cx.Objetivo(); ok {
			args = append([]string{"persona:" + obj}, resto...)
		}
		if e.CitaID != "" {
			args = append(append([]string(nil), args...), citaPrefijo+e.CitaID)
		}
		cf, err := d.Alm.CrearConfirmacion(d.BotID, persona, cmd.Nombre, args, e.Chat, e.MensajeID, d.vida(), d.ahora())
		if err != nil {
			registro("error", "confirmacion")
			return Resultado{Manejado: true, Texto: d.R("cmd.confirmacion_falla")}
		}
		registro("confirmacion", "")
		if d.AlCodigo != nil {
			d.AlCodigo(*cf, descripcion(cmd, args))
		}
		pref := d.Prefijo()
		if cx.Dueno {
			return Resultado{Manejado: true, Texto: d.R("cmd.confirma_dueno", "prefijo", pref)}
		}
		return Resultado{Manejado: true, Texto: d.R("cmd.confirma_otro", "prefijo", pref)}
	}
	return d.correr(cmd, cx, registro)
}

func (d *Despachador) vida() time.Duration {
	s := d.Alm.AjusteBotInt(d.BotID, "confirm_seg")
	if s < 15 {
		s = 120
	}
	return time.Duration(s) * time.Second
}

func descripcion(c *Comando, args []string) string {
	t := "Acción: " + c.Nombre
	if c.Resumen != "" {
		t = c.Resumen
	}
	return t
}

func (d *Despachador) correr(cmd *Comando, cx *Contexto, registro func(res, det string)) Resultado {
	texto, err := func() (t string, err error) {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("error interno")
			}
		}()
		return cmd.Ejecutar(cx)
	}()
	if err != nil {
		if errors.Is(err, permisos.ErrJerarquia) || errors.Is(err, permisos.ErrSinPermiso) {
			registro("denegado", "jerarquia")
		} else {
			registro("error", "")
		}
		return Resultado{Manejado: true, Texto: mensajeError(err)}
	}
	registro("ok", "")
	return Resultado{Manejado: true, Texto: texto, Archivo: cx.Archivo}
}

func mensajeError(err error) string {
	m := err.Error()
	if m == "" {
		return "No se pudo."
	}
	m = strings.ToUpper(m[:1]) + m[1:]
	if strings.HasSuffix(m, ".") || strings.HasSuffix(m, "!") || strings.HasSuffix(m, "?") {
		return m
	}
	return m + "."
}

// Confirmar gasta un código y ejecuta la acción guardada. Si `persona` no es "", el código tiene que ser de esa persona (quien
// escribió "!confirmar"); si es "" lo aprueba el dueño desde la app y la respuesta se manda al chat de quien la pidió.
func (d *Despachador) Confirmar(codigo, persona string) (string, error) {
	cf, err := d.Alm.ConsumirConfirmacion(d.BotID, codigo, persona, d.ahora())
	if err != nil {
		d.Alm.RegistrarAccion(d.BotID, nombreActor(persona), "cmd.confirmar", "", "", "codigo_invalido", "")
		return "", errors.New("ese código no sirve: ya se usó, venció o no es tuyo")
	}
	d.mu.RLock()
	cmd := d.mapa[clave(cf.Comando)]
	d.mu.RUnlock()
	if cmd == nil {
		return "", errors.New("esa acción ya no existe")
	}
	esGrupo := strings.HasSuffix(cf.Chat, "@g.us")
	args, citaID := sacaCita(cf.Args)
	cx := &Contexto{BotID: d.BotID, Persona: cf.Persona, Chat: cf.Chat, EsGrupo: esGrupo, MensajeID: cf.CitaID, Args: args, Resto: strings.Join(args, " "),
		CitaID: citaID, Dueno: cf.Persona == almacen.PersonaDueno, D: d}
	cx.Rol = d.Perm.RolDe(cf.Persona)
	// Los permisos se vuelven a revisar al ejecutar (pudieron cambiar mientras tanto).
	if !cx.Dueno {
		if cmd.Casilla != "" && !d.Perm.Puede(cf.Persona, cmd.Casilla, esGrupo, cf.Chat) {
			d.Alm.RegistrarAccion(d.BotID, cf.Persona, "cmd."+cmd.Nombre, "", cf.Chat, "denegado", "permiso")
			return "", errors.New("ya no tienes permiso para eso")
		}
	}
	registro := func(res, det string) {
		d.Alm.RegistrarAccion(d.BotID, cf.Persona, "cmd."+cmd.Nombre, "", cf.Chat, res, det)
	}
	r := d.correr(cmd, cx, registro)
	if persona == "" && d.Responder != nil && cf.Chat != "" && r.Texto != "" {
		d.Responder(cf.Chat, cf.CitaID, r.Texto)
	}
	return r.Texto, nil
}

// citaPrefijo marca, entre los argumentos guardados de una confirmación, el mensaje que se había citado al pedir la acción.
const citaPrefijo = "cita:"

// sacaCita separa del final de los argumentos guardados el id del mensaje citado (si lo hay).
func sacaCita(args []string) ([]string, string) {
	out := make([]string, 0, len(args))
	cita := ""
	for _, a := range args {
		if strings.HasPrefix(a, citaPrefijo) {
			cita = strings.TrimPrefix(a, citaPrefijo)
			continue
		}
		out = append(out, a)
	}
	return out, cita
}

func nombreActor(p string) string {
	if p == "" {
		return almacen.PersonaDueno
	}
	return p
}

// Lista devuelve los comandos que el contexto puede usar, en el orden en que se registraron.
func (d *Despachador) Lista(cx *Contexto) []*Comando {
	d.mu.RLock()
	defer d.mu.RUnlock()
	var out []*Comando
	for _, n := range d.orden {
		c := d.mapa[n]
		if c == nil {
			continue
		}
		if c.SoloDueno && !cx.Dueno {
			continue
		}
		if !cx.Dueno {
			if (cx.EsGrupo && c.Ambito&Grupo == 0) || (!cx.EsGrupo && c.Ambito&Privado == 0) {
				continue
			}
			if c.Casilla != "" && !permisos.Permite(cx.Rol, c.Casilla, cx.EsGrupo, cx.Chat) {
				continue
			}
		}
		out = append(out, c)
	}
	return out
}

// Nombres devuelve todos los comandos registrados (para pantallas de ajustes).
func (d *Despachador) Nombres() []string {
	d.mu.RLock()
	defer d.mu.RUnlock()
	out := make([]string, 0, len(d.orden))
	for _, n := range d.orden {
		if c := d.mapa[n]; c != nil {
			out = append(out, c.Nombre)
		}
	}
	sort.Strings(out)
	return out
}

// ---------- utilidades para los comandos ----------

// Objetivo busca a quién se refiere un comando: una mención, el autor del mensaje citado o un número escrito en los argumentos.
// Devuelve la persona (clave canónica) y los argumentos sin la parte que era la persona.
func (c *Contexto) Objetivo() (persona string, resto []string, ok bool) {
	resto = append([]string(nil), c.Args...)
	numero := func(s string) string {
		d := identidad.Digitos(s)
		if len(d) < 7 || len(d) > 15 {
			return ""
		}
		return d
	}
	for i, a := range resto {
		if strings.HasPrefix(a, "persona:") {
			return c.D.Perm.Persona(strings.TrimPrefix(a, "persona:")), append(resto[:i:i], resto[i+1:]...), true
		}
		if strings.HasPrefix(a, "@") || numero(a) != "" {
			if len(c.Menciones) > 0 && strings.HasPrefix(a, "@") {
				p := c.D.Perm.Persona(c.Menciones[0])
				return p, append(resto[:i:i], resto[i+1:]...), true
			}
			if n := numero(a); n != "" {
				p := c.D.Perm.Persona(n + "@s.whatsapp.net")
				return p, append(resto[:i:i], resto[i+1:]...), true
			}
		}
	}
	if len(c.Menciones) > 0 {
		return c.D.Perm.Persona(c.Menciones[0]), resto, true
	}
	if c.CitaAutor != "" && c.CitaAutor != almacen.QuienYo {
		return c.D.Perm.Persona(c.CitaAutor), resto, true
	}
	return "", resto, false
}

// R da una respuesta del catálogo (la propia del dueño o la de fábrica).
func (d *Despachador) R(id string, vars ...string) string {
	return respuestas.Texto(d.Alm, d.BotID, id, vars...)
}
