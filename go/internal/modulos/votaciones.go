package modulos

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"botchat/internal/almacen"
	"botchat/internal/comandos"
	"botchat/internal/permisos"
)

// ---------- Votaciones ----------
//
// Una votación es una encuesta de WhatsApp que se manda a un GRUPO a la hora que elijas (una vez o repetida), se cierra sola después de
// unas horas y publica el resultado. Los votos que llegan se cuentan en el teléfono; si limitas los rangos que cuentan, los votos de los
// demás se ignoran en el resultado (WhatsApp no deja impedir que alguien vote). Solo funciona en grupos. Una fila de la agenda por
// votación; nunca se guardan textos de mensajes de nadie, solo opciones y votos.

// IDVotaciones es el id del módulo.
const IDVotaciones = "votaciones"

const (
	qVotacion = "votacion"

	votProgramada = "programada"
	votAbierta    = "abierta"
	votCerrada    = "cerrada"
)

type votacionDatos struct {
	Pregunta  string           `json:"pregunta"`
	Opciones  []string         `json:"opciones"`
	Multiple  bool             `json:"multiple"`
	Grupo     string           `json:"grupo"`
	Rangos    []int64          `json:"rangos"` // si hay, solo cuentan los votos de personas con alguno de estos rangos
	SinRango  bool             `json:"sinRango"`
	Horas     int              `json:"horas"` // cuánto dura abierta (0 = no se cierra sola)
	Fase      string           `json:"fase"`
	PollID    string           `json:"pollId"`
	Secreto   []byte           `json:"secreto"`
	Inicio    int64            `json:"inicio"`
	Cierre    int64            `json:"cierre"`
	Votos     map[string][]int `json:"votos"` // persona → opciones elegidas (0..n-1)
	Aviso     bool             `json:"aviso"` // ya se avisó que está por cerrar
	Resultado string           `json:"resultado"`
}

func votaciones() Modulo {
	return Nuevo(Def{
		ID: IDVotaciones, Nombre: "Votaciones", Icono: "difusion", Riesgo: RiesgoMedio, Iniciativa: true,
		Descripcion: "Manda una encuesta a un grupo a la hora que elijas, la cierra sola y publica el resultado. Puedes repetirla cada día o cada semana y hacer que solo cuenten los votos de ciertos rangos. Solo funciona en grupos.",
		Casillas: []permisos.Casilla{
			{ID: "votaciones.ver", Nombre: "Ver las votaciones", Descripcion: "Pide con «!votaciones» la lista de votaciones abiertas y su avance.", Grupo: "Votaciones", Riesgo: permisos.RiesgoBajo, Grupos: true, Base: true},
		},
		Esquema: []Campo{
			{Clave: "aviso_antes_min", Titulo: "Avisar antes de cerrar (minutos)", Ayuda: "0 = sin aviso. Solo se avisa si la votación dura el triple o más.", Tipo: CampoNumero, Defecto: 10.0, Min: 0, Max: 1440},
			{Clave: "publicar_resultado", Titulo: "Publicar el resultado en el grupo", Tipo: CampoInterruptor, Defecto: true},
			{Clave: "avisarme", Titulo: "Avisarme cuando se cierre una votación", Tipo: CampoInterruptor, Defecto: true},
		},
		TopeHora: 20, TopeDia: 80,
		AlActivar: func(g *Gestor) error {
			// Si la app se cerró mientras una encuesta salía, no se sabe si llegó: se avisa y no se reenvía para no duplicar.
			fs, _ := g.Alm.AgendaConEstado(g.BotID, IDVotaciones, "enviando")
			for _, f := range fs {
				_ = g.Alm.ActualizarAgenda(g.BotID, f.ID, pgCancelado, 0, g.ahora(), "interrumpida", false)
				if g.Aviso != nil {
					g.Aviso("Votación interrumpida", "«"+f.Nombre+"»: se cerró la app mientras salía y no se sabe si llegó al grupo. No se reenvió para no duplicar.")
				}
			}
			votacionesTick(g)
			return nil
		},
		AlEvento: func(g *Gestor, e Evento) {
			if e.Tipo == EvTick {
				votacionesTick(g)
			}
		},
		AlEstado: func(g *Gestor) string {
			ab := 0
			fs, _ := g.Alm.ListarAgenda(g.BotID, IDVotaciones)
			for _, f := range fs {
				var d votacionDatos
				if f.Estado == pgActivo && json.Unmarshal([]byte(f.Contenido), &d) == nil && d.Fase == votAbierta {
					ab++
				}
			}
			if ab == 0 {
				return ""
			}
			return fmt.Sprintf("Encendido · %d votaciones abiertas", ab)
		},
		ComandosG: comandosVotaciones,
		Paneles:   panelesVotaciones,
		AlFilas:   filasVotaciones,
		AlAccion:  accionVotaciones,
	})
}

