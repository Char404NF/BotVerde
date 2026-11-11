package almacen

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestMigracion18TablasDeModulosH(t *testing.T) {
	a := nuevo(t)
	if a.Version() != len(migraciones) || len(migraciones) < 18 {
		t.Fatalf("versión %d", a.Version())
	}
	for _, tabla := range []string{"mod_listas", "mod_agenda", "mod_dest", "mod_pendientes", "mod_chats", "mod_conectores"} {
		var n int
		if err := a.db.QueryRow(`SELECT COUNT(*) FROM ` + tabla).Scan(&n); err != nil {
			t.Fatalf("%s: %v", tabla, err)
		}
	}
}

func TestListasAgendaYDestinatarios(t *testing.T) {
	a := nuevo(t)
	ahora := time.Now()
	if n, err := a.AgregarALista(1, "suscritos", pnAna, "palabra", ahora); err != nil || !n {
		t.Fatalf("nueva: %v %v", n, err)
	}
	if n, _ := a.AgregarALista(1, "suscritos", pnAna, "palabra", ahora); n {
		t.Fatal("repetida no es nueva")
	}
	_, _ = a.AgregarALista(1, "vip", pnAna, "a mano", ahora)
	_, _ = a.AgregarALista(1, "vip", pnLuis, "a mano", ahora)
	cs := a.ListasConConteo(1, "suscritos", "vacia")
	if len(cs) != 3 || cs[0].Lista != "suscritos" || cs[1].Lista != "vacia" || cs[1].N != 0 || cs[2].Lista != "vip" || cs[2].N != 2 {
		t.Fatalf("conteos: %+v", cs)
	}
	if n := a.QuitarDeTodasLasListas(1, pnAna); n != 2 {
		t.Fatalf("baja de todas las listas: %d", n)
	}
	if a.EstaEnLista(1, "vip", pnAna) || !a.EstaEnLista(1, "vip", pnLuis) {
		t.Fatal("solo salió ella")
	}

	// agenda: el contenido va cifrado
	id, err := a.CrearAgenda(1, AgendaFila{Modulo: "difusion", Nombre: "Oferta", Destino: "lista:vip", Contenido: `{"texto":"SECRETO-PROMO"}`, ProximoTS: ahora.Add(time.Hour).Unix()}, ahora)
	if err != nil {
		t.Fatal(err)
	}
	var crudo []byte
	_ = a.db.QueryRow(`SELECT contenido FROM mod_agenda WHERE id=?`, id).Scan(&crudo)
	if bytes.Contains(crudo, []byte("SECRETO-PROMO")) || len(crudo) == 0 {
		t.Fatal("el contenido de la agenda debe ir cifrado")
	}
	f, _ := a.ObtenerAgenda(1, id)
	if f == nil || !strings.Contains(f.Contenido, "SECRETO-PROMO") || f.Estado != "activo" {
		t.Fatalf("lectura: %+v", f)
	}
	if v, _ := a.AgendaVencida(1, "difusion", ahora); len(v) != 0 {
		t.Fatal("todavía no vence")
	}
	if v, _ := a.AgendaVencida(1, "difusion", ahora.Add(2*time.Hour)); len(v) != 1 {
		t.Fatal("ya vence")
	}
	if a.ProximaAgenda(1, "difusion", "programados") != ahora.Add(time.Hour).Unix() || a.ProximaAgenda(1) != 0 {
		t.Fatal("próxima")
	}
	// destinatarios
	_ = a.AgregarDest(1, id, []string{pnAna, pnLuis})
	a.MarcarDest(1, id, pnAna, "encolado", ahora)
	if a.ContarDest(1, id)["encolado"] != 1 || a.DestEncoladosAPendientes(1) != 1 || a.ContarDest(1, id)["pendiente"] != 2 {
		t.Fatalf("destinos: %v", a.ContarDest(1, id))
	}
	if err := a.ActualizarAgenda(1, id, "enviando", 0, ahora, "iniciada", false); err != nil {
		t.Fatal(err)
	}
	if a.ProximaAgenda(1, "difusion") != 0 {
		t.Fatal("enviando no tiene próxima fecha")
	}
	if err := a.BorrarAgenda(1, id); err != nil {
		t.Fatal(err)
	}
	if len(a.ContarDest(1, id)) != 0 {
		t.Fatal("al borrar la difusión se van sus destinatarios")
	}
}

