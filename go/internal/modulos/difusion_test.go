package modulos

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"botchat/internal/almacen"
)

// simula deja correr el reloj `minutos` minutos, en pasos de 10 s, con la alarma de cada minuto.
func (e *ent) simula(minutos int) {
	for i := 0; i < minutos*6; i++ {
		e.hora = e.hora.Add(10 * time.Second)
		if i%6 == 5 {
			e.tick()
		}
		e.corre()
	}
}

func difusionLista(t *testing.T, n int) (*ent, []string) {
	e, _ := entH(t)
	e.activa(IDDifusion)
	var ps []string
	for i := 0; i < n; i++ {
		p := fmt.Sprintf("52155000%05d@s.whatsapp.net", i)
		e.conChat(p, false, fmt.Sprintf("Persona %d", i))
		if _, err := e.g.AccionModulo(IDDifusion, "listas.agregar", Args{"lista": ListaSuscritos, "persona": p}); err != nil {
			t.Fatal(err)
		}
		ps = append(ps, p)
	}
	return e, ps
}

func creaAnuncio(t *testing.T, e *ent, texto, fecha string) int64 {
	t.Helper()
	m, err := e.g.AccionModulo(IDDifusion, "difusiones.agregar", Args{"nombre": "Oferta", "texto": texto, "lista": ListaSuscritos, "fecha": fecha})
	if err != nil {
		t.Fatal(err)
	}
	_ = m
	fs, _ := e.alm.ListarAgenda(1, IDDifusion)
	return fs[0].ID
}

func TestDifusionSuscribirYBaja(t *testing.T) {
	e, _ := entH(t)
	e.activa(IDDifusion)
	e.conChat(pnAna, false, "Ana")
	// en un grupo no cuenta
	e.g.Evento(Evento{Tipo: EvMensaje, Chat: grupo, EsGrupo: true, Persona: pnAna, Texto: "SUSCRIBIR"})
	if len(mustMiembros(e.g, ListaSuscritos)) != 0 {
		t.Fatal("en un grupo no se suscribe")
	}
	// frase con más palabras tampoco: mensaje completo
	e.g.Evento(Evento{Tipo: EvMensaje, Chat: pnAna, Persona: pnAna, Texto: "quiero suscribirme a todo"})
	if len(mustMiembros(e.g, ListaSuscritos)) != 0 {
		t.Fatal("la palabra va sola")
	}
	e.g.Evento(Evento{Tipo: EvMensaje, Chat: pnAna, Persona: pnAna, Texto: " ¡Suscribir! "})
	if m := mustMiembros(e.g, ListaSuscritos); len(m) != 1 || m[0] != pnAna {
		t.Fatalf("suscritos: %v", m)
	}
	e.simula(1)
	if !e.hay("quedaste suscrito") || !strings.HasPrefix(e.enviados()[0], pnAna+"|") {
		t.Fatalf("confirmación en privado: %v", e.enviados())
	}
	// ALTA también
	e.conChat(pnLuis, false, "Luis")
	e.g.Evento(Evento{Tipo: EvMensaje, Chat: pnLuis, Persona: pnLuis, Texto: "alta"})
	if len(mustMiembros(e.g, ListaSuscritos)) != 2 {
		t.Fatal("ALTA suscribe")
	}
	// BAJA: sale de las listas y entra a No molestar (lo hace el marco)
	ok, conf := e.g.Baja(pnAna, "BAJA")
	if !ok || conf() == "" {
		t.Fatal("la baja debe confirmarse una vez")
	}
	e.g.Evento(Evento{Tipo: EvMensaje, Chat: pnAna, Persona: pnAna, Texto: "BAJA"})
	if m := mustMiembros(e.g, ListaSuscritos); len(m) != 1 || m[0] != pnLuis {
		t.Fatalf("tras la baja: %v", m)
	}
	if !e.alm.EstaNoMolestar(1, pnAna) {
		t.Fatal("la baja va a No molestar")
	}
	// volver a escribir la palabra es pedirlo otra vez
	e.g.Evento(Evento{Tipo: EvMensaje, Chat: pnAna, Persona: pnAna, Texto: "suscribir"})
	if e.alm.EstaNoMolestar(1, pnAna) || len(mustMiembros(e.g, ListaSuscritos)) != 2 {
		t.Fatal("suscribirse de nuevo la saca de No molestar si fue ella quien se dio de baja")
	}
	// pero lo agregado a mano no se toca
	_ = e.g.AgregarNoMolestar(pnLuis, "a mano")
	e.g.Evento(Evento{Tipo: EvMensaje, Chat: pnLuis, Persona: pnLuis, Texto: "suscribir"})
	if !e.alm.EstaNoMolestar(1, pnLuis) {
		t.Fatal("No molestar puesto a mano se respeta")
	}
}

