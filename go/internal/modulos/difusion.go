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

// ---------- H2 · Difusión y anuncios (riesgo ALTO) ----------
//
// Mensajes a una lista de personas que YA me escribieron o que se suscribieron escribiendo una palabra clave ("SUSCRIBIR" o "ALTA").
// WhatsApp prohíbe el envío masivo y la ruta oficial sin riesgo es la API de WhatsApp Business. Por eso: lotes chicos, espera al azar
// de 8 a 30 s entre mensajes, tope diario (30 por defecto), calentamiento progresivo, pausa automática ante errores, "BAJA" a No
// molestar y reporte al terminar. La cola de iniciativas es en memoria: la difusión guarda su propia agenda (mod_agenda y mod_dest) y
// vuelve a encolar lo pendiente al arrancar.

// IDDifusion es el id del módulo.
const IDDifusion = "difusion"

// ListaSuscritos es la lista donde caen quienes escriben la palabra clave.
const ListaSuscritos = "suscritos"

const (
	qDifusion = "difusion"
	qAlta     = "alta"

	dfBorrador   = "borrador"
	dfProgramada = "activo" // con fecha: la agenda la inicia cuando llega
	dfEnviando   = "enviando"
	dfPausada    = "pausada"
	dfTerminada  = "terminada"
	dfCancelada  = "cancelado"
)

type contenidoDif struct {
	Texto    string   `json:"texto"`
	Adjuntos []AdjRef `json:"adjuntos,omitempty"`
}

func leeContenido(f *almacen.AgendaFila) contenidoDif {
	var c contenidoDif
	_ = json.Unmarshal([]byte(f.Contenido), &c)
	return c
}

// capDiario es cuántos mensajes de difusión pueden salir en 24 h: el tope del módulo o, con calentamiento, lo que va ganando cada día
// (10 el primer día y 5 más cada día).
func (g *Gestor) capDiario(cfg Config) int {
	_, tope := g.TopesModulo(IDDifusion)
	if tope <= 0 {
		tope = 30
	}
	if cfg.Bool("calentamiento") {
		dias := 0
		if primero := g.primerEnvioDifusion(); !primero.IsZero() {
			dias = int(g.ahora().Sub(primero).Hours() / 24)
		}
		if c := 10 + 5*dias; c < tope {
			tope = c
		}
	}
	return tope
}

func (g *Gestor) primerEnvioDifusion() time.Time {
	g.h.mu.Lock()
	t := g.h.difPrimero
	g.h.mu.Unlock()
	if !t.IsZero() {
		return t
	}
	regs, _ := g.Alm.ListarRegistroModulos(g.BotID, IDDifusion, 500, 0)
	var min int64
	for _, r := range regs {
		if r.Que == qDifusion && r.Resultado == ResEnviado && (min == 0 || r.TS < min) {
			min = r.TS
		}
	}
	if min > 0 {
		t = time.Unix(min, 0)
		g.h.mu.Lock()
		g.h.difPrimero = t
		g.h.mu.Unlock()
	}
	return t
}

func (g *Gestor) marcaPrimerEnvio() {
	g.h.mu.Lock()
	if g.h.difPrimero.IsZero() {
		g.h.difPrimero = g.ahora()
	}
	g.h.mu.Unlock()
}

func (g *Gestor) textoDifusion(cfg Config, c contenidoDif, nombre string) string {
	t := sustituir(c.Texto, map[string]string{"nombre": nombre, "fecha": g.Hora().Format("02/01/2006"), "hora": g.Hora().Format("15:04")})
	if cfg.Bool("pie_baja") {
		if p := strings.TrimSpace(cfg.Texto("pie_texto")); p != "" {
			t += "\n\n" + p
		}
	}
	return strings.TrimSpace(t)
}

