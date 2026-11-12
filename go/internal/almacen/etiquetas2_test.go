package almacen

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestMigracion21Etiquetas(t *testing.T) {
	a := nuevo(t)
	if a.Version() < 21 {
		t.Fatalf("versión %d", a.Version())
	}
	for _, tb := range []string{"persona_etiqueta", "msg_marca", "grupo_miembros", "grupo_cambios", "persona_foto"} {
		if !a.existeTabla(tb) {
			t.Fatalf("falta %s", tb)
		}
	}
	// sin límite de etiquetas (antes eran 50) y con emoji
	for i := 0; i < 60; i++ {
		if _, err := a.CrearEtiquetaEmoji(1, fmt.Sprintf("E%d", i), "", "🔥"); err != nil {
			t.Fatalf("etiqueta %d: %v", i, err)
		}
	}
	es, _ := a.ListarEtiquetas(1)
	if len(es) != 60 || es[0].Emoji != "🔥" {
		t.Fatalf("%d %+v", len(es), es[0])
	}
	if err := a.EditarEtiqueta(1, es[0].ID, "Nueva", "#112233"); err != nil {
		t.Fatal(err)
	}
	if e, _ := a.EtiquetaPorNombre(1, "Nueva"); e == nil || e.Emoji != "🔥" {
		t.Fatal("editar sin emoji debe conservarlo")
	}
	if _, err := a.CrearEtiquetaEmoji(1, "x", "", strings.Repeat("a", 40)); err == nil {
		t.Fatal("emoji demasiado largo")
	}
}

func TestEtiquetasDePersonaConAlias(t *testing.T) {
	a := nuevo(t)
	id, _ := a.CrearEtiquetaEmoji(1, "VIP", "#FF0000", "⭐")
	pn, lid := "5215500000001@s.whatsapp.net", "99887766@lid"
	a.AprenderAlias(1, pn, lid)
	if err := a.PonerEtiquetaPersona(1, lid, id, true); err != nil { // se guarda con la canónica
		t.Fatal(err)
	}
	for _, p := range []string{pn, lid} {
		es, err := a.EtiquetasDePersona(1, p)
		if err != nil || len(es) != 1 || es[0].Emoji != "⭐" {
			t.Fatalf("%s: %v %v", p, es, err)
		}
	}
	lote := a.EtiquetasDePersonas(1, []string{pn, lid, "otro@s.whatsapp.net"})
	if len(lote[pn]) != 1 || len(lote[lid]) != 1 || len(lote["otro@s.whatsapp.net"]) != 0 {
		t.Fatalf("%v", lote)
	}
	if err := a.PonerEtiquetaPersona(1, pn, 999, true); err == nil {
		t.Fatal("etiqueta inexistente")
	}
	if err := a.PonerEtiquetaPersona(1, PersonaDueno, id, true); err == nil {
		t.Fatal("no se etiqueta al dueño")
	}
	// la ficha la muestra y el listado la cuenta
	f, _ := a.FichaDe(1, pn, time.Now())
	if len(f.Etiquetas) != 1 {
		t.Fatalf("%v", f.Etiquetas)
	}
	if n := a.ContarMarcados(1, id)[MarcadoPersona]; n != 1 {
		t.Fatalf("personas: %d", n)
	}
	// alias que se aprende después: la etiqueta del LID pasa a la canónica
	lid2, pn2 := "5544@lid", "5215500000002@s.whatsapp.net"
	_ = a.PonerEtiquetaPersona(1, lid2, id, true)
	a.AprenderAlias(1, pn2, lid2)
	if es, _ := a.EtiquetasDePersona(1, pn2); len(es) != 1 {
		t.Fatalf("fusión: %v", es)
	}
	// quitar
	if err := a.PonerEtiquetaPersona(1, lid, id, false); err != nil {
		t.Fatal(err)
	}
	if es, _ := a.EtiquetasDePersona(1, pn); len(es) != 0 {
		t.Fatal("no se quitó")
	}
}

