package grupos

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"botchat/internal/almacen"
)

// Dias válidos del ranking y de los inactivos que ofrece la app (los comandos aceptan cualquier número entre 1 y 365).
var DiasPeriodo = []int{7, 30, 90}

// NormalizaDias da el periodo a usar: el pedido (1..365) o el ajuste `grupo_dias` si no se pidió ninguno.
func (s *Servicio) NormalizaDias(dias int) int {
	if dias <= 0 {
		dias = s.Alm.AjusteBotInt(s.BotID, "grupo_dias")
	}
	if dias <= 0 {
		dias = 30
	}
	if dias > 365 {
		dias = 365
	}
	return dias
}

// FilaRanking es una persona en el ranking.
type FilaRanking struct {
	Puesto     int     `json:"puesto"`
	Persona    string  `json:"persona"`
	Nombre     string  `json:"nombre"`
	Telefono   string  `json:"telefono"`
	Mensajes   int     `json:"mensajes"`
	Porcentaje float64 `json:"porcentaje"`
	Yo         bool    `json:"yo"`
}

// Ranking es la actividad del grupo en un periodo.
type Ranking struct {
	Dias         int           `json:"dias"`
	Total        int           `json:"total"`
	Filas        []FilaRanking `json:"filas"`
	SoloMiembros bool          `json:"soloMiembros"` // true = solo cuenta a quienes están ahora en el grupo
	Desde        string        `json:"desde"`        // primer día con datos guardados por esta app
	Aviso        string        `json:"aviso"`
}

// miembrosPorPersona da, si hay conexión, los miembros actuales por clave canónica (nil si no se pudo saber).
func (s *Servicio) miembrosPorPersona(ctx context.Context, chat string) map[string]Miembro {
	if _, err := parseaGrupo(chat); err != nil || !s.conectado() {
		return nil
	}
	_, inf, err := s.preparar(ctx, chat, false)
	if err != nil {
		return nil
	}
	out := map[string]Miembro{}
	for _, m := range s.miembros(chat, inf) {
		out[m.Persona] = m
	}
	return out
}

// sumaPorPersona junta los remitentes de daily_stats que son la misma persona (número y LID).
func (s *Servicio) sumaPorPersona(act []almacen.ActividadMiembro) map[string]almacen.ActividadMiembro {
	out := map[string]almacen.ActividadMiembro{}
	for _, a := range act {
		k := s.persona(a.Sender)
		x := out[k]
		x.Sender = k
		x.Mensajes += a.Mensajes
		if a.UltimoDia > x.UltimoDia {
			x.UltimoDia = a.UltimoDia
		}
		out[k] = x
	}
	return out
}

func avisoDesde(desde string, inicio string) string {
	if desde != "" && desde > inicio {
		return "Esta app solo cuenta los mensajes desde " + desde + ": antes de esa fecha no hay datos."
	}
	return ""
}

// Ranking cuenta los mensajes por persona desde hace `dias` días (periodo 7, 30 o 90 en la app). Solo lee daily_stats.
func (s *Servicio) Ranking(ctx context.Context, chat string, dias, limite int) (*Ranking, error) {
	if _, err := parseaGrupo(chat); err != nil {
		return nil, err
	}
	dias = s.NormalizaDias(dias)
	if limite <= 0 || limite > 200 {
		limite = 10
	}
	inicio := s.Alm.DiaHace(s.ahora(), dias-1)
	act, err := s.Alm.ActividadGrupo(s.BotID, chat, inicio)
	if err != nil {
		return nil, err
	}
	por := s.sumaPorPersona(act)
	miembros := s.miembrosPorPersona(ctx, chat)
	r := &Ranking{Dias: dias, Filas: []FilaRanking{}, SoloMiembros: miembros != nil, Desde: s.Alm.PrimerDiaGrupo(s.BotID, chat)}
	r.Aviso = avisoDesde(r.Desde, inicio)
	var todas []FilaRanking
	for k, a := range por {
		if a.Mensajes <= 0 {
			continue
		}
		f := FilaRanking{Persona: k, Mensajes: a.Mensajes, Yo: k == almacen.PersonaDueno}
		if miembros != nil {
			m, ok := miembros[k]
			if !ok {
				continue // ya no está en el grupo
			}
			f.Nombre, f.Telefono = m.Nombre, m.Telefono
		} else {
			f.Nombre = s.nombreDe(chat, k)
		}
		if f.Yo {
			f.Nombre = "Yo (este bot)"
		}
		r.Total += a.Mensajes
		todas = append(todas, f)
	}
	sort.SliceStable(todas, func(i, j int) bool {
		if todas[i].Mensajes != todas[j].Mensajes {
			return todas[i].Mensajes > todas[j].Mensajes
		}
		return todas[i].Persona < todas[j].Persona
	})
	for i := range todas {
		todas[i].Puesto = i + 1
		if r.Total > 0 {
			todas[i].Porcentaje = float64(todas[i].Mensajes) * 100 / float64(r.Total)
		}
	}
	if len(todas) > limite {
		todas = todas[:limite]
	}
	r.Filas = append(r.Filas, todas...)
	return r, nil
}

