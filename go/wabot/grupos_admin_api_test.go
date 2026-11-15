package wabot

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"botchat/internal/almacen"

	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

func TestApiAdministrarGrupoMiembrosPerfilYResumen(t *testing.T) {
	reinicia(t)
	if err := AbrirAlmacen(t.TempDir(), clavePrueba(), &obsFalso{}); err != nil {
		t.Fatal(err)
	}
	a, _ := almacenAbierto()
	g := "120363000000000009@g.us"
	ana, beto := "34600000001@s.whatsapp.net", "34600000002@s.whatsapp.net"
	if _, err := a.ActualizarChat(1, g, true, func(c *almacen.Chat) { c.Guardado = "Probar" }); err != nil {
		t.Fatal(err)
	}
	_, _ = a.ActualizarMiembros(1, g, []almacen.MiembroGrupo{{Persona: ana, JID: ana, Rol: almacen.RolGrupoAdmin}, {Persona: beto, JID: beto, Rol: almacen.RolGrupoMiembro}}, time.Now())
	ahora := time.Now().Unix()
	poner := func(id, de, tipo string, nota bool, ts int64) {
		m := &almacen.Mensaje{ID: id, ChatJID: g, BotID: 1, RemitenteJID: de, RemitenteNombre: "X", Tipo: tipo, NotaVoz: nota, Texto: "t", TS: ts}
		if _, err := a.GuardarMensaje(m); err != nil {
			t.Fatal(err)
		}
	}
	poner("1", ana, almacen.TipoTexto, false, ahora-60)
	poner("2", ana, almacen.TipoTexto, false, ahora-50)
	poner("3", ana, almacen.TipoAudio, true, ahora-40)
	poner("4", ana, almacen.TipoImagen, false, ahora-30)
	poner("5", beto, almacen.TipoTexto, false, ahora-20)

	// presencia: Ana en línea
	presMu.Lock()
	pres[ana] = &presenciaChat{EnLinea: true, UltimaVez: ahora - 5}
	presMu.Unlock()
	t.Cleanup(func() { presMu.Lock(); delete(pres, ana); presMu.Unlock() })

	j, err := GrupoMiembrosAdmin(1, g)
	if err != nil {
		t.Fatal(err)
	}
	var lista struct {
		Desde    int64 `json:"desde"`
		Miembros []struct {
			Persona string `json:"persona"`
			Rol     string `json:"rol"`
			EnLinea bool   `json:"enLinea"`
			Conteo  struct {
				Mensajes, Texto, Imagenes, NotasVoz int
				UltimoTs                            int64
			} `json:"conteo"`
		} `json:"miembros"`
	}
	if err := json.Unmarshal([]byte(j), &lista); err != nil {
		t.Fatal(err, j)
	}
	if len(lista.Miembros) != 2 || lista.Desde != ahora-60 {
		t.Fatalf("%s", j)
	}
	var vAna, vBeto = -1, -1
	for i, m := range lista.Miembros {
		if m.Persona == ana {
			vAna = i
		} else {
			vBeto = i
		}
	}
	x := lista.Miembros[vAna]
	if x.Conteo.Mensajes != 4 || x.Conteo.Texto != 2 || x.Conteo.NotasVoz != 1 || x.Conteo.Imagenes != 1 || !x.EnLinea || x.Rol != "admin" || x.Conteo.UltimoTs != ahora-30 {
		t.Fatalf("ana: %+v", x)
	}
	if lista.Miembros[vBeto].EnLinea || lista.Miembros[vBeto].Conteo.Mensajes != 1 {
		t.Fatalf("beto: %+v", lista.Miembros[vBeto])
	}

	pj, err := GrupoPerfilMiembro(1, g, ana)
	if err != nil {
		t.Fatal(err)
	}
	var p struct {
		Miembro struct{ Conteo struct{ Mensajes int } } `json:"miembro"`
		Serie   []struct{ N int }                       `json:"serie"`
		Puesto  int                                     `json:"puesto"`
		Activos int                                     `json:"activos"`
		Media   float64                                 `json:"mediaGrupo"`
	}
	if err := json.Unmarshal([]byte(pj), &p); err != nil {
		t.Fatal(err, pj)
	}
	tot := 0
	for _, d := range p.Serie {
		tot += d.N
	}
	if p.Miembro.Conteo.Mensajes != 4 || len(p.Serie) != 30 || tot != 4 || p.Puesto != 1 || p.Activos != 2 || p.Media != 2.5 {
		t.Fatalf("%s", pj)
	}

	rj, err := GrupoResumenAdmin(1, g)
	if err != nil {
		t.Fatal(err)
	}
	var r struct {
		Miembros, Admins, Mensajes7, Activos7, Baneados int
		Top                                             []struct{ Persona string }
		Serie                                           []struct{ N int }
	}
	if err := json.Unmarshal([]byte(rj), &r); err != nil {
		t.Fatal(err, rj)
	}
	if r.Miembros != 2 || r.Admins != 1 || r.Activos7 != 2 || r.Mensajes7 != 5 || len(r.Serie) != 14 || len(r.Top) != 2 || r.Top[0].Persona != ana {
		t.Fatalf("%s", rj)
	}
}