func difusion() Modulo {
	return Nuevo(Def{
		ID: IDDifusion, Nombre: "Difusión y anuncios", Icono: "difusion", Riesgo: RiesgoAlto, Iniciativa: true,
		Descripcion: "Manda un anuncio a una lista de personas que ya te escribieron o que se suscribieron. Manda de a poquito, con esperas al azar, tope diario y calentamiento.",
		Casillas: []permisos.Casilla{
			{ID: "difusion.ver", Nombre: "Ver el estado de las difusiones", Descripcion: "Puede pedir con «!difusion» cuántas personas hay suscritas y cómo van los envíos.", Grupo: "Difusión y anuncios", Riesgo: permisos.RiesgoMedio},
		},
		Esquema: []Campo{
			{Clave: "palabra_alta", Titulo: "Palabra para suscribirse", Ayuda: "Quien la escriba sola, en privado, entra a la lista «suscritos». «ALTA» también sirve siempre.", Tipo: CampoTexto, Defecto: "SUSCRIBIR", Largo: 30, Obligatorio: true},
			{Clave: "responder_alta", Titulo: "Confirmar la suscripción", Tipo: CampoInterruptor, Defecto: true},
			{Clave: "texto_alta", Titulo: "Mensaje de suscripción", Tipo: CampoTexto, Varias: true, Largo: 400, Defecto: "Listo, quedaste suscrito a los anuncios. Para dejar de recibirlos escribe BAJA."},
			{Clave: "pie_baja", Titulo: "Avisar cómo darse de baja", Ayuda: "Agrega el pie de abajo a cada anuncio.", Tipo: CampoInterruptor, Defecto: true},
			{Clave: "pie_texto", Titulo: "Pie de cada anuncio", Tipo: CampoTexto, Largo: 200, Defecto: "Para no recibir más mensajes, escribe BAJA."},
			{Clave: "espera_min", Titulo: "Espera mínima entre mensajes (segundos)", Tipo: CampoNumero, Defecto: 8.0, Min: 8, Max: 600},
			{Clave: "espera_max", Titulo: "Espera máxima entre mensajes (segundos)", Tipo: CampoNumero, Defecto: 30.0, Min: 8, Max: 900},
			{Clave: "lote", Titulo: "Mensajes por minuto en la cola", Ayuda: "Cuántos pone en la cola cada minuto; salen con las esperas de arriba.", Tipo: CampoNumero, Defecto: 3.0, Min: 1, Max: 10},
			{Clave: "calentamiento", Titulo: "Calentamiento progresivo", Ayuda: "Empieza con 10 mensajes al día y sube 5 cada día hasta el tope del módulo.", Tipo: CampoInterruptor, Defecto: true},
			{Clave: "errores_pausa", Titulo: "Pausar tras errores seguidos", Tipo: CampoNumero, Defecto: 3.0, Min: 1, Max: 20},
		},
		TopeHora: 40, TopeDia: 30,
		Validar: func(c map[string]any) error {
			cfg := Config(c)
			if cfg.Num("espera_max") < cfg.Num("espera_min") {
				return errors.New("la espera máxima no puede ser menor que la mínima")
			}
			if w := llave(cfg.Texto("palabra_alta")); w == "" || EsBaja(cfg.Texto("palabra_alta")) {
				return errors.New("esa palabra no sirve para suscribirse")
			}
			return nil
		},
		AlActivar: func(g *Gestor) error {
			// la cola de envío vive en memoria: lo que quedó «encolado» al cerrar la app vuelve a pendiente
			g.Alm.DestEncoladosAPendientes(g.BotID)
			g.h.mu.Lock()
			g.h.difEncolado = 0
			g.h.mu.Unlock()
			return nil
		},
		AlDesactivar: func(g *Gestor) error {
			g.Alm.DestEncoladosAPendientes(g.BotID)
			return nil
		},
		AlEvento: eventoDifusion,
		AlEstado: func(g *Gestor) string {
			n := len(mustMiembros(g, ListaSuscritos))
			en, _ := g.Alm.AgendaConEstado(g.BotID, IDDifusion, dfEnviando)
			switch {
			case len(en) > 0:
				return fmt.Sprintf("Encendido · %d difusión en curso · %d suscritos", len(en), n)
			case n > 0:
				return fmt.Sprintf("Encendido · %d suscritos", n)
			}
			return ""
		},
		ComandosG: func(g *Gestor) []comandos.Comando {
			return []comandos.Comando{{
				Nombre: "difusion", Resumen: "Ver cómo van las difusiones", Uso: "difusion", Casilla: "difusion.ver", Ambito: comandos.Privado,
				Ejecutar: func(cx *comandos.Contexto) (string, error) {
					var b strings.Builder
					fmt.Fprintf(&b, "Suscritos: %d", len(mustMiembros(g, ListaSuscritos)))
					fs, _ := g.Alm.ListarAgenda(g.BotID, IDDifusion)
					n := 0
					for _, f := range fs {
						if f.Estado == dfEnviando || f.Estado == dfPausada || f.Estado == dfProgramada {
							n++
							fmt.Fprintf(&b, "\n• %s — %s (%s)", f.Nombre, textoEstadoDif(f.Estado), resumenDest(g, f.ID))
						}
					}
					if n == 0 {
						b.WriteString("\nNo hay difusiones en curso.")
					}
					return b.String(), nil
				},
			}}
		},
		Paneles:  panelesDifusion,
		AlFilas:  filasDifusion,
		AlAccion: accionDifusion,
	})
}

