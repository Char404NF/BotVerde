package modulos

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"
	"time"

	"botchat/internal/almacen"
	"botchat/internal/comandos"
)

const grupoSorteo = "120363000000000010@g.us"

var (
	jidMaria = "5215533333333@s.whatsapp.net"
	jidPedro = "5215544444444@s.whatsapp.net"
)

// sorteoListo arma un grupo con cuatro miembros: Ana y Luis con rango «Socio», María y Pedro sin rango.
func sorteoListo(t *testing.T) (*ent, int64) {
	e, _ := entH(t)
	e.conChat(grupoSorteo, true, "Club")
	e.conChat(pnAna, false, "Ana")
	e.activa(IDSorteos)
	e.fija("tope_hora", "200")
	e.fija("tope_dia", "200")
	_, _ = e.alm.ActualizarMiembros(1, grupoSorteo, []almacen.MiembroGrupo{
		{Persona: pnAna, JID: pnAna, Rol: almacen.RolGrupoMiembro}, {Persona: pnLuis, JID: pnLuis, Rol: almacen.RolGrupoMiembro},
		{Persona: jidMaria, JID: jidMaria, Rol: almacen.RolGrupoMiembro}, {Persona: jidPedro, JID: jidPedro, Rol: almacen.RolGrupoMiembro},
	}, e.hora)
	socio, err := e.alm.CrearRol(1, almacen.Rol{Nombre: "Socio", Posicion: 5, Casillas: map[string]almacen.Alcance{}})
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{pnAna, pnLuis} {
		if err := e.alm.AsignarRol(1, p, socio, 0, "yo"); err != nil {
			t.Fatal(err)
		}
	}
	e.g.AzarSeguro = func(n int64) int64 { return 0 }
	return e, socio
}

func crearSorteo(t *testing.T, e *ent, extra Args) {
	t.Helper()
	a := Args{"nombre": "Navidad", "grupo": grupoSorteo, "premio": "Una pizza", "ganadores": 1.0, "reclamo_min": 30.0}
	for k, v := range extra {
		a[k] = v
	}
	if _, err := e.g.AccionModulo(IDSorteos, "sorteos.agregar", a); err != nil {
		t.Fatal(err)
	}
	e.corre()
}

func sorteoFilaDe(t *testing.T, e *ent) (almacen.AgendaFila, sorteoDatos) {
	t.Helper()
	fs, _ := e.alm.ListarAgenda(1, IDSorteos)
	if len(fs) == 0 {
		t.Fatal("no hay sorteo")
	}
	var d sorteoDatos
	_ = json.Unmarshal([]byte(fs[0].Contenido), &d)
	return fs[0], d
}

func TestSorteoSoloEntreLosRangosElegidosYElGanadorRecibeAvisoPorPrivado(t *testing.T) {
	e, socio := sorteoListo(t)
	// Ana ya le escribió al bot: por privado; los demás no.
	_, _ = e.alm.GuardarMensaje(&almacen.Mensaje{ID: "m1", ChatJID: pnAna, BotID: 1, RemitenteJID: pnAna, Tipo: almacen.TipoTexto, Texto: "hola", TS: e.hora.Unix() - 100})
	crearSorteo(t, e, Args{"rango1": "" + itoa64(socio)})
	f, d := sorteoFilaDe(t, e)
	if d.Fase != faseReclamo || len(d.Slots) != 1 {
		t.Fatalf("fase %s slots %+v", d.Fase, d.Slots)
	}
	if g := d.Slots[0].Persona; g != pnAna && g != pnLuis {
		t.Fatalf("solo podían ganar los Socio: %s", g)
	}
	if !e.hay(grupoSorteo + "|🎉 Sorteo «Navidad»") {
		t.Fatalf("falta el anuncio en el grupo: %v", e.enviados())
	}
	if d.Slots[0].Persona == pnAna && !e.hay(pnAna+"|🎉 ¡Ganaste el sorteo") {
		t.Fatalf("Ana debía recibir el aviso por privado: %v", e.enviados())
	}
	if f.Estado != pgActivo || f.ProximoTS == 0 {
		t.Fatalf("debía quedar esperando el plazo: %+v", f)
	}
}

