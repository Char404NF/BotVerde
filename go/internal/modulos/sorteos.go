package modulos

import (
	"context"
	crand "crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"

	"botchat/internal/almacen"
	"botchat/internal/comandos"
	"botchat/internal/permisos"
)

// ---------- Sorteos ----------
//
// El dueño elige un grupo, los rangos que participan y el premio. El sorteo se hace con azar criptográfico entre los miembros del grupo
// que tienen esos rangos. A cada ganador el bot le escribe por privado (o, si nunca habló con el bot, lo menciona en el grupo) para que
// reclame con «!reclamar» dentro de un plazo. Si no reclama, «perdió su oportunidad»: se le avisa y se sortea otra persona, hasta un
// tope de intentos. Todo vive en la agenda del módulo (una fila por sorteo) y nunca guarda textos de mensajes de nadie.

// IDSorteos es el id del módulo.
const IDSorteos = "sorteos"

const (
	qSorteo = "sorteo"

	faseProgramado = "programado"
	faseReclamo    = "reclamo"
	faseFin        = "fin"

	slotEsperando = "esperando"
	slotReclamado = "reclamado"
	slotPerdido   = "perdido" // nadie lo reclamó y ya no quedan intentos
)

type sorteoSlot struct {
	Persona string `json:"persona"`
	JID     string `json:"jid"`
	Estado  string `json:"estado"`
	Limite  int64  `json:"limite"`
	Intento int    `json:"intento"`
	Aviso   bool   `json:"aviso"` // ya se le recordó que se le acaba el tiempo
}

type sorteoDatos struct {
	Premio      string       `json:"premio"`
	Grupo       string       `json:"grupo"`
	Rangos      []int64      `json:"rangos"`
	SinRango    bool         `json:"sinRango"`
	N           int          `json:"n"`
	Fase        string       `json:"fase"`
	ReclamoMin  int          `json:"reclamoMin"`
	IntentosMax int          `json:"intentosMax"`
	Excluidos   []string     `json:"excluidos"`
	Slots       []sorteoSlot `json:"slots"`
	Part        int          `json:"part"` // cuántas personas participaban al sortear
	Perdieron   []string     `json:"perdieron"`
}

// sorteoAzar elige un número de 0 a n-1 (con azar criptográfico salvo que las pruebas lo fijen).
func (g *Gestor) sorteoAzar(n int) int {
	if n <= 1 {
		return 0
	}
	if g.AzarSeguro != nil {
		return int(g.AzarSeguro(int64(n)))
	}
	x, err := crand.Int(crand.Reader, big.NewInt(int64(n)))
	if err != nil {
		return g.azar(n)
	}
	return int(x.Int64())
}