func mustMiembros(g *Gestor, lista string) []string {
	m, _ := g.Alm.MiembrosLista(g.BotID, lista, 0)
	return m
}

func textoEstadoDif(e string) string {
	switch e {
	case dfBorrador:
		return "borrador"
	case dfProgramada:
		return "programada"
	case dfEnviando:
		return "enviando"
	case dfPausada:
		return "en pausa"
	case dfTerminada:
		return "terminada"
	case dfCancelada:
		return "cancelada"
	}
	return e
}

func resumenDest(g *Gestor, id int64) string {
	c := g.Alm.ContarDest(g.BotID, id)
	var p []string
	if n := c["enviado"]; n > 0 {
		p = append(p, fmt.Sprintf("%d enviados", n))
	}
	if n := c["pendiente"] + c["encolado"]; n > 0 {
		p = append(p, fmt.Sprintf("%d por enviar", n))
	}
	if n := c["fallido"]; n > 0 {
		p = append(p, fmt.Sprintf("%d fallidos", n))
	}
	if n := c["omitido"]; n > 0 {
		p = append(p, fmt.Sprintf("%d omitidos", n))
	}
	if len(p) == 0 {
		return "sin envíos todavía"
	}
	return strings.Join(p, ", ")
}

// ---------- eventos ----------

func eventoDifusion(g *Gestor, e Evento) {
	switch e.Tipo {
	case EvMensaje:
		if e.EsGrupo || e.EsMio || e.Persona == "" {
			return
		}
		cfg := g.Config(IDDifusion)
		if EsBaja(e.Texto) {
			if n := g.Alm.QuitarDeTodasLasListas(g.BotID, e.Persona); n > 0 {
				g.Alm.RegistrarModulo(g.BotID, IDDifusion, "baja", "ok", e.Chat, g.ahora())
			}
			return
		}
		t := llave(e.Texto)
		if t == "" || (t != llave(cfg.Texto("palabra_alta")) && t != "alta") {
			return
		}
		g.suscribir(cfg, e.Persona, e.Chat)
	case EvTick:
		difusionTick(g)
	}
}

