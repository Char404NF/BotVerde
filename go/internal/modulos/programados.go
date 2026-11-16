package modulos

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"botchat/internal/almacen"
	"botchat/internal/comandos"
	"botchat/internal/permisos"
)

// ---------- H3 · Mensajes programados ----------
//
// Un mensaje a un chat o a una lista de destinatarios, con fecha y hora y repetición opcional (diaria, semanal o anual para
// cumpleaños). La cola de iniciativas es en memoria, así que la agenda es propia (mod_agenda): cada minuto se revisa qué ya toca. En
// Android, una alarma (AlarmManager) despierta a la app a la hora del siguiente. Si el teléfono estuvo apagado a la hora, el mensaje
// sale tarde SOLO si faltan menos de X minutos (configurable); si no, se descarta y me llega un aviso. Va con tope por día.

// IDProgramados es el id del módulo.
const IDProgramados = "programados"

const (
	qProgramado = "programado"

	repNinguna = ""
	repDiaria  = "diaria"
	repSemanal = "semanal"
	repAnual   = "anual"

	pgActivo    = "activo"
	pgTerminado = "terminada"
	pgCancelado = "cancelado"
)

func textoRepeticion(r string) string {
	switch r {
	case repDiaria:
		return "cada día"
	case repSemanal:
		return "cada semana"
	case repAnual:
		return "cada año"
	}
	return "una sola vez"
}

// siguienteFecha da la primera ocurrencia posterior a `ahora` de un mensaje que se repite.
func siguienteFecha(t time.Time, rep string, ahora time.Time) time.Time {
	for i := 0; i < 4000 && !t.After(ahora); i++ {
		switch rep {
		case repDiaria:
			t = t.AddDate(0, 0, 1)
		case repSemanal:
			t = t.AddDate(0, 0, 7)
		case repAnual:
			t = t.AddDate(1, 0, 0)
		default:
			return time.Time{}
		}
	}
	return t
}

func programados() Modulo {
	return Nuevo(Def{
		ID: IDProgramados, Nombre: "Mensajes programados", Icono: "reloj", Riesgo: RiesgoMedio, Iniciativa: true,
		Descripcion: "Manda un mensaje a un chat o a una lista en la fecha y hora que elijas, una vez o repetido (cada día, cada semana o cada año, para cumpleaños). Si el teléfono estaba apagado, sale tarde solo si faltan pocos minutos; si no, te avisa y lo descarta.",
		Casillas: []permisos.Casilla{
			{ID: "programados.ver", Nombre: "Ver los mensajes programados", Descripcion: "Puede pedir con «!agenda» la lista de lo que está programado (sin los textos).", Grupo: "Mensajes programados", Riesgo: permisos.RiesgoMedio},
		},
		Esquema: []Campo{
			{Clave: "tolerancia_min", Titulo: "Minutos de tolerancia", Ayuda: "Si el teléfono estaba apagado o sin conexión a la hora, el mensaje sale tarde solo si pasaron menos de estos minutos.", Tipo: CampoNumero, Defecto: 30.0, Min: 1, Max: 1440},
			{Clave: "avisar_perdido", Titulo: "Avisarme si uno no salió", Ayuda: "Te llega una notificación cuando un mensaje se descarta.", Tipo: CampoInterruptor, Defecto: true},
		},
		TopeHora: 10, TopeDia: 20,
		AlActivar: func(g *Gestor) error {
			programadosRecupera(g)
			programadosTick(g)
			return nil
		},
		AlEvento: func(g *Gestor, e Evento) {
			if e.Tipo == EvTick {
				programadosTick(g)
			}
		},
		AlEstado: func(g *Gestor) string {
			fs, _ := g.Alm.ListarAgenda(g.BotID, IDProgramados)
			n := 0
			var prox int64
			for _, f := range fs {
				if f.Estado == pgActivo {
					n++
					if prox == 0 || (f.ProximoTS > 0 && f.ProximoTS < prox) {
						prox = f.ProximoTS
					}
				}
			}
			if n == 0 {
				return ""
			}
			return fmt.Sprintf("Encendido · %d programados · próximo %s", n, g.FechaTexto(prox))
		},
		ComandosG: func(g *Gestor) []comandos.Comando {
			return []comandos.Comando{{
				Nombre: "agenda", Resumen: "Ver lo que está programado", Uso: "agenda", Casilla: "programados.ver", Ambito: comandos.Privado,
				Ejecutar: func(cx *comandos.Contexto) (string, error) {
					fs, _ := g.Alm.ListarAgenda(g.BotID, IDProgramados)
					var b strings.Builder
					n := 0
					for _, f := range fs {
						if f.Estado != pgActivo || n >= 8 {
							continue
						}
						n++
						fmt.Fprintf(&b, "• %s — %s (%s)\n", f.Nombre, g.FechaTexto(f.ProximoTS), textoRepeticion(f.Repeticion))
					}
					if n == 0 {
						return "No hay mensajes programados.", nil
					}
					return strings.TrimSpace(b.String()), nil
				},
			}}
		},
		Paneles:  panelesProgramados,
		AlFilas:  filasProgramados,
		AlAccion: accionProgramados,
	})
}

