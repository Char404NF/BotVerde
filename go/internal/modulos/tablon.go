package modulos

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"botchat/internal/almacen"
	"botchat/internal/comandos"
	"botchat/internal/permisos"
)

// ---------- Tablón de anuncios ----------
//
// Un mensaje fijo por grupo con lo importante: un texto tuyo, las reglas, el próximo evento, el sorteo en curso y la votación abierta.
// WhatsApp solo deja editar un mensaje durante 15 minutos, así que «actualizar» significa: mandar el tablón nuevo, fijarlo (reemplaza
// al anterior) y borrar el viejo. Solo se vuelve a publicar si cambió algo y pasó al menos una hora desde la última vez.

// IDTablon es el id del módulo.
const IDTablon = "tablon"

const qTablon = "tablon"

type tablonDatos struct {
	Grupo    string `json:"grupo"`
	Titulo   string `json:"titulo"`
	Texto    string `json:"texto"`
	Reglas   bool   `json:"reglas"`
	Sorteo   bool   `json:"sorteo"`
	Votacion bool   `json:"votacion"`
	Evento   string `json:"evento"`
	EventoTS int64  `json:"eventoTs"`
	Dias     int    `json:"dias"`
	Auto     bool   `json:"auto"`
	MsgID    string `json:"msgId"`
	Hash     string `json:"hash"`
	Ultimo   int64  `json:"ultimo"`
	Ocupado  bool   `json:"ocupado"`
}

func tablon() Modulo {
	return Nuevo(Def{
		ID: IDTablon, Nombre: "Tablón de anuncios", Icono: "difusion", Riesgo: RiesgoMedio, Iniciativa: true,
		Descripcion: "Un mensaje fijo por grupo con tu anuncio, las reglas, el próximo evento, el sorteo en curso y la votación abierta. El bot lo vuelve a publicar solo cuando algo cambia (como mucho una vez por hora).",
		Casillas: []permisos.Casilla{
			{ID: "tablon.ver", Nombre: "Ver el tablón", Descripcion: "Pide con «!tablon» el texto del tablón del grupo.", Grupo: "Tablón", Riesgo: permisos.RiesgoBajo, Grupos: true, Base: true},
		},
		Esquema: []Campo{
			{Clave: "minimo_min", Titulo: "Minutos mínimos entre publicaciones", Ayuda: "Evita llenar el grupo si algo cambia seguido.", Tipo: CampoNumero, Defecto: 60.0, Min: 10, Max: 1440},
		},
		TopeHora: 6, TopeDia: 20,
		AlEvento: func(g *Gestor, e Evento) {
			if e.Tipo == EvTick {
				tablonTick(g)
			}
		},
		AlEstado: func(g *Gestor) string {
			n := 0
			fs, _ := g.Alm.ListarAgenda(g.BotID, IDTablon)
			for _, f := range fs {
				if f.Estado == pgActivo {
					n++
				}
			}
			if n == 0 {
				return ""
			}
			return fmt.Sprintf("Encendido · %d tablones", n)
		},
		ComandosG: comandosTablon,
		Paneles:   panelesTablon,
		AlFilas:   filasTablon,
		AlAccion:  accionTablon,
	})
}

