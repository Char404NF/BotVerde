package almacen

import "testing"

func TestFijarChatSubeArribaYVaciarConservaElChat(t *testing.T) {
	a := nuevo(t)
	const c1, c2 = "5215500000001@s.whatsapp.net", "5215500000002@s.whatsapp.net"
	chatBasico(t, a, c1, "5215500000001", "Ana", false)
	chatBasico(t, a, c2, "5215500000002", "Beto", false)
	for i, c := range []string{c1, c2} {
		if _, err := a.GuardarMensaje(msg(c, "m1", int64(1000+i*100), "hola")); err != nil {
			t.Fatal(err)
		}
	}
	l, _ := a.ListarChats(1, false, "", 10, 0)
	if len(l) != 2 || l[0].JID != c2 || l[0].Fijado {
		t.Fatalf("orden normal: %+v", l)
	}
	if err := a.FijarChat(1, c1, true); err != nil {
		t.Fatal(err)
	}
	l, _ = a.ListarChats(1, false, "", 10, 0)
	if l[0].JID != c1 || !l[0].Fijado || l[1].Fijado {
		t.Fatalf("el fijado va primero: %+v", l)
	}
	if !a.EstadoDeChat(1, c1).Fijado {
		t.Fatal("el estado debe decir fijado")
	}
	_ = a.Silenciar(1, c1, true)
	if _, err := a.VaciarChat(1, c1); err != nil {
		t.Fatal(err)
	}
	ms, _ := a.ListarMensajes(1, c1, 0, "", 50)
	if len(ms) != 0 {
		t.Fatalf("quedaron mensajes: %d", len(ms))
	}
	if ch, _ := a.ObtenerChat(1, c1); ch == nil {
		t.Fatal("el chat debe seguir existiendo")
	}
	if e := a.EstadoDeChat(1, c1); !e.Fijado || !e.Silenciado {
		t.Fatalf("fijado y silencio se conservan: %+v", e)
	}
	if ms, _ := a.ListarMensajes(1, c2, 0, "", 50); len(ms) != 1 {
		t.Fatal("el otro chat no se toca")
	}
	if err := a.FijarChat(1, c1, false); err != nil || a.EstadoDeChat(1, c1).Fijado {
		t.Fatal("soltar")
	}
}
