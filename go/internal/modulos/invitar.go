package modulos

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"botchat/internal/almacen"
	"botchat/internal/comandos"
	"botchat/internal/permisos"
)

// ---------- H4 · Invitar y agregar a grupos ----------
//
// Método por defecto: mandar a la persona el ENLACE de invitación del grupo; ella decide si entra. «Agregar directo» es una opción
// aparte (apagada, con tope bajo y espera de 30 a 90 s) y solo vale para personas de la lista «Quieren entrar»
// (ellas mismas lo pidieron escribiendo la frase). Agregar a alguien a un grupo con la acción real de WhatsApp usa el token de
// privacidad de esa persona; si su privacidad no lo permite WhatsApp falla en silencio, y entonces se cae a la invitación con enlace.
// Solo se escribe a quien ya tiene chat en el teléfono (quien ya escribió).

// IDInvitar es el id del módulo.
const IDInvitar = "invitar"

// ListaQuierenEntrar es la lista de quienes pidieron que los inviten.
const ListaQuierenEntrar = "quieren_entrar"

const (
	qInvitar = "invitacion"
	qAgregar = "agregar"
	qAnotado = "anotado"
)

func invitar() Modulo {
	return Nuevo(Def{
		ID: IDInvitar, Nombre: "Invitar y agregar a grupos", Icono: "grupo", Riesgo: RiesgoMedio, Iniciativa: true,
		Descripcion: "Manda a quien te lo pidió el enlace de invitación a un grupo (ella decide si entra). Aparte, y apagado, puedes agregarla directo al grupo: tiene tope bajo y, si su privacidad no lo permite, se le manda la invitación.",
		Casillas: []permisos.Casilla{
			{ID: "invitar.usar", Nombre: "Invitar a alguien con «!invitar»", Descripcion: "Puede pedir en un grupo que le manden el enlace de invitación a una persona que ya escribió al bot.", Grupo: "Invitar y agregar", Grupos: true, Riesgo: permisos.RiesgoMedio},
		},
		Esquema: []Campo{
			{Clave: "palabra", Titulo: "Frase para pedir entrar", Ayuda: "Quien la escriba sola, en privado, entra a la lista «Quieren entrar».", Tipo: CampoTexto, Defecto: "QUIERO ENTRAR", Largo: 40, Obligatorio: true},
			{Clave: "responder", Titulo: "Confirmar a quien lo pide", Tipo: CampoInterruptor, Defecto: true},
			{Clave: "texto_anotado", Titulo: "Mensaje de confirmación", Tipo: CampoTexto, Varias: true, Largo: 300, Defecto: "Anotado. En cuanto pueda te mando la invitación al grupo."},
			{Clave: "texto_invitacion", Titulo: "Mensaje con la invitación", Ayuda: "Variables: {nombre}, {grupo} y {enlace}.", Tipo: CampoTexto, Varias: true, Largo: 500, Defecto: "Hola {nombre}, aquí está la invitación al grupo *{grupo}*: {enlace}\nTú decides si entras.", Obligatorio: true},
			{Clave: "espera_min", Titulo: "Invitación · espera mínima (segundos)", Tipo: CampoNumero, Defecto: 20.0, Min: 5, Max: 600},
			{Clave: "espera_max", Titulo: "Invitación · espera máxima (segundos)", Tipo: CampoNumero, Defecto: 60.0, Min: 5, Max: 900},
			{Clave: "directo", Titulo: "Permitir agregar directo", Ayuda: "Agrega a la persona al grupo sin esperar a que acepte una invitación. Apagado por defecto.", Tipo: CampoInterruptor, Defecto: false},
			{Clave: "directo_tope_dia", Titulo: "Agregar directo · máximo por día", Tipo: CampoNumero, Defecto: 5.0, Min: 1, Max: 30},
			{Clave: "directo_espera_min", Titulo: "Agregar directo · espera mínima (segundos)", Tipo: CampoNumero, Defecto: 30.0, Min: 30, Max: 900},
			{Clave: "directo_espera_max", Titulo: "Agregar directo · espera máxima (segundos)", Tipo: CampoNumero, Defecto: 90.0, Min: 30, Max: 1800},
		},
		TopeHora: 10, TopeDia: 30,
		Validar: func(c map[string]any) error {
			cfg := Config(c)
			if cfg.Num("espera_max") < cfg.Num("espera_min") || cfg.Num("directo_espera_max") < cfg.Num("directo_espera_min") {
				return errors.New("la espera máxima no puede ser menor que la mínima")
			}
			if !strings.Contains(cfg.Texto("texto_invitacion"), "{enlace}") {
				return errors.New("el mensaje de la invitación debe llevar {enlace}")
			}
			if llave(cfg.Texto("palabra")) == "" || EsBaja(cfg.Texto("palabra")) {
				return errors.New("esa frase no sirve para pedir entrar")
			}
			return nil
		},
		AlEvento: eventoInvitar,
		AlEstado: func(g *Gestor) string {
			if n := len(mustMiembros(g, ListaQuierenEntrar)); n > 0 {
				return fmt.Sprintf("Encendido · %d quieren entrar", n)
			}
			return ""
		},
		ComandosG: func(g *Gestor) []comandos.Comando {
			return []comandos.Comando{{
				Nombre: "invitar", Resumen: "Mandar el enlace del grupo a una persona", Uso: "invitar @persona (o responde a su mensaje)", Casilla: "invitar.usar", Ambito: comandos.Grupo,
				Ejecutar: func(cx *comandos.Contexto) (string, error) {
					p := ""
					switch {
					case len(cx.Menciones) > 0:
						p = cx.Menciones[0]
					case cx.CitaAutor != "":
						p = cx.CitaAutor
					}
					if p == "" {
						return "", errors.New("menciona a la persona o responde a su mensaje")
					}
					if err := g.invitarA(context.Background(), cx.Chat, p); err != nil {
						return "", err
					}
					return "Listo, le mando la invitación por privado en un momento.", nil
				},
			}}
		},
		Paneles:  panelesInvitar,
		AlFilas:  filasInvitar,
		AlAccion: accionInvitar,
	})
}