func sorteos() Modulo {
	return Nuevo(Def{
		ID: IDSorteos, Nombre: "Sorteos", Icono: "difusion", Riesgo: RiesgoMedio, Iniciativa: true,
		Descripcion: "Sortea un premio entre los miembros de un grupo con los rangos que elijas. El ganador recibe un aviso por privado y tiene un tiempo para reclamarlo con «!reclamar»; si no lo hace, pierde su oportunidad y se sortea otra persona.",
		Casillas: []permisos.Casilla{
			{ID: "sorteos.reclamar", Nombre: "Reclamar un premio", Descripcion: "Quien ganó un sorteo reclama su premio con «!reclamar».", Grupo: "Sorteos", Riesgo: permisos.RiesgoBajo, Privado: true, Grupos: true, Base: true},
			{ID: "sorteos.ver", Nombre: "Ver los sorteos", Descripcion: "Pide con «!sorteos» la lista de sorteos en marcha.", Grupo: "Sorteos", Riesgo: permisos.RiesgoBajo, Privado: true, Grupos: true},
		},
		Esquema: []Campo{
			{Clave: "reclamo_min", Titulo: "Minutos para reclamar", Ayuda: "Lo que tiene el ganador para escribir !reclamar. Cada sorteo puede cambiarlo.", Tipo: CampoNumero, Defecto: 30.0, Min: 1, Max: 10080},
			{Clave: "intentos_max", Titulo: "Intentos para encontrar ganador", Ayuda: "Cuántas veces se vuelve a sortear si el ganador no reclama.", Tipo: CampoNumero, Defecto: 3.0, Min: 1, Max: 20},
			{Clave: "aviso_antes_min", Titulo: "Recordatorio antes de que se acabe (minutos)", Ayuda: "0 = sin recordatorio. Solo se manda si el plazo es de al menos el triple.", Tipo: CampoNumero, Defecto: 5.0, Min: 0, Max: 1440},
			{Clave: "avisar_privado", Titulo: "Avisar al ganador por privado", Tipo: CampoInterruptor, Defecto: true},
			{Clave: "privado_conocidos", Titulo: "Solo escribir por privado a quien ya me escribió antes", Ayuda: "Escribirle a alguien que nunca habló con el bot puede hacer que WhatsApp sospeche de la cuenta. Con esto encendido, a los demás se les menciona en el grupo.", Tipo: CampoInterruptor, Defecto: true},
		},
		TopeHora: 20, TopeDia: 80,
		AlActivar: func(g *Gestor) error {
			sorteosTick(g)
			return nil
		},
		AlEvento: func(g *Gestor, e Evento) {
			if e.Tipo == EvTick {
				sorteosTick(g)
			}
		},
		AlEstado: func(g *Gestor) string {
			n := 0
			fs, _ := g.Alm.ListarAgenda(g.BotID, IDSorteos)
			for _, f := range fs {
				if f.Estado == pgActivo {
					n++
				}
			}
			if n == 0 {
				return ""
			}
			return fmt.Sprintf("Encendido · %d sorteos en marcha", n)
		},
		ComandosG: comandosSorteos,
		Paneles:   panelesSorteos,
		AlFilas:   filasSorteos,
		AlAccion:  accionSorteos,
	})
}

// ---------- participantes y sorteo ----------

// participantesSorteo da las personas del grupo que pueden ganar: tienen alguno de los rangos (o ninguno si SinRango), no son quien
// maneja el bot y no están excluidas. Sin rangos elegidos participa todo el grupo.
func (g *Gestor) participantesSorteo(d *sorteoDatos) []almacen.MiembroVista {
	ms, err := g.Alm.MiembrosDe(g.BotID, d.Grupo, false, g.ahora())
	if err != nil {
		return nil
	}
	rangos := map[int64]bool{}
	for _, r := range d.Rangos {
		rangos[r] = true
	}
	excl := map[string]bool{}
	for _, x := range d.Excluidos {
		excl[g.Alm.ResolverPersona(g.BotID, x)] = true
	}
	var out []almacen.MiembroVista
	for _, m := range ms {
		p := g.Alm.ResolverPersona(g.BotID, m.Persona)
		if p == "" || p == almacen.PersonaDueno || excl[p] {
			continue
		}
		if len(rangos) > 0 || d.SinRango {
			r, _, _ := g.Alm.RolDePersona(g.BotID, p, g.ahora())
			switch {
			case r == nil && !d.SinRango:
				continue
			case r != nil && !rangos[r.ID]:
				continue
			}
		}
		m.Persona = p
		out = append(out, m)
	}
	return out
}

func (d *sorteoDatos) yaTiene(persona string) bool {
	for _, s := range d.Slots {
		if s.Persona == persona && s.Estado != slotPerdido {
			return true
		}
	}
	return false
}

// elegir saca a una persona al azar entre las que pueden ganar (y no tienen ya un lugar). Devuelve false si ya no queda nadie.
func (g *Gestor) sorteoElegir(d *sorteoDatos) (almacen.MiembroVista, bool) {
	var libres []almacen.MiembroVista
	for _, m := range g.participantesSorteo(d) {
		if !d.yaTiene(m.Persona) {
			libres = append(libres, m)
		}
	}
	if len(libres) == 0 {
		return almacen.MiembroVista{}, false
	}
	return libres[g.sorteoAzar(len(libres))], true
}

