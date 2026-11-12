package wabot

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"botchat/internal/almacen"
	"botchat/internal/ingesta"
)

func TestApiMultimediaTopeEstadoYPortada(t *testing.T) {
	a := conChats(t)
	defer func() { polMu.Lock(); pol = ingesta.PoliticaPorDefecto(); polMu.Unlock() }()
	if tp, err := TopeAlmacenamiento(1); err != nil || tp != 5<<30 {
		t.Fatal(tp, err)
	}
	if err := FijarTope(1, 12345); err != nil {
		t.Fatal(err)
	}
	if tp, _ := TopeAlmacenamiento(1); tp != 12345 {
		t.Fatal(tp)
	}
	_ = FijarTope(1, 0)
	// Un video descargado sin portada.
	ts := time.Now().Add(-5 * time.Hour).Unix()
	ruta := almacen.RutaMedio(1, pnAna, "VID")
	if err := a.EscribirCifrado(ruta, []byte("0123456789")); err != nil {
		t.Fatal(err)
	}
	_, _ = a.GuardarMensaje(&almacen.Mensaje{ID: "VID", ChatJID: pnAna, BotID: 1, RemitenteJID: pnAna, Tipo: almacen.TipoVideo, TS: ts, ArchivoRuta: ruta, ArchivoBytes: 10, ArchivoEstado: almacen.EstadoDescargado})
	if err := ConfigurarDescargas(`{"topeArchivo":{"video":1000},"sticker":{"wifi":true,"datos":true,"apagada":true}}`); err != nil {
		t.Fatal(err)
	}
	var st struct {
		Uso        int64 `json:"uso"`
		SinPortada int   `json:"sinPortada"`
		Tipos      []struct {
			Tipo        string `json:"tipo"`
			Bytes       int64  `json:"bytes"`
			Apagado     bool   `json:"apagado"`
			TopeArchivo int64  `json:"topeArchivo"`
		} `json:"tipos"`
	}
	s, err := EstadoAlmacenamiento(1)
	if err != nil || json.Unmarshal([]byte(s), &st) != nil {
		t.Fatal(s, err)
	}
	if st.SinPortada != 1 || len(st.Tipos) != 5 || st.Uso == 0 {
		t.Fatalf("%s", s)
	}
	for _, x := range st.Tipos {
		if x.Tipo == "video" && (x.Bytes != 10 || x.TopeArchivo != 1000) || x.Tipo == "sticker" && !x.Apagado {
			t.Fatalf("%+v", x)
		}
	}
	var sp []almacen.SinPortada
	s, _ = VideosSinPortada(1, 10)
	if json.Unmarshal([]byte(s), &sp) != nil || len(sp) != 1 || sp[0].ID != "VID" {
		t.Fatal(s)
	}
	if GuardarPortada(1, pnAna, "VID", []byte("no es jpeg"), 10, 10) == nil {
		t.Fatal("debe exigir un JPEG")
	}
	if err := GuardarPortada(1, pnAna, "VID", []byte{0xFF, 0xD8, 0xFF, 0xE0, 1, 2, 3}, 10, 10); err != nil {
		t.Fatal(err)
	}
	if s, _ = VideosSinPortada(1, 10); s != "[]" {
		t.Fatal(s)
	}
	// Con tope pequeño, AplicarTopeAhora borra el video (tiene portada y llegó hace horas) y deja su portada.
	if err := FijarTope(1, 1); err != nil {
		t.Fatal(err)
	}
	abs, _ := a.Abs(ruta)
	hace := time.Now().Add(-3 * time.Hour)
	_ = os.Chtimes(abs, hace, hace) // lo recién bajado (por la hora del archivo) no se borra
	if _, err := AplicarTopeAhora(1); err != nil {
		t.Fatal(err)
	}
	m, _ := a.ObtenerMensaje(1, pnAna, "VID")
	if m.ArchivoEstado != almacen.EstadoBorrado || m.PortadaRuta == "" || m.ArchivoBorradoMotivo != "tope" {
		t.Fatalf("%+v", m)
	}
}

func TestEstablecerBateria(t *testing.T) {
	defer bateriaIniciada.Store(false)
	if b := bateriaActual(); b.Nivel != -1 {
		t.Fatal(b)
	}
	EstablecerBateria(12, true)
	if b := bateriaActual(); b.Nivel != 12 || !b.Cargando {
		t.Fatal(b)
	}
	EstablecerBateria(-5, false)
	if bateriaActual().Nivel != -1 {
		t.Fatal("fuera de rango = no se sabe")
	}
}
