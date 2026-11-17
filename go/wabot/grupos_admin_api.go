package wabot

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"

	"botchat/internal/almacen"
	"botchat/internal/grupos"
)

// ---------- Administrar grupo: lista rica de miembros, perfil, baneos, horario, historial y resumen ----------

// MiembroAdmin es una fila de la lista de miembros: lo guardado más lo que envió, si está en línea y si está baneada.
type MiembroAdmin struct {
	almacen.MiembroVista
	Conteo       almacen.ConteoMiembro `json:"conteo"`
	EnLinea      bool                  `json:"enLinea"`
	UltimaVez    int64                 `json:"ultimaVez"` // 0 = no se sabe (no lo permite)
	Baneado      bool                  `json:"baneado"`
	Advertencias int                   `json:"advertencias"`
}

// PerfilMiembro es todo lo de una persona dentro de un grupo.
type PerfilMiembro struct {
	Miembro       MiembroAdmin          `json:"miembro"`
	Serie         []almacen.PuntoDia    `json:"serie"`
	Horas         [24]int               `json:"horas"`
	PromedioDia   float64               `json:"promedioDia"`
	MediaGrupo    float64               `json:"mediaGrupo"` // mensajes por persona activa en el grupo
	Puesto        int                   `json:"puesto"`     // lugar por mensajes (0 = sin mensajes)
	Activos       int                   `json:"activos"`    // personas con mensajes en el grupo
	MotivoBan     string                `json:"motivoBan"`
	SilencioHasta int64                 `json:"silencioHasta"`
	Silenciado    bool                  `json:"silenciado"`
	Historial     []almacen.EventoGrupo `json:"historial"`
	Desde         int64                 `json:"desde"` // primer mensaje que el bot guardó del grupo
	// Infracciones: cuántas lleva la persona en cada filtro del grupo.
	Infracciones []almacen.InfraccionesPersona `json:"infracciones"`
}

func presenciaDePersona(persona, jid string) (bool, int64) {
	presMu.Lock()
	defer presMu.Unlock()
	for _, k := range []string{persona, jid} {
		if p := pres[k]; p != nil && k != "" {
			return p.EnLinea, p.UltimaVez
		}
	}
	return false, 0
}

func miembrosAdmin(a *almacen.Almacen, botID int64, grupo string, ahora time.Time) ([]MiembroAdmin, int64, error) {
	base, err := a.MiembrosDe(botID, grupo, false, ahora)
	if err != nil {
		return nil, 0, err
	}
	conteos := a.ConteosMiembros(botID, grupo)
	var desde int64
	for _, c := range conteos {
		if c.PrimerTS > 0 && (desde == 0 || c.PrimerTS < desde) {
			desde = c.PrimerTS
		}
	}
	baneados, _ := a.ListarBaneados(botID, grupo)
	ban := map[string]bool{}
	for _, b := range baneados {
		ban[b.Persona] = true
	}
	out := make([]MiembroAdmin, 0, len(base))
	for _, m := range base {
		x := MiembroAdmin{MiembroVista: m, Baneado: ban[m.Persona] || a.EstaBaneado(botID, grupo, m.Persona)}
		if c := conteos[m.Persona]; c != nil {
			x.Conteo = *c
			x.MiembroVista.Mensajes = c.Mensajes
			if c.UltimoTS > 0 {
				x.MiembroVista.UltimoTS = c.UltimoTS
			}
		}
		x.EnLinea, x.UltimaVez = presenciaDePersona(m.Persona, m.JID)
		x.Advertencias = a.ContarAdvertenciasEn(botID, m.Persona, ahora)
		out = append(out, x)
	}
	return out, desde, nil
}

// GrupoMiembrosAdmin devuelve (JSON) {desde, miembros:[…]}: cada miembro con foto, rango, rol, contadores por tipo de mensaje,
// último mensaje, si está en línea y si está baneada. Sale de lo guardado: funciona con el bot apagado.
func GrupoMiembrosAdmin(botID int64, grupo string) (string, error) {
	a, err := almacenAbierto()
	if err != nil {
		return "", err
	}
	ms, desde, err := miembrosAdmin(a, botID, grupo, time.Now())
	if err != nil {
		return "", err
	}
	return js(map[string]any{"desde": desde, "miembros": ms}, nil)
}