// ---------- avisos ----------

// sorteoDecir le avisa a una persona: por privado si se puede; si no, mencionándola en el grupo.
func (g *Gestor) sorteoDecir(d *sorteoDatos, persona, jid, privado, grupo string) {
	cfg := g.Config(IDSorteos)
	dm := cfg.Bool("avisar_privado")
	if dm && cfg.Bool("privado_conocidos") {
		c, _ := g.Alm.ObtenerChat(g.BotID, persona)
		dm = c != nil && c.UltimoTS > 0
	}
	if dm {
		_, _ = g.Enviar(Iniciativa{Modulo: IDSorteos, Que: qSorteo, Chat: persona, Personas: []string{persona}, Texto: privado, Noche: NocheDiferir})
		return
	}
	g.sorteoGrupo(d, "@"+usuarioDe(jid)+" "+grupo, []string{jid}, []string{persona})
}

func (g *Gestor) sorteoGrupo(d *sorteoDatos, texto string, menciones, personas []string) {
	_, _ = g.Enviar(Iniciativa{Modulo: IDSorteos, Que: qSorteo, Chat: d.Grupo, Personas: personas, Texto: texto, Menciones: menciones, Noche: NocheDiferir})
}

func minutosTexto(m int) string {
	switch {
	case m%1440 == 0 && m >= 1440:
		return plural1(m/1440, "día", "días")
	case m%60 == 0 && m >= 60:
		return plural1(m/60, "hora", "horas")
	}
	return plural1(m, "minuto", "minutos")
}

func plural1(n int, uno, varios string) string {
	if n == 1 {
		return "1 " + uno
	}
	return strconv.Itoa(n) + " " + varios
}

// ---------- el sorteo ----------

// sorteoSortear hace el sorteo: elige a los ganadores, les avisa y deja la fase de reclamo. Devuelve cuántos salieron.
func (g *Gestor) sorteoSortear(nombre string, d *sorteoDatos) (int, error) {
	if !g.Maestro() {
		return 0, errors.New("«Iniciativa propia» está apagada: enciéndela para que el bot pueda avisar a los ganadores")
	}
	d.Part = len(g.participantesSorteo(d))
	if d.Part == 0 {
		return 0, errors.New("no hay nadie que pueda participar con esos rangos en ese grupo")
	}
	ahora := g.ahora()
	d.Slots = nil
	for i := 0; i < d.N; i++ {
		m, ok := g.sorteoElegir(d)
		if !ok {
			break
		}
		d.Slots = append(d.Slots, sorteoSlot{Persona: m.Persona, JID: m.JID, Estado: slotEsperando, Limite: ahora.Add(time.Duration(d.ReclamoMin) * time.Minute).Unix(), Intento: 1})
	}
	d.Fase = faseReclamo
	g.sorteoAnunciar(nombre, d, d.Slots)
	return len(d.Slots), nil
}

// sorteoAnunciar avisa en el grupo y a cada ganador del plazo para reclamar.
func (g *Gestor) sorteoAnunciar(nombre string, d *sorteoDatos, slots []sorteoSlot) {
	var men, pers, nombres []string
	for _, s := range slots {
		men = append(men, s.JID)
		pers = append(pers, s.Persona)
		nombres = append(nombres, "@"+usuarioDe(s.JID))
	}
	if len(slots) == 0 {
		return
	}
	g.sorteoGrupo(d, fmt.Sprintf("🎉 Sorteo «%s»: %s\nPremio: %s\nTienes %s para reclamarlo escribiendo !reclamar (por privado al bot o aquí).", nombre, strings.Join(nombres, ", "), d.Premio, minutosTexto(d.ReclamoMin)), men, pers)
	for _, s := range slots {
		g.sorteoDecir(d, s.Persona, s.JID,
			fmt.Sprintf("🎉 ¡Ganaste el sorteo «%s»!\nPremio: %s\nResponde !reclamar en los próximos %s para quedártelo. Si no lo haces, se sortea otra persona.", nombre, d.Premio, minutosTexto(d.ReclamoMin)),
			fmt.Sprintf("ganaste el sorteo «%s» (%s). Escribe !reclamar al bot en %s o se sortea otra persona.", nombre, d.Premio, minutosTexto(d.ReclamoMin)))
	}
}

