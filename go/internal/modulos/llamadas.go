package modulos

import (
	"context"
	"errors"
	"fmt"
	"time"

	"botchat/internal/almacen"
)

// ---------- H8 · Llamadas ----------
//
// Detecta llamadas entrantes (1 a 1) y, según el ajuste: ignorarlas, avisarme, rechazarlas automáticamente (whatsmeow SÍ tiene
// Client.RejectCall) o rechazarlas y responder un mensaje (una vez por persona por periodo). Queda un registro de llamadas perdidas.
// HACER llamadas NO está soportado: la librería no tiene nada para iniciarlas ni contestarlas, así que no se ofrece.
// Todo lo que hace pasa por la cola de iniciativas (interruptor general, modo seguro, topes y registro).

// IDLlamadas es el id del módulo.
const IDLlamadas = "llamadas"

const textoSinLlamadas = "Este módulo NO hace ni contesta llamadas: WhatsApp no lo permite con esta librería. Solo las detecta, las rechaza o te avisa."

func llamadas() Modulo {
	return Nuevo(Def{
		ID: IDLlamadas, Nombre: "Llamadas", Icono: "llamada", Riesgo: RiesgoBajo, Iniciativa: true,
		Descripcion: "Cuando alguien te llama por WhatsApp puede ignorar la llamada, avisarte, rechazarla sola o rechazarla y contestar un mensaje (una vez por persona cada cierto tiempo). Guarda el registro de llamadas perdidas. " + textoSinLlamadas,
		Esquema: []Campo{
			{Clave: "modo", Titulo: "Qué hacer con una llamada", Tipo: CampoLista, Defecto: "avisar",
				Opciones: []Opcion{{"ignorar", "Ignorar (solo anotar)"}, {"avisar", "Solo avisarme"}, {"rechazar", "Rechazar automáticamente"}, {"rechazar_responder", "Rechazar y responder un mensaje"}}},
			{Clave: "avisar", Titulo: "Avisarme con una notificación en cualquier modo", Tipo: CampoInterruptor, Defecto: true},
			{Clave: "mensaje", Titulo: "Mensaje de respuesta", Ayuda: "Solo en «Rechazar y responder».", Tipo: CampoTexto, Varias: true, Largo: 500, Defecto: "Hola, no puedo contestar llamadas por WhatsApp. Escríbeme un mensaje y te respondo en cuanto pueda.", Obligatorio: true},
			{Clave: "periodo_horas", Titulo: "No repetir la respuesta a la misma persona antes de (horas)", Tipo: CampoNumero, Defecto: 24.0, Min: 1, Max: 720},
			{Clave: "desconocidos", Titulo: "También responder a quien nunca me ha escrito", Ayuda: "Apagado por defecto: escribirle a alguien que no te ha escrito es lo que más expone tu cuenta. Apagado, solo se rechaza.", Tipo: CampoInterruptor, Defecto: false},
			{Clave: "retraso_min", Titulo: "Respuesta · espera mínima (segundos)", Tipo: CampoNumero, Defecto: 3.0, Min: 1, Max: 300},
			{Clave: "retraso_max", Titulo: "Respuesta · espera máxima (segundos)", Tipo: CampoNumero, Defecto: 10.0, Min: 1, Max: 600},
		},
		TopeHora: 30, TopeDia: 200,
		Validar: func(c map[string]any) error {
			cfg := Config(c)
			if cfg.Num("retraso_max") < cfg.Num("retraso_min") {
				return errors.New("la espera máxima no puede ser menor que la mínima")
			}
			return nil
		},
		AlEvento: eventoLlamadas,
		AlEstado: func(g *Gestor) string {
			n := len(g.llamadasDesde(g.ahora().Add(-24 * time.Hour)))
			if n == 0 {
				return ""
			}
			return fmt.Sprintf("Encendido · %d llamadas en 24 h", n)
		},
		Paneles: func(g *Gestor) []Panel {
			return []Panel{{ID: "perdidas", Titulo: "Llamadas perdidas", Ayuda: "Las últimas llamadas que llegaron mientras el módulo estaba encendido. " + textoSinLlamadas}}
		},
		AlFilas: func(g *Gestor, panel string) []Fila {
			var out []Fila
			rs := g.llamadasDesde(time.Time{})
			if len(rs) > 40 {
				rs = rs[:40]
			}
			for _, r := range rs {
				tipo := "Llamada"
				if r.Que == "videollamada" {
					tipo = "Videollamada"
				}
				out = append(out, Fila{ID: fmt.Sprint(r.ID), Titulo: tipo + " de " + g.nombreChat(r.Chat), Detalle: g.FechaTexto(r.TS) + " · " + textoModoLlamada(r.Resultado)})
			}
			return out
		},
	})
}