func eventoInvitar(g *Gestor, e Evento) {
	if e.Tipo != EvMensaje || e.EsGrupo || e.EsMio || e.Persona == "" {
		return
	}
	cfg := g.Config(IDInvitar)
	if EsBaja(e.Texto) {
		g.Alm.QuitarDeTodasLasListas(g.BotID, e.Persona)
		return
	}
	if t := llave(e.Texto); t == "" || t != llave(cfg.Texto("palabra")) {
		return
	}
	nuevo, _ := g.Alm.AgregarALista(g.BotID, ListaQuierenEntrar, e.Persona, "palabra", g.ahora())
	g.Alm.RegistrarModulo(g.BotID, IDInvitar, "pidio_entrar", map[bool]string{true: "ok", false: "ya_estaba"}[nuevo], e.Chat, g.ahora())
	if nuevo && g.Aviso != nil {
		g.Aviso("Alguien quiere entrar a un grupo", "Hay una persona nueva en la lista «Quieren entrar». Ábrela en Módulos → Invitar y agregar.")
	}
	if cfg.Bool("responder") {
		if t := strings.TrimSpace(cfg.Texto("texto_anotado")); t != "" {
			_, _ = g.Enviar(Iniciativa{Modulo: IDInvitar, Que: qAnotado, Chat: e.Chat, Personas: []string{e.Persona}, Texto: t,
				RetrasoMin: 2 * time.Second, RetrasoMax: 8 * time.Second, Noche: NocheDiferir})
		}
	}
}

// invitarA manda el enlace del grupo a una persona (iniciativa por la cola).
func (g *Gestor) invitarA(ctx context.Context, grupoChat, persona string) error {
	cfg := g.Config(IDInvitar)
	if g.Acc == nil {
		return errors.New("sin conexión con WhatsApp: enciende el bot")
	}
	if !g.Activo(IDInvitar) {
		return errors.New("el módulo está apagado")
	}
	if !strings.HasSuffix(grupoChat, "@g.us") {
		return errors.New("elige un grupo")
	}
	if !g.hayChat(persona) {
		return errors.New("esa persona nunca te ha escrito: solo se invita a quien ya tiene chat contigo")
	}
	if g.Alm.EstaNoMolestar(g.BotID, persona) {
		return errors.New("esa persona pidió no recibir mensajes")
	}
	enlace, err := g.Acc.Enlace(ctx, grupoChat)
	if err != nil {
		return err
	}
	texto := sustituir(cfg.Texto("texto_invitacion"), map[string]string{"nombre": g.nombreOPalabra(persona, ""), "grupo": g.nombreChat(grupoChat), "enlace": enlace})
	res, err := g.Enviar(Iniciativa{Modulo: IDInvitar, Que: qInvitar, Chat: persona, Personas: []string{persona}, Texto: texto, Noche: NocheDiferir,
		RetrasoMin: segundos(cfg.Num("espera_min")), RetrasoMax: segundos(cfg.Num("espera_max")),
		AlResultado: func(r string) {
			if r == ResEnviado {
				_ = g.Alm.QuitarDeLista(g.BotID, ListaQuierenEntrar, persona)
			}
		}})
	if err != nil {
		return err
	}
	if res != EnCola {
		return errors.New("no se pudo preparar la invitación (" + strings.TrimPrefix(res, ResDescartado) + ")")
	}
	return nil
}