// HashOpcion es lo que manda WhatsApp en un voto: el SHA-256 del texto de la opción.
func HashOpcion(texto string) []byte {
	h := sha256.Sum256([]byte(texto))
	return h[:]
}

// cuentaVoto dice si el voto de esa persona entra en el resultado según los rangos de la votación.
func (g *Gestor) cuentaVoto(d *votacionDatos, persona string) bool {
	if len(d.Rangos) == 0 && !d.SinRango {
		return true
	}
	r, _, _ := g.Alm.RolDePersona(g.BotID, g.Alm.ResolverPersona(g.BotID, persona), g.ahora())
	if r == nil {
		return d.SinRango
	}
	for _, id := range d.Rangos {
		if id == r.ID {
			return true
		}
	}
	return false
}

func (d *votacionDatos) conteo(g *Gestor) (por []int, votantes int) {
	por = make([]int, len(d.Opciones))
	for p, ops := range d.Votos {
		if !g.cuentaVoto(d, p) {
			continue
		}
		votantes++
		for _, o := range ops {
			if o >= 0 && o < len(por) {
				por[o]++
			}
		}
	}
	return
}

// VotoEncuesta anota el voto de una persona en una encuesta de una votación. Devuelve false si esa encuesta no es de este módulo.
// `hashes` son las opciones elegidas tal como las manda WhatsApp (vacío = retiró su voto).
func (g *Gestor) VotoEncuesta(chat, pollID, persona string, hashes [][]byte) bool {
	if !g.Activo(IDVotaciones) {
		return false
	}
	fs, _ := g.Alm.AgendaConEstado(g.BotID, IDVotaciones, pgActivo)
	for _, f := range fs {
		var d votacionDatos
		if json.Unmarshal([]byte(f.Contenido), &d) != nil || d.PollID != pollID || d.Fase != votAbierta || d.Grupo != chat {
			continue
		}
		var elegidas []int
		for _, h := range hashes {
			for i, o := range d.Opciones {
				if string(HashOpcion(o)) == string(h) {
					elegidas = append(elegidas, i)
				}
			}
		}
		sort.Ints(elegidas)
		if d.Votos == nil {
			d.Votos = map[string][]int{}
		}
		p := g.Alm.ResolverPersona(g.BotID, persona)
		if len(elegidas) == 0 {
			delete(d.Votos, p)
		} else {
			d.Votos[p] = elegidas
		}
		b, _ := json.Marshal(&d)
		_ = g.Alm.ActualizarContenidoAgenda(g.BotID, f.ID, string(b))
		return true
	}
	return false
}

// SecretoVotacion da el secreto de la encuesta de una votación abierta (para abrir votos que la librería no logra abrir sola).
func (g *Gestor) SecretoVotacion(pollID string) []byte {
	fs, _ := g.Alm.AgendaConEstado(g.BotID, IDVotaciones, pgActivo)
	for _, f := range fs {
		var d votacionDatos
		if json.Unmarshal([]byte(f.Contenido), &d) == nil && d.PollID == pollID {
			return d.Secreto
		}
	}
	return nil
}

// ---------- tick ----------