// destinosProgramado da las personas o chats a los que sale un mensaje: el chat tal cual, o los miembros de la lista.
func (g *Gestor) destinosProgramado(destino string) []string {
	if lista, ok := strings.CutPrefix(destino, "lista:"); ok {
		m, _ := g.Alm.MiembrosLista(g.BotID, lista, 100)
		return m
	}
	return []string{destino}
}

func programadosAviso(g *Gestor, cfg Config, f almacen.AgendaFila, motivo string) {
	if g.Aviso != nil && cfg.Bool("avisar_perdido") {
		g.Aviso("Mensaje programado sin enviar", "«"+f.Nombre+"»: "+motivo)
	}
}

// programadosTick encola lo que ya toca, descarta lo que llegó demasiado tarde y deja lista la siguiente fecha de lo que se repite.
func programadosTick(g *Gestor) {
	cfg := g.Config(IDProgramados)
	ahora := g.ahora()
	vencidas, _ := g.Alm.AgendaVencida(g.BotID, IDProgramados, ahora)
	tol := time.Duration(cfg.Int("tolerancia_min")) * time.Minute
	for _, f := range vencidas {
		prog := time.Unix(f.ProximoTS, 0).In(g.zona())
		tarde := ahora.Sub(prog)
		avanza := func(res string) {
			if f.Repeticion != repNinguna {
				if sig := siguienteFecha(prog, f.Repeticion, ahora); !sig.IsZero() {
					_ = g.Alm.ActualizarAgenda(g.BotID, f.ID, pgActivo, sig.Unix(), ahora, res, res == "encolado")
					return
				}
			}
			_ = g.Alm.ActualizarAgenda(g.BotID, f.ID, pgTerminado, 0, ahora, res, res == "encolado")
		}
		if tarde > tol {
			g.Alm.RegistrarModulo(g.BotID, IDProgramados, qProgramado, ResDescartado+"tarde", "", ahora)
			programadosAviso(g, cfg, f, fmt.Sprintf("debía salir el %s, pero el teléfono estaba apagado o sin conexión y ya pasó el tiempo de tolerancia. Se descartó.", g.FechaTexto(f.ProximoTS)))
			avanza("tarde")
			continue
		}
		if !g.Maestro() || g.EnModoSeguro() {
			continue // se reintenta cada minuto hasta que se acabe la tolerancia
		}
		destinos := g.destinosProgramado(f.Destino)
		if len(destinos) == 0 {
			programadosAviso(g, cfg, f, "la lista de destinatarios está vacía.")
			avanza("sin_destino")
			continue
		}
		avanza("encolado") // primero se anota, para que el resultado del envío (que puede llegar enseguida) quede después
		encolaProgramado(g, cfg, f, destinos, tol)
	}
}

func encolaProgramado(g *Gestor, cfg Config, f almacen.AgendaFila, destinos []string, tol time.Duration) {
	var c contenidoDif
	_ = json.Unmarshal([]byte(f.Contenido), &c)
	fila := f
	for _, d := range destinos {
		d := d
		texto := sustituir(c.Texto, map[string]string{"nombre": g.nombreOPalabra(d, ""), "fecha": g.Hora().Format("02/01/2006"), "hora": g.Hora().Format("15:04")})
		medio, extras := medioDe(c.Adjuntos, texto)
		personas := []string{d}
		if strings.HasSuffix(d, "@g.us") {
			personas = nil
		}
		ini := Iniciativa{Modulo: IDProgramados, Que: qProgramado, Chat: d, Personas: personas, Texto: texto, Medio: medio, Extras: extras,
			Noche: NocheDescartar, EsperaMax: tol, RetrasoMin: 0, RetrasoMax: 3 * time.Second,
			AlResultado: func(r string) {
				switch {
				case r == ResEnviado:
					if cur, _ := g.Alm.ObtenerAgenda(g.BotID, fila.ID); cur != nil && cur.UltimoRes == "encolado" {
						_ = g.Alm.ActualizarAgenda(g.BotID, fila.ID, cur.Estado, cur.ProximoTS, time.Time{}, "enviado", false)
					}
				case r == ResFallido:
					programadosAviso(g, cfg, fila, "WhatsApp no dejó enviarlo.")
				case r == ResDescartado+MotNoMolestar:
					// esa persona pidió no recibir mensajes: no hay nada que avisar
				case strings.HasPrefix(r, ResDescartado):
					programadosAviso(g, cfg, fila, "no salió ("+motivoLegible(strings.TrimPrefix(r, ResDescartado))+").")
				}
			}}
		if _, err := g.Enviar(ini); err != nil {
			programadosAviso(g, cfg, fila, "no se pudo preparar el envío.")
		}
	}
}

