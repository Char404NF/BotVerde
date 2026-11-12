package almacen

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func msgP(t testing.TB, a *Almacen, chat, id, de, tipo, texto string, ts int64, enlaces bool) {
	t.Helper()
	m := &Mensaje{ID: id, ChatJID: chat, BotID: 1, RemitenteJID: de, RemitenteNombre: "Ana", Tipo: tipo, Texto: texto, TS: ts, Enlaces: enlaces}
	if tipo != TipoTexto {
		m.ArchivoBytes = 1000
	}
	if _, err := a.GuardarMensaje(m); err != nil {
		t.Fatal(err)
	}
}

func escenarioPerfil(t *testing.T) (*Almacen, string, string, string, string) {
	a := nuevo(t)
	pn, lid := "5215500000001@s.whatsapp.net", "99887766@lid"
	g1, g2 := "120363000000000001@g.us", "120363000000000002@g.us"
	for _, c := range []struct {
		j string
		g bool
	}{{pn, false}, {g1, true}, {g2, true}} {
		if _, err := a.ActualizarChat(1, c.j, c.g, func(x *Chat) { x.Guardado = "Chat " + c.j[:4] }); err != nil {
			t.Fatal(err)
		}
	}
	a.AprenderAlias(1, pn, lid)
	base := time.Now().Add(-48 * time.Hour).Unix()
	msgP(t, a, pn, "P1", pn, TipoTexto, "Hola, ¿cómo estás? Mira https://ejemplo.mx", base, true)
	msgP(t, a, pn, "P2", QuienYo, TipoTexto, "Bien gracias", base+10, false)
	msgP(t, a, g1, "G1", lid, TipoTexto, "El Camión llegó", base+20, false) // la persona en el grupo viene como LID
	msgP(t, a, g1, "G2", pn, TipoImagen, "", base+30, false)
	msgP(t, a, g1, "G3", pn, TipoVideo, "", base+40, false)
	msgP(t, a, g1, "G4", pn, TipoAudio, "", base+50, false)
	msgP(t, a, g1, "G5", pn, TipoDocumento, "", base+60, false)
	msgP(t, a, g1, "G6", pn, TipoTexto, "otro enlace http://x.org", base+70, true)
	msgP(t, a, g1, "G7", "otro@s.whatsapp.net", TipoTexto, "no es de ella", base+80, false)
	msgP(t, a, g2, "H1", pn, TipoTexto, "mensaje en el 2", base+90, false)
	return a, pn, lid, g1, g2
}

func TestActividadYPerfilDePersona(t *testing.T) {
	a, pn, lid, g1, g2 := escenarioPerfil(t)
	// miembro sin mensajes en un tercer grupo
	g3 := "120363000000000003@g.us"
	_, _ = a.ActualizarChat(1, g3, true, func(x *Chat) { x.Guardado = "Tercero" })
	_, _ = a.ActualizarMiembros(1, g3, []MiembroGrupo{{Persona: pn, JID: pn, Rol: RolGrupoMiembro}}, time.Now())
	ts, err := a.ActividadPersona(1, lid) // por el alias
	if err != nil {
		t.Fatal(err)
	}
	if len(ts) != 4 || !ts[0].Privado || ts[0].Nombre != "Chat privado" {
		t.Fatalf("%+v", ts)
	}
	if ts[0].Mensajes != 1 || ts[0].Enlaces != 1 || ts[0].MisEnvios != 1 {
		t.Fatalf("privado %+v", ts[0])
	}
	por := map[string]TarjetaActividad{}
	for _, x := range ts[1:] {
		por[x.Chat] = x
	}
	if x := por[g1]; x.Mensajes != 6 || x.Fotos != 1 || x.Videos != 1 || x.Audios != 1 || x.Archivos != 1 || x.Enlaces != 1 || x.UltimoTS == 0 {
		t.Fatalf("g1 %+v", x)
	}
	if x := por[g2]; x.Mensajes != 1 {
		t.Fatalf("g2 %+v", x)
	}
	if x := por[g3]; x.Mensajes != 0 || !x.Miembro {
		t.Fatalf("g3 %+v", x)
	}
	p, err := a.PerfilDe(1, lid, time.Now())
	if err != nil || p.Persona != pn || p.Mensajes != 8 || p.Archivos != 4 || p.ArchivosBytes != 4000 || p.Telefono == "" || len(p.GruposComun) != 1 {
		t.Fatalf("%+v %v", p, err)
	}
}

