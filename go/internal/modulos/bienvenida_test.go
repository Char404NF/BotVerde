package modulos

import (
	"strings"
	"testing"
	"time"

	"botchat/internal/almacen"
)

const (
	grupoB   = "120363000000000002@g.us"
	nuevoJID = "5215533333333@s.whatsapp.net"
)

func bienvenidaLista(t *testing.T) (*ent, *accFalsa) {
	e, acc := entH(t)
	e.conChat(grupo, true, "Vecinos")
	e.conChat(grupoB, true, "Otro")
	e.activa(IDBienvenida)
	if _, err := e.g.AccionModulo(IDBienvenida, "grupos.agregar", Args{"chat": grupo}); err != nil {
		t.Fatal(err)
	}
	return e, acc
}

func entra(e *ent, chat string, personas ...string) {
	e.g.Evento(Evento{Tipo: EvMiembroEntra, Chat: chat, EsGrupo: true, Personas: personas, JIDs: personas})
}

func TestBienvenidaApagadaPorDefecto(t *testing.T) {
	e, _ := entH(t)
	e.fija("maestro", "1")
	entra(e, grupo, nuevoJID)
	e.avanza(time.Minute)
	if e.cuantos() != 0 {
		t.Fatal("un módulo apagado no debe escribir")
	}
	i, _ := e.g.InfoDe(IDBienvenida)
	if i.Activo || i.Riesgo != RiesgoMedio || !i.Iniciativa || len(i.Esquema) == 0 || len(i.Paneles) == 0 {
		t.Fatalf("info: %+v", i)
	}
}

func (e *ent) cuantos() int { return e.env.cuantos() }

func TestBienvenidaEnElGrupoConMencionYReglas(t *testing.T) {
	e, _ := bienvenidaLista(t)
	if err := e.alm.FijarReglasGrupo(1, grupo, "1. Respeto", e.hora); err != nil {
		t.Fatal(err)
	}
	entra(e, grupo, nuevoJID)
	if e.cuantos() != 0 {
		t.Fatal("antes de la espera al azar no debe salir")
	}
	e.avanza(time.Minute)
	if len(e.enviados()) != 1 {
		t.Fatalf("enviados: %v", e.enviados())
	}
	s := e.enviados()[0]
	if !strings.HasPrefix(s, grupo+"|") || !strings.Contains(s, "@5215533333333") || !strings.Contains(s, "*Vecinos*") || !strings.Contains(s, "1. Respeto") {
		t.Fatalf("mensaje: %q", s)
	}
	// el registro dice qué pasó pero nunca el texto
	for _, r := range e.registro() {
		if strings.Contains(r.Que+r.Resultado+r.Chat, "Respeto") {
			t.Fatal("el registro guardó texto")
		}
	}
}

func TestBienvenidaGruposElegidos(t *testing.T) {
	e, _ := bienvenidaLista(t)
	entra(e, grupoB, nuevoJID) // no elegido
	e.avanza(time.Minute)
	if e.cuantos() != 0 {
		t.Fatal("un grupo no elegido no recibe bienvenida")
	}
	e.config(IDBienvenida, map[string]any{"grupos_modo": "todos"})
	entra(e, grupoB, nuevoJID)
	e.avanza(time.Minute)
	if e.cuantos() != 1 {
		t.Fatal("con «todos» sí")
	}
	e.config(IDBienvenida, map[string]any{"grupos_modo": "todos_menos"})
	entra(e, grupo, nuevoJID) // elegido = excluido
	e.avanza(time.Minute)
	if e.cuantos() != 1 {
		t.Fatal("con «todos menos» el elegido queda fuera")
	}
	if _, err := e.g.AccionModulo(IDBienvenida, "grupos.agregar", Args{"chat": "no-es-grupo"}); err == nil {
		t.Fatal("solo grupos que existen")
	}
	if _, err := e.g.AccionModulo(IDBienvenida, "grupos.quitar", Args{"id": grupo}); err != nil {
		t.Fatal(err)
	}
	if f, _ := e.g.FilasPanel(IDBienvenida, "grupos"); len(f) != 0 {
		t.Fatal("el grupo debió salir de la lista")
	}
}