func votacionesTick(g *Gestor) {
	if !g.Maestro() || g.EnModoSeguro() {
		return
	}
	cfg := g.Config(IDVotaciones)
	ahora := g.ahora()
	vencidas, _ := g.Alm.AgendaVencida(g.BotID, IDVotaciones, ahora)
	for _, f := range vencidas {
		var d votacionDatos
		if json.Unmarshal([]byte(f.Contenido), &d) != nil {
			_ = g.Alm.ActualizarAgenda(g.BotID, f.ID, pgCancelado, 0, ahora, "datos_dañados", false)
			continue
		}
		switch d.Fase {
		case votProgramada:
			if err := g.votacionAbrir(f, &d); err != nil {
				if g.Aviso != nil {
					g.Aviso("Votación sin enviar", "«"+f.Nombre+"»: "+err.Error()+".")
				}
				g.votacionSiguiente(f, &d, "error")
			}
		case votAbierta:
			if d.Cierre > 0 && ahora.Unix() >= d.Cierre {
				g.votacionCerrar(f, &d, cfg)
			} else if !d.Aviso && d.Cierre > 0 {
				// era la hora de avisar que falta poco
				d.Aviso = true
				if mins := cfg.Int("aviso_antes_min"); mins > 0 {
					por, n := d.conteo(g)
					_ = por
					g.votacionDecir(&d, fmt.Sprintf("⏳ Faltan %s para cerrar la votación «%s». Ya votaron %d.", minutosTexto(int((d.Cierre-ahora.Unix()+59)/60)), d.Pregunta, n))
				}
				g.votacionGuarda(f, &d, d.Cierre)
			}
		}
	}
}

func (g *Gestor) votacionDecir(d *votacionDatos, texto string) {
	_, _ = g.Enviar(Iniciativa{Modulo: IDVotaciones, Que: qVotacion, Chat: d.Grupo, Texto: texto, Noche: NocheDiferir})
}

func (g *Gestor) votacionGuarda(f almacen.AgendaFila, d *votacionDatos, prox int64) {
	b, _ := json.Marshal(d)
	_ = g.Alm.ActualizarContenidoAgenda(g.BotID, f.ID, string(b))
	_ = g.Alm.ActualizarAgenda(g.BotID, f.ID, pgActivo, prox, g.ahora(), d.Fase, false)
}

// votacionAbrir encola la encuesta (por la cola, con sus topes, su pausa nocturna y su registro). La fila queda «enviando» para que
// el siguiente minuto no la mande dos veces; cuando la cola la manda de verdad, la acción deja abierta la votación.
func (g *Gestor) votacionAbrir(f almacen.AgendaFila, d *votacionDatos) error {
	if g.Acc == nil {
		return errors.New("no hay conexión con WhatsApp")
	}
	acc := g.Acc
	_ = g.Alm.ActualizarAgenda(g.BotID, f.ID, "enviando", f.ProximoTS, time.Time{}, "enviando", false)
	fila := f
	datos := *d
	ini := Iniciativa{Modulo: IDVotaciones, Que: qVotacion, Chat: d.Grupo, Noche: NocheDescartar, EsperaMax: 30 * time.Minute,
		Accion: func(ctx context.Context) error {
			id, sec, err := acc.EnviarEncuesta(ctx, datos.Grupo, datos.Pregunta, datos.Opciones, datos.Multiple)
			if err != nil {
				return err
			}
			ahora := g.ahora()
			datos.PollID, datos.Secreto, datos.Fase, datos.Inicio, datos.Votos, datos.Aviso = id, sec, votAbierta, ahora.Unix(), map[string][]int{}, false
			prox := int64(0)
			if datos.Horas > 0 {
				datos.Cierre = ahora.Add(time.Duration(datos.Horas) * time.Hour).Unix()
				prox = datos.Cierre
				if mins := g.Config(IDVotaciones).Int("aviso_antes_min"); mins > 0 && datos.Horas*60 >= 3*mins {
					prox = datos.Cierre - int64(mins)*60
				}
			}
			g.votacionGuarda(fila, &datos, prox)
			return nil
		},
		AlResultado: func(r string) {
			if r == ResEnviado {
				return
			}
			if cur, _ := g.Alm.ObtenerAgenda(g.BotID, fila.ID); cur != nil && cur.Estado == "enviando" {
				if g.Aviso != nil {
					g.Aviso("Votación sin enviar", "«"+fila.Nombre+"»: no salió ("+motivoLegible(strings.TrimPrefix(r, ResDescartado))+").")
				}
				g.votacionSiguiente(*cur, &datos, "no_salio")
			}
		}}
	if _, err := g.Enviar(ini); err != nil {
		return err
	}
	return nil
}

