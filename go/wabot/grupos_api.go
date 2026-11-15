package wabot

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"botchat/internal/almacen"
	"botchat/internal/grupos"
)

// ---------- armado dentro del bot ----------

// armarGrupos crea el servicio de grupos del bot que se está iniciando, le conecta el envío, el borrado de mensajes ajenos y las
// advertencias, y registra sus comandos. Va después de armarPermisos (necesita rangos y comandos) y antes de armarPersonas.
func (s *estado) armarGrupos() {
	if s.perm == nil || s.cmd == nil {
		return
	}
	g := grupos.Nuevo(alm, s.botID, grupos.Real{Client: s.cli}, s.perm)
	g.Log = s.log
	g.Borrar = func(ctx context.Context, chat, id string) error { return s.envio.BorrarParaTodos(ctx, chat, id) }
	g.Enviar = func(ctx context.Context, chat, texto, cita string, menciones []string) error {
		r, err := s.envio.EnviarAuto(ctx, chat, texto, cita, menciones)
		if err != nil {
			return err
		}
		if r.Estado != "enviado" {
			return errNoEnviado
		}
		return nil
	}
	g.Advertir = func(ctx context.Context, objetivo, chat, msgID, motivo string) error {
		if s.pers == nil {
			return grupos.ErrNoDisponible
		}
		_, err := s.pers.Advertir(ctx, almacen.PersonaDueno, objetivo, chat, msgID, motivo)
		return err
	}
	g.RegistrarComandos(s.cmd)
	s.gru = g
	s.iniciarHorarios()
}

// iniciarHorarios abre y cierra los grupos con horario: revisa cada minuto (y una vez al arrancar).
func (s *estado) iniciarHorarios() {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		defer func() {
			if r := recover(); r != nil {
				s.log(fmt.Sprintf("error interno en el horario de los grupos: %v", r))
			}
		}()
		t := time.NewTicker(time.Minute)
		defer t.Stop()
		for {
			select {
			case <-s.ctx.Done():
				return
			case <-t.C:
				ctx, cancel := context.WithTimeout(s.ctx, 50*time.Second)
				s.gru.AplicarHorarios(ctx, time.Now())
				cancel()
			}
		}
	}()
}

// alCambiarGrupo: WhatsApp avisó de un cambio en un grupo (administradores, ajustes, miembros): lo recordado deja de valer.
// Solo borra cosas en memoria, así que es seguro en el manejador de eventos.
func (s *estado) alCambiarGrupo(chat string) {
	if s.gru != nil {
		s.gru.Invalidar(chat)
	}
	if s.envio != nil {
		s.envio.InvalidarGrupo(chat)
	}
	s.admins.olvida(chat)
}

// filtrarGrupo revisa un mensaje de grupo con los filtros del grupo (se llama desde el trabajador, nunca desde el manejador de
// eventos). Revisar es rápido y no usa la red; la acción (borrar, expulsar, avisar…) corre en una tarea aparte.
func (s *estado) filtrarGrupo(e grupos.Entrada) {
	if s.gru == nil || s.perm == nil {
		return
	}
	e.Persona = s.perm.Persona(e.Persona)
	inf := s.gru.Evaluar(e)
	if inf == nil {
		return
	}
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		ctx, cancel := context.WithTimeout(s.ctx, 45*time.Second)
		defer cancel()
		s.gru.Aplicar(ctx, inf)
	}()
}

// ---------- servicios sin el bot en marcha ----------

var (
	gruLibres   = map[int64]*grupos.Servicio{}
	gruLibresMu sync.Mutex
)

// servicioGrupos da el servicio de grupos del bot en marcha, o uno sin conexión (ranking, filtros y reglas se pueden ver y editar con
// el bot apagado; lo que necesita WhatsApp contesta que no hay conexión).
func servicioGrupos(botID int64) (*grupos.Servicio, error) {
	mu.Lock()
	s := st
	a := alm
	mu.Unlock()
	if a == nil {
		return nil, errors.New("el almacén no está abierto")
	}
	if s != nil && s.botID == botID && s.gru != nil {
		return s.gru, nil
	}
	p, _, err := servicioPerm(botID)
	if err != nil {
		return nil, err
	}
	gruLibresMu.Lock()
	defer gruLibresMu.Unlock()
	if x, ok := gruLibres[botID]; ok && x.Alm == a {
		return x, nil
	}
	x := grupos.Nuevo(a, botID, nil, p)
	gruLibres[botID] = x
	return x, nil
}

func conGrupos(botID int64, f func(ctx context.Context, g *grupos.Servicio) error) error {
	g, err := servicioGrupos(botID)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	return f(ctx, g)
}