// sorteoRevisa atiende los plazos de un sorteo en reclamo: recordatorios, oportunidades perdidas y nuevos sorteos. Devuelve la fecha de lo
// siguiente que toca (0 = el sorteo terminó).
func (g *Gestor) sorteoRevisa(nombre string, d *sorteoDatos) int64 {
	cfg := g.Config(IDSorteos)
	ahora := g.ahora().Unix()
	antes := int64(cfg.Int("aviso_antes_min")) * 60
	for i := range d.Slots {
		s := &d.Slots[i]
		if s.Estado != slotEsperando {
			continue
		}
		if ahora >= s.Limite {
			g.sorteoDecir(d, s.Persona, s.JID, fmt.Sprintf("⌛ Se acabó el tiempo y no reclamaste el premio del sorteo «%s». Perdiste tu oportunidad.", nombre),
				fmt.Sprintf("se acabó el tiempo y no reclamaste el premio del sorteo «%s». Perdiste tu oportunidad.", nombre))
			d.Excluidos = append(d.Excluidos, s.Persona)
			d.Perdieron = append(d.Perdieron, s.Persona)
			if s.Intento >= d.IntentosMax {
				s.Estado = slotPerdido
				continue
			}
			nuevo, ok := g.sorteoElegir(d)
			if !ok {
				s.Estado = slotPerdido
				continue
			}
			*s = sorteoSlot{Persona: nuevo.Persona, JID: nuevo.JID, Estado: slotEsperando, Limite: g.ahora().Add(time.Duration(d.ReclamoMin) * time.Minute).Unix(), Intento: s.Intento + 1}
			g.sorteoAnunciar(nombre, d, []sorteoSlot{*s})
			continue
		}
		if antes > 0 && !s.Aviso && int64(d.ReclamoMin)*60 >= 3*antes && ahora >= s.Limite-antes {
			s.Aviso = true
			g.sorteoDecir(d, s.Persona, s.JID, fmt.Sprintf("⏳ Te quedan %s para reclamar el premio del sorteo «%s». Escribe !reclamar.", minutosTexto(int((s.Limite-ahora+59)/60)), nombre),
				fmt.Sprintf("te quedan %s para reclamar el premio del sorteo «%s». Escribe !reclamar.", minutosTexto(int((s.Limite-ahora+59)/60)), nombre))
		}
	}
	return g.sorteoSiguiente(d, antes)
}

// sorteoSiguiente da cuándo toca revisar de nuevo, o 0 y deja el sorteo terminado si ya nadie espera.
func (g *Gestor) sorteoSiguiente(d *sorteoDatos, antes int64) int64 {
	var prox int64
	for _, s := range d.Slots {
		if s.Estado != slotEsperando {
			continue
		}
		t := s.Limite
		if antes > 0 && !s.Aviso && int64(d.ReclamoMin)*60 >= 3*antes {
			t = s.Limite - antes
		}
		if prox == 0 || t < prox {
			prox = t
		}
	}
	if prox == 0 {
		d.Fase = faseFin
	}
	return prox
}