// textoTablon arma el texto del tablón con lo que hay ahora.
func (g *Gestor) textoTablon(d *tablonDatos) string {
	var b strings.Builder
	t := strings.TrimSpace(d.Titulo)
	if t == "" {
		t = "Tablón del grupo"
	}
	b.WriteString("📌 *" + t + "*")
	if x := strings.TrimSpace(d.Texto); x != "" {
		b.WriteString("\n\n" + x)
	}
	if strings.TrimSpace(d.Evento) != "" {
		b.WriteString("\n\n📅 *Próximo evento:* " + strings.TrimSpace(d.Evento))
		if d.EventoTS > g.ahora().Unix() {
			b.WriteString(" — " + g.FechaTexto(d.EventoTS))
		}
	}
	if d.Sorteo {
		fs, _ := g.Alm.AgendaConEstado(g.BotID, IDSorteos, pgActivo)
		for _, f := range fs {
			var s sorteoDatos
			if json.Unmarshal([]byte(f.Contenido), &s) == nil && s.Grupo == d.Grupo {
				b.WriteString("\n\n🎁 *Sorteo:* " + f.Nombre + " — premio: " + s.Premio)
				break
			}
		}
	}
	if d.Votacion {
		fs, _ := g.Alm.AgendaConEstado(g.BotID, IDVotaciones, pgActivo)
		for _, f := range fs {
			var v votacionDatos
			if json.Unmarshal([]byte(f.Contenido), &v) == nil && v.Grupo == d.Grupo && v.Fase == votAbierta {
				b.WriteString("\n\n🗳️ *Votación abierta:* " + v.Pregunta)
				break
			}
		}
	}
	if d.Reglas {
		if r := strings.TrimSpace(g.Alm.ReglasGrupo(g.BotID, d.Grupo)); r != "" {
			b.WriteString("\n\n📜 *Reglas:*\n" + r)
		}
	}
	return b.String()
}

func hashTexto(t string) string {
	h := sha256.Sum256([]byte(t))
	return hex.EncodeToString(h[:8])
}

func tablonTick(g *Gestor) {
	if !g.Maestro() || g.EnModoSeguro() {
		return
	}
	minimo := int64(g.Config(IDTablon).Int("minimo_min")) * 60
	ahora := g.ahora().Unix()
	fs, _ := g.Alm.AgendaConEstado(g.BotID, IDTablon, pgActivo)
	for _, f := range fs {
		var d tablonDatos
		if json.Unmarshal([]byte(f.Contenido), &d) != nil || !d.Auto || d.Ocupado {
			continue
		}
		if d.Ultimo > 0 && ahora-d.Ultimo < minimo {
			continue
		}
		if hashTexto(g.textoTablon(&d)) == d.Hash && d.MsgID != "" {
			continue
		}
		g.tablonPublicar(f, &d)
	}
}

// tablonPublicar encola la publicación: manda el tablón nuevo, lo fija y borra el anterior.
func (g *Gestor) tablonPublicar(f almacen.AgendaFila, d *tablonDatos) {
	if g.Acc == nil {
		return
	}
	acc := g.Acc
	texto := g.textoTablon(d)
	d.Ocupado = true
	g.tablonGuarda(f, d)
	fila := f
	datos := *d
	ini := Iniciativa{Modulo: IDTablon, Que: qTablon, Chat: d.Grupo, Noche: NocheDescartar, EsperaMax: 30 * time.Minute,
		Accion: func(ctx context.Context) error {
			id, err := acc.EnviarTextoID(ctx, datos.Grupo, texto, nil)
			if err != nil {
				return err
			}
			viejo := datos.MsgID
			datos.MsgID, datos.Hash, datos.Ultimo, datos.Ocupado = id, hashTexto(texto), g.ahora().Unix(), false
			g.tablonGuarda(fila, &datos)
			dias := datos.Dias
			if dias != 1 && dias != 7 && dias != 30 {
				dias = 30
			}
			if err := acc.FijarMensaje(ctx, datos.Grupo, id, dias, true); err != nil && g.Aviso != nil {
				g.Aviso("Tablón sin fijar", "«"+fila.Nombre+"»: lo publiqué pero no pude fijarlo ("+err.Error()+"). Hace falta que el bot sea administrador.")
			}
			if viejo != "" {
				_ = acc.BorrarMensaje(ctx, datos.Grupo, viejo) // si ya no se puede borrar, queda como mensaje normal
			}
			return nil
		},
		AlResultado: func(r string) {
			if r == ResEnviado {
				return
			}
			datos.Ocupado = false
			datos.Ultimo = g.ahora().Unix() // no se reintenta cada minuto: se vuelve a intentar pasado el tiempo mínimo
			g.tablonGuarda(fila, &datos)
		}}
	if _, err := g.Enviar(ini); err != nil {
		d.Ocupado = false
		g.tablonGuarda(f, d)
	}
}