func textoModoLlamada(r string) string {
	switch r {
	case "ignorada":
		return "ignorada"
	case "avisada":
		return "te avisé"
	case "rechazar":
		return "rechazada"
	case "rechazar_responder":
		return "rechazada y respondida"
	}
	return r
}

// llamadasDesde devuelve el registro de llamadas (lo más nuevo primero) desde `desde` (cero = todo lo que hay).
func (g *Gestor) llamadasDesde(desde time.Time) []almacen.RegistroModulo {
	rs, _ := g.Alm.ListarRegistroModulos(g.BotID, IDLlamadas, 200, 0)
	var out []almacen.RegistroModulo
	for _, r := range rs {
		if (r.Que == "llamada" || r.Que == "videollamada") && (desde.IsZero() || r.TS >= desde.Unix()) {
			out = append(out, r)
		}
	}
	return out
}

func eventoLlamadas(g *Gestor, e Evento) {
	if e.Tipo != EvLlamada || e.Persona == "" {
		return
	}
	cfg := g.Config(IDLlamadas)
	persona := g.Alm.ResolverPersona(g.BotID, e.Persona)
	que := "llamada"
	if e.LlamadaVideo {
		que = "videollamada"
	}
	modo := cfg.Texto("modo")
	g.Alm.RegistrarModulo(g.BotID, IDLlamadas, que, modo, persona, g.ahora())
	if modo == "avisar" || cfg.Bool("avisar") {
		if g.Aviso != nil {
			quien := g.nombreDe(persona)
			if quien == "" {
				quien = enmascarar(persona)
			}
			g.Aviso("Llamada perdida", "Te llamó "+quien+" por WhatsApp"+map[bool]string{true: " (video)", false: ""}[e.LlamadaVideo]+".")
		}
	}
	if modo != "rechazar" && modo != "rechazar_responder" {
		return
	}
	// rechazar: no le escribe a nadie, pero sigue las reglas de la cola (interruptor general, modo seguro, topes, registro)
	de, id := e.LlamadaDe, e.LlamadaID
	if g.Acc == nil || id == "" || de == "" {
		g.Alm.RegistrarModulo(g.BotID, IDLlamadas, "rechazar_llamada", ResFallido, persona, g.ahora())
		return
	}
	_, _ = g.Enviar(Iniciativa{Modulo: IDLlamadas, Que: "rechazar_llamada", Chat: persona, Personas: []string{persona}, NoEscribe: true,
		RetrasoMin: 500 * time.Millisecond, RetrasoMax: 2 * time.Second, EsperaMax: 20 * time.Second,
		Accion: func(ctx context.Context) error { return g.Acc.RechazarLlamada(ctx, de, id) }})
	if modo != "rechazar_responder" {
		return
	}
	periodo := time.Duration(cfg.Int("periodo_horas")) * time.Hour
	if ts := g.Alm.UltimoRegistroModulo(g.BotID, IDLlamadas, "aviso_llamada", persona); ts > 0 && g.ahora().Sub(time.Unix(ts, 0)) < periodo {
		return
	}
	if !g.hayChat(persona) {
		if !cfg.Bool("desconocidos") {
			g.Alm.RegistrarModulo(g.BotID, IDLlamadas, "aviso_llamada_omitido", "desconocido", persona, g.ahora())
			return
		}
		_, _ = g.Alm.ActualizarChat(g.BotID, persona, false, func(c *almacen.Chat) {})
	}
	if !g.limite("llamada:"+persona, 1, periodo) {
		return
	}
	_, _ = g.Enviar(Iniciativa{Modulo: IDLlamadas, Que: "aviso_llamada", Chat: persona, Personas: []string{persona}, Texto: cfg.Texto("mensaje"), Noche: NocheDescartar,
		RetrasoMin: segundos(cfg.Num("retraso_min")), RetrasoMax: segundos(cfg.Num("retraso_max"))})
}