func mustJSON(v any) string { b, _ := json.Marshal(v); return string(b) }

// votacionCerrar publica el resultado y deja lista la siguiente si se repite.
func (g *Gestor) votacionCerrar(f almacen.AgendaFila, d *votacionDatos, cfg Config) {
	por, n := d.conteo(g)
	d.Resultado = textoResultadoVotacion(d, por, n)
	d.Fase = votCerrada
	if cfg.Bool("publicar_resultado") {
		g.votacionDecir(d, "📊 Resultado de la votación\n"+d.Resultado)
	}
	if cfg.Bool("avisarme") && g.Aviso != nil {
		g.Aviso("Votación cerrada", "«"+d.Pregunta+"»: votaron "+strconv.Itoa(n)+".")
	}
	g.Alm.RegistrarAccion(g.BotID, almacen.PersonaDueno, "votacion.cerrar", "", d.Grupo, "ok", "")
	b, _ := json.Marshal(d)
	_ = g.Alm.ActualizarContenidoAgenda(g.BotID, f.ID, string(b))
	g.votacionSiguiente(f, d, "cerrada")
}

// votacionSiguiente: si la votación se repite, prepara la siguiente apertura; si no, la deja terminada.
func (g *Gestor) votacionSiguiente(f almacen.AgendaFila, d *votacionDatos, res string) {
	ahora := g.ahora()
	if f.Repeticion != repNinguna {
		base := time.Unix(f.ProximoTS, 0).In(g.zona())
		if d.Inicio > 0 {
			base = time.Unix(d.Inicio, 0).In(g.zona())
		}
		if sig := siguienteFecha(base, f.Repeticion, ahora); !sig.IsZero() {
			d.Fase, d.PollID, d.Secreto, d.Votos, d.Aviso, d.Cierre = votProgramada, "", nil, nil, false, 0
			g.votacionGuarda(f, d, sig.Unix())
			return
		}
	}
	_ = g.Alm.ActualizarAgenda(g.BotID, f.ID, pgTerminado, 0, ahora, res, true)
}

func textoResultadoVotacion(d *votacionDatos, por []int, n int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "«%s»\n", d.Pregunta)
	max := 0
	for _, v := range por {
		if v > max {
			max = v
		}
	}
	for i, o := range d.Opciones {
		marca := "▫️"
		if por[i] == max && max > 0 {
			marca = "🏆"
		}
		pc := 0
		if n > 0 {
			pc = por[i] * 100 / n
		}
		fmt.Fprintf(&b, "%s %s — %d (%d%%)\n", marca, o, por[i], pc)
	}
	fmt.Fprintf(&b, "Votaron %d persona", n)
	if n != 1 {
		b.WriteString("s")
	}
	b.WriteString(".")
	return b.String()
}

// ---------- comandos y paneles ----------