func (g *Gestor) tablonGuarda(f almacen.AgendaFila, d *tablonDatos) {
	_ = g.Alm.ActualizarContenidoAgenda(g.BotID, f.ID, mustJSON(d))
}

func comandosTablon(g *Gestor) []comandos.Comando {
	return []comandos.Comando{{
		Nombre: "tablon", Resumen: "Ver el tablón de anuncios del grupo", Uso: "tablon", Casilla: "tablon.ver", Ambito: comandos.Grupo,
		Ejecutar: func(cx *comandos.Contexto) (string, error) {
			fs, _ := g.Alm.AgendaConEstado(g.BotID, IDTablon, pgActivo)
			for _, f := range fs {
				var d tablonDatos
				if json.Unmarshal([]byte(f.Contenido), &d) == nil && d.Grupo == cx.Chat {
					return g.textoTablon(&d), nil
				}
			}
			return "Este grupo no tiene tablón de anuncios.", nil
		}}}
}

func panelesTablon(g *Gestor) []Panel {
	return []Panel{{
		ID: "tablon", Titulo: "Tablones",
		Ayuda: "Un tablón por grupo. Escribe tu anuncio y elige qué más lleva. Al publicar, el bot lo manda, lo fija y borra el anterior. Con «Actualizar solo» lo vuelve a publicar cuando algo cambia (como mucho una vez por hora).",
		Campos: []Campo{
			{Clave: "grupo", Titulo: "Grupo", Tipo: CampoLista, Opciones: g.opcionesChats(true, 100)},
			{Clave: "titulo", Titulo: "Título", Tipo: CampoTexto, Largo: 60, Defecto: "Tablón del grupo"},
			{Clave: "texto", Titulo: "Anuncio", Tipo: CampoTexto, Largo: 1200, Varias: true},
			{Clave: "evento", Titulo: "Próximo evento (opcional)", Tipo: CampoTexto, Largo: 120},
			{Clave: "fecha_evento", Titulo: "Fecha y hora del evento (opcional)", Ayuda: "Por ejemplo 25/12/2026 18:30", Tipo: CampoTexto, Largo: 20},
			{Clave: "reglas", Titulo: "Incluir las reglas del grupo", Tipo: CampoInterruptor, Defecto: true},
			{Clave: "sorteo", Titulo: "Incluir el sorteo en curso", Tipo: CampoInterruptor, Defecto: true},
			{Clave: "votacion", Titulo: "Incluir la votación abierta", Tipo: CampoInterruptor, Defecto: true},
			{Clave: "dias", Titulo: "Días fijado", Tipo: CampoLista, Defecto: "30", Opciones: []Opcion{{"1", "24 horas"}, {"7", "7 días"}, {"30", "30 días"}}},
			{Clave: "auto", Titulo: "Actualizar solo cuando algo cambie", Tipo: CampoInterruptor, Defecto: true},
		},
		Alta:    "Crear tablón",
		Extras:  []Boton{{Accion: "tablon.vista", Titulo: "Vista previa"}},
		PorFila: []Boton{{Accion: "tablon.publicar", Titulo: "Publicar ahora"}, {Accion: "tablon.quitar", Titulo: "Quitar del grupo", Confirma: "Se borra el tablón del grupo y se deja de actualizar. ¿Seguir?"}},
	}}
}

func filasTablon(g *Gestor, panel string) []Fila {
	var out []Fila
	fs, _ := g.Alm.ListarAgenda(g.BotID, IDTablon)
	for _, f := range fs {
		var d tablonDatos
		_ = json.Unmarshal([]byte(f.Contenido), &d)
		det := g.nombreChat(d.Grupo) + " · "
		switch {
		case f.Estado != pgActivo:
			det += "quitado"
		case d.MsgID == "":
			det += "sin publicar"
		default:
			det += "publicado " + g.FechaTexto(d.Ultimo)
		}
		out = append(out, Fila{ID: fmt.Sprint(f.ID), Titulo: f.Nombre, Detalle: det})
	}
	return out
}