// GrupoSuscribirPresencias pide a WhatsApp la presencia (en línea) de los miembros del grupo, en segundo plano. Solo llega la de
// quien lo permite. La pantalla vuelve a leer GrupoMiembrosAdmin al recibir el aviso "presencia:…".
func GrupoSuscribirPresencias(botID int64, grupo string) error {
	a, err := almacenAbierto()
	if err != nil {
		return err
	}
	sv, err := servicioEnvio(botID)
	if err != nil {
		return nil
	}
	ms, _ := a.MiembrosDe(botID, grupo, false, time.Now())
	go func() {
		defer func() { _ = recover() }()
		for i, m := range ms {
			if m.Persona == almacen.PersonaDueno || m.JID == "" || i >= 150 {
				continue
			}
			ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
			_ = sv.Suscribir(ctx, m.JID)
			cancel()
			time.Sleep(120 * time.Millisecond)
		}
	}()
	return nil
}

// GrupoPerfilMiembro devuelve (JSON) el perfil de una persona en un grupo: contadores, serie de 30 días, horas, comparación con el
// grupo, silencio, baneo e historial de moderación.
func GrupoPerfilMiembro(botID int64, grupo, persona string) (string, error) {
	a, err := almacenAbierto()
	if err != nil {
		return "", err
	}
	ahora := time.Now()
	canon := a.ResolverPersona(botID, persona)
	ms, desde, err := miembrosAdmin(a, botID, grupo, ahora)
	if err != nil {
		return "", err
	}
	p := PerfilMiembro{Desde: desde}
	for _, m := range ms {
		if m.Persona == canon || m.Persona == persona {
			p.Miembro = m
			break
		}
	}
	if p.Miembro.Persona == "" { // ya no está en el grupo (o nunca se guardó): lo que haya de ella
		p.Miembro.Persona = canon
		p.Miembro.Nombre = a.NombreDePersona(botID, canon)
		if i := strings.Index(canon, "@s.whatsapp.net"); i > 0 {
			p.Miembro.Telefono = "+" + canon[:i]
		}
		p.Miembro.Conteo = a.ConteoPersona(botID, grupo, canon)
		p.Miembro.MiembroVista.Mensajes = p.Miembro.Conteo.Mensajes
		p.Miembro.MiembroVista.UltimoTS = p.Miembro.Conteo.UltimoTS
		p.Miembro.Baneado = a.EstaBaneado(botID, grupo, canon)
		p.Miembro.Etiquetas = []int64{}
		p.Miembro.Foto = a.FotoDePersona(botID, canon)
	}
	p.Serie, p.Horas = a.SerieMiembro(botID, grupo, canon, 30, ahora)
	total := 0
	for _, d := range p.Serie {
		total += d.N
	}
	p.PromedioDia = float64(total) / 30
	suma, activos, mejores := 0, 0, 0
	for _, m := range ms {
		if m.Conteo.Mensajes > 0 {
			activos++
			suma += m.Conteo.Mensajes
			if m.Conteo.Mensajes > p.Miembro.Conteo.Mensajes {
				mejores++
			}
		}
	}
	p.Activos = activos
	if activos > 0 {
		p.MediaGrupo = float64(suma) / float64(activos)
	}
	if p.Miembro.Conteo.Mensajes > 0 {
		p.Puesto = mejores + 1
	}
	if bs, _ := a.ListarBaneados(botID, grupo); len(bs) > 0 {
		for _, b := range bs {
			if b.Persona == canon {
				p.MotivoBan = b.Motivo
			}
		}
	}
	if ss, err := a.SilenciosActivos(botID, canon, ahora); err == nil {
		for _, s := range ss {
			if s.Cubre(grupo) {
				p.Silenciado = true
				if s.HastaTS == 0 || p.SilencioHasta == 0 || s.HastaTS > p.SilencioHasta {
					p.SilencioHasta = s.HastaTS
				}
			}
		}
	}
	p.Historial, _ = a.HistorialGrupo(botID, grupo, canon, 0, 50)
	p.Infracciones = a.InfraccionesDePersona(botID, grupo, canon)
	return js(p, nil)
}

// GrupoHistorial devuelve (JSON) lo ocurrido en el grupo (cambios de miembros y acciones de moderación), lo más reciente primero.
func GrupoHistorial(botID int64, grupo string, antes int64, limite int) (string, error) {
	a, err := almacenAbierto()
	if err != nil {
		return "", err
	}
	return js(a.HistorialGrupo(botID, grupo, "", antes, limite))
}

// GrupoBaneados devuelve (JSON) la lista de baneados del grupo.
func GrupoBaneados(botID int64, grupo string) (string, error) {
	a, err := almacenAbierto()
	if err != nil {
		return "", err
	}
	return js(a.ListarBaneados(botID, grupo))
}

// GrupoBanear saca a la persona del grupo y no la deja volver (la pantalla ya pidió confirmar).
func GrupoBanear(botID int64, grupo, persona, motivo string) error {
	return conGrupos(botID, func(ctx context.Context, g *grupos.Servicio) error {
		return g.Banear(ctx, dueno(), grupo, persona, motivo)
	})
}