func jsGrupos(botID int64, f func(ctx context.Context, g *grupos.Servicio) (any, error)) (string, error) {
	var out any
	err := conGrupos(botID, func(ctx context.Context, g *grupos.Servicio) error {
		v, e := f(ctx, g)
		out = v
		return e
	})
	return js(out, err)
}

// ---------- API de grupos (la usa el dueño desde la app) ----------

// GrupoEstado dice (JSON) si el bot está conectado, si sigue en el grupo y si es administrador, con una explicación clara en `motivo`
// cuando algo no se puede. refrescar = preguntar a WhatsApp aunque haya información reciente. La app lo usa para activar o apagar las acciones.
func GrupoEstado(botID int64, chat string, refrescar bool) (string, error) {
	return jsGrupos(botID, func(ctx context.Context, g *grupos.Servicio) (any, error) { return g.Estado(ctx, chat, refrescar), nil })
}

// GrupoInfo devuelve (JSON) la información del grupo: nombre, descripción, creación, miembros, administradores, ajustes y reglas.
func GrupoInfo(botID int64, chat string) (string, error) {
	return jsGrupos(botID, func(ctx context.Context, g *grupos.Servicio) (any, error) { return g.Info(ctx, chat) })
}

// GrupoMiembros devuelve (JSON) las personas del grupo, administradores primero.
func GrupoMiembros(botID int64, chat string) (string, error) {
	return jsGrupos(botID, func(ctx context.Context, g *grupos.Servicio) (any, error) { return g.Miembros(ctx, chat) })
}

// GrupoCerrar cierra el grupo (solo administradores escriben) o lo abre. Requiere ser administrador.
func GrupoCerrar(botID int64, chat string, cerrado bool) error {
	return conGrupos(botID, func(ctx context.Context, g *grupos.Servicio) error { return g.Cerrar(ctx, dueno(), chat, cerrado) })
}

// GrupoBloquearInfo hace que solo los administradores cambien nombre, foto y descripción (o lo libera). Requiere ser administrador.
func GrupoBloquearInfo(botID int64, chat string, bloqueada bool) error {
	return conGrupos(botID, func(ctx context.Context, g *grupos.Servicio) error {
		return g.BloquearInfo(ctx, dueno(), chat, bloqueada)
	})
}

// GrupoExpulsar saca a una persona del grupo (la pantalla ya pidió confirmar). `persona` es el jid de GrupoMiembros. Requiere ser administrador.
func GrupoExpulsar(botID int64, chat, persona string) error {
	return conGrupos(botID, func(ctx context.Context, g *grupos.Servicio) error {
		return g.ExpulsarComo(ctx, dueno(), chat, persona)
	})
}

// GrupoAscender hace administradora a una persona (admin = true) o le quita el cargo (admin = false). Requiere ser administrador.
func GrupoAscender(botID int64, chat, persona string, admin bool) error {
	return conGrupos(botID, func(ctx context.Context, g *grupos.Servicio) error {
		if admin {
			return g.Ascender(ctx, dueno(), chat, persona)
		}
		return g.Bajar(ctx, dueno(), chat, persona)
	})
}

// GrupoSolicitudes devuelve (JSON) a quienes piden entrar al grupo. Requiere ser administrador.
func GrupoSolicitudes(botID int64, chat string) (string, error) {
	return jsGrupos(botID, func(ctx context.Context, g *grupos.Servicio) (any, error) { return g.Solicitudes(ctx, dueno(), chat) })
}

// GrupoResolverSolicitudes aprueba o rechaza solicitudes. personasJSON es una lista JSON de jid (vacía = todas). Devuelve cuántas se resolvieron.
func GrupoResolverSolicitudes(botID int64, chat, personasJSON string, aprobar bool) (int, error) {
	var ps []string
	if personasJSON != "" {
		if err := json.Unmarshal([]byte(personasJSON), &ps); err != nil {
			return 0, errors.New("la lista de personas no es válida")
		}
	}
	n := 0
	err := conGrupos(botID, func(ctx context.Context, g *grupos.Servicio) error {
		x, e := g.ResolverSolicitudes(ctx, dueno(), chat, ps, aprobar)
		n = x
		return e
	})
	return n, err
}

// GrupoRenombrar cambia el nombre del grupo. Requiere ser administrador.
func GrupoRenombrar(botID int64, chat, nombre string) error {
	return conGrupos(botID, func(ctx context.Context, g *grupos.Servicio) error { return g.Renombrar(ctx, dueno(), chat, nombre) })
}