func TestDifusionListasSoloQuienEscribio(t *testing.T) {
	e, _ := entH(t)
	e.activa(IDDifusion)
	_, err := e.g.AccionModulo(IDDifusion, "listas.agregar", Args{"lista": "vip", "persona": pnAna})
	esperaErr(t, err, "nunca te ha escrito")
	e.conChat(pnAna, false, "Ana")
	if _, err := e.g.AccionModulo(IDDifusion, "listas.agregar", Args{"lista": "vip", "persona": pnAna}); err != nil {
		t.Fatal(err)
	}
	fs, _ := e.g.FilasPanel(IDDifusion, "listas")
	if len(fs) != 2 { // suscritos (vacía) y vip
		t.Fatalf("filas: %+v", fs)
	}
	if _, err := e.g.AccionModulo(IDDifusion, "listas.quitar_persona", Args{"lista": "vip", "persona": pnAna}); err != nil {
		t.Fatal(err)
	}
	if len(mustMiembros(e.g, "vip")) != 0 {
		t.Fatal("quitada")
	}
}

func TestDifusionEnviaPorLotesConCalentamientoYReporte(t *testing.T) {
	e, ps := difusionLista(t, 14)
	id := creaAnuncio(t, e, "Hola {nombre}, hay oferta", "")
	// el tope por día sube a 30 para que solo el calentamiento limite (10 el primer día)
	if err := e.g.FijarTopesModulo(IDDifusion, 100, 30); err != nil {
		t.Fatal(err)
	}
	e.fija("tope_hora", "100")
	e.fija("tope_dia", "100")
	if _, err := e.g.AccionModulo(IDDifusion, "difusiones.iniciar", Args{"id": float64(id)}); err != nil {
		t.Fatal(err)
	}
	e.simula(40)
	n := e.cuantos()
	if n != 10 {
		t.Fatalf("el primer día salen 10 por el calentamiento, salieron %d", n)
	}
	if !e.hay("Hola Persona 0, hay oferta") || !e.hay("escribe BAJA") {
		t.Fatalf("texto con variables y pie de baja: %v", e.enviados()[0])
	}
	if c := e.alm.ContarDest(1, id); c["enviado"] != 10 || c["pendiente"] != 4 {
		t.Fatalf("conteo: %v", c)
	}
	// mañana: calentamiento 15
	e.hora = e.hora.Add(24 * time.Hour)
	e.simula(40)
	if e.cuantos() != 14 {
		t.Fatalf("faltaban 4 y entran todos: %d", e.cuantos())
	}
	f, _ := e.alm.ObtenerAgenda(1, id)
	if f.Estado != dfTerminada {
		t.Fatalf("estado: %s", f.Estado)
	}
	if !tiene(e.avisos, "Difusión terminada") {
		t.Fatalf("falta el reporte: %v", e.avisos)
	}
	_ = ps
	// el registro nunca trae el texto
	for _, r := range e.registro() {
		if strings.Contains(r.Que+r.Resultado, "oferta") {
			t.Fatal("registro con texto")
		}
	}
}