// GrupoDesbanear deja que la persona vuelva a entrar.
func GrupoDesbanear(botID int64, grupo, persona string) error {
	return conGrupos(botID, func(_ context.Context, g *grupos.Servicio) error { return g.Desbanear(dueno(), grupo, persona) })
}

// GrupoHorario devuelve (JSON) el horario de cierre del grupo.
func GrupoHorario(botID int64, grupo string) (string, error) {
	a, err := almacenAbierto()
	if err != nil {
		return "", err
	}
	return js(a.HorarioDe(botID, grupo), nil)
}

// GrupoGuardarHorario guarda el horario ({activo, cierra, abre, dias}) del grupo.
func GrupoGuardarHorario(botID int64, grupo, horarioJSON string) error {
	a, err := almacenAbierto()
	if err != nil {
		return err
	}
	var h almacen.HorarioGrupo
	if err := json.Unmarshal([]byte(horarioJSON), &h); err != nil {
		return errors.New("el horario no es válido")
	}
	h.Grupo = grupo
	if err := a.GuardarHorario(botID, h); err != nil {
		return err
	}
	a.RegistrarAccion(botID, almacen.PersonaDueno, "grupo.horario", "", grupo, "ok", "")
	return nil
}

// ResumenGrupoAdmin son los números de la portada de Administrar.
type ResumenGrupoAdmin struct {
	Miembros    int                  `json:"miembros"`
	Admins      int                  `json:"admins"`
	MensajesHoy int                  `json:"mensajesHoy"`
	Mensajes7   int                  `json:"mensajes7"`
	Activos7    int                  `json:"activos7"`
	Inactivos30 int                  `json:"inactivos30"`
	Baneados    int                  `json:"baneados"`
	Silenciados int                  `json:"silenciados"`
	Advertidos  int                  `json:"advertidos"`
	Serie       []almacen.PuntoDia   `json:"serie"` // últimos 14 días
	Horario     almacen.HorarioGrupo `json:"horario"`
	Top         []MiembroAdmin       `json:"top"` // los 3 más activos
}

// GrupoResumenAdmin devuelve (JSON) los números de la portada de Administrar.
func GrupoResumenAdmin(botID int64, grupo string) (string, error) {
	a, err := almacenAbierto()
	if err != nil {
		return "", err
	}
	ahora := time.Now().In(a.Zona())
	ms, _, err := miembrosAdmin(a, botID, grupo, ahora)
	if err != nil {
		return "", err
	}
	r := ResumenGrupoAdmin{Miembros: len(ms), Horario: a.HorarioDe(botID, grupo)}
	corte7, corte30 := ahora.AddDate(0, 0, -7).Unix(), ahora.AddDate(0, 0, -30).Unix()
	inicioHoy := time.Date(ahora.Year(), ahora.Month(), ahora.Day(), 0, 0, 0, 0, ahora.Location()).Unix()
	for _, m := range ms {
		if m.Rol == almacen.RolGrupoAdmin || m.Rol == almacen.RolGrupoCreador {
			r.Admins++
		}
		if m.Conteo.UltimoTS >= corte7 {
			r.Activos7++
		}
		if m.Conteo.UltimoTS < corte30 && m.Persona != almacen.PersonaDueno {
			r.Inactivos30++
		}
		if m.Silenciado {
			r.Silenciados++
		}
		if m.Advertencias > 0 {
			r.Advertidos++
		}
	}
	bs, _ := a.ListarBaneados(botID, grupo)
	r.Baneados = len(bs)
	r.Serie = a.SerieGrupo(botID, grupo, 14, ahora)
	for i, p := range r.Serie {
		if i >= len(r.Serie)-7 {
			r.Mensajes7 += p.N
		}
	}
	r.MensajesHoy = a.MensajesDesde(botID, grupo, inicioHoy)
	sort.Slice(ms, func(i, j int) bool { return ms[i].Conteo.Mensajes > ms[j].Conteo.Mensajes })
	for _, m := range ms {
		if m.Conteo.Mensajes == 0 || len(r.Top) == 3 {
			break
		}
		r.Top = append(r.Top, m)
	}
	return js(r, nil)
}

// GrupoEntrada devuelve (JSON) la protección de entrada del grupo: modo raid y prefijos de país bloqueados.
func GrupoEntrada(botID int64, grupo string) (string, error) {
	a, err := almacenAbierto()
	if err != nil {
		return "", err
	}
	return js(a.EntradaDe(botID, grupo), nil)
}