// GrupoDescripcion cambia la descripción del grupo (vacía = la borra). Requiere ser administrador.
func GrupoDescripcion(botID int64, chat, texto string) error {
	return conGrupos(botID, func(ctx context.Context, g *grupos.Servicio) error { return g.Descripcion(ctx, dueno(), chat, texto) })
}

// GrupoEnlace devuelve el enlace de invitación; reiniciar = revoca el anterior y crea uno nuevo (la pantalla ya pidió confirmar).
func GrupoEnlace(botID int64, chat string, reiniciar bool) (string, error) {
	var l string
	err := conGrupos(botID, func(ctx context.Context, g *grupos.Servicio) error {
		x, e := g.Enlace(ctx, dueno(), chat, reiniciar)
		l = x
		return e
	})
	return l, err
}

// GrupoBorrarMensaje borra para todos un mensaje de otra persona (la pantalla ya pidió confirmar). Requiere ser administrador.
func GrupoBorrarMensaje(botID int64, chat, msgID string) error {
	return conGrupos(botID, func(ctx context.Context, g *grupos.Servicio) error { return g.BorrarMensaje(ctx, dueno(), chat, msgID) })
}

// GrupoMencionarTodos manda la mención a todo el grupo (con el tope y los lotes de Ajustes). Devuelve (JSON) cuántas personas se mencionaron.
func GrupoMencionarTodos(botID int64, chat, texto string) (string, error) {
	return jsGrupos(botID, func(ctx context.Context, g *grupos.Servicio) (any, error) {
		return g.MencionarTodos(ctx, dueno(), chat, "", texto)
	})
}

// GrupoRanking devuelve (JSON) quién escribe más: periodo de `dias` días (7, 30 o 90; 0 = el de Ajustes) y hasta `limite` personas.
func GrupoRanking(botID int64, chat string, dias, limite int) (string, error) {
	return jsGrupos(botID, func(ctx context.Context, g *grupos.Servicio) (any, error) { return g.Ranking(ctx, chat, dias, limite) })
}

// GrupoInactivos devuelve (JSON {dias, aviso, inactivos}) los miembros actuales sin mensajes en `dias` días.
func GrupoInactivos(botID int64, chat string, dias int) (string, error) {
	return jsGrupos(botID, func(ctx context.Context, g *grupos.Servicio) (any, error) {
		d := g.NormalizaDias(dias)
		l, aviso, err := g.Inactivos(ctx, chat, d)
		if err != nil {
			return nil, err
		}
		return struct {
			Dias      int               `json:"dias"`
			Aviso     string            `json:"aviso"`
			Inactivos []grupos.Inactivo `json:"inactivos"`
		}{d, aviso, l}, nil
	})
}

// GrupoReglas devuelve las reglas del grupo que guardaste ("" si no hay).
func GrupoReglas(botID int64, chat string) (string, error) {
	a, err := almacenAbierto()
	if err != nil {
		return "", err
	}
	return a.ReglasGrupo(botID, chat), nil
}

// GrupoGuardarReglas guarda las reglas del grupo ("" las quita).
func GrupoGuardarReglas(botID int64, chat, texto string) error {
	g, err := servicioGrupos(botID)
	if err != nil {
		return err
	}
	return g.FijarReglas(dueno(), chat, texto)
}

// GrupoFiltros devuelve (JSON) los cuatro filtros del grupo (enlaces, palabras, repetidos, flood) con sus valores de fábrica los que no se han tocado.
func GrupoFiltros(botID int64, chat string) (string, error) {
	g, err := servicioGrupos(botID)
	if err != nil {
		return "", err
	}
	return js(g.FiltrosDe(chat), nil)
}

// GrupoGuardarFiltro guarda un filtro del grupo. datosJSON es el filtro tal como lo devuelve GrupoFiltros (activo, accion, config).
func GrupoGuardarFiltro(botID int64, chat, datosJSON string) error {
	g, err := servicioGrupos(botID)
	if err != nil {
		return err
	}
	var f grupos.Filtro
	if err := json.Unmarshal([]byte(datosJSON), &f); err != nil {
		return errors.New("los datos del filtro no son válidos")
	}
	f.Chat = chat
	return g.GuardarFiltro(dueno(), f)
}

// GrupoRestaurarFiltro vuelve un filtro a sus valores de fábrica (apagado).
func GrupoRestaurarFiltro(botID int64, chat, filtro string) error {
	g, err := servicioGrupos(botID)
	if err != nil {
		return err
	}
	return g.RestaurarFiltro(dueno(), chat, filtro)
}