func comandosVotaciones(g *Gestor) []comandos.Comando {
	return []comandos.Comando{{
		Nombre: "votaciones", Resumen: "Ver las votaciones abiertas", Uso: "votaciones", Casilla: "votaciones.ver", Ambito: comandos.Grupo,
		Ejecutar: func(cx *comandos.Contexto) (string, error) {
			fs, _ := g.Alm.ListarAgenda(g.BotID, IDVotaciones)
			var b strings.Builder
			for _, f := range fs {
				var d votacionDatos
				if json.Unmarshal([]byte(f.Contenido), &d) != nil || d.Fase != votAbierta || d.Grupo != cx.Chat {
					continue
				}
				_, n := d.conteo(g)
				fmt.Fprintf(&b, "• %s — votaron %d", d.Pregunta, n)
				if d.Cierre > 0 {
					fmt.Fprintf(&b, ", cierra %s", g.FechaTexto(d.Cierre))
				}
				b.WriteString("\n")
			}
			if b.Len() == 0 {
				return "No hay votaciones abiertas en este grupo.", nil
			}
			return strings.TrimSpace(b.String()), nil
		}}}
}

func panelesVotaciones(g *Gestor) []Panel {
	rangos := g.opcionesRangos()
	return []Panel{{
		ID: "votaciones", Titulo: "Votaciones",
		Ayuda: "La encuesta sale al grupo a la fecha y hora que elijas (vacío = ahora) y se cierra sola. Si eliges rangos, solo cuentan sus votos. Opciones: una por línea, de 2 a 12.",
		Campos: []Campo{
			{Clave: "nombre", Titulo: "Nombre (solo para ti)", Tipo: CampoTexto, Largo: 60, Obligatorio: true},
			{Clave: "grupo", Titulo: "Grupo", Tipo: CampoLista, Opciones: g.opcionesChats(true, 100)},
			{Clave: "pregunta", Titulo: "Pregunta", Tipo: CampoTexto, Largo: 200, Obligatorio: true},
			{Clave: "opciones", Titulo: "Opciones (una por línea)", Tipo: CampoTexto, Largo: 600, Varias: true, Obligatorio: true},
			{Clave: "multiple", Titulo: "Se puede elegir más de una", Tipo: CampoInterruptor, Defecto: false},
			{Clave: "horas", Titulo: "Horas abierta (0 = no se cierra sola)", Tipo: CampoNumero, Defecto: 24.0, Min: 0, Max: 720},
			{Clave: "rango1", Titulo: "Solo cuentan los votos de (1)", Tipo: CampoLista, Defecto: "", Opciones: rangos},
			{Clave: "rango2", Titulo: "Solo cuentan los votos de (2)", Tipo: CampoLista, Defecto: "", Opciones: rangos},
			{Clave: "fecha", Titulo: "Fecha y hora de envío (opcional)", Ayuda: "Por ejemplo 25/12/2026 18:30. Vacío = enviar ahora.", Tipo: CampoTexto, Largo: 20},
			{Clave: "repeticion", Titulo: "Repetir", Tipo: CampoLista, Defecto: "ninguna", Opciones: []Opcion{{"ninguna", "Una sola vez"}, {"diaria", "Cada día"}, {"semanal", "Cada semana"}}},
		},
		Alta:    "Crear votación",
		PorFila: []Boton{{Accion: "votaciones.cerrar", Titulo: "Cerrar y publicar"}, {Accion: "votaciones.cancelar", Titulo: "Cancelar", Confirma: "¿Cancelar esta votación?"}, {Accion: "votaciones.borrar", Titulo: "Borrar", Confirma: "¿Borrar esta votación de la lista?"}},
	}}
}

func filasVotaciones(g *Gestor, panel string) []Fila {
	var out []Fila
	fs, _ := g.Alm.ListarAgenda(g.BotID, IDVotaciones)
	for _, f := range fs {
		var d votacionDatos
		_ = json.Unmarshal([]byte(f.Contenido), &d)
		det := g.nombreChat(d.Grupo) + " · "
		switch {
		case f.Estado == pgCancelado:
			det += "cancelada"
		case f.Estado == pgTerminado:
			det += "cerrada"
		case d.Fase == votAbierta:
			_, n := d.conteo(g)
			det += fmt.Sprintf("abierta · %d votos", n)
			if d.Cierre > 0 {
				det += " · cierra " + g.FechaTexto(d.Cierre)
			}
		default:
			det += "sale " + g.FechaTexto(f.ProximoTS) + " · " + textoRepeticion(f.Repeticion)
		}
		out = append(out, Fila{ID: fmt.Sprint(f.ID), Titulo: f.Nombre, Detalle: det})
	}
	return out
}

