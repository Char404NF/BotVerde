package almacen

import (
	"bytes"
	"testing"
	"time"
)

func TestLabSesionCifradaYReemplazo(t *testing.T) {
	a := nuevo(t)
	ahora := time.Now()
	s1 := SesionLab{ID: "A1", Chat: pnAna, Persona: pnAna, Tipo: LabEncuesta, TS: ahora.Unix(), ExpiraTS: ahora.Add(time.Hour).Unix(),
		Secreto: bytes.Repeat([]byte{7}, 32), Opciones: []OpcionLab{{Num: 1, Texto: "Horario"}, {Num: 2, Texto: "Precios"}}}
	if err := a.GuardarSesionLab(1, s1); err != nil {
		t.Fatal(err)
	}
	// en disco no queda nada legible
	var sec, ops []byte
	if err := a.db.QueryRow(`SELECT secreto_cifrado, opciones_cifrado FROM poll_sessions WHERE id='A1'`).Scan(&sec, &ops); err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(ops, []byte("Precios")) || bytes.Contains(sec, bytes.Repeat([]byte{7}, 32)) {
		t.Fatal("la sesión debe ir cifrada")
	}
	g := a.ObtenerSesionLab(1, "A1")
	if g == nil || len(g.Opciones) != 2 || g.Opciones[1].Texto != "Precios" || !bytes.Equal(g.Secreto, s1.Secreto) || g.Estado != LabActiva {
		t.Fatalf("lectura: %+v", g)
	}
	// una sesión nueva de la misma persona reemplaza a la anterior; la de otra persona no
	s2 := s1
	s2.ID = "A2"
	s3 := s1
	s3.ID, s3.Chat, s3.Persona = "B1", pnLuis, pnLuis
	_ = a.GuardarSesionLab(1, s3)
	_ = a.GuardarSesionLab(1, s2)
	if a.ObtenerSesionLab(1, "A1").Estado != LabReemplazada || a.ObtenerSesionLab(1, "A2").Estado != LabActiva || a.ObtenerSesionLab(1, "B1").Estado != LabActiva {
		t.Fatal("reemplazo mal")
	}
	// las de prueba no reemplazan a las de menú
	p := s1
	p.ID, p.Origen = "P1", LabOrigenPrueba
	_ = a.GuardarSesionLab(1, p)
	if a.ObtenerSesionLab(1, "A2").Estado != LabActiva {
		t.Fatal("una prueba no debe reemplazar un menú")
	}
	if a.ObtenerSesionLab(1, "nada") != nil {
		t.Fatal("no existe")
	}
}

func TestLabVotosGuardanCambianYSeBorran(t *testing.T) {
	a := nuevo(t)
	ahora := time.Now()
	if _, ok := a.ObtenerVotoLab(1, "A1", pnAna); ok {
		t.Fatal("no votó")
	}
	_ = a.GuardarVotoLab(1, "A1", pnAna, []int{2}, ahora)
	v, ok := a.ObtenerVotoLab(1, "A1", pnAna)
	if !ok || len(v.Nums) != 1 || v.Nums[0] != 2 {
		t.Fatalf("voto: %+v", v)
	}
	var raw []byte
	_ = a.db.QueryRow(`SELECT seleccion_cifrado FROM poll_votos`).Scan(&raw)
	if bytes.Contains(raw, []byte("[2]")) {
		t.Fatal("el voto va cifrado")
	}
	_ = a.GuardarVotoLab(1, "A1", pnAna, nil, ahora) // retiro
	v, ok = a.ObtenerVotoLab(1, "A1", pnAna)
	if !ok || len(v.Nums) != 0 {
		t.Fatalf("retiro: %+v", v)
	}
	// olvidar a la persona borra sus sesiones y votos
	llenaPersona(t, a)
	_ = a.GuardarSesionLab(1, SesionLab{ID: "A1", Chat: pnAna, Persona: pnAna, TS: ahora.Unix(), ExpiraTS: ahora.Unix() + 100})
	_ = a.GuardarSesionLab(1, SesionLab{ID: "B1", Chat: pnLuis, Persona: pnLuis, TS: ahora.Unix(), ExpiraTS: ahora.Unix() + 100})
	if _, err := a.OlvidarPersona(1, pnAna); err != nil {
		t.Fatal(err)
	}
	if a.ObtenerSesionLab(1, "A1") != nil || a.ObtenerSesionLab(1, "B1") == nil {
		t.Fatal("olvidar debía borrar solo lo de Ana")
	}
	if _, ok := a.ObtenerVotoLab(1, "A1", pnAna); ok {
		t.Fatal("el voto debía borrarse")
	}
}