// suscribir mete a una persona a «suscritos» (una suscripción explícita borra una baja anterior hecha por ella misma).
func (g *Gestor) suscribir(cfg Config, persona, chat string) {
	if lista, _ := g.Alm.ListarNoMolestar(g.BotID); len(lista) > 0 {
		p := g.Alm.ResolverPersona(g.BotID, persona)
		for _, x := range lista {
			if x.Persona == p && x.Motivo == "baja" {
				_ = g.Alm.QuitarNoMolestar(g.BotID, persona)
			}
		}
	}
	nuevo, _ := g.Alm.AgregarALista(g.BotID, ListaSuscritos, persona, "palabra", g.ahora())
	g.Alm.RegistrarModulo(g.BotID, IDDifusion, "suscripcion", map[bool]string{true: "ok", false: "ya_estaba"}[nuevo], chat, g.ahora())
	if cfg.Bool("responder_alta") {
		texto := strings.TrimSpace(cfg.Texto("texto_alta"))
		if !nuevo {
			texto = "Ya estabas suscrito. Para dejar de recibir anuncios escribe BAJA."
		}
		if texto != "" {
			_, _ = g.Enviar(Iniciativa{Modulo: IDDifusion, Que: qAlta, Chat: chat, Personas: []string{persona}, Texto: texto,
				RetrasoMin: 2 * time.Second, RetrasoMax: 8 * time.Second, Noche: NocheDiferir})
		}
	}
}

// ---------- el envío por lotes ----------

func difusionTick(g *Gestor) {
	cfg := g.Config(IDDifusion)
	ahora := g.ahora()
	// las programadas que ya les toca empiezan
	if vencidas, _ := g.Alm.AgendaVencida(g.BotID, IDDifusion, ahora); len(vencidas) > 0 {
		for _, f := range vencidas {
			if err := g.iniciarDifusion(&f); err != nil {
				_ = g.Alm.ActualizarAgenda(g.BotID, f.ID, dfPausada, 0, ahora, "inicio:"+err.Error(), false)
				if g.Aviso != nil {
					g.Aviso("Difusión no iniciada", "La difusión «"+f.Nombre+"» no pudo empezar: "+err.Error())
				}
			}
		}
	}
	en, _ := g.Alm.AgendaConEstado(g.BotID, IDDifusion, dfEnviando)
	if len(en) == 0 {
		return
	}
	if dentro, _ := g.enPausaNocturna(ahora); dentro || !g.Maestro() || g.EnModoSeguro() {
		return
	}
	// cupo del día: tope (con calentamiento) menos lo enviado en 24 h y lo que ya está en la cola
	hechos := g.Alm.ContarRegistroModulo(g.BotID, IDDifusion, qDifusion, ResEnviado, ahora.Add(-24*time.Hour))
	g.h.mu.Lock()
	cola := g.h.difEncolado
	g.h.mu.Unlock()
	cupo := g.capDiario(cfg) - hechos - cola
	if l := g.Cola.Libres(IDDifusion); l < cupo {
		cupo = l
	}
	if lote := cfg.Int("lote"); lote > 0 && lote < cupo {
		cupo = lote
	}
	for _, f := range en {
		if cupo <= 0 {
			return
		}
		pend := g.Alm.DestConEstado(g.BotID, f.ID, "pendiente", cupo)
		c := leeContenido(&f)
		for _, p := range pend {
			g.encolarDifusion(cfg, f, c, p)
			cupo--
		}
		if len(pend) == 0 {
			difusionRevisaFin(g, f)
		}
	}
}

func (g *Gestor) encolarDifusion(cfg Config, f almacen.AgendaFila, c contenidoDif, persona string) {
	g.Alm.MarcarDest(g.BotID, f.ID, persona, "encolado", g.ahora())
	g.h.mu.Lock()
	g.h.difEncolado++
	g.h.mu.Unlock()
	hecho := false
	res := func(r string) {
		if hecho {
			return
		}
		hecho = true
		g.h.mu.Lock()
		if g.h.difEncolado > 0 {
			g.h.difEncolado--
		}
		g.h.mu.Unlock()
		difusionResultado(g, cfg, f, persona, r)
	}
	texto := g.textoDifusion(cfg, c, g.nombreOPalabra(persona, ""))
	medio, extras := medioDe(c.Adjuntos, texto)
	ini := Iniciativa{Modulo: IDDifusion, Que: qDifusion, Chat: persona, Personas: []string{persona}, Texto: texto, Medio: medio, Extras: extras,
		Noche: NocheDescartar, RetrasoMin: segundos(cfg.Num("espera_min")), RetrasoMax: segundos(cfg.Num("espera_max")), AlResultado: res}
	if r, err := g.Enviar(ini); err != nil {
		res("fallido")
	} else if r != EnCola {
		_ = r // el motivo ya llegó por AlResultado
	}
}