func TestAgendaMantieneVivosLosArchivosQueUsa(t *testing.T) {
	a := nuevo(t)
	id, _, _ := a.GuardarAdjuntoMenu(1, bytes.NewReader([]byte("img")))
	cont := `{"texto":"x","adjuntos":[{"id":"` + id + `","tipo":"imagen"}]}`
	fid, _ := a.CrearAgenda(1, AgendaFila{Modulo: "programados", Contenido: cont, ProximoTS: 5}, time.Now())
	if !a.ReferenciasAdjuntos(1)[id] {
		t.Fatal("el archivo de un mensaje programado no es huérfano")
	}
	_ = a.ActualizarAgenda(1, fid, "terminada", 0, time.Now(), "ok", true)
	if a.ReferenciasAdjuntos(1)[id] {
		t.Fatal("ya salió: se puede limpiar")
	}
}

func TestConectoresCifradosYVerificacionesYOlvidar(t *testing.T) {
	a := nuevo(t)
	if _, err := a.GuardarConector(1, "clima", `{"cabeceras":"Authorization: Bearer LLAVE-SECRETA"}`); err != nil {
		t.Fatal(err)
	}
	var crudo []byte
	_ = a.db.QueryRow(`SELECT config FROM mod_conectores WHERE nombre='clima'`).Scan(&crudo)
	if bytes.Contains(crudo, []byte("LLAVE-SECRETA")) || len(crudo) == 0 {
		t.Fatal("las llaves del conector van cifradas")
	}
	c, _ := a.ObtenerConector(1, "CLIMA")
	if c == nil || !strings.Contains(c.Config, "LLAVE-SECRETA") {
		t.Fatalf("lectura: %+v", c)
	}
	_, _ = a.GuardarConector(1, "clima", `{"v":2}`)
	if l, _ := a.ListarConectores(1); len(l) != 1 || l[0].Config != `{"v":2}` {
		t.Fatalf("se reemplaza por nombre: %+v", l)
	}
	// verificaciones
	_ = a.PonerPendiente(1, "bienvenida", Pendiente{Chat: "g@g.us", Persona: pnAna, DesdeTS: 1, VenceTS: 9, Esperada: "2"})
	if a.SumarIntentoPendiente(1, "bienvenida", "g@g.us", pnAna) != 1 || len(a.PendientesDe(1, "bienvenida", "g@g.us")) != 1 {
		t.Fatal("pendientes")
	}
	// grupos de un módulo
	_ = a.AgregarChatMod(1, "bienvenida", "g@g.us")
	if !a.EstaChatMod(1, "bienvenida", "g@g.us") || a.EstaChatMod(1, "bienvenida", "otro@g.us") {
		t.Fatal("chats del módulo")
	}
	// olvidar a una persona se lleva sus listas y verificaciones
	_, _ = a.AgregarALista(1, "suscritos", pnAna, "palabra", time.Now())
	if _, err := a.OlvidarPersona(1, pnAna); err != nil {
		t.Fatal(err)
	}
	if a.EstaEnLista(1, "suscritos", pnAna) || len(a.PendientesDe(1, "bienvenida", "")) != 0 {
		t.Fatal("olvidar borra sus listas y verificaciones")
	}
	// y eliminar el bot limpia todo
	_, _ = a.AgregarALista(1, "suscritos", pnLuis, "palabra", time.Now())
	if err := a.EliminarBot(1); err != nil {
		t.Fatal(err)
	}
	for _, tabla := range []string{"mod_listas", "mod_agenda", "mod_dest", "mod_pendientes", "mod_chats", "mod_conectores"} {
		var n int
		_ = a.db.QueryRow(`SELECT COUNT(*) FROM ` + tabla).Scan(&n)
		if n != 0 {
			t.Fatalf("%s no se limpió", tabla)
		}
	}
}

func TestPersonasQueEscribieron(t *testing.T) {
	a := nuevo(t)
	_, _ = a.ActualizarChat(1, pnAna, false, func(c *Chat) {})
	_, _ = a.GuardarMensaje(&Mensaje{ID: "m1", ChatJID: pnAna, BotID: 1, RemitenteJID: pnAna, Tipo: TipoTexto, Texto: "hola", TS: 1})
	_, _ = a.ActualizarChat(1, pnLuis, false, func(c *Chat) {})
	_, _ = a.GuardarMensaje(&Mensaje{ID: "m2", ChatJID: pnLuis, BotID: 1, RemitenteJID: QuienYo, EsMio: true, Tipo: TipoTexto, Texto: "yo le escribí", TS: 2})
	got := a.PersonasQueEscribieron(1)
	if len(got) != 1 || got[0] != pnAna {
		t.Fatalf("solo quien escribió: %v", got)
	}
}