func TestBienvenidaRespetaMaestroModoSeguroYNoMolestar(t *testing.T) {
	e, _ := bienvenidaLista(t)
	_ = e.g.AgregarNoMolestar(nuevoJID, "a mano")
	entra(e, grupo, nuevoJID)
	e.avanza(time.Minute)
	if e.cuantos() != 0 {
		t.Fatal("No molestar manda: ni en el grupo")
	}
	_ = e.g.QuitarNoMolestar(nuevoJID)
	e.fija("maestro", "0")
	entra(e, grupo, nuevoJID)
	e.avanza(time.Minute)
	if e.cuantos() != 0 {
		t.Fatal("con el maestro apagado no sale nada")
	}
	e.fija("maestro", "1")
	e.g.ModoSeguro("prueba")
	entra(e, grupo, nuevoJID)
	e.avanza(time.Minute)
	if e.cuantos() != 0 {
		t.Fatal("en modo seguro no sale nada")
	}
}

func TestBienvenidaVerificacionPorReglas(t *testing.T) {
	e, acc := bienvenidaLista(t)
	e.config(IDBienvenida, map[string]any{"verificar": true, "expulsar": true, "verif_minutos": 5.0})
	otro := "5215544444444@s.whatsapp.net"
	entra(e, grupo, nuevoJID, otro)
	e.avanza(time.Minute)
	if !strings.Contains(e.enviados()[0], "responde *1*") {
		t.Fatalf("falta la instrucción: %v", e.enviados())
	}
	if n := len(e.alm.PendientesDe(1, IDBienvenida, grupo)); n != 2 {
		t.Fatalf("pendientes: %d", n)
	}
	// uno responde bien, la otra no
	e.g.Evento(Evento{Tipo: EvMensaje, Chat: grupo, EsGrupo: true, Persona: nuevoJID, Texto: "Acepto"})
	e.g.Evento(Evento{Tipo: EvMensaje, Chat: grupo, EsGrupo: true, Persona: otro, Texto: "hola"})
	if n := len(e.alm.PendientesDe(1, IDBienvenida, grupo)); n != 1 {
		t.Fatalf("debía quedar uno: %d", n)
	}
	e.avanza(2 * time.Minute)
	e.tick()
	e.avanza(time.Minute)
	if len(acc.expulsados) != 0 {
		t.Fatal("todavía no vence")
	}
	e.hora = e.hora.Add(10 * time.Minute)
	e.tick()
	e.avanza(time.Minute)
	if len(acc.expulsados) != 1 || acc.expulsados[0] != grupo+"|"+otro {
		t.Fatalf("expulsados: %v", acc.expulsados)
	}
	if len(e.alm.PendientesDe(1, IDBienvenida, "")) != 0 {
		t.Fatal("no debe quedar pendiente")
	}
}

func TestBienvenidaSumaYNoAdmin(t *testing.T) {
	e, acc := bienvenidaLista(t)
	acc.admin = false
	e.config(IDBienvenida, map[string]any{"verificar": true, "expulsar": true, "verif_tipo": "suma", "verif_minutos": 1.0})
	entra(e, grupo, nuevoJID)
	e.avanza(time.Minute)
	// Aleatorio devuelve 0 → 1 + 1
	if !strings.Contains(e.enviados()[0], "*1 + 1*") {
		t.Fatalf("pregunta: %v", e.enviados())
	}
	p := e.alm.PendientesDe(1, IDBienvenida, grupo)
	if len(p) != 1 || p[0].Esperada != "2" {
		t.Fatalf("pendiente: %+v", p)
	}
	e.hora = e.hora.Add(5 * time.Minute)
	e.tick()
	e.avanza(time.Minute)
	if len(acc.expulsados) != 0 {
		t.Fatal("sin ser admin no se expulsa")
	}
	if !tiene(e.avisos, "No pude expulsar a alguien") {
		t.Fatalf("faltó el aviso: %v", e.avisos)
	}
	if !tiene(e.resultados(), "descartado:omitida") {
		t.Fatalf("registro: %v", e.resultados())
	}
	if e.g.Cola.ErroresSeguidos() != 0 {
		t.Fatal("una acción omitida no cuenta como error")
	}
	// responder la suma bien
	entra(e, grupo, nuevoJID)
	e.avanza(time.Minute)
	e.g.Evento(Evento{Tipo: EvMensaje, Chat: grupo, EsGrupo: true, Persona: nuevoJID, Texto: "es 2"})
	if len(e.alm.PendientesDe(1, IDBienvenida, "")) != 0 {
		t.Fatal("2 es la respuesta correcta")
	}
}