func difusionResultado(g *Gestor, cfg Config, f almacen.AgendaFila, persona, r string) {
	ahora := g.ahora()
	switch {
	case r == ResEnviado:
		g.Alm.MarcarDest(g.BotID, f.ID, persona, "enviado", ahora)
		g.marcaPrimerEnvio()
		g.h.mu.Lock()
		delete(g.h.difFallos, f.ID)
		g.h.mu.Unlock()
	case r == ResFallido:
		g.Alm.MarcarDest(g.BotID, f.ID, persona, "fallido", ahora)
		g.h.mu.Lock()
		if g.h.difFallos == nil {
			g.h.difFallos = map[int64]int{}
		}
		g.h.difFallos[f.ID]++
		n := g.h.difFallos[f.ID]
		g.h.mu.Unlock()
		if n >= cfg.Int("errores_pausa") {
			_ = g.Alm.ActualizarAgenda(g.BotID, f.ID, dfPausada, 0, ahora, "errores", false)
			g.Cola.VaciarDonde(func(i Iniciativa) bool { return i.Modulo == IDDifusion && i.Que == qDifusion }, "pausa_errores")
			g.Alm.DestEncoladosAPendientes(g.BotID)
			if g.Aviso != nil {
				g.Aviso("Difusión en pausa", "La difusión «"+f.Nombre+"» se pausó porque fallaron varios envíos seguidos. Revisa tu conexión y reanúdala cuando quieras.")
			}
		}
	case r == ResDescartado+MotNoMolestar:
		g.Alm.MarcarDest(g.BotID, f.ID, persona, "omitido", ahora)
	default: // descartado por modo seguro, tope, noche, conexión, apagado…: queda para otro intento
		g.Alm.MarcarDest(g.BotID, f.ID, persona, "pendiente", ahora)
	}
	if cur, _ := g.Alm.ObtenerAgenda(g.BotID, f.ID); cur != nil && cur.Estado == dfEnviando {
		difusionRevisaFin(g, *cur)
	}
}

// difusionRevisaFin cierra la difusión cuando ya no queda nadie por atender y deja el reporte.
func difusionRevisaFin(g *Gestor, f almacen.AgendaFila) {
	c := g.Alm.ContarDest(g.BotID, f.ID)
	if c["pendiente"]+c["encolado"] > 0 {
		return
	}
	_ = g.Alm.ActualizarAgenda(g.BotID, f.ID, dfTerminada, 0, g.ahora(), "ok", true)
	g.Alm.RegistrarModulo(g.BotID, IDDifusion, "reporte", "terminada", "", g.ahora())
	if g.Aviso != nil {
		g.Aviso("Difusión terminada", fmt.Sprintf("«%s»: %d enviados, %d fallidos, %d omitidos.", f.Nombre, c["enviado"], c["fallido"], c["omitido"]))
	}
}

// iniciarDifusion arma los destinatarios (si faltan) y la pone a enviar.
func (g *Gestor) iniciarDifusion(f *almacen.AgendaFila) error {
	total := 0
	for _, n := range g.Alm.ContarDest(g.BotID, f.ID) {
		total += n
	}
	if total == 0 {
		lista := strings.TrimPrefix(f.Destino, "lista:")
		miembros, _ := g.Alm.MiembrosLista(g.BotID, lista, 0)
		var ok []string
		for _, p := range miembros {
			if g.hayChat(p) && !g.Alm.EstaNoMolestar(g.BotID, p) {
				ok = append(ok, p)
			}
		}
		if len(ok) == 0 {
			return errors.New("la lista «" + lista + "» no tiene a nadie a quien se pueda escribir")
		}
		if err := g.Alm.AgregarDest(g.BotID, f.ID, ok); err != nil {
			return err
		}
	}
	return g.Alm.ActualizarAgenda(g.BotID, f.ID, dfEnviando, 0, time.Time{}, "iniciada", false)
}