func tablonDeArgs(g *Gestor, a Args) (*tablonDatos, error) {
	grupo := a.Texto("grupo")
	if !strings.HasSuffix(grupo, "@g.us") || !g.hayChat(grupo) {
		return nil, errors.New("elige el grupo del tablón")
	}
	d := &tablonDatos{Grupo: grupo, Titulo: strings.TrimSpace(a.Texto("titulo")), Texto: strings.TrimSpace(valorTexto(a, "texto")), Evento: strings.TrimSpace(a.Texto("evento")),
		Reglas: a.Bool("reglas"), Sorteo: a.Bool("sorteo"), Votacion: a.Bool("votacion"), Auto: a.Bool("auto"), Dias: 30}
	switch a.Texto("dias") {
	case "1":
		d.Dias = 1
	case "7":
		d.Dias = 7
	}
	if t := strings.TrimSpace(a.Texto("fecha_evento")); t != "" {
		f, err := g.ParseFechaHora(t)
		if err != nil {
			return nil, err
		}
		d.EventoTS = f.Unix()
	}
	return d, nil
}

func accionTablon(g *Gestor, accion string, a Args) (string, error) {
	switch accion {
	case "tablon.vista":
		d, err := tablonDeArgs(g, a)
		if err != nil {
			return "", err
		}
		return g.textoTablon(d), nil
	case "tablon.agregar":
		d, err := tablonDeArgs(g, a)
		if err != nil {
			return "", err
		}
		if d.Texto == "" && d.Evento == "" && !d.Reglas && !d.Sorteo && !d.Votacion {
			return "", errors.New("el tablón está vacío: escribe un anuncio o elige qué incluir")
		}
		fs, _ := g.Alm.ListarAgenda(g.BotID, IDTablon)
		for _, f := range fs {
			var o tablonDatos
			if f.Estado == pgActivo && json.Unmarshal([]byte(f.Contenido), &o) == nil && o.Grupo == d.Grupo {
				return "", errors.New("ese grupo ya tiene un tablón: quítalo o publícalo de nuevo")
			}
		}
		id, err := g.Alm.CrearAgenda(g.BotID, almacen.AgendaFila{Modulo: IDTablon, Nombre: g.nombreChat(d.Grupo), Destino: d.Grupo, Contenido: mustJSON(d), Estado: pgActivo}, g.ahora())
		if err != nil {
			return "", err
		}
		g.Alm.RegistrarAccion(g.BotID, almacen.PersonaDueno, "tablon.crear", "", d.Grupo, "ok", "")
		if g.Maestro() {
			if f, _ := g.Alm.ObtenerAgenda(g.BotID, id); f != nil {
				g.tablonPublicar(*f, d)
				return "Tablón creado: se publica en el grupo ahora.", nil
			}
		}
		return "Tablón creado. Enciende «Iniciativa propia» para que se publique.", nil
	case "tablon.publicar", "tablon.quitar":
		f, err := g.Alm.ObtenerAgenda(g.BotID, idArg(a))
		if err != nil || f == nil || f.Modulo != IDTablon {
			return "", errors.New("ese tablón ya no existe")
		}
		var d tablonDatos
		if json.Unmarshal([]byte(f.Contenido), &d) != nil {
			return "", errors.New("ese tablón está dañado")
		}
		if accion == "tablon.publicar" {
			if !g.Maestro() {
				return "", errors.New("«Iniciativa propia» está apagada")
			}
			d.Ocupado = false
			g.tablonPublicar(*f, &d)
			return "Publicando el tablón en el grupo.", nil
		}
		if d.MsgID != "" && g.Acc != nil {
			acc, chat, id := g.Acc, d.Grupo, d.MsgID
			_, _ = g.Enviar(Iniciativa{Modulo: IDTablon, Que: qTablon, Chat: chat, NoEscribe: true, Accion: func(ctx context.Context) error {
				_ = acc.FijarMensaje(ctx, chat, id, 0, false)
				return acc.BorrarMensaje(ctx, chat, id)
			}})
		}
		return "Tablón quitado.", g.Alm.BorrarAgenda(g.BotID, f.ID)
	}
	return "", errors.New("esa acción no existe")
}