func TestMensajesDePersonaPestanasFiltrosYMarcas(t *testing.T) {
	a, pn, lid, g1, _ := escenarioPerfil(t)
	ids := func(p *PaginaPersona) string {
		var s []string
		for _, m := range p.Items {
			s = append(s, m.ID)
		}
		return strings.Join(s, ",")
	}
	pide := func(f FiltroPersona) *PaginaPersona {
		p, err := a.MensajesDePersona(1, f, 0, "", 50)
		if err != nil {
			t.Fatal(err)
		}
		return p
	}
	if p := pide(FiltroPersona{Chat: g1, Persona: lid}); ids(p) != "G6,G5,G4,G3,G2,G1" || p.Total != 6 || !p.Fin {
		t.Fatalf("todo: %s %+v", ids(p), p)
	}
	if p := pide(FiltroPersona{Chat: g1, Persona: pn, Pestana: PestanaMensajes}); ids(p) != "G6,G1" {
		t.Fatal(ids(p))
	}
	if p := pide(FiltroPersona{Chat: g1, Persona: pn, Pestana: PestanaFotos}); ids(p) != "G3,G2" {
		t.Fatal(ids(p))
	}
	if p := pide(FiltroPersona{Chat: g1, Persona: pn, Pestana: PestanaAudios}); ids(p) != "G4" {
		t.Fatal(ids(p))
	}
	if p := pide(FiltroPersona{Chat: g1, Persona: pn, Pestana: PestanaArchivos}); ids(p) != "G6,G5" {
		t.Fatal(ids(p)) // documento y texto con enlace
	}
	if _, err := a.MensajesDePersona(1, FiltroPersona{Pestana: "x"}, 0, "", 5); err == nil {
		t.Fatal("pestaña mala")
	}
	// todos los chats de la persona
	if p := pide(FiltroPersona{Persona: pn}); p.Total != 8 {
		t.Fatalf("%d", p.Total)
	}
	// paginación
	p1, _ := a.MensajesDePersona(1, FiltroPersona{Chat: g1, Persona: pn}, 0, "", 4)
	p2, _ := a.MensajesDePersona(1, FiltroPersona{Chat: g1, Persona: pn}, p1.SigTS, p1.SigID, 4)
	if ids(p1) != "G6,G5,G4,G3" || p1.Fin || ids(p2) != "G2,G1" || !p2.Fin {
		t.Fatalf("%s %s", ids(p1), ids(p2))
	}
	// fechas
	base := time.Now().Add(-48 * time.Hour).Unix()
	if p := pide(FiltroPersona{Chat: g1, Persona: pn, Desde: base + 35, Hasta: base + 65}); ids(p) != "G5,G4,G3" {
		t.Fatal(ids(p))
	}
	// palabra sin acentos ni mayúsculas, y dos palabras
	if p := pide(FiltroPersona{Persona: pn, Palabra: "CAMION"}); ids(p) != "G1" || p.Total != 1 || !p.Fin || p.Escaneados == 0 {
		t.Fatalf("%s %+v", ids(p), p)
	}
	if p := pide(FiltroPersona{Persona: pn, Palabra: "cómo ESTAS"}); ids(p) != "P1" {
		t.Fatal(ids(p))
	}
	if p := pide(FiltroPersona{Persona: pn, Palabra: "inexistente"}); len(p.Items) != 0 || !p.Fin {
		t.Fatal("nada")
	}
	// marcas: borrado por la persona, borrado por un admin, editado, marcado
	_, _ = a.MarcarEliminado(1, g1, "G1", "", 0)
	_, _ = a.MarcarEliminado(1, g1, "G2", "otro@s.whatsapp.net", 0)
	if ok, _ := a.EditarTexto(1, g1, "G6", "versión nueva"); !ok {
		t.Fatal("editar")
	}
	et, _ := a.CrearEtiquetaEmoji(1, "Ojo", "#FF0000", "👁")
	if err := a.MarcarMensaje(1, g1, "G3", et, "ver luego", time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := a.MarcarMensaje(1, g1, "G4", 0, "solo nota", time.Now()); err != nil {
		t.Fatal(err)
	}
	p := pide(FiltroPersona{Chat: g1, Persona: pn})
	por := map[string]MensajePersona{}
	for _, m := range p.Items {
		por[m.ID] = m
	}
	if m := por["G1"]; m.MarcaRoja != "borrado" || m.BorradoPor != "autor" {
		t.Fatalf("%+v", m)
	}
	if m := por["G2"]; m.MarcaRoja != "borrado" || m.BorradoPor == "autor" || m.BorradoPor == "" {
		t.Fatalf("admin: %+v", m)
	}
	if m := por["G6"]; m.MarcaRoja != "editado" || m.Versiones != 1 || m.Texto != "versión nueva" {
		t.Fatalf("%+v", m)
	}
	if m := por["G3"]; !m.Marcado || len(m.Etiquetas) != 1 || m.Etiquetas[0] != et {
		t.Fatalf("%+v", m)
	}
	if m := por["G4"]; !m.Marcado || len(m.Etiquetas) != 0 {
		t.Fatalf("solo nota: %+v", m)
	}
	if m := por["G5"]; m.Marcado || m.MarcaRoja != "" {
		t.Fatalf("%+v", m)
	}
	if ids(pide(FiltroPersona{Persona: pn, SoloBorrados: true})) != "G2,G1" {
		t.Fatal("borrados")
	}
	if ids(pide(FiltroPersona{Persona: pn, SoloEditados: true})) != "G6" {
		t.Fatal("editados")
	}
	if ids(pide(FiltroPersona{Persona: pn, SoloMarcados: true})) != "G4,G3" {
		t.Fatal("marcados")
	}
}

func TestBuscarEnChatPuntitosYExportar(t *testing.T) {
	a, pn, _, g1, _ := escenarioPerfil(t)
	p, err := a.BuscarEnChat(1, g1, "enlace", 0, 0, 0, "", 10)
	if err != nil || len(p.Items) != 1 || p.Items[0].ID != "G6" {
		t.Fatalf("%+v %v", p, err)
	}
	if p, _ := a.BuscarEnChat(1, g1, "  ", 0, 0, 0, "", 10); len(p.Items) != 0 || !p.Fin {
		t.Fatal("vacío")
	}
	if p, _ := a.BuscarEnChat(1, g1, "de ella", 0, 0, 0, "", 10); len(p.Items) != 1 || p.Items[0].ID != "G7" {
		t.Fatal("sin filtro de persona")
	}
	// búsqueda por pedazos: limite 1 y seguir con el cursor
	pa, _ := a.BuscarEnChat(1, g1, "e", 0, 0, 0, "", 1)
	if len(pa.Items) != 1 || pa.Fin {
		t.Fatalf("%+v", pa)
	}
	vistos := map[string]bool{pa.Items[0].ID: true}
	for i := 0; i < 20 && !pa.Fin; i++ {
		pa, _ = a.BuscarEnChat(1, g1, "e", 0, 0, pa.SigTS, pa.SigID, 1)
		for _, m := range pa.Items {
			if vistos[m.ID] {
				t.Fatalf("repetido %s", m.ID)
			}
			vistos[m.ID] = true
		}
	}
	if len(vistos) != 3 { // G1 "camión llegó", G6 "otro enlace", G7 "no es de ella"
		t.Fatalf("%v", vistos)
	}
	// puntitos: etiqueta de persona
	et, _ := a.CrearEtiquetaEmoji(1, "VIP", "#00FF00", "⭐")
	if err := a.PonerEtiquetaPersona(1, pn, et, true); err != nil {
		t.Fatal(err)
	}
	pt := a.PuntitosDeMensajes(1, g1, []string{"G1", "G2", "G7", "NOEXISTE"})
	if len(pt) != 2 || len(pt["G1"]) != 1 || pt["G1"][0] != et || pt["G2"][0] != et {
		t.Fatalf("%v", pt) // G1 viene con el LID y aun así sale
	}
	// exportar
	rel, n, err := a.EscribirExporteChat(1, g1, "txt", 0, 0, time.Now())
	if err != nil || n != 7 || !strings.HasSuffix(rel, ".txt.bcf") {
		t.Fatalf("%q %d %v", rel, n, err)
	}
	ex, _ := a.DatosDeChat(1, g1, 0, 0, time.Now())
	if len(ex.Mensajes) != 7 || ex.Mensajes[0].ID != "G1" || ex.Mensajes[6].ID != "G7" {
		t.Fatalf("%+v", ex.Mensajes)
	}
	if tx := a.TextoDeExporte(ex); !strings.Contains(tx, "El Camión llegó") || !strings.Contains(tx, "[image") && !strings.Contains(tx, "[imagen") {
		t.Fatal(tx)
	}
	if _, _, err := a.EscribirExporteChat(1, "no@g.us", "json", 0, 0, time.Now()); err == nil {
		t.Fatal("chat inexistente")
	}
	rel2, _, _ := a.EscribirExporteChat(1, g1, "json", 0, 0, time.Now().Add(time.Second))
	if rel2 == rel {
		t.Fatal("debe cambiar de nombre")
	}
}

func TestPlegar(t *testing.T) {
	for in, want := range map[string]string{"CAMIÓN": "camion", "Ñandú": "nandu", "abc": "abc", "ÁÉÍÓÚ": "aeiou", "Üñ\u0301": "un", "ÇÀ": "ca"} {
		if got := plegar(in); got != want {
			t.Errorf("%q -> %q", in, got)
		}
	}
}

// Rendimiento con 20,000 mensajes (se anota en PROGRESO.md).
func TestMedirPerfilYBusquedaConMilesDeMensajes(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	a := nuevo(t)
	chat, err := a.ChatSintetico(1, 20000)
	if err != nil {
		t.Fatal(err)
	}
	pers := "521550000003@s.whatsapp.net"
	suyos := 0
	for i := 0; i < 20000; i++ {
		if i%25 == 3 && i%7 != 0 {
			suyos++
		}
	}
	mide := func(nombre string, f func()) time.Duration {
		t0 := time.Now()
		f()
		d := time.Since(t0)
		t.Logf("%-48s %v", nombre, d)
		return d
	}
	// chat grande: 25 personas, 1/25 de los mensajes cada una
	mide("MensajesDePersona primera página (sin palabra)", func() {
		p, err := a.MensajesDePersona(1, FiltroPersona{Chat: chat, Persona: pers}, 0, "", 50)
		if err != nil || len(p.Items) != 50 || p.Total != int64(suyos) {
			t.Fatal(err, len(p.Items), p.Total)
		}
	})
	mide("MensajesDePersona fotos y videos", func() {
		if _, err := a.MensajesDePersona(1, FiltroPersona{Chat: chat, Persona: pers, Pestana: PestanaFotos}, 0, "", 50); err != nil {
			t.Fatal(err)
		}
	})
	var esc int
	d := mide("palabra rara (recorre y descifra los 20,000)", func() {
		p, err := a.BuscarEnChat(1, chat, "zzzz", 0, 0, 0, "", 50)
		if err != nil || !p.Fin || len(p.Items) != 0 {
			t.Fatal(err, p)
		}
		esc = p.Escaneados
	})
	if esc != 20000 {
		t.Fatalf("escaneados %d", esc)
	}
	// el tiempo solo se anota: con -race y otros paquetes corriendo en paralelo se dispara 50 veces y no dice nada del código
	if d > 5*time.Minute {
		t.Errorf("la búsqueda completa tarda %v", d)
	}
	mide("palabra común (primeros 50 resultados)", func() {
		p, _ := a.BuscarEnChat(1, chat, "enlace", 0, 0, 0, "", 50)
		if len(p.Items) != 50 {
			t.Fatal(len(p.Items))
		}
	})
	mide("palabra de una persona (recorre solo los suyos)", func() {
		p, _ := a.MensajesDePersona(1, FiltroPersona{Chat: chat, Persona: pers, Palabra: "zzzz"}, 0, "", 50)
		if p.Escaneados != suyos {
			t.Fatal(p.Escaneados)
		}
	})
	mide("ActividadPersona", func() {
		ts, err := a.ActividadPersona(1, pers)
		if err != nil || len(ts) != 2 || ts[1].Mensajes != suyos {
			t.Fatal(err, ts)
		}
	})
	mide("PerfilDe", func() {
		if _, err := a.PerfilDe(1, pers, time.Now()); err != nil {
			t.Fatal(err)
		}
	})
	ids := make([]string, 0, 60)
	for i := 0; i < 60; i++ {
		ids = append(ids, fmt.Sprintf("S%06d", 19900+i))
	}
	mide("PuntitosDeMensajes (60 mensajes)", func() { _ = a.PuntitosDeMensajes(1, chat, ids) })
	for i := 0; i < 25; i++ {
		j := fmt.Sprintf("52155%07d@s.whatsapp.net", i)
		_, _ = a.ActualizarMiembros(1, chat, []MiembroGrupo{{Persona: j, JID: j, Rol: RolGrupoMiembro}}, time.Now())
	}
	mide("MiembrosDe (25 miembros, 20,000 mensajes)", func() { _, _ = a.MiembrosDe(1, chat, false, time.Now()) })
	mide("ExportarChat json (20,000 mensajes)", func() {
		if _, n, err := a.EscribirExporteChat(1, chat, "json", 0, 0, time.Now()); err != nil || n != 20000 {
			t.Fatal(err, n)
		}
	})
}