// ---------- paneles ----------

func (g *Gestor) opcionesListas(incluirSuscritos bool) []Opcion {
	var out []Opcion
	for _, l := range g.Alm.ListasConConteo(g.BotID, ListaSuscritos) {
		out = append(out, Opcion{Valor: l.Lista, Titulo: fmt.Sprintf("%s (%d)", l.Lista, l.N)})
	}
	return out
}

func panelesDifusion(g *Gestor) []Panel {
	return []Panel{
		{
			ID: "listas", Titulo: "Listas de destinatarios",
			Ayuda: "Solo se escribe a quien ya te escribió o se suscribió. Quien escriba la palabra de suscripción entra solo a «suscritos». Quien escriba BAJA sale de todas las listas y no recibe nada más.",
			Campos: []Campo{
				{Clave: "lista", Titulo: "Lista", Ayuda: "Escribe un nombre (se crea sola) o usa «suscritos».", Tipo: CampoTexto, Defecto: ListaSuscritos, Largo: 30, Obligatorio: true},
				{Clave: "persona", Titulo: "Persona que ya te escribió", Tipo: CampoLista, Opciones: g.opcionesChats(false, 200)},
			},
			Alta:    "Agregar a la lista",
			Extras:  []Boton{{Accion: "listas.quitar_persona", Titulo: "Quitar de la lista"}, {Accion: "listas.importar", Titulo: "Agregar a todos los que ya me escribieron", Confirma: "Esto mete a la lista a todas las personas que alguna vez te escribieron en privado. ¿Seguro?"}},
			PorFila: []Boton{{Accion: "listas.vaciar", Titulo: "Vaciar", Confirma: "¿Vaciar esta lista?"}},
		},
		{
			ID: "difusiones", Titulo: "Anuncios",
			Ayuda: "Guarda el anuncio y luego pulsa Iniciar. Variables: {nombre}, {fecha}, {hora}. Hasta 3 archivos. Si pones fecha (25/12/2026 18:30) empieza sola a esa hora.",
			Campos: []Campo{
				{Clave: "nombre", Titulo: "Nombre del anuncio", Tipo: CampoTexto, Largo: 60, Obligatorio: true},
				{Clave: "texto", Titulo: "Mensaje", Tipo: CampoTexto, Varias: true, Largo: 1500},
				{Clave: "adjuntos", Titulo: "Archivos", Tipo: CampoAdjuntos},
				{Clave: "lista", Titulo: "Lista de destinatarios", Tipo: CampoLista, Defecto: ListaSuscritos, Opciones: g.opcionesListas(true)},
				{Clave: "fecha", Titulo: "Programar para (opcional)", Ayuda: "día/mes/año hora:minutos", Tipo: CampoTexto, Largo: 20},
			},
			Alta:   "Guardar anuncio",
			Extras: []Boton{{Accion: "difusiones.vista", Titulo: "Vista previa"}},
			PorFila: []Boton{
				{Accion: "difusiones.iniciar", Titulo: "Iniciar", Confirma: "Esto manda mensajes a muchas personas por tu cuenta. WhatsApp prohíbe el envío masivo y puede restringir tu cuenta. ¿Iniciar el envío?"},
				{Accion: "difusiones.pausar", Titulo: "Pausar"},
				{Accion: "difusiones.cancelar", Titulo: "Cancelar", Confirma: "¿Cancelar este anuncio? Lo que no salió ya no saldrá."},
				{Accion: "difusiones.borrar", Titulo: "Borrar", Confirma: "¿Borrar este anuncio y su reporte?"},
			},
		},
	}
}