// sorteoCierra avisa el resultado final en el grupo y al dueño.
func (g *Gestor) sorteoCierra(nombre string, d *sorteoDatos) {
	var ganaron, sin []string
	var men []string
	for _, s := range d.Slots {
		if s.Estado == slotReclamado {
			ganaron = append(ganaron, "@"+usuarioDe(s.JID))
			men = append(men, s.JID)
		} else {
			sin = append(sin, "")
		}
	}
	switch {
	case len(ganaron) > 0:
		g.sorteoGrupo(d, fmt.Sprintf("✅ Sorteo «%s» terminado. Se llevó el premio (%s): %s", nombre, d.Premio, strings.Join(ganaron, ", ")), men, nil)
	default:
		g.sorteoGrupo(d, fmt.Sprintf("Sorteo «%s» terminado sin ganador: nadie reclamó el premio a tiempo.", nombre), nil, nil)
	}
	if g.Aviso != nil {
		if len(sin) > 0 {
			g.Aviso("Sorteo sin ganador", "«"+nombre+"»: "+plural1(len(sin), "premio sin reclamar", "premios sin reclamar")+" después de "+plural1(d.IntentosMax, "intento", "intentos")+".")
		} else {
			g.Aviso("Sorteo terminado", "«"+nombre+"»: todos los premios fueron reclamados.")
		}
	}
}

// sorteosTick atiende los sorteos que ya tocan (la hora del sorteo o un plazo de reclamo).
func sorteosTick(g *Gestor) {
	if !g.Maestro() || g.EnModoSeguro() {
		return
	}
	vencidas, _ := g.Alm.AgendaVencida(g.BotID, IDSorteos, g.ahora())
	for _, f := range vencidas {
		var d sorteoDatos
		if json.Unmarshal([]byte(f.Contenido), &d) != nil {
			_ = g.Alm.ActualizarAgenda(g.BotID, f.ID, pgCancelado, 0, g.ahora(), "datos_dañados", false)
			continue
		}
		var prox int64
		switch d.Fase {
		case faseProgramado:
			if _, err := g.sorteoSortear(f.Nombre, &d); err != nil {
				if g.Aviso != nil {
					g.Aviso("Sorteo sin hacer", "«"+f.Nombre+"»: "+err.Error()+".")
				}
				_ = g.Alm.ActualizarAgenda(g.BotID, f.ID, pgCancelado, 0, g.ahora(), "sin_participantes", false)
				continue
			}
			prox = g.sorteoSiguiente(&d, 0)
			if prox > 0 {
				prox = g.sorteoProximoConAviso(&d)
			}
		case faseReclamo:
			prox = g.sorteoRevisa(f.Nombre, &d)
		}
		g.sorteoGuarda(f, &d, prox)
	}
}

func (g *Gestor) sorteoProximoConAviso(d *sorteoDatos) int64 {
	return g.sorteoSiguiente(d, int64(g.Config(IDSorteos).Int("aviso_antes_min"))*60)
}

// sorteoGuarda deja el estado del sorteo y su próxima revisión; si ya terminó, lo cierra y avisa el resultado.
func (g *Gestor) sorteoGuarda(f almacen.AgendaFila, d *sorteoDatos, prox int64) {
	b, _ := json.Marshal(d)
	_ = g.Alm.ActualizarContenidoAgenda(g.BotID, f.ID, string(b))
	if d.Fase == faseFin || prox == 0 {
		d.Fase = faseFin
		b, _ = json.Marshal(d)
		_ = g.Alm.ActualizarContenidoAgenda(g.BotID, f.ID, string(b))
		_ = g.Alm.ActualizarAgenda(g.BotID, f.ID, pgTerminado, 0, g.ahora(), "terminado", true)
		g.sorteoCierra(f.Nombre, d)
		return
	}
	_ = g.Alm.ActualizarAgenda(g.BotID, f.ID, pgActivo, prox, g.ahora(), "en_reclamo", false)
}

// ---------- comandos ----------

