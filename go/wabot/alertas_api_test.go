package wabot

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"botchat/internal/almacen"
)

func TestVigilanteAvisaDesconexionLargaYCuandoVuelve(t *testing.T) {
	var mu sync.Mutex
	ahora := time.Unix(1000, 0)
	conectado := false
	var largo, vuelve []int
	v := &vigilante{vinculado: func() bool { return true }, conectado: func() bool { mu.Lock(); defer mu.Unlock(); return conectado },
		conectar: func() {}, log: func(string) {}, limite: 3 * time.Minute, paso: 5 * time.Millisecond,
		ahora:     func() time.Time { mu.Lock(); defer mu.Unlock(); return ahora },
		avisoTras: func() time.Duration { return 10 * time.Minute },
		alLargo:   func(m int) { mu.Lock(); largo = append(largo, m); mu.Unlock() },
		alVolver:  func(m int) { mu.Lock(); vuelve = append(vuelve, m); mu.Unlock() }}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go v.correr(ctx)
	avanza := func(d time.Duration) { mu.Lock(); ahora = ahora.Add(d); mu.Unlock(); time.Sleep(40 * time.Millisecond) }
	time.Sleep(30 * time.Millisecond)
	avanza(5 * time.Minute)
	mu.Lock()
	if len(largo) != 0 {
		t.Fatal("a los 5 minutos aún no avisa")
	}
	mu.Unlock()
	avanza(7 * time.Minute)
	avanza(5 * time.Minute) // no repite el aviso
	mu.Lock()
	if len(largo) != 1 || largo[0] < 10 {
		t.Fatalf("debía avisar una sola vez: %v", largo)
	}
	conectado = true
	mu.Unlock()
	avanza(time.Minute)
	mu.Lock()
	if len(vuelve) != 1 || vuelve[0] < 12 {
		t.Fatalf("debía avisar que volvió: %v", vuelve)
	}
	mu.Unlock()
}

func TestAlertasAjustesYTope(t *testing.T) {
	reinicia(t)
	if err := AbrirAlmacen(t.TempDir(), clavePrueba(), &obsFalso{}); err != nil {
		t.Fatal(err)
	}
	j, err := AlertasLeer(1)
	if err != nil || !strings.Contains(j, `"wa":false`) || !strings.Contains(j, `"desconexion":10`) || !strings.Contains(j, "reporte") {
		t.Fatalf("valores de fábrica: %s %v", j, err)
	}
	if err := AlertasGuardar(1, `{"wa":true,"tipos":["reporte","raro","grupos"],"desconexion":5,"resumen":true}`); err != nil {
		t.Fatal(err)
	}
	j, _ = AlertasLeer(1)
	if !strings.Contains(j, `"wa":true`) || !strings.Contains(j, `"desconexion":5`) || !strings.Contains(j, `"resumen":true`) || strings.Contains(j, "raro") {
		t.Fatalf("guardado: %s", j)
	}
	if err := AlertasGuardar(1, `{"wa":true,"tipos":[],"desconexion":999,"resumen":false}`); err == nil {
		t.Fatal("999 minutos no es válido")
	}
	if !tipoAlertable("reporte,grupos", "grupos") || tipoAlertable("reporte,grupos", "filtros") {
		t.Fatal("tipoAlertable")
	}
	alertaMu.Lock()
	alertaMarca = nil
	alertaMu.Unlock()
	ahora := time.Now()
	for i := 0; i < alertasPorHora; i++ {
		if !tomarAlerta(ahora) {
			t.Fatalf("la alerta %d debía pasar", i)
		}
	}
	if tomarAlerta(ahora) {
		t.Fatal("pasado el tope por hora no debe pasar")
	}
	if !tomarAlerta(ahora.Add(61 * time.Minute)) {
		t.Fatal("pasada la hora vuelve a pasar")
	}
}

func TestAvisoLlamaAlCallback(t *testing.T) {
	reinicia(t)
	if err := AbrirAlmacen(t.TempDir(), clavePrueba(), &obsFalso{}); err != nil {
		t.Fatal(err)
	}
	a, _ := almacenAbierto()
	ch := make(chan string, 1)
	a.AlAviso = func(_ int64, tipo, _, titulo, _ string) { ch <- tipo + ":" + titulo }
	if _, err := a.CrearAviso(1, "conexion", "", "Sin conexión", "x"); err != nil {
		t.Fatal(err)
	}
	select {
	case v := <-ch:
		if v != "conexion:Sin conexión" {
			t.Fatal(v)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no llamó")
	}
	_ = almacen.TipoTexto
}

func TestResumenSemanal(t *testing.T) {
	reinicia(t)
	if err := AbrirAlmacen(t.TempDir(), clavePrueba(), &obsFalso{}); err != nil {
		t.Fatal(err)
	}
	a, _ := almacenAbierto()
	g := "120363000000000055@g.us"
	if _, err := a.ActualizarChat(1, g, true, func(c *almacen.Chat) { c.Guardado = "Vecinos" }); err != nil {
		t.Fatal(err)
	}
	if t0, _ := ResumenSemanalAhora(1); t0 != "" {
		t.Fatalf("sin actividad no hay resumen: %q", t0)
	}
	ana := "34600000001@s.whatsapp.net"
	for i := 0; i < 5; i++ {
		_, _ = a.GuardarMensaje(&almacen.Mensaje{ID: "S" + string(rune('a'+i)), ChatJID: g, BotID: 1, RemitenteJID: ana, RemitenteNombre: "Ana", Tipo: almacen.TipoTexto, Texto: "hola", TS: time.Now().Unix() - int64(i)})
	}
	a.RegistrarAccion(1, almacen.PersonaDueno, "filtro.flood", ana, g, "ok", "")
	txt, err := ResumenSemanalAhora(1)
	if err != nil || !strings.Contains(txt, "Vecinos (5)") || !strings.Contains(txt, "Filtros que actuaron: 1") || strings.Contains(txt, "34600000001") {
		t.Fatalf("%q %v", txt, err)
	}
	l, _ := a.AvisosRecientes(1, 10)
	hay := false
	for _, x := range l {
		if x.Tipo == "resumen" {
			hay = true
		}
	}
	if !hay {
		t.Fatal("debía crear el aviso del resumen")
	}
}