func TestMarcasDeMensajeNotaCifradaYListado(t *testing.T) {
	a := nuevo(t)
	c := "1@s.whatsapp.net"
	chatBasico(t, a, c, "525512345678", "Ana", false)
	if _, err := a.GuardarMensaje(msg(c, "M1", time.Now().Unix()-10, "texto uno")); err != nil {
		t.Fatal(err)
	}
	_, _ = a.GuardarMensaje(msg(c, "M2", time.Now().Unix()-5, "texto dos"))
	e1, _ := a.CrearEtiquetaEmoji(1, "Urgente", "#FF0000", "")
	e2, _ := a.CrearEtiquetaEmoji(1, "Luego", "", "")
	if err := a.MarcarMensaje(1, c, "NOEXISTE", e1, "", time.Now()); err == nil {
		t.Fatal("mensaje inexistente")
	}
	if err := a.MarcarMensaje(1, c, "M1", 0, "", time.Now()); err == nil {
		t.Fatal("sin etiqueta ni nota")
	}
	nota := "nota-secreta-xyz"
	now := time.Now()
	_ = a.MarcarMensaje(1, c, "M1", e1, nota, now.Add(-time.Minute))
	_ = a.MarcarMensaje(1, c, "M1", e2, "", now.Add(-time.Minute))
	_ = a.MarcarMensaje(1, c, "M2", 0, "solo nota", now)
	// la nota no está en claro en la base
	_ = a.db.Close
	var crudo []byte
	_ = a.db.QueryRow(`SELECT nota_cifrado FROM msg_marca WHERE msg_id='M1' AND etiqueta_id=?`, e1).Scan(&crudo)
	if len(crudo) == 0 || bytes.Contains(crudo, []byte(nota)) {
		t.Fatal("la nota debe ir cifrada")
	}
	ms, _ := a.MarcasDeMensaje(1, c, "M1")
	if len(ms) != 2 {
		t.Fatalf("%+v", ms)
	}
	lista, err := a.ListarMarcados(1, FiltroMarcados{Tipo: MarcadoMensaje}, 10, 0)
	if err != nil || len(lista) != 2 || lista[0].MsgID != "M2" || lista[0].Nota != "solo nota" || lista[1].Nota != nota || len(lista[1].Etiquetas) != 2 {
		t.Fatalf("%+v %v", lista, err)
	}
	if l, _ := a.ListarMarcados(1, FiltroMarcados{Tipo: MarcadoMensaje, Etiqueta: e2}, 10, 0); len(l) != 1 || l[0].MsgID != "M1" || l[0].Texto != "texto uno" {
		t.Fatalf("%+v", l)
	}
	if l, _ := a.ListarMarcados(1, FiltroMarcados{Tipo: MarcadoMensaje}, 1, 1); len(l) != 1 || l[0].MsgID != "M1" {
		t.Fatalf("paginar: %+v", l)
	}
	if n := a.ContarMarcados(1, 0)[MarcadoMensaje]; n != 2 {
		t.Fatalf("%d", n)
	}
	// etiquetas por lote en un chat
	if m := a.EtiquetasDeMensajes(1, c, []string{"M1", "M2"}); len(m["M1"]) != 2 {
		t.Fatalf("%v", m)
	}
	// chats con etiqueta
	_ = a.PonerEtiqueta(1, c, e1, true)
	if l, _ := a.ListarMarcados(1, FiltroMarcados{Tipo: MarcadoChat, Etiqueta: e1}, 10, 0); len(l) != 1 || l[0].Chat != c {
		t.Fatalf("%+v", l)
	}
	if l, _ := a.ListarMarcados(1, FiltroMarcados{Tipo: MarcadoGrupo}, 10, 0); len(l) != 0 {
		t.Fatalf("%+v", l)
	}
	// borrar la etiqueta: la marca con nota queda como "solo nota"; la que no tenía nota desaparece
	if err := a.BorrarEtiqueta(1, e1); err != nil {
		t.Fatal(err)
	}
	ms, _ = a.MarcasDeMensaje(1, c, "M1")
	if len(ms) != 2 || ms[0].EtiquetaID != 0 && ms[1].EtiquetaID != 0 {
		t.Fatalf("%+v", ms)
	}
	if err := a.BorrarEtiqueta(1, e2); err != nil {
		t.Fatal(err)
	}
	ms, _ = a.MarcasDeMensaje(1, c, "M1")
	if len(ms) != 1 || ms[0].Nota != nota {
		t.Fatalf("%+v", ms)
	}
	// borrar el mensaje para mí limpia la marca
	if _, err := a.BorrarParaMi(1, c, []string{"M1"}); err != nil {
		t.Fatal(err)
	}
	if ms, _ := a.MarcasDeMensaje(1, c, "M1"); len(ms) != 0 {
		t.Fatal("la marca debía borrarse con el mensaje")
	}
	// borrar el chat limpia el resto
	if _, err := a.BorrarChat(1, c); err != nil {
		t.Fatal(err)
	}
	if n := a.ContarMarcados(1, 0)[MarcadoMensaje]; n != 0 {
		t.Fatalf("%d", n)
	}
}