func comandosSorteos(g *Gestor) []comandos.Comando {
	return []comandos.Comando{
		{Nombre: "reclamar", Resumen: "Reclamar el premio de un sorteo que ganaste", Uso: "reclamar", Casilla: "sorteos.reclamar",
			Ejecutar: func(cx *comandos.Contexto) (string, error) {
				quien := g.Alm.ResolverPersona(g.BotID, cx.Persona)
				fs, _ := g.Alm.AgendaConEstado(g.BotID, IDSorteos, pgActivo)
				for _, f := range fs {
					var d sorteoDatos
					if json.Unmarshal([]byte(f.Contenido), &d) != nil || d.Fase != faseReclamo {
						continue
					}
					for i := range d.Slots {
						s := &d.Slots[i]
						if s.Estado != slotEsperando || g.Alm.ResolverPersona(g.BotID, s.Persona) != quien {
							continue
						}
						if g.ahora().Unix() >= s.Limite {
							continue // el plazo ya venció: lo atiende el próximo minuto
						}
						s.Estado = slotReclamado
						prox := g.sorteoProximoConAviso(&d)
						g.sorteoGuarda(f, &d, prox)
						if g.Aviso != nil {
							g.Aviso("Premio reclamado", g.nombreOPalabra(s.Persona, "")+" reclamó «"+d.Premio+"» del sorteo «"+f.Nombre+"».")
						}
						g.Alm.RegistrarAccion(g.BotID, s.Persona, "sorteo.reclamar", "", d.Grupo, "ok", "")
						return fmt.Sprintf("🎁 ¡Listo! Reclamaste «%s» del sorteo «%s». El administrador se pondrá en contacto contigo para entregártelo.", d.Premio, f.Nombre), nil
					}
				}
				return "No tienes ningún premio por reclamar ahora mismo.", nil
			}},
		{Nombre: "sorteos", Resumen: "Ver los sorteos en marcha", Uso: "sorteos", Casilla: "sorteos.ver",
			Ejecutar: func(cx *comandos.Contexto) (string, error) {
				fs, _ := g.Alm.AgendaConEstado(g.BotID, IDSorteos, pgActivo)
				var b strings.Builder
				for _, f := range fs {
					var d sorteoDatos
					if json.Unmarshal([]byte(f.Contenido), &d) != nil {
						continue
					}
					fmt.Fprintf(&b, "• %s — %s\n", f.Nombre, textoFaseSorteo(g, f, &d))
				}
				if b.Len() == 0 {
					return "No hay sorteos en marcha.", nil
				}
				return strings.TrimSpace(b.String()), nil
			}},
	}
}

func textoFaseSorteo(g *Gestor, f almacen.AgendaFila, d *sorteoDatos) string {
	switch d.Fase {
	case faseProgramado:
		return "se sortea el " + g.FechaTexto(f.ProximoTS)
	case faseReclamo:
		esp := 0
		for _, s := range d.Slots {
			if s.Estado == slotEsperando {
				esp++
			}
		}
		return plural1(esp, "ganador esperando reclamar", "ganadores esperando reclamar")
	}
	return "terminado"
}

// ---------- paneles ----------

func (g *Gestor) opcionesRangos() []Opcion {
	out := []Opcion{{Valor: "", Titulo: "—"}}
	rs, _ := g.Alm.ListarRoles(g.BotID)
	for _, r := range rs {
		if !r.EsDueno {
			out = append(out, Opcion{Valor: strconv.FormatInt(r.ID, 10), Titulo: r.Nombre})
		}
	}
	return append(out, Opcion{Valor: "sin", Titulo: "Personas sin rango"})
}