func TestDifusionRespetaTopeDiarioDelModulo(t *testing.T) {
	e, _ := difusionLista(t, 6)
	e.config(IDDifusion, map[string]any{"calentamiento": false})
	e.fija("tope_hora", "100")
	e.fija("tope_dia", "100")
	_ = e.g.FijarTopesModulo(IDDifusion, 100, 3)
	id := creaAnuncio(t, e, "Hola", "")
	if _, err := e.g.AccionModulo(IDDifusion, "difusiones.iniciar", Args{"id": float64(id)}); err != nil {
		t.Fatal(err)
	}
	e.simula(30)
	if e.cuantos() != 3 {
		t.Fatalf("tope diario 3, salieron %d", e.cuantos())
	}
	e.hora = e.hora.Add(24 * time.Hour)
	e.simula(30)
	if e.cuantos() != 6 {
		t.Fatalf("al día siguiente salen los otros 3: %d", e.cuantos())
	}
}

func TestDifusionEsperaEntreMensajes(t *testing.T) {
	e, _ := difusionLista(t, 3)
	e.config(IDDifusion, map[string]any{"calentamiento": false})
	e.fija("tope_hora", "100")
	e.fija("tope_dia", "100")
	_ = e.g.FijarTopesModulo(IDDifusion, 100, 100)
	id := creaAnuncio(t, e, "Hola", "")
	_, _ = e.g.AccionModulo(IDDifusion, "difusiones.iniciar", Args{"id": float64(id)})
	e.hora = e.hora.Add(time.Minute)
	e.tick()
	e.corre()
	if e.cuantos() != 0 {
		t.Fatal("antes de la espera mínima de 8 s no sale nada")
	}
	e.hora = e.hora.Add(9 * time.Second)
	e.corre()
	if e.cuantos() != 1 {
		t.Fatalf("a los 9 s sale el primero: %d", e.cuantos())
	}
	e.hora = e.hora.Add(3 * time.Second)
	e.corre()
	if e.cuantos() != 1 {
		t.Fatal("el segundo espera otros 8 s como mínimo")
	}
	e.hora = e.hora.Add(10 * time.Second)
	e.corre()
	if e.cuantos() != 2 {
		t.Fatalf("pasaron más de 8 s: %d", e.cuantos())
	}
}

func TestDifusionPausaPorErroresDeEnvio(t *testing.T) {
	e, _ := difusionLista(t, 8)
	e.config(IDDifusion, map[string]any{"calentamiento": false, "errores_pausa": 3.0})
	e.fija("tope_hora", "100")
	e.fija("tope_dia", "100")
	e.fija("errores", "50") // que no salte el modo seguro del marco: aquí se prueba la pausa propia
	_ = e.g.FijarTopesModulo(IDDifusion, 100, 100)
	id := creaAnuncio(t, e, "Hola", "")
	_, _ = e.g.AccionModulo(IDDifusion, "difusiones.iniciar", Args{"id": float64(id)})
	e.env.falla = errors.New("sin red")
	e.simula(10)
	f, _ := e.alm.ObtenerAgenda(1, id)
	if f.Estado != dfPausada {
		t.Fatalf("debía pausarse: %s", f.Estado)
	}
	if !tiene(e.avisos, "Difusión en pausa") {
		t.Fatalf("avisos: %v", e.avisos)
	}
	c := e.alm.ContarDest(1, id)
	if c["fallido"] != 3 || c["pendiente"] != 5 {
		t.Fatalf("conteo: %v", c)
	}
	// al reanudar sigue donde se quedó
	e.env.falla = nil
	_, _ = e.g.AccionModulo(IDDifusion, "difusiones.iniciar", Args{"id": float64(id)})
	e.simula(20)
	if e.cuantos() != 5 {
		t.Fatalf("los 5 pendientes salen: %d", e.cuantos())
	}
}