// agregarDirecto mete a la persona al grupo por la cola (con su tope y espera propios). Si su privacidad no lo deja, cae a la invitación.
func (g *Gestor) agregarDirecto(grupoChat, persona string) error {
	cfg := g.Config(IDInvitar)
	if !g.Activo(IDInvitar) || g.Acc == nil {
		return errors.New("el módulo está apagado o no hay conexión con WhatsApp")
	}
	if !cfg.Bool("directo") {
		return errors.New("agregar directo está apagado: enciéndelo en los ajustes del módulo")
	}
	if !strings.HasSuffix(grupoChat, "@g.us") {
		return errors.New("elige un grupo")
	}
	if !g.Alm.EstaEnLista(g.BotID, ListaQuierenEntrar, persona) {
		return errors.New("solo se agrega directo a quien está en «Quieren entrar» (ella lo pidió)")
	}
	if g.Alm.EstaNoMolestar(g.BotID, persona) {
		return errors.New("esa persona pidió no recibir mensajes")
	}
	hechos := g.Alm.ContarRegistroModulo(g.BotID, IDInvitar, qAgregar, ResEnviado, g.ahora().Add(-24*time.Hour))
	g.h.mu.Lock()
	cola := g.h.agregarCola
	if hechos+cola >= cfg.Int("directo_tope_dia") {
		g.h.mu.Unlock()
		return fmt.Errorf("ya llegaste al tope de %d personas agregadas por día", cfg.Int("directo_tope_dia"))
	}
	g.h.agregarCola++
	g.h.mu.Unlock()
	libera := func() { g.h.mu.Lock(); g.h.agregarCola--; g.h.mu.Unlock() }
	res, err := g.Enviar(Iniciativa{Modulo: IDInvitar, Que: qAgregar, Chat: grupoChat, Personas: []string{persona}, Noche: NocheDescartar,
		RetrasoMin: segundos(cfg.Num("directo_espera_min")), RetrasoMax: segundos(cfg.Num("directo_espera_max")),
		AlResultado: func(string) { libera() },
		Accion: func(ctx context.Context) error {
			r, err := g.Acc.Agregar(ctx, grupoChat, persona)
			if err != nil {
				return err
			}
			switch r {
			case AgregOK, AgregYaEsta:
				_ = g.Alm.QuitarDeLista(g.BotID, ListaQuierenEntrar, persona)
			case AgregPrivacidad:
				// WhatsApp no la dejó agregar (falla en silencio): se cae a la invitación con enlace
				g.Alm.RegistrarModulo(g.BotID, IDInvitar, "agregar_privacidad", "respaldo_invitacion", grupoChat, g.ahora())
				if err := g.invitarA(ctx, grupoChat, persona); err != nil {
					return ErrOmitida
				}
			}
			return nil
		}})
	if err != nil {
		libera()
		return err
	}
	if res != EnCola {
		return errors.New("no se pudo preparar (" + strings.TrimPrefix(res, ResDescartado) + ")")
	}
	return nil
}

// ---------- paneles ----------

func (g *Gestor) opcionesQuieren() []Opcion {
	out := []Opcion{{Valor: "__todos__", Titulo: "Todos los de la lista"}}
	ms, _ := g.Alm.MiembrosLista(g.BotID, ListaQuierenEntrar, 200)
	for _, p := range ms {
		out = append(out, Opcion{Valor: p, Titulo: g.nombreChat(p)})
	}
	return out
}