func filasDifusion(g *Gestor, panel string) []Fila {
	var out []Fila
	switch panel {
	case "listas":
		for _, l := range g.Alm.ListasConConteo(g.BotID, ListaSuscritos) {
			out = append(out, Fila{ID: l.Lista, Titulo: l.Lista, Detalle: fmt.Sprintf("%d personas", l.N)})
		}
	case "difusiones":
		fs, _ := g.Alm.ListarAgenda(g.BotID, IDDifusion)
		for _, f := range fs {
			d := textoEstadoDif(f.Estado) + " · " + strings.TrimPrefix(f.Destino, "lista:") + " · " + resumenDest(g, f.ID)
			if f.Estado == dfProgramada && f.ProximoTS > 0 {
				d = "programada para " + g.FechaTexto(f.ProximoTS) + " · " + strings.TrimPrefix(f.Destino, "lista:")
			}
			out = append(out, Fila{ID: fmt.Sprint(f.ID), Titulo: f.Nombre, Detalle: d})
		}
	}
	return out
}

func idArg(a Args) int64 { return int64(a.Num("id")) }

func (g *Gestor) difusionDe(a Args) (*almacen.AgendaFila, error) {
	f, err := g.Alm.ObtenerAgenda(g.BotID, idArg(a))
	if err != nil || f == nil || f.Modulo != IDDifusion {
		return nil, errors.New("ese anuncio ya no existe")
	}
	return f, nil
}