func (s *Servicio) nombreDe(chat, persona string) string {
	if n := s.Alm.NombreConocido(s.BotID, chat, persona); n != "" {
		return n
	}
	if strings.HasSuffix(persona, "@s.whatsapp.net") {
		return "+" + strings.TrimSuffix(persona, "@s.whatsapp.net")
	}
	return "Participante"
}

// Inactivo es un miembro actual sin mensajes en el periodo.
type Inactivo struct {
	Persona   string `json:"persona"`
	Nombre    string `json:"nombre"`
	Telefono  string `json:"telefono"`
	UltimoDia string `json:"ultimoDia"` // "" = nunca desde que esta app guarda datos
	DiasSin   int    `json:"diasSin"`   // -1 = no se sabe
	EsAdmin   bool   `json:"esAdmin"`
}

// Inactivos dice quiénes, entre los miembros ACTUALES, no escribieron en los últimos `dias` días. Necesita la lista de miembros
// (conexión); la actividad sale de daily_stats. Los más callados van primero.
func (s *Servicio) Inactivos(ctx context.Context, chat string, dias int) ([]Inactivo, string, error) {
	j, err := parseaGrupo(chat)
	if err != nil {
		return nil, "", err
	}
	dias = s.NormalizaDias(dias)
	inf, err := s.info(ctx, j, false)
	if err != nil {
		return nil, "", err
	}
	inicio := s.Alm.DiaHace(s.ahora(), dias-1)
	act, err := s.Alm.ActividadGrupo(s.BotID, chat, inicio)
	if err != nil {
		return nil, "", err
	}
	por := s.sumaPorPersona(act)
	hoy := s.Alm.DiaHace(s.ahora(), 0)
	out := []Inactivo{}
	for _, m := range s.miembros(chat, inf) {
		if m.Yo {
			continue
		}
		a := por[m.Persona]
		if a.Mensajes > 0 {
			continue
		}
		x := Inactivo{Persona: m.Persona, Nombre: m.Nombre, Telefono: m.Telefono, UltimoDia: a.UltimoDia, DiasSin: -1, EsAdmin: m.EsAdmin}
		if a.UltimoDia != "" {
			x.DiasSin = diasEntre(a.UltimoDia, hoy)
		}
		out = append(out, x)
	}
	sort.SliceStable(out, func(i, k int) bool {
		if out[i].UltimoDia != out[k].UltimoDia {
			return out[i].UltimoDia < out[k].UltimoDia // "" (nunca) primero
		}
		return out[i].Nombre < out[k].Nombre
	})
	return out, avisoDesde(s.Alm.PrimerDiaGrupo(s.BotID, chat), inicio), nil
}

func diasEntre(a, b string) int {
	ta, e1 := time.Parse("2006-01-02", a)
	tb, e2 := time.Parse("2006-01-02", b)
	if e1 != nil || e2 != nil {
		return -1
	}
	return int(tb.Sub(ta).Hours() / 24)
}

// ---------- mencionar a todos ----------

// ResultadoTodos dice cómo salió "mencionar a todos".
type ResultadoTodos struct {
	Mencionadas int    `json:"mencionadas"`
	Total       int    `json:"total"`
	Mensajes    int    `json:"mensajes"`
	Truncado    bool   `json:"truncado"`
	Aviso       string `json:"aviso"`
}