func panelesInvitar(g *Gestor) []Panel {
	return []Panel{
		{
			ID: "invitar", Titulo: "Quieren entrar",
			Ayuda: "Quienes escriben la frase de arriba (o los que anotes a mano) aparecen aquí. «Enviar invitación» les manda el enlace del grupo (necesitas ser administrador del grupo). «Agregar directo» solo funciona si lo permites en los ajustes y mete a la persona sin que acepte.",
			Campos: []Campo{
				{Clave: "grupo", Titulo: "Grupo", Tipo: CampoLista, Opciones: g.opcionesChats(true, 200)},
				{Clave: "persona", Titulo: "Persona", Tipo: CampoLista, Opciones: g.opcionesQuieren()},
			},
			Alta: "Enviar invitación",
			Extras: []Boton{
				{Accion: "invitar.directo", Titulo: "Agregar directo", Confirma: "¿Agregar a esta persona directo al grupo? Si su privacidad no lo permite se le manda la invitación."},
				{Accion: "invitar.quitar", Titulo: "Quitar de la lista"},
			},
		},
		{
			ID: "anotar", Titulo: "Anotar a mano",
			Ayuda:  "Solo personas que ya te escribieron.",
			Campos: []Campo{{Clave: "persona", Titulo: "Persona", Tipo: CampoLista, Opciones: g.opcionesChats(false, 200)}},
			Alta:   "Anotar como «quiere entrar»",
		},
	}
}

func filasInvitar(g *Gestor, panel string) []Fila {
	if panel != "invitar" {
		return nil
	}
	var out []Fila
	ms, _ := g.Alm.MiembrosLista(g.BotID, ListaQuierenEntrar, 200)
	for _, p := range ms {
		out = append(out, Fila{ID: p, Titulo: g.nombreChat(p), Detalle: "Quiere entrar"})
	}
	return out
}

func accionInvitar(g *Gestor, accion string, a Args) (string, error) {
	personasDe := func() ([]string, error) {
		p := a.Texto("persona")
		if p == "" {
			return nil, errors.New("elige a la persona")
		}
		if p == "__todos__" {
			ms, _ := g.Alm.MiembrosLista(g.BotID, ListaQuierenEntrar, 200)
			if len(ms) == 0 {
				return nil, errors.New("la lista está vacía")
			}
			return ms, nil
		}
		return []string{p}, nil
	}
	switch accion {
	case "anotar.agregar":
		p := a.Texto("persona")
		if p == "" || !g.hayChat(p) {
			return "", errors.New("elige a una persona que ya te escribió")
		}
		if _, err := g.Alm.AgregarALista(g.BotID, ListaQuierenEntrar, p, "a mano", g.ahora()); err != nil {
			return "", err
		}
		return "Anotada.", nil
	case "invitar.quitar":
		ps, err := personasDe()
		if err != nil {
			return "", err
		}
		for _, p := range ps {
			_ = g.Alm.QuitarDeLista(g.BotID, ListaQuierenEntrar, p)
		}
		return "Quitada de la lista.", nil
	case "invitar.agregar", "invitar.directo":
		grupo := a.Texto("grupo")
		if !strings.HasSuffix(grupo, "@g.us") {
			return "", errors.New("elige el grupo")
		}
		ps, err := personasDe()
		if err != nil {
			return "", err
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		ok, fallos := 0, 0
		var ultimo error
		for _, p := range ps {
			var e error
			if accion == "invitar.directo" {
				e = g.agregarDirecto(grupo, p)
			} else {
				e = g.invitarA(ctx, grupo, p)
			}
			if e != nil {
				fallos++
				ultimo = e
			} else {
				ok++
			}
		}
		g.Alm.RegistrarAccion(g.BotID, almacen.PersonaDueno, "modulo."+accion, "", grupo, map[bool]string{true: "ok", false: "error"}[ok > 0], "")
		if ok == 0 {
			return "", ultimo
		}
		verbo := "invitaciones en camino"
		if accion == "invitar.directo" {
			verbo = "personas por agregar (salen con esperas de 30 a 90 s)"
		}
		msg := fmt.Sprintf("%d %s.", ok, verbo)
		if fallos > 0 {
			msg += fmt.Sprintf(" %d no se pudieron preparar: %v.", fallos, ultimo)
		}
		if !g.Maestro() {
			msg += " OJO: «Iniciativa propia» está apagada y no saldrá nada hasta que la enciendas."
		}
		return msg, nil
	}
	return "", errors.New("esa acción no existe")
}