func TestTopeProtegePersonasEtiquetadasYMensajesMarcados(t *testing.T) {
	a := nuevo(t)
	c1, c2, c3 := "1@s.whatsapp.net", "2@s.whatsapp.net", "3@s.whatsapp.net"
	for _, c := range []string{c1, c2, c3} {
		chatBasico(t, a, c, "52551234"+c[:1], "X", false)
	}
	base := time.Now().Add(-48 * time.Hour).Unix()
	var ms []*Mensaje
	for i, c := range []string{c1, c2, c3, c3} {
		m := conArchivo(t, a, c, string(rune('A'+i)), TipoDocumento, 10000, base+int64(i))
		viejoEnDisco(t, a, m)
		ms = append(ms, m)
	}
	id, _ := a.CrearEtiquetaEmoji(1, "No borrar", "", "")
	_ = a.PonerEtiquetaPersona(1, c1, id, true) // persona etiquetada: su chat privado
	_ = a.MarcarMensaje(1, c3, "D", id, "", time.Now())
	r, err := a.AplicarTope(1, 1000, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	for i, m := range ms {
		est := estadoArch(t, a, m.ChatJID, m.ID)
		protegido := i == 0 || i == 3
		if protegido && est != EstadoDescargado {
			t.Fatalf("%s debía quedar (%s)", m.ID, est)
		}
		if !protegido && est != EstadoBorrado {
			t.Fatalf("%s debía borrarse (%s) %+v", m.ID, est, r)
		}
	}
	// una persona etiquetada solo por su LID, con el archivo escrito en un grupo
	g := "g1@g.us"
	chatBasico(t, a, g, "", "Grupo", true)
	lid, pn := "777@lid", "5215599999999@s.whatsapp.net"
	a.AprenderAlias(1, pn, lid)
	m := msg(g, "G1", base, "")
	m.RemitenteJID, m.Tipo, m.ArchivoEstado = lid, TipoDocumento, EstadoDescargado
	m.ArchivoRuta, m.ArchivoBytes, m.ArchivoMime, m.ArchivoInfo = RutaMedio(1, g, "G1"), 5000, "application/pdf", []byte(`{}`)
	_ = a.EscribirCifrado(m.ArchivoRuta, bytes.Repeat([]byte{1}, 5000))
	if _, err := a.GuardarMensaje(m); err != nil {
		t.Fatal(err)
	}
	viejoEnDisco(t, a, m)
	_ = a.PonerEtiquetaPersona(1, pn, id, true)
	_, _ = a.AplicarTope(1, 100, time.Now())
	if estadoArch(t, a, g, "G1") != EstadoDescargado {
		t.Fatal("lo que escribió una persona etiquetada (por alias) no se borra")
	}
}

func TestOlvidarPersonaLimpiaEtiquetasYMarcas(t *testing.T) {
	a := nuevo(t)
	g := "g1@g.us"
	chatBasico(t, a, g, "", "Grupo", true)
	p := "5215500000009@s.whatsapp.net"
	m := msg(g, "Q1", time.Now().Unix()-100, "hola")
	m.RemitenteJID = p
	_, _ = a.GuardarMensaje(m)
	id, _ := a.CrearEtiquetaEmoji(1, "T", "", "")
	_ = a.PonerEtiquetaPersona(1, p, id, true)
	_ = a.MarcarMensaje(1, g, "Q1", id, "n", time.Now())
	_, _ = a.ActualizarMiembros(1, g, []MiembroGrupo{{Persona: p, JID: p, Rol: RolGrupoMiembro}}, time.Now())
	ex, _ := a.DatosDePersona(1, p, time.Now())
	if len(ex.Etiquetas) != 1 {
		t.Fatalf("exportar etiquetas: %v", ex.Etiquetas)
	}
	if _, err := a.OlvidarPersona(1, p); err != nil {
		t.Fatal(err)
	}
	var n int
	for _, q := range []string{"persona_etiqueta", "msg_marca", "grupo_miembros"} {
		_ = a.db.QueryRow(`SELECT COUNT(*) FROM ` + q).Scan(&n)
		if n != 0 {
			t.Fatalf("%s quedó con %d", q, n)
		}
	}
}

func TestFotoDePersonaSinChatYReparar(t *testing.T) {
	a := nuevo(t)
	p := "5215500000011@s.whatsapp.net"
	rel := RutaFotoPersona(1, p)
	if err := a.EscribirCifrado(rel, []byte("jpg")); err != nil {
		t.Fatal(err)
	}
	_ = a.FijarFotoPersona(1, p, rel, "id1", time.Now().Unix())
	if a.FotoDePersona(1, p) != rel {
		t.Fatal("foto")
	}
	if _, err := a.Reparar(1); err != nil {
		t.Fatal(err)
	}
	abs, _ := a.Abs(rel)
	if _, err := os.Stat(abs); err != nil {
		t.Fatal("Reparar no debe borrar la foto de una persona")
	}
	_ = filepath.Base
	a.borrarFotoPersona(1, []string{p})
	if _, err := os.Stat(abs); err == nil {
		t.Fatal("debía borrarse")
	}
}