// MencionarTodos manda uno o varios mensajes que mencionan de verdad (MentionedJID) a los miembros del grupo, de a `todos_lote` por
// mensaje y hasta `todos_tope` personas en total. Es la respuesta a un comando (o una acción mía en la app): nunca sale solo. Entre dos
// usos en el mismo grupo hay una espera (`todos_espera_seg`) que no aplica al dueño. No necesita ser administrador.
func (s *Servicio) MencionarTodos(ctx context.Context, actor, chat, citaID, texto string) (*ResultadoTodos, error) {
	if s.Enviar == nil {
		return nil, ErrNoDisponible
	}
	ctx = almacen.ConEtiqueta(ctx, "comando")
	_, inf, err := s.preparar(ctx, chat, false)
	if err != nil {
		return nil, err
	}
	esDueno := s.persona(actor) == almacen.PersonaDueno
	if espera := time.Duration(s.Alm.AjusteBotInt(s.BotID, "todos_espera_seg")) * time.Second; espera > 0 && !esDueno {
		s.mu.Lock()
		ult := s.todos[chat]
		s.mu.Unlock()
		if falta := espera - s.ahora().Sub(ult); !ult.IsZero() && falta > 0 {
			s.registra(actor, "grupo.todos", "", chat, "espera", "")
			return nil, fallo(fmt.Sprintf("ya mencioné a todos hace poco: espera %d segundos", int(falta.Seconds())+1), nil)
		}
	}
	var destino []Miembro
	for _, m := range s.miembros(chat, inf) {
		if !m.Yo {
			destino = append(destino, m)
		}
	}
	if len(destino) == 0 {
		return nil, ErrSinMenciones
	}
	tope := s.Alm.AjusteBotInt(s.BotID, "todos_tope")
	if tope < 1 {
		tope = 100
	}
	if tope > 1024 {
		tope = 1024
	}
	lote := s.Alm.AjusteBotInt(s.BotID, "todos_lote")
	if lote < 1 {
		lote = 30
	}
	if lote > 50 {
		lote = 50
	}
	r := &ResultadoTodos{Total: len(destino)}
	if len(destino) > tope {
		destino = destino[:tope]
		r.Truncado = true
		r.Aviso = fmt.Sprintf("Mencioné a %d de %d personas porque el tope es %d (se cambia en Ajustes).", tope, r.Total, tope)
	}
	texto = strings.TrimSpace(texto)
	if rs := []rune(texto); len(rs) > maxTextoTodos {
		texto = string(rs[:maxTextoTodos])
	}
	for i := 0; i < len(destino); i += lote {
		fin := i + lote
		if fin > len(destino) {
			fin = len(destino)
		}
		var partes, jids []string
		for _, m := range destino[i:fin] {
			partes = append(partes, "@"+usuarioDe(m.JID))
			jids = append(jids, m.JID)
		}
		linea := strings.Join(partes, " ")
		cita := ""
		if i == 0 {
			cita = citaID
			if texto != "" {
				linea = texto + "\n" + linea
			}
		}
		if i > 0 {
			s.pausa(ctx, 2*time.Second)
		}
		if err := s.Enviar(ctx, chat, linea, cita, jids); err != nil {
			s.registra(actor, "grupo.todos", "", chat, "error", fmt.Sprintf("n=%d", r.Mencionadas))
			if r.Mensajes == 0 {
				return nil, Explicar(err)
			}
			r.Aviso = fmt.Sprintf("Solo alcancé a mencionar a %d de %d personas: WhatsApp dejó de aceptar los mensajes.", r.Mencionadas, len(destino))
			r.Truncado = true
			break
		}
		r.Mensajes++
		r.Mencionadas += len(jids)
		if r.Mensajes == 1 {
			s.mu.Lock()
			s.todos[chat] = s.ahora()
			s.mu.Unlock()
		}
	}
	s.registra(actor, "grupo.todos", "", chat, "ok", fmt.Sprintf("n=%d", r.Mencionadas))
	return r, nil
}

func usuarioDe(j string) string {
	if i := strings.IndexByte(j, '@'); i >= 0 {
		j = j[:i]
	}
	if i := strings.IndexByte(j, ':'); i >= 0 {
		j = j[:i]
	}
	return j
}