func TestApiHorarioBaneadosEHistorial(t *testing.T) {
	reinicia(t)
	if err := AbrirAlmacen(t.TempDir(), clavePrueba(), &obsFalso{}); err != nil {
		t.Fatal(err)
	}
	g := "120363000000000009@g.us"
	if err := GrupoGuardarHorario(1, g, `{"activo":true,"cierra":"23:30","abre":"06:00","dias":31}`); err != nil {
		t.Fatal(err)
	}
	if j, _ := GrupoHorario(1, g); !strings.Contains(j, `"cierra":"23:30"`) || !strings.Contains(j, `"dias":31`) || !strings.Contains(j, `"activo":true`) {
		t.Fatal(j)
	}
	if err := GrupoGuardarHorario(1, g, `{"activo":true,"cierra":"9","abre":"06:00"}`); err == nil {
		t.Fatal("hora inválida")
	}
	if err := GrupoGuardarHorario(1, g, `no es json`); err == nil {
		t.Fatal("json inválido")
	}
	// sin conexión no se puede banear (ni se anota nada a medias)
	if err := GrupoBanear(1, g, "34600000007@s.whatsapp.net", "x"); err == nil {
		t.Fatal("sin WhatsApp no debía poder expulsar")
	}
	if j, _ := GrupoBaneados(1, g); strings.Contains(j, "34600000007") {
		t.Fatalf("no debía quedar baneada: %s", j)
	}
	a, _ := almacenAbierto()
	_ = a.Banear(1, g, "34600000007@s.whatsapp.net", almacen.PersonaDueno, "spam", time.Now())
	if j, _ := GrupoBaneados(1, g); !strings.Contains(j, "34600000007") || !strings.Contains(j, "spam") {
		t.Fatal(j)
	}
	if err := GrupoDesbanear(1, g, "34600000007@s.whatsapp.net"); err != nil {
		t.Fatal(err)
	}
	if err := GrupoDesbanear(1, g, "34600000007@s.whatsapp.net"); err == nil {
		t.Fatal("ya no estaba baneada")
	}
	// historial: cambios de miembros y acciones
	_ = a.RegistrarCambio(1, g, almacen.CambioEnlace, "34600000008@s.whatsapp.net", "34600000008@s.whatsapp.net", "", "invite", time.Now().Add(-time.Hour))
	a.RegistrarAccion(1, almacen.PersonaDueno, "grupo.expulsar", "34600000008@s.whatsapp.net", g, "ok", "")
	hj, err := GrupoHistorial(1, g, 0, 50)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(hj, "entró por el enlace") || !strings.Contains(hj, "expulsó a") {
		t.Fatalf("%s", hj)
	}
	var ev []struct {
		Clase string
		TS    int64
	}
	_ = json.Unmarshal([]byte(hj), &ev)
	if len(ev) < 2 || ev[0].Clase != "accion" || ev[len(ev)-1].Clase != "cambio" {
		t.Fatalf("orden: %+v", ev)
	}
	for i := 1; i < len(ev); i++ {
		if ev[i].TS > ev[i-1].TS {
			t.Fatalf("debía venir lo más reciente primero: %+v", ev)
		}
	}
}

// Una persona baneada que vuelve a entrar provoca el intento de sacarla (queda en el registro de acciones aunque no haya conexión).
func TestBaneadaQueVuelveSeIntentaSacar(t *testing.T) {
	reinicia(t)
	raiz := t.TempDir()
	if err := AbrirAlmacen(raiz, clavePrueba(), &obsFalso{}); err != nil {
		t.Fatal(err)
	}
	if err := Start(raiz, 1, clavePrueba(), &falso{}); err != nil {
		t.Fatal(err)
	}
	defer Stop()
	mu.Lock()
	s := st
	mu.Unlock()
	a, _ := almacenAbierto()
	mala := mustJID("34600000009@s.whatsapp.net")
	_ = a.Banear(1, grpJID, mala.String(), almacen.PersonaDueno, "x", time.Now())
	s.evento(&events.GroupInfo{JID: mustJID(grpJID), Timestamp: time.Now(), Join: []types.JID{mala}})
	esperaHasta(t, "intento de expulsión registrado", func() bool {
		acc, _ := a.ListarAcciones(1, 50, 0)
		for _, x := range acc {
			if x.Accion == "grupo.expulsar" && strings.HasPrefix(x.Objetivo, "34600000009") {
				return true
			}
		}
		return false
	})
}