func panelesSorteos(g *Gestor) []Panel {
	rangos := g.opcionesRangos()
	return []Panel{{
		ID: "sorteos", Titulo: "Sorteos",
		Ayuda: "Elige el grupo, hasta tres rangos que participan (si no eliges ninguno participa todo el grupo) y el premio. Si dejas la fecha vacía, se sortea de inmediato. El ganador tiene un tiempo para escribir !reclamar; si no lo hace, pierde su oportunidad y se sortea otra persona.",
		Campos: []Campo{
			{Clave: "nombre", Titulo: "Nombre del sorteo", Tipo: CampoTexto, Largo: 60, Obligatorio: true},
			{Clave: "grupo", Titulo: "Grupo", Tipo: CampoLista, Opciones: g.opcionesChats(true, 100)},
			{Clave: "premio", Titulo: "Premio", Tipo: CampoTexto, Largo: 200, Obligatorio: true},
			{Clave: "rango1", Titulo: "Rango que participa (1)", Tipo: CampoLista, Defecto: "", Opciones: rangos},
			{Clave: "rango2", Titulo: "Rango que participa (2)", Tipo: CampoLista, Defecto: "", Opciones: rangos},
			{Clave: "rango3", Titulo: "Rango que participa (3)", Tipo: CampoLista, Defecto: "", Opciones: rangos},
			{Clave: "ganadores", Titulo: "Cuántos ganadores", Tipo: CampoNumero, Defecto: 1.0, Min: 1, Max: 10},
			{Clave: "reclamo_min", Titulo: "Minutos para reclamar (0 = el del módulo)", Tipo: CampoNumero, Defecto: 0.0, Min: 0, Max: 10080},
			{Clave: "fecha", Titulo: "Fecha y hora del sorteo (opcional)", Ayuda: "Por ejemplo 25/12/2026 18:30. Vacío = sortear ahora.", Tipo: CampoTexto, Largo: 20},
		},
		Alta:    "Crear sorteo",
		Extras:  []Boton{{Accion: "sorteos.vista", Titulo: "Ver quiénes participan"}},
		PorFila: []Boton{{Accion: "sorteos.sortear", Titulo: "Sortear ya"}, {Accion: "sorteos.cancelar", Titulo: "Cancelar", Confirma: "¿Cancelar este sorteo?"}, {Accion: "sorteos.borrar", Titulo: "Borrar", Confirma: "¿Borrar este sorteo de la lista?"}},
	}}
}

func filasSorteos(g *Gestor, panel string) []Fila {
	var out []Fila
	fs, _ := g.Alm.ListarAgenda(g.BotID, IDSorteos)
	for _, f := range fs {
		var d sorteoDatos
		_ = json.Unmarshal([]byte(f.Contenido), &d)
		det := g.nombreChat(d.Grupo) + " · " + d.Premio + " · "
		switch f.Estado {
		case pgActivo:
			det += textoFaseSorteo(g, f, &d)
		case pgTerminado:
			rec := 0
			for _, s := range d.Slots {
				if s.Estado == slotReclamado {
					rec++
				}
			}
			det += plural1(rec, "premio reclamado", "premios reclamados")
		default:
			det += "cancelado"
		}
		out = append(out, Fila{ID: fmt.Sprint(f.ID), Titulo: f.Nombre, Detalle: det})
	}
	return out
}