// GrupoGuardarEntrada guarda la protección de entrada ({raidActivo, raidMax, raidMinutos, prefijos}).
func GrupoGuardarEntrada(botID int64, grupo, entradaJSON string) error {
	a, err := almacenAbierto()
	if err != nil {
		return err
	}
	var e almacen.ProteccionEntrada
	if err := json.Unmarshal([]byte(entradaJSON), &e); err != nil {
		return errors.New("la configuración no es válida")
	}
	e.Grupo = grupo
	if err := a.GuardarEntrada(botID, e); err != nil {
		return err
	}
	a.RegistrarAccion(botID, almacen.PersonaDueno, "grupo.entrada", "", grupo, "ok", "")
	return nil
}

// ---------- configuración base ----------

// GrupoBaseInfo devuelve (JSON) si hay configuración base, de qué grupo salió y qué trae.
func GrupoBaseInfo(botID int64) (string, error) {
	g, err := servicioGrupos(botID)
	if err != nil {
		return "", err
	}
	return js(g.InfoBase(), nil)
}

// GrupoBaseGuardar copia la configuración de ese grupo como base para los demás.
func GrupoBaseGuardar(botID int64, chat string) error {
	g, err := servicioGrupos(botID)
	if err != nil {
		return err
	}
	return g.GuardarBase(dueno(), chat)
}

// GrupoBaseQuitar borra la configuración base.
func GrupoBaseQuitar(botID int64) error {
	g, err := servicioGrupos(botID)
	if err != nil {
		return err
	}
	return g.QuitarBase(dueno())
}

// GrupoBaseAuto enciende o apaga ponerle la base a cada grupo nuevo.
func GrupoBaseAuto(botID int64, si bool) error {
	g, err := servicioGrupos(botID)
	if err != nil {
		return err
	}
	return g.FijarBaseAuto(dueno(), si)
}

// GrupoBaseAplicar pone la base en los grupos de la lista (JSON: ["…@g.us", …]) y devuelve a cuántos se la puso.
func GrupoBaseAplicar(botID int64, gruposJSON string) (int, error) {
	g, err := servicioGrupos(botID)
	if err != nil {
		return 0, err
	}
	var lista []string
	if json.Unmarshal([]byte(gruposJSON), &lista) != nil || len(lista) == 0 {
		return 0, errors.New("elige al menos un grupo")
	}
	return g.AplicarBase(dueno(), lista)
}

// GrupoEncuesta manda una encuesta al grupo. opcionesJSON es una lista de textos.
func GrupoEncuesta(botID int64, chat, titulo, opcionesJSON string, multiple bool) error {
	g, err := servicioGrupos(botID)
	if err != nil {
		return err
	}
	var ops []string
	if json.Unmarshal([]byte(opcionesJSON), &ops) != nil {
		return errors.New("las opciones no son válidas")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	return g.Encuestar(ctx, dueno(), chat, titulo, ops, multiple)
}

// GrupoFijar fija (dias = 1, 7 o 30) o suelta (fijar = false) un mensaje del grupo.
func GrupoFijar(botID int64, chat, msgID string, dias int, fijar bool) error {
	g, err := servicioGrupos(botID)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	return g.FijarMensaje(ctx, dueno(), chat, msgID, dias, fijar)
}

// GrupoInfractores devuelve (JSON) quién lleva más infracciones en un filtro del grupo (filtro vacío = todos los filtros juntos).
func GrupoInfractores(botID int64, grupo, filtro string, limite int) (string, error) {
	a, err := almacenAbierto()
	if err != nil {
		return "", err
	}
	if filtro != "" && !grupos.EsFiltro(filtro) {
		return "", errors.New("Ese filtro no existe.")
	}
	return js(a.InfractoresDeFiltro(botID, grupo, filtro, limite), nil)
}

// GrupoReiniciarInfracciones pone en cero el contador de una persona (o de todas si persona va vacía) en un filtro (o en todos si va vacío).
// Devuelve cuántas infracciones borró.
func GrupoReiniciarInfracciones(botID int64, grupo, persona, filtro string) (int64, error) {
	a, err := almacenAbierto()
	if err != nil {
		return 0, err
	}
	if filtro != "" && !grupos.EsFiltro(filtro) {
		return 0, errors.New("Ese filtro no existe.")
	}
	n, err := a.BorrarInfraccionesFiltro(botID, grupo, persona, filtro)
	if err != nil {
		return 0, err
	}
	a.RegistrarAccion(botID, almacen.PersonaDueno, "grupo.infracciones_reiniciar", persona, grupo, "ok", filtro)
	return int64(n), nil
}