func TestSorteoSinChatPrivadoPrevioMencionaEnElGrupo(t *testing.T) {
	e, _ := sorteoListo(t)
	e.g.AzarSeguro = func(n int64) int64 { return n - 1 } // el último de la lista
	crearSorteo(t, e, Args{"rango1": "sin"})              // solo personas sin rango: María y Pedro
	_, d := sorteoFilaDe(t, e)
	if d.Slots[0].Persona != jidPedro {
		t.Fatalf("ganador: %+v", d.Slots)
	}
	for _, x := range e.enviados() {
		if strings.HasPrefix(x, jidPedro+"|") {
			t.Fatalf("no debía escribirle por privado a quien nunca habló con el bot: %s", x)
		}
	}
	if !e.hay(grupoSorteo + "|@5215544444444 ganaste el sorteo «Navidad»") {
		t.Fatalf("debía mencionarlo en el grupo: %v", e.enviados())
	}
}

func TestSorteoReclamarConComando(t *testing.T) {
	e, socio := sorteoListo(t)
	crearSorteo(t, e, Args{"rango1": itoa64(socio)})
	_, d := sorteoFilaDe(t, e)
	ganador := d.Slots[0].Persona
	otro := pnLuis
	if ganador == pnLuis {
		otro = pnAna
	}
	cmdReclamar := func(quien string) string {
		r, err := comandoDe(t, e, IDSorteos, "reclamar").Ejecutar(&comandos.Contexto{Persona: quien, Chat: quien})
		if err != nil {
			t.Fatal(err)
		}
		return r
	}
	if r := cmdReclamar(otro); !strings.Contains(r, "No tienes ningún premio") {
		t.Fatalf("quien no ganó: %q", r)
	}
	r := cmdReclamar(ganador)
	if !strings.Contains(r, "Reclamaste «Una pizza»") {
		t.Fatalf("reclamo: %q", r)
	}
	f, d2 := sorteoFilaDe(t, e)
	if f.Estado != pgTerminado || d2.Slots[0].Estado != slotReclamado {
		t.Fatalf("debía terminar: %s %+v", f.Estado, d2.Slots)
	}
	e.corre()
	if !e.hay("Sorteo «Navidad» terminado. Se llevó el premio") {
		t.Fatalf("falta el cierre en el grupo: %v", e.enviados())
	}
	if len(e.avisos) == 0 || !strings.Contains(strings.Join(e.avisos, "|"), "Premio reclamado") {
		t.Fatalf("debía avisar al dueño: %v", e.avisos)
	}
}

func TestSorteoSiNoReclamaPierdeLaOportunidadYSeSorteaOtro(t *testing.T) {
	e, socio := sorteoListo(t)
	_, _ = e.alm.GuardarMensaje(&almacen.Mensaje{ID: "m1", ChatJID: pnAna, BotID: 1, RemitenteJID: pnAna, Tipo: almacen.TipoTexto, Texto: "hola", TS: e.hora.Unix() - 100})
	_, _ = e.alm.GuardarMensaje(&almacen.Mensaje{ID: "m2", ChatJID: pnLuis, BotID: 1, RemitenteJID: pnLuis, Tipo: almacen.TipoTexto, Texto: "hola", TS: e.hora.Unix() - 100})
	crearSorteo(t, e, Args{"rango1": itoa64(socio)})
	_, d := sorteoFilaDe(t, e)
	primero := d.Slots[0].Persona
	e.simula(31)
	f, d2 := sorteoFilaDe(t, e)
	if !e.hay(primero + "|⌛ Se acabó el tiempo") {
		t.Fatalf("falta el aviso «perdiste tu oportunidad»: %v", e.enviados())
	}
	if d2.Slots[0].Persona == primero || d2.Slots[0].Intento != 2 || d2.Slots[0].Estado != slotEsperando {
		t.Fatalf("debía sortear a otro: %+v", d2.Slots)
	}
	if f.Estado != pgActivo {
		t.Fatalf("sigue en marcha: %s", f.Estado)
	}
	// el segundo tampoco reclama y ya no queda nadie con ese rango: termina sin ganador
	e.simula(31)
	f, d3 := sorteoFilaDe(t, e)
	if f.Estado != pgTerminado || d3.Slots[0].Estado != slotPerdido {
		t.Fatalf("debía terminar sin ganador: %s %+v", f.Estado, d3.Slots)
	}
	if !e.hay("terminado sin ganador") {
		t.Fatalf("falta el cierre: %v", e.enviados())
	}
	if !strings.Contains(strings.Join(e.avisos, "|"), "Sorteo sin ganador") {
		t.Fatalf("avisos: %v", e.avisos)
	}
}