func TestDifusionOmiteNoMolestarYBorradoresYProgramadas(t *testing.T) {
	e, ps := difusionLista(t, 3)
	e.config(IDDifusion, map[string]any{"calentamiento": false})
	e.fija("tope_hora", "100")
	e.fija("tope_dia", "100")
	_ = e.g.FijarTopesModulo(IDDifusion, 100, 100)
	// programada para dentro de 30 minutos
	fecha := e.hora.Add(30 * time.Minute).Format("02/01/2006 15:04")
	id := creaAnuncio(t, e, "Hola", fecha)
	f, _ := e.alm.ObtenerAgenda(1, id)
	if f.Estado != dfProgramada || f.ProximoTS == 0 {
		t.Fatalf("programada: %+v", f)
	}
	_ = e.g.AgregarNoMolestar(ps[1], "a mano")
	e.simula(20)
	if e.cuantos() != 0 {
		t.Fatal("todavía no es la hora")
	}
	e.simula(40)
	if e.cuantos() != 2 {
		t.Fatalf("sale a 2: la tercera pidió no recibir (%d)", e.cuantos())
	}
	if c := e.alm.ContarDest(1, id); c["omitido"] != 0 && c["enviado"] != 2 {
		t.Fatalf("conteo %v", c)
	}
	// hay que haber iniciado: una fecha pasada se rechaza
	_, err := e.g.AccionModulo(IDDifusion, "difusiones.agregar", Args{"nombre": "x", "texto": "t", "lista": ListaSuscritos, "fecha": "01/01/2020 10:00"})
	esperaErr(t, err, "ya pasó")
	_, err = e.g.AccionModulo(IDDifusion, "difusiones.agregar", Args{"nombre": "x", "texto": "", "lista": ListaSuscritos})
	esperaErr(t, err, "escribe el mensaje")
}

func TestDifusionNoSaleConMaestroApagadoNiEnModoSeguro(t *testing.T) {
	e, _ := difusionLista(t, 2)
	e.config(IDDifusion, map[string]any{"calentamiento": false})
	id := creaAnuncio(t, e, "Hola", "")
	_, _ = e.g.AccionModulo(IDDifusion, "difusiones.iniciar", Args{"id": float64(id)})
	e.fija("maestro", "0")
	e.simula(10)
	if e.cuantos() != 0 {
		t.Fatal("maestro apagado")
	}
	e.fija("maestro", "1")
	e.g.ModoSeguro("prueba")
	e.simula(10)
	if e.cuantos() != 0 {
		t.Fatal("modo seguro")
	}
	// lo encolado y descartado vuelve a pendiente, no se pierde
	if c := e.alm.ContarDest(1, id); c["pendiente"] != 2 {
		t.Fatalf("conteo: %v", c)
	}
}

func TestDifusionRecuperaLoEncoladoAlArrancar(t *testing.T) {
	e, ps := difusionLista(t, 2)
	id := creaAnuncio(t, e, "Hola", "")
	_, _ = e.g.AccionModulo(IDDifusion, "difusiones.iniciar", Args{"id": float64(id)})
	e.alm.MarcarDest(1, id, ps[0], "encolado", e.hora) // como si la app se hubiera cerrado con eso en la cola
	m, _ := Obtener(IDDifusion)
	if err := m.Activar(e.g); err != nil {
		t.Fatal(err)
	}
	if c := e.alm.ContarDest(1, id); c["pendiente"] != 2 || c["encolado"] != 0 {
		t.Fatalf("al arrancar se vuelve a encolar: %v", c)
	}
}