func TestBienvenidaExentoPorCasilla(t *testing.T) {
	e, acc := bienvenidaLista(t)
	e.config(IDBienvenida, map[string]any{"verificar": true, "expulsar": true})
	id, err := e.alm.CrearRol(1, almacen.Rol{Nombre: "Conocidos", Posicion: 5, Casillas: map[string]almacen.Alcance{"bienvenida.exento": {}}})
	if err != nil {
		t.Fatal(err)
	}
	if err := e.alm.AsignarRol(1, nuevoJID, id, 0, "yo"); err != nil {
		t.Fatal(err)
	}
	entra(e, grupo, nuevoJID)
	e.avanza(time.Minute)
	if len(e.alm.PendientesDe(1, IDBienvenida, "")) != 0 {
		t.Fatal("quien tiene la casilla no verifica")
	}
	e.hora = e.hora.Add(time.Hour)
	e.tick()
	e.avanza(time.Minute)
	if len(acc.expulsados) != 0 {
		t.Fatal("ni se le expulsa")
	}
}

func TestBienvenidaPrivadoPideAceptarYTieneTope(t *testing.T) {
	e, _ := bienvenidaLista(t)
	b, _ := jsonDe(map[string]any{"privado": true})
	esperaErr(t, e.g.GuardarConfig(IDBienvenida, b), "Entiendo el riesgo")
	e.config(IDBienvenida, map[string]any{"privado": true, "privado_acepto": true, "privado_tope_dia": 2.0})
	var jids []string
	for _, n := range []string{"5215500000001", "5215500000002", "5215500000003"} {
		jids = append(jids, n+"@s.whatsapp.net")
	}
	entra(e, grupo, jids...)
	e.avanza(10 * time.Second)
	// el del grupo sale pronto; el privado espera 30 s como mínimo
	if e.cuantos() != 1 {
		t.Fatalf("a los 10 s solo el del grupo: %v", e.enviados())
	}
	for i := 0; i < 12; i++ {
		e.avanza(40 * time.Second)
	}
	n := 0
	for _, x := range e.enviados() {
		if strings.HasPrefix(x, jids[0]) || strings.HasPrefix(x, jids[1]) || strings.HasPrefix(x, jids[2]) {
			n++
		}
	}
	if n != 2 {
		t.Fatalf("el tope diario del privado es 2: salieron %d (%v)", n, e.enviados())
	}
	if !tiene(e.resultados(), "descartado:tope") {
		t.Fatalf("el tercero debía quedar en el registro: %v", e.resultados())
	}
	if c, _ := e.alm.ObtenerChat(1, jids[0]); c == nil {
		t.Fatal("el chat del nuevo se crea para poder escribirle")
	}
}

func TestBienvenidaValidaEspera(t *testing.T) {
	e, _ := bienvenidaLista(t)
	b, _ := jsonDe(map[string]any{"retraso_min": 30.0, "retraso_max": 10.0})
	esperaErr(t, e.g.GuardarConfig(IDBienvenida, b), "espera máxima")
	b, _ = jsonDe(map[string]any{"expulsar": true, "verificar": false})
	esperaErr(t, e.g.GuardarConfig(IDBienvenida, b), "verificación")
}