func TestSorteoHaceElRecordatorioAntesDeQueSeAcabe(t *testing.T) {
	e, socio := sorteoListo(t)
	_, _ = e.alm.GuardarMensaje(&almacen.Mensaje{ID: "m1", ChatJID: pnAna, BotID: 1, RemitenteJID: pnAna, Tipo: almacen.TipoTexto, Texto: "hola", TS: e.hora.Unix() - 100})
	_, _ = e.alm.GuardarMensaje(&almacen.Mensaje{ID: "m2", ChatJID: pnLuis, BotID: 1, RemitenteJID: pnLuis, Tipo: almacen.TipoTexto, Texto: "hola", TS: e.hora.Unix() - 100})
	crearSorteo(t, e, Args{"rango1": itoa64(socio)})
	e.simula(20)
	if e.hay("⏳ Te quedan") {
		t.Fatal("aún falta mucho para el recordatorio")
	}
	e.simula(6)
	if !e.hay("⏳ Te quedan") {
		t.Fatalf("debía recordarle: %v", e.enviados())
	}
}

func TestSorteoProgramadoSeHaceALaHoraYSinParticipantesAvisa(t *testing.T) {
	e, socio := sorteoListo(t)
	crearSorteo(t, e, Args{"rango1": itoa64(socio), "fecha": e.hora.Add(10 * time.Minute).Format("02/01/2006 15:04")})
	if _, d := sorteoFilaDe(t, e); d.Fase != faseProgramado || len(d.Slots) != 0 {
		t.Fatalf("aún no se sortea: %+v", d)
	}
	e.simula(12)
	if _, d := sorteoFilaDe(t, e); d.Fase != faseReclamo || len(d.Slots) != 1 {
		t.Fatalf("debía sortear a la hora: %+v", d)
	}
	// un rango que nadie tiene
	otro, _ := e.alm.CrearRol(1, almacen.Rol{Nombre: "Vacío", Posicion: 6, Casillas: map[string]almacen.Alcance{}})
	_, err := e.g.AccionModulo(IDSorteos, "sorteos.agregar", Args{"nombre": "X", "grupo": grupoSorteo, "premio": "Nada", "rango1": itoa64(otro)})
	esperaErr(t, err, "no participa nadie")
	_, err = e.g.AccionModulo(IDSorteos, "sorteos.agregar", Args{"nombre": "X", "grupo": pnAna, "premio": "Nada"})
	esperaErr(t, err, "elige el grupo")
}

func TestSorteoConVariosGanadoresYVistaPrevia(t *testing.T) {
	e, _ := sorteoListo(t)
	msg, err := e.g.AccionModulo(IDSorteos, "sorteos.vista", Args{"grupo": grupoSorteo, "rango1": "sin"})
	if err != nil || !strings.Contains(msg, "Participan 2") {
		t.Fatalf("%q %v", msg, err)
	}
	n := 0
	e.g.AzarSeguro = func(x int64) int64 { n++; return 0 }
	crearSorteo(t, e, Args{"ganadores": 3.0})
	_, d := sorteoFilaDe(t, e)
	vistos := map[string]bool{}
	for _, s := range d.Slots {
		vistos[s.Persona] = true
	}
	if len(d.Slots) != 3 || len(vistos) != 3 {
		t.Fatalf("tres ganadores distintos: %+v", d.Slots)
	}
	for _, s := range d.Slots {
		if s.Persona == almacen.PersonaDueno {
			t.Fatal("el dueño no participa")
		}
	}
}

func itoa64(n int64) string { return strconv.FormatInt(n, 10) }

// comandoDe saca un comando de un módulo (los comandos se registran de verdad cuando el bot arranca: aquí se llaman directo).
func comandoDe(t *testing.T, e *ent, id, nombre string) *comandos.Comando {
	t.Helper()
	m, _ := Obtener(id)
	for _, c := range m.(ComandosDeGestor).ComandosG(e.g) {
		if c.Nombre == nombre {
			c := c
			return &c
		}
	}
	t.Fatalf("el módulo %s no tiene el comando %s", id, nombre)
	return nil
}