func motivoLegible(m string) string {
	switch m {
	case MotNoche:
		return "cayó en la pausa nocturna"
	case MotTope:
		return "se pasó del tope de mensajes"
	case MotSinConexion:
		return "no había conexión"
	case MotMaestro:
		return "«Iniciativa propia» estaba apagada"
	case MotSeguro:
		return "el modo seguro estaba activo"
	case MotModulo:
		return "el módulo se apagó"
	case MotColaLlena:
		return "había demasiados envíos en espera"
	}
	return m
}

// programadosRecupera: la cola se perdió al cerrar la app. Lo que quedó «encolado» sin confirmarse y todavía está dentro de la
// tolerancia se vuelve a mandar si iba a un solo chat; si iba a una lista no se sabe a quién ya le llegó, así que solo se avisa.
func programadosRecupera(g *Gestor) {
	cfg := g.Config(IDProgramados)
	tol := time.Duration(cfg.Int("tolerancia_min")) * time.Minute
	ahora := g.ahora()
	fs, _ := g.Alm.ListarAgenda(g.BotID, IDProgramados)
	for _, f := range fs {
		if f.UltimoRes != "encolado" {
			continue
		}
		_ = g.Alm.ActualizarAgenda(g.BotID, f.ID, f.Estado, f.ProximoTS, time.Time{}, "recuperado", false)
		if ahora.Sub(time.Unix(f.UltimoTS, 0)) > tol {
			programadosAviso(g, cfg, f, "se cerró la app antes de que saliera y ya pasó el tiempo de tolerancia. Se descartó.")
			continue
		}
		if strings.HasPrefix(f.Destino, "lista:") {
			programadosAviso(g, cfg, f, "se cerró la app mientras salía a una lista y no se sabe a quién le llegó. No se reenvió para no duplicar.")
			continue
		}
		encolaProgramado(g, cfg, f, []string{f.Destino}, tol)
	}
}

// ---------- paneles ----------

func (g *Gestor) opcionesDestino() []Opcion {
	out := g.opcionesChats(false, 100)
	out = append(out, g.opcionesChats(true, 60)...)
	for _, l := range g.Alm.ListasConConteo(g.BotID) {
		out = append(out, Opcion{Valor: "lista:" + l.Lista, Titulo: fmt.Sprintf("Lista «%s» (%d)", l.Lista, l.N)})
	}
	return out
}

func panelesProgramados(g *Gestor) []Panel {
	return []Panel{{
		ID: "programados", Titulo: "Mensajes programados",
		Ayuda: "Elige a dónde, la fecha (día/mes/año hora:minutos) y cómo se repite. Variables: {nombre}, {fecha}, {hora}. Un cumpleaños es un mensaje «cada año». Las listas se crean en el módulo Difusión.",
		Campos: []Campo{
			{Clave: "nombre", Titulo: "Nombre (solo para ti)", Tipo: CampoTexto, Largo: 60, Obligatorio: true},
			{Clave: "destino", Titulo: "Para", Tipo: CampoLista, Opciones: g.opcionesDestino()},
			{Clave: "texto", Titulo: "Mensaje", Tipo: CampoTexto, Varias: true, Largo: 1500},
			{Clave: "adjuntos", Titulo: "Archivos", Tipo: CampoAdjuntos},
			{Clave: "fecha", Titulo: "Fecha y hora", Ayuda: "Por ejemplo 25/12/2026 18:30", Tipo: CampoTexto, Largo: 20, Obligatorio: true},
			{Clave: "repeticion", Titulo: "Repetir", Tipo: CampoLista, Defecto: "ninguna", Opciones: []Opcion{{"ninguna", "Una sola vez"}, {"diaria", "Cada día"}, {"semanal", "Cada semana"}, {"anual", "Cada año (cumpleaños)"}}},
		},
		Alta:    "Programar",
		Extras:  []Boton{{Accion: "programados.vista", Titulo: "Vista previa"}},
		PorFila: []Boton{{Accion: "programados.cancelar", Titulo: "Cancelar", Confirma: "¿Cancelar este mensaje programado?"}, {Accion: "programados.borrar", Titulo: "Borrar", Confirma: "¿Borrar este mensaje de la lista?"}},
	}}
}