func accionDifusion(g *Gestor, accion string, a Args) (string, error) {
	ahora := g.ahora()
	switch accion {
	case "listas.agregar":
		lista, persona := a.Texto("lista"), a.Texto("persona")
		if lista == "" || persona == "" {
			return "", errors.New("elige la persona y escribe la lista")
		}
		if !g.hayChat(persona) {
			return "", errors.New("esa persona nunca te ha escrito: solo se puede escribir a quien ya te escribió o se suscribió")
		}
		nuevo, err := g.Alm.AgregarALista(g.BotID, lista, persona, "a mano", ahora)
		if err != nil {
			return "", err
		}
		if !nuevo {
			return "Ya estaba en la lista.", nil
		}
		return "Agregada a «" + lista + "».", nil
	case "listas.quitar_persona":
		if err := g.Alm.QuitarDeLista(g.BotID, a.Texto("lista"), a.Texto("persona")); err != nil {
			return "", err
		}
		return "Quitada de la lista.", nil
	case "listas.importar":
		lista := a.Texto("lista")
		if lista == "" {
			return "", errors.New("escribe el nombre de la lista")
		}
		n := 0
		for _, p := range g.Alm.PersonasQueEscribieron(g.BotID) {
			if g.Alm.EstaNoMolestar(g.BotID, p) {
				continue
			}
			if nuevo, _ := g.Alm.AgregarALista(g.BotID, lista, p, "importada", ahora); nuevo {
				n++
			}
		}
		return fmt.Sprintf("Se agregaron %d personas a «%s» (sin contar a quien pidió no recibir mensajes).", n, lista), nil
	case "listas.vaciar":
		if err := g.Alm.BorrarListaMod(g.BotID, a.Texto("id")); err != nil {
			return "", err
		}
		return "Lista vaciada.", nil
	case "difusiones.agregar", "difusiones.vista":
		nombre, texto := a.Texto("nombre"), valorTexto(a, "texto")
		adj, err := g.adjuntosDe(a, "adjuntos")
		if err != nil {
			return "", err
		}
		if texto == "" && len(adj) == 0 {
			return "", errors.New("escribe el mensaje o agrega un archivo")
		}
		if accion == "difusiones.vista" {
			cfg := g.Config(IDDifusion)
			n := len(mustMiembros(g, strings.TrimSpace(a.Texto("lista"))))
			return fmt.Sprintf("Así lo vería María (%d personas en la lista, %d archivos):\n\n%s", n, len(adj), g.textoDifusion(cfg, contenidoDif{Texto: texto}, "María")), nil
		}
		if nombre == "" {
			return "", errors.New("ponle un nombre al anuncio")
		}
		lista := a.Texto("lista")
		if lista == "" {
			lista = ListaSuscritos
		}
		var proximo int64
		estado := dfBorrador
		if f := a.Texto("fecha"); f != "" {
			t, err := g.ParseFechaHora(f)
			if err != nil {
				return "", err
			}
			if !t.After(ahora) {
				return "", errors.New("esa fecha ya pasó")
			}
			proximo, estado = t.Unix(), dfProgramada
		}
		b, _ := json.Marshal(contenidoDif{Texto: texto, Adjuntos: adj})
		if _, err := g.Alm.CrearAgenda(g.BotID, almacen.AgendaFila{Modulo: IDDifusion, Nombre: nombre, Destino: "lista:" + lista, Contenido: string(b), ProximoTS: proximo, Estado: estado}, ahora); err != nil {
			return "", err
		}
		g.Alm.RegistrarAccion(g.BotID, almacen.PersonaDueno, "modulo.difusion_crear", "", "", "ok", "")
		if estado == dfProgramada {
			return "Anuncio guardado: empieza el " + g.FechaTexto(proximo) + ".", nil
		}
		return "Anuncio guardado como borrador. Pulsa Iniciar cuando quieras mandarlo.", nil
	case "difusiones.iniciar":
		f, err := g.difusionDe(a)
		if err != nil {
			return "", err
		}
		if f.Estado == dfEnviando || f.Estado == dfTerminada || f.Estado == dfCancelada {
			return "", errors.New("este anuncio ya está " + textoEstadoDif(f.Estado))
		}
		if !g.Activo(IDDifusion) {
			return "", errors.New("enciende el módulo para poder mandar")
		}
		if err := g.iniciarDifusion(f); err != nil {
			return "", err
		}
		g.Alm.RegistrarAccion(g.BotID, almacen.PersonaDueno, "modulo.difusion_iniciar", "", "", "ok", "")
		g.h.mu.Lock()
		delete(g.h.difFallos, f.ID)
		g.h.mu.Unlock()
		msg := "Envío iniciado: sale de a poco, con esperas al azar y el tope diario."
		if !g.Maestro() {
			msg += " OJO: «Iniciativa propia» está apagada y no saldrá nada hasta que la enciendas."
		}
		return msg, nil
	case "difusiones.pausar":
		f, err := g.difusionDe(a)
		if err != nil {
			return "", err
		}
		if f.Estado != dfEnviando {
			return "", errors.New("solo se pausa lo que se está enviando")
		}
		_ = g.Alm.ActualizarAgenda(g.BotID, f.ID, dfPausada, 0, time.Time{}, "pausada", false)
		g.Cola.VaciarDonde(func(i Iniciativa) bool { return i.Modulo == IDDifusion && i.Que == qDifusion }, "pausada")
		g.Alm.DestEncoladosAPendientes(g.BotID)
		return "Pausada. Pulsa Iniciar para continuar donde se quedó.", nil
	case "difusiones.cancelar":
		f, err := g.difusionDe(a)
		if err != nil {
			return "", err
		}
		_ = g.Alm.ActualizarAgenda(g.BotID, f.ID, dfCancelada, 0, ahora, "cancelada", false)
		g.Cola.VaciarDonde(func(i Iniciativa) bool { return i.Modulo == IDDifusion && i.Que == qDifusion }, "cancelada")
		g.Alm.DestEncoladosAPendientes(g.BotID)
		return "Cancelada.", nil
	case "difusiones.borrar":
		f, err := g.difusionDe(a)
		if err != nil {
			return "", err
		}
		if f.Estado == dfEnviando {
			return "", errors.New("pausa o cancela el anuncio antes de borrarlo")
		}
		return "Borrado.", g.Alm.BorrarAgenda(g.BotID, f.ID)
	}
	return "", errors.New("esa acción no existe")
}