func TestDifusionConArchivosYVistaPrevia(t *testing.T) {
	e, _ := difusionLista(t, 1)
	e.config(IDDifusion, map[string]any{"calentamiento": false})
	e.fija("tope_hora", "100")
	e.fija("tope_dia", "100")
	_ = e.g.FijarTopesModulo(IDDifusion, 100, 100)
	idA, _, err := e.alm.GuardarAdjuntoMenu(1, bytes.NewReader([]byte("imagen")))
	if err != nil {
		t.Fatal(err)
	}
	adj := []any{map[string]any{"id": idA, "tipo": "imagen", "mime": "image/jpeg", "nombre": "a.jpg", "bytes": 6.0}}
	m, err := e.g.AccionModulo(IDDifusion, "difusiones.vista", Args{"nombre": "x", "texto": "Hola {nombre}", "lista": ListaSuscritos, "adjuntos": adj})
	if err != nil || !strings.Contains(m, "Hola María") || !strings.Contains(m, "1 archivos") {
		t.Fatalf("vista: %q %v", m, err)
	}
	_, err = e.g.AccionModulo(IDDifusion, "difusiones.agregar", Args{"nombre": "x", "texto": "t", "lista": ListaSuscritos, "adjuntos": []any{map[string]any{"id": "../../etc", "tipo": "imagen"}}})
	esperaErr(t, err, "no es válido")
	_, err = e.g.AccionModulo(IDDifusion, "difusiones.agregar", Args{"nombre": "x", "texto": "t", "lista": ListaSuscritos, "adjuntos": []any{map[string]any{"id": "0123456789abcdef01234567", "tipo": "imagen"}}})
	esperaErr(t, err, "ya no está")
	if _, err := e.g.AccionModulo(IDDifusion, "difusiones.agregar", Args{"nombre": "Foto", "texto": "Mira", "lista": ListaSuscritos, "adjuntos": adj}); err != nil {
		t.Fatal(err)
	}
	fs, _ := e.alm.ListarAgenda(1, IDDifusion)
	// el archivo sigue referenciado mientras el anuncio exista y no haya terminado
	if !e.alm.ReferenciasAdjuntos(1)[idA] {
		t.Fatal("el archivo del anuncio no debe limpiarse")
	}
	if _, err := e.g.AccionModulo(IDDifusion, "difusiones.iniciar", Args{"id": float64(fs[0].ID)}); err != nil {
		t.Fatal(err)
	}
	e.simula(3)
	if !e.hay("[medio:imagen]") {
		t.Fatalf("debió salir la imagen: %v", e.enviados())
	}
}

func TestDifusionComandoEstadoYControles(t *testing.T) {
	e, _ := difusionLista(t, 2)
	id := creaAnuncio(t, e, "Hola", "")
	if _, err := e.g.AccionModulo(IDDifusion, "difusiones.pausar", Args{"id": float64(id)}); err == nil {
		t.Fatal("solo se pausa lo que se envía")
	}
	if _, err := e.g.AccionModulo(IDDifusion, "difusiones.cancelar", Args{"id": float64(id)}); err != nil {
		t.Fatal(err)
	}
	if _, err := e.g.AccionModulo(IDDifusion, "difusiones.iniciar", Args{"id": float64(id)}); err == nil {
		t.Fatal("una cancelada no se inicia")
	}
	if _, err := e.g.AccionModulo(IDDifusion, "difusiones.borrar", Args{"id": float64(id)}); err != nil {
		t.Fatal(err)
	}
	if f, _ := e.g.FilasPanel(IDDifusion, "difusiones"); len(f) != 0 {
		t.Fatal("borrado")
	}
	var _ = almacen.PersonaDueno
}

func TestDifusionApagadaNoIniciaYValida(t *testing.T) {
	e, _ := entH(t)
	e.conChat(pnAna, false, "Ana")
	_, _ = e.g.AccionModulo(IDDifusion, "listas.agregar", Args{"lista": ListaSuscritos, "persona": pnAna})
	id := creaAnuncio(t, e, "Hola", "")
	_, err := e.g.AccionModulo(IDDifusion, "difusiones.iniciar", Args{"id": float64(id)})
	esperaErr(t, err, "enciende el módulo")
	b, _ := jsonDe(map[string]any{"palabra_alta": "baja"})
	esperaErr(t, e.g.GuardarConfig(IDDifusion, b), "no sirve")
	b, _ = jsonDe(map[string]any{"espera_min": 20.0, "espera_max": 10.0})
	esperaErr(t, e.g.GuardarConfig(IDDifusion, b), "espera máxima")
	i, _ := e.g.InfoDe(IDDifusion)
	if i.Riesgo != RiesgoAlto || i.DefDia != 30 {
		t.Fatalf("difusión es de riesgo alto con 30 al día: %+v", i)
	}
}