func sorteoDeArgs(g *Gestor, a Args) (*sorteoDatos, error) {
	grupo := a.Texto("grupo")
	if grupo == "" || !strings.HasSuffix(grupo, "@g.us") || !g.hayChat(grupo) {
		return nil, errors.New("elige el grupo del sorteo")
	}
	cfg := g.Config(IDSorteos)
	d := &sorteoDatos{Premio: strings.TrimSpace(valorTexto(a, "premio")), Grupo: grupo, Fase: faseProgramado, N: int(a.Num("ganadores")),
		ReclamoMin: int(a.Num("reclamo_min")), IntentosMax: cfg.Int("intentos_max")}
	if d.N < 1 {
		d.N = 1
	}
	if d.ReclamoMin < 1 {
		d.ReclamoMin = cfg.Int("reclamo_min")
	}
	if d.IntentosMax < 1 {
		d.IntentosMax = 3
	}
	for _, k := range []string{"rango1", "rango2", "rango3"} {
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
	return d, nil
}

func accionSorteos(g *Gestor, accion string, a Args) (string, error) {
	ahora := g.ahora()
	switch accion {
	case "sorteos.vista":
		d, err := sorteoDeArgs(g, a)
		if err != nil {
			return "", err
		}
		ps := g.participantesSorteo(d)
		if len(ps) == 0 {
			return "Con esos rangos no participa nadie en ese grupo.", nil
		}
		var nombres []string
		for i, p := range ps {
			if i >= 15 {
				nombres = append(nombres, fmt.Sprintf("y %d más", len(ps)-15))
				break
			}
			nombres = append(nombres, g.nombreOPalabra(p.Persona, ""))
		}
		return fmt.Sprintf("Participan %d: %s.", len(ps), strings.Join(nombres, ", ")), nil
	case "sorteos.agregar":
		nombre := strings.TrimSpace(a.Texto("nombre"))
		d, err := sorteoDeArgs(g, a)
		if err != nil {
			return "", err
		}
		if nombre == "" || d.Premio == "" {
			return "", errors.New("pon el nombre y el premio")
		}
		if len(g.participantesSorteo(d)) == 0 {
			return "", errors.New("con esos rangos no participa nadie en ese grupo")
		}
		var fecha time.Time
		if t := strings.TrimSpace(a.Texto("fecha")); t != "" {
			fecha, err = g.ParseFechaHora(t)
			if err != nil {
				return "", err
			}
			if !fecha.After(ahora) {
				return "", errors.New("esa fecha ya pasó")
			}
		} else if !g.Maestro() {
			return "", errors.New("«Iniciativa propia» está apagada: enciéndela para sortear y avisar al ganador")
		}
		b, _ := json.Marshal(d)
		prox := ahora.Unix()
		if !fecha.IsZero() {
			prox = fecha.Unix()
		}
		id, err := g.Alm.CrearAgenda(g.BotID, almacen.AgendaFila{Modulo: IDSorteos, Nombre: nombre, Destino: d.Grupo, Contenido: string(b), ProximoTS: prox, Estado: pgActivo}, ahora)
		if err != nil {
			return "", err
		}
		g.Alm.RegistrarAccion(g.BotID, almacen.PersonaDueno, "sorteo.crear", "", d.Grupo, "ok", "")
		if fecha.IsZero() {
			sorteosTick(g)
			f, _ := g.Alm.ObtenerAgenda(g.BotID, id)
			if f != nil && f.Estado == pgCancelado {
				return "", errors.New("no se pudo sortear: revisa que haya participantes")
			}
			return "Sorteo hecho: avisé a los ganadores.", nil
		}
		return "Sorteo programado para el " + g.FechaTexto(fecha.Unix()) + ".", nil
	case "sorteos.sortear":
		f, d, err := sorteoFila(g, a)
		if err != nil {
			return "", err
		}
		if d.Fase != faseProgramado {
			return "", errors.New("ese sorteo ya se hizo")
		}
		n, err := g.sorteoSortear(f.Nombre, d)
		if err != nil {
			return "", err
		}
		g.sorteoGuarda(*f, d, g.sorteoProximoConAviso(d))
		return fmt.Sprintf("Sorteo hecho: %s.", plural1(n, "ganador avisado", "ganadores avisados")), nil
	case "sorteos.cancelar", "sorteos.borrar":
		f, _, err := sorteoFila(g, a)
		if err != nil {
			return "", err
		}
		if accion == "sorteos.borrar" {
			return "Borrado.", g.Alm.BorrarAgenda(g.BotID, f.ID)
		}
		return "Cancelado.", g.Alm.ActualizarAgenda(g.BotID, f.ID, pgCancelado, 0, time.Time{}, "cancelado", false)
	}
	return "", errors.New("esa acción no existe")
}

func sorteoFila(g *Gestor, a Args) (*almacen.AgendaFila, *sorteoDatos, error) {
	f, err := g.Alm.ObtenerAgenda(g.BotID, idArg(a))
	if err != nil || f == nil || f.Modulo != IDSorteos {
		return nil, nil, errors.New("ese sorteo ya no existe")
	}
	var d sorteoDatos
	if json.Unmarshal([]byte(f.Contenido), &d) != nil {
		return nil, nil, errors.New("ese sorteo está dañado")
	}
	return f, &d, nil
}

var _ = context.Background