func accionVotaciones(g *Gestor, accion string, a Args) (string, error) {
	ahora := g.ahora()
	switch accion {
	case "votaciones.agregar":
		nombre := strings.TrimSpace(a.Texto("nombre"))
		grupo := a.Texto("grupo")
		if nombre == "" {
			return "", errors.New("ponle un nombre")
		}
		if !strings.HasSuffix(grupo, "@g.us") || !g.hayChat(grupo) {
			return "", errors.New("las votaciones solo funcionan en grupos: elige un grupo")
		}
		var ops []string
		for _, l := range strings.Split(valorTexto(a, "opciones"), "\n") {
			if l = strings.TrimSpace(l); l != "" {
				ops = append(ops, l)
			}
		}
		d := &votacionDatos{Pregunta: strings.TrimSpace(a.Texto("pregunta")), Opciones: ops, Multiple: a.Bool("multiple"), Grupo: grupo, Horas: int(a.Num("horas")), Fase: votProgramada}
		if d.Pregunta == "" || len(ops) < 2 || len(ops) > 12 {
			return "", errors.New("una votación lleva una pregunta y de 2 a 12 opciones")
		}
		for _, k := range []string{"rango1", "rango2"} {
			switch v := a.Texto(k); v {
			case "":
			case "sin":
				d.SinRango = true
			default:
				if id, err := strconv.ParseInt(v, 10, 64); err == nil {
					d.Rangos = append(d.Rangos, id)
				}
			}
		}
		prox := ahora.Unix()
		if t := strings.TrimSpace(a.Texto("fecha")); t != "" {
			fecha, err := g.ParseFechaHora(t)
			if err != nil {
				return "", err
			}
			if !fecha.After(ahora) {
				return "", errors.New("esa fecha ya pasó")
			}
			prox = fecha.Unix()
		} else if !g.Maestro() {
			return "", errors.New("«Iniciativa propia» está apagada: enciéndela para enviar la votación")
		}
		rep := repNinguna
		if r := a.Texto("repeticion"); r == repDiaria || r == repSemanal {
			rep = r
		}
		if _, err := g.Alm.CrearAgenda(g.BotID, almacen.AgendaFila{Modulo: IDVotaciones, Nombre: nombre, Destino: grupo, Contenido: mustJSON(d), ProximoTS: prox, Repeticion: rep, Estado: pgActivo}, ahora); err != nil {
			return "", err
		}
		g.Alm.RegistrarAccion(g.BotID, almacen.PersonaDueno, "votacion.crear", "", grupo, "ok", "")
		if a.Texto("fecha") == "" {
			votacionesTick(g)
			return "Votación enviada al grupo.", nil
		}
		return "Votación programada para el " + g.FechaTexto(prox) + ".", nil
	case "votaciones.cerrar", "votaciones.cancelar", "votaciones.borrar":
		f, err := g.Alm.ObtenerAgenda(g.BotID, idArg(a))
		if err != nil || f == nil || f.Modulo != IDVotaciones {
			return "", errors.New("esa votación ya no existe")
		}
		switch accion {
		case "votaciones.borrar":
			return "Borrada.", g.Alm.BorrarAgenda(g.BotID, f.ID)
		case "votaciones.cancelar":
			return "Cancelada.", g.Alm.ActualizarAgenda(g.BotID, f.ID, pgCancelado, 0, time.Time{}, "cancelado", false)
		}
		var d votacionDatos
		if json.Unmarshal([]byte(f.Contenido), &d) != nil || d.Fase != votAbierta {
			return "", errors.New("esa votación no está abierta")
		}
		g.votacionCerrar(*f, &d, g.Config(IDVotaciones))
		return "Votación cerrada: " + strings.ReplaceAll(d.Resultado, "\n", " · "), nil
	}
	return "", errors.New("esa acción no existe")
}