func filasProgramados(g *Gestor, panel string) []Fila {
	var out []Fila
	fs, _ := g.Alm.ListarAgenda(g.BotID, IDProgramados)
	for _, f := range fs {
		para := "lista " + strings.TrimPrefix(f.Destino, "lista:")
		if !strings.HasPrefix(f.Destino, "lista:") {
			para = g.nombreChat(f.Destino)
		}
		d := para + " · "
		switch f.Estado {
		case pgActivo:
			d += g.FechaTexto(f.ProximoTS) + " · " + textoRepeticion(f.Repeticion)
		case pgTerminado:
			d += "ya salió"
		case pgCancelado:
			d += "cancelado"
		}
		out = append(out, Fila{ID: fmt.Sprint(f.ID), Titulo: f.Nombre, Detalle: d})
	}
	return out
}

func repDe(a Args) string {
	switch r := a.Texto("repeticion"); r {
	case repDiaria, repSemanal, repAnual:
		return r
	}
	return repNinguna
}

func accionProgramados(g *Gestor, accion string, a Args) (string, error) {
	ahora := g.ahora()
	switch accion {
	case "programados.agregar", "programados.vista":
		texto := valorTexto(a, "texto")
		adj, err := g.adjuntosDe(a, "adjuntos")
		if err != nil {
			return "", err
		}
		if texto == "" && len(adj) == 0 {
			return "", errors.New("escribe el mensaje o agrega un archivo")
		}
		fecha, err := g.ParseFechaHora(a.Texto("fecha"))
		if err != nil {
			return "", err
		}
		if !fecha.After(ahora) {
			return "", errors.New("esa fecha ya pasó")
		}
		if accion == "programados.vista" {
			v := sustituir(texto, map[string]string{"nombre": "María", "fecha": fecha.Format("02/01/2006"), "hora": fecha.Format("15:04")})
			return fmt.Sprintf("Saldría el %s (%s) con %d archivos:\n\n%s", g.FechaTexto(fecha.Unix()), textoRepeticion(repDe(a)), len(adj), v), nil
		}
		nombre, destino := a.Texto("nombre"), a.Texto("destino")
		if nombre == "" {
			return "", errors.New("ponle un nombre")
		}
		if lista, ok := strings.CutPrefix(destino, "lista:"); ok {
			if len(mustMiembros(g, lista)) == 0 {
				return "", errors.New("esa lista está vacía")
			}
		} else if destino == "" || !g.hayChat(destino) {
			return "", errors.New("elige a dónde va el mensaje")
		}
		b, _ := json.Marshal(contenidoDif{Texto: texto, Adjuntos: adj})
		if _, err := g.Alm.CrearAgenda(g.BotID, almacen.AgendaFila{Modulo: IDProgramados, Nombre: nombre, Destino: destino, Contenido: string(b),
			ProximoTS: fecha.Unix(), Repeticion: repDe(a), Estado: pgActivo}, ahora); err != nil {
			return "", err
		}
		g.Alm.RegistrarAccion(g.BotID, almacen.PersonaDueno, "modulo.programado_crear", "", "", "ok", "")
		return "Programado para el " + g.FechaTexto(fecha.Unix()) + " (" + textoRepeticion(repDe(a)) + ").", nil
	case "programados.cancelar", "programados.borrar":
		f, err := g.Alm.ObtenerAgenda(g.BotID, idArg(a))
		if err != nil || f == nil || f.Modulo != IDProgramados {
			return "", errors.New("ese mensaje ya no existe")
		}
		if accion == "programados.borrar" {
			return "Borrado.", g.Alm.BorrarAgenda(g.BotID, f.ID)
		}
		return "Cancelado.", g.Alm.ActualizarAgenda(g.BotID, f.ID, pgCancelado, 0, time.Time{}, "cancelado", false)
	}
	return "", errors.New("esa acción no existe")
}

// ProximoAgenda dice cuándo toca lo siguiente que tiene agenda propia entre los módulos encendidos (segundos desde 1970; 0 = nada).
// La app programa con esto la alarma de Android.
func (g *Gestor) ProximoAgenda() int64 {
	var ids []string
	for _, id := range []string{IDProgramados, IDDifusion, IDSorteos, IDVotaciones, IDTablon} {
		if g.Activo(id) {
			ids = append(ids, id)
		}
	}
	return g.Alm.ProximaAgenda(g.BotID, ids...)
}

// TickAgenda corre ya la revisión de la agenda de los módulos encendidos (la alarma de Android la llama a la hora justa).
func (g *Gestor) TickAgenda() {
	if g.estaVivo() {
		g.Evento(Evento{Tipo: EvTick})
	}
}