func TestLabResultadosYTopes(t *testing.T) {
	a := nuevo(t)
	chatBasico(t, a, pnAna, "5215511111111", "Ana", false)
	ahora := time.Now()
	id, err := a.NuevoResultadoLab(1, "encuesta", "12", pnAna, "2.26.1", "2.3000.1", ahora)
	if err != nil || id == 0 {
		t.Fatal(err)
	}
	a.ResultadoLabEnviado(1, id, "M1", true, "")
	l := a.ListarResultadosLab(1, 10)
	if len(l) != 1 || l[0].Enviado != 1 || l[0].Llego != -1 || l[0].Visible != -1 || l[0].VersionWA != "2.26.1" || l[0].VersionCliente != "2.3000.1" {
		t.Fatalf("lista: %+v", l)
	}
	// un acuse de entrega de ese mensaje lo marca como llegado
	mio := msg(pnAna, "M1", ahora.Unix(), "x")
	mio.EsMio, mio.RemitenteJID = true, ""
	_, _ = a.GuardarMensaje(mio)
	if _, err := a.GuardarAcuse(1, pnAna, "M1", pnAna, AcuseEntregado, ahora.Unix()); err != nil {
		t.Fatal(err)
	}
	if l = a.ListarResultadosLab(1, 10); l[0].Llego != 1 {
		t.Fatalf("llegó: %+v", l[0])
	}
	if err := a.MarcarResultadoLab(1, id, "visible", 1); err != nil {
		t.Fatal(err)
	}
	if err := a.MarcarResultadoLab(1, id, "tocable", 0); err != nil {
		t.Fatal(err)
	}
	if a.MarcarResultadoLab(1, id, "llego", 1) == nil || a.MarcarResultadoLab(1, id, "visible", 5) == nil || a.MarcarResultadoLab(1, 999, "visible", 1) == nil {
		t.Fatal("debía rechazar campo/valor/id malos")
	}
	if l = a.ListarResultadosLab(1, 10); l[0].Visible != 1 || l[0].Tocable != 0 {
		t.Fatalf("marcas: %+v", l[0])
	}
	// un envío fallido
	id2, _ := a.NuevoResultadoLab(1, "botones", "3", pnAna, "", "", ahora)
	a.ResultadoLabEnviado(1, id2, "", false, "no soportado")
	if l = a.ListarResultadosLab(1, 10); l[0].Enviado != 0 || l[0].Llego != 0 || l[0].Error != "no soportado" {
		t.Fatalf("fallido: %+v", l[0])
	}
	if a.ContarEnviosLab(1, ahora.Add(-time.Hour)) != 2 || a.ContarEnviosLab(1, ahora.Add(time.Hour)) != 0 {
		t.Fatal("conteo")
	}
	if n := a.BorrarResultadosLab(1); n != 2 || len(a.ListarResultadosLab(1, 10)) != 0 {
		t.Fatal("borrar")
	}
}

func TestLabAjustesPorDefectoApagados(t *testing.T) {
	a := nuevo(t)
	if a.AjusteBot(1, "lab_activo") != "0" || a.AjusteBot(1, "lab_modo_menu") != "texto" || a.AjusteBot(1, "lab_grupos") != "0" {
		t.Fatal("el Laboratorio debe nacer apagado, en texto y solo privado")
	}
	if a.Version() != len(migraciones) || len(migraciones) != 23 {
		t.Fatalf("versión %d", a.Version())
	}
}
