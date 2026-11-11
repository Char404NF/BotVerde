package auto

import (
	"strings"
	"testing"
	"time"

	"botchat/internal/almacen"
)

func mx(t *testing.T) *time.Location {
	t.Helper()
	l, err := time.LoadLocation("America/Mexico_City")
	if err != nil {
		t.Fatal(err)
	}
	return l
}

func en(l *time.Location, y int, m time.Month, d, h, mi int) time.Time {
	return time.Date(y, m, d, h, mi, 0, 0, l)
}

func TestHorarioConDosRangosPorDia(t *testing.T) {
	l := mx(t)
	h := SemanaLaboral() // lunes a viernes 9-13 y 15-18
	casos := []struct {
		t    time.Time
		esp  bool
		desc string
	}{
		{en(l, 2026, 11, 9, 10, 0), true, "lunes 10:00 (primer rango)"},
		{en(l, 2026, 11, 9, 13, 0), false, "lunes 13:00 (el final no cuenta)"},
		{en(l, 2026, 11, 9, 14, 0), false, "lunes 14:00 (entre los dos rangos)"},
		{en(l, 2026, 11, 9, 15, 0), true, "lunes 15:00 (segundo rango)"},
		{en(l, 2026, 11, 9, 17, 59), true, "lunes 17:59"},
		{en(l, 2026, 11, 9, 18, 0), false, "lunes 18:00"},
		{en(l, 2026, 11, 14, 10, 0), false, "sábado"},
		{en(l, 2026, 11, 15, 10, 0), false, "domingo"},
	}
	for _, c := range casos {
		if got := SemanaContiene(h, c.t); got != c.esp {
			t.Errorf("%s: %v, esperaba %v", c.desc, got, c.esp)
		}
	}
	// fuera de horario = lo contrario
	fuera := Cuando{Horario: h, HorarioModo: "fuera"}
	dentro := Cuando{Horario: h, HorarioModo: "dentro"}
	if fuera.Activo(en(l, 2026, 11, 9, 10, 0), l) || !fuera.Activo(en(l, 2026, 11, 9, 14, 0), l) || !fuera.Activo(en(l, 2026, 11, 14, 10, 0), l) {
		t.Fatal("ausencia fuera de horario")
	}
	if !dentro.Activo(en(l, 2026, 11, 9, 10, 0), l) || dentro.Activo(en(l, 2026, 11, 9, 20, 0), l) {
		t.Fatal("dentro de horario")
	}
}

func TestHorarioQueCruzaLaMedianoche(t *testing.T) {
	l := mx(t)
	h := []DiaHorario{{Dia: 5, Rangos: []Rango{{"22:00", "02:00"}}}} // viernes noche
	if !SemanaContiene(h, en(l, 2026, 11, 13, 23, 0)) {
		t.Fatal("viernes 23:00")
	}
	if !SemanaContiene(h, en(l, 2026, 11, 14, 1, 0)) {
		t.Fatal("sábado 01:00 pertenece al rango del viernes")
	}
	if SemanaContiene(h, en(l, 2026, 11, 14, 3, 0)) || SemanaContiene(h, en(l, 2026, 11, 13, 21, 0)) {
		t.Fatal("fuera del rango")
	}
}

func TestFechasEspecialesSeRepitenCadaAnio(t *testing.T) {
	l := mx(t)
	c := Cuando{Fechas: []FechaAnual{{Dia: 25, Mes: 12}, {Dia: 1, Mes: 1}, {Dia: 29, Mes: 2}}}
	for _, y := range []int{2026, 2027, 2031} {
		if !c.Activo(en(l, y, 12, 25, 9, 0), l) || !c.Activo(en(l, y, 1, 1, 23, 59), l) {
			t.Fatalf("año %d: Navidad y Año nuevo", y)
		}
		if c.Activo(en(l, y, 12, 24, 9, 0), l) || c.Activo(en(l, y, 12, 26, 9, 0), l) {
			t.Fatalf("año %d: solo ese día", y)
		}
	}
	// 29 de febrero: en año no bisiesto se celebra el 28; en bisiesto, el 29
	if !c.Activo(en(l, 2027, 2, 28, 9, 0), l) || c.Activo(en(l, 2028, 2, 28, 9, 0), l) || !c.Activo(en(l, 2028, 2, 29, 9, 0), l) {
		t.Fatal("29 de febrero")
	}
}

func TestVacacionesIncluyenLosDosExtremos(t *testing.T) {
	l := mx(t)
	c := Cuando{Vacaciones: []Vacacion{{Desde: "2026-12-20", Hasta: "2027-01-05"}}}
	if !c.Activo(en(l, 2026, 12, 20, 0, 0), l) || !c.Activo(en(l, 2027, 1, 5, 23, 59), l) || !c.Activo(en(l, 2026, 12, 31, 12, 0), l) {
		t.Fatal("dentro del rango")
	}
	if c.Activo(en(l, 2026, 12, 19, 23, 59), l) || c.Activo(en(l, 2027, 1, 6, 0, 0), l) {
		t.Fatal("fuera del rango")
	}
}

func TestLaZonaHorariaCambiaElMomento(t *testing.T) {
	cdmx, tokio := mx(t), func() *time.Location { l, _ := time.LoadLocation("Asia/Tokyo"); return l }()
	h := SemanaLaboral()
	// 10:00 del martes en Ciudad de México = 01:00 del miércoles en Tokio
	t0 := en(cdmx, 2026, 11, 10, 10, 0)
	dentro := Cuando{Horario: h, HorarioModo: "dentro"}
	if !dentro.Activo(t0, cdmx) {
		t.Fatal("en la zona del bot es horario laboral")
	}
	dentro.Zona = "Asia/Tokyo"
	if dentro.Activo(t0, cdmx) {
		t.Fatal("con zona de Tokio son las 01:00: fuera de horario")
	}
	_ = tokio
	if (Cuando{Siempre: true}).Activo(t0, cdmx) != true {
		t.Fatal("siempre")
	}
}

func TestQuien(t *testing.T) {
	ana, ben := "5215511111111@s.whatsapp.net", "5215522222222@s.whatsapp.net"
	pers := func(clave string, grupo bool, conocida bool) Persona {
		chat := clave
		if grupo {
			chat = "g1@g.us"
		}
		return Persona{Clave: clave, Chat: chat, EsGrupo: grupo, Conocida: func() bool { return conocida }}
	}
	todos := Quien{Personas: "todos", Privados: true}
	if !todos.Aplica(pers(ana, false, false)) || todos.Aplica(pers(ana, true, false)) {
		t.Fatal("todos, solo privados")
	}
	grupos := Quien{Personas: "todos", Privados: true, Grupos: true}
	if !grupos.Aplica(pers(ana, true, false)) {
		t.Fatal("grupos sí")
	}
	elegidos := Quien{Personas: "todos", Grupos: true, GruposElegidos: []string{"g2@g.us"}}
	if elegidos.Aplica(pers(ana, true, false)) {
		t.Fatal("grupo no elegido")
	}
	elegidos.GruposElegidos = []string{"g1@g.us"}
	if !elegidos.Aplica(pers(ana, true, false)) {
		t.Fatal("grupo elegido")
	}
	desc := Quien{Personas: "desconocidos", Privados: true}
	cont := Quien{Personas: "contactos", Privados: true}
	if !desc.Aplica(pers(ana, false, false)) || desc.Aplica(pers(ana, false, true)) {
		t.Fatal("solo desconocidos")
	}
	if cont.Aplica(pers(ana, false, false)) || !cont.Aplica(pers(ana, false, true)) {
		t.Fatal("solo contactos")
	}
	exc := Quien{Personas: "todos", Privados: true, Excepto: []string{ana}}
	if exc.Aplica(pers(ana, false, false)) || !exc.Aplica(pers(ben, false, false)) {
		t.Fatal("excepto lista")
	}
	solo := Quien{Personas: "todos", Privados: true, SoloLista: []string{ben}}
	if solo.Aplica(pers(ana, false, false)) || !solo.Aplica(pers(ben, false, false)) {
		t.Fatal("solo lista")
	}
	// una persona escrita solo con su número también coincide, y con su LID mediante la persona canónica
	soloNum := Quien{Personas: "todos", Privados: true, SoloLista: []string{"5215522222222"}}
	if !soloNum.Aplica(pers(ben, false, false)) {
		t.Fatal("número sin dominio")
	}
	lid := "9988776655@lid"
	p := pers(ben, false, false)
	p.Canon = func(x string) string {
		if x == lid {
			return ben
		}
		return x
	}
	soloLid := Quien{Personas: "todos", Privados: true, SoloLista: []string{lid}}
	if !soloLid.Aplica(p) {
		t.Fatal("el LID de la lista es la misma persona canónica")
	}
}

func TestValidar(t *testing.T) {
	mk := func(tipo, texto string) (*almacen.AutoMensaje, *Config) {
		c := ConfigPorDefecto(tipo)
		return &almacen.AutoMensaje{Tipo: tipo, Texto: texto, Activo: true}, &c
	}
	for _, tipo := range []string{almacen.AutoBienvenida, almacen.AutoAusencia, almacen.AutoRecibido, almacen.AutoCierre} {
		m, c := mk(tipo, "Hola")
		if err := Validar(m, c); err != nil {
			t.Fatalf("%s por defecto debe ser válido: %v", tipo, err)
		}
		if m.Nombre == "" {
			t.Fatalf("%s: nombre por defecto", tipo)
		}
	}
	m, c := mk(almacen.AutoRecibido, "  ")
	if Validar(m, c) == nil {
		t.Fatal("sin texto ni archivos")
	}
	m, c = mk(almacen.AutoAusencia, "x")
	c.Cuando = Cuando{}
	if err := Validar(m, c); err == nil || !strings.Contains(err.Error(), "Elige cuándo") {
		t.Fatalf("ausencia sin cuándo: %v", err)
	}
	m, c = mk(almacen.AutoAusencia, "x")
	c.Cuando.Horario[0].Rangos = append(c.Cuando.Horario[0].Rangos, Rango{"19:00", "20:00"})
	if err := Validar(m, c); err == nil || !strings.Contains(err.Error(), "dos rangos") {
		t.Fatalf("tres rangos: %v", err)
	}
	m, c = mk(almacen.AutoAusencia, "x")
	c.Cuando.Zona = "Marte/Olimpo"
	if Validar(m, c) == nil {
		t.Fatal("zona inexistente")
	}
	m, c = mk(almacen.AutoAusencia, "x")
	c.Cuando = Cuando{Fechas: []FechaAnual{{Dia: 31, Mes: 4}}}
	if Validar(m, c) == nil {
		t.Fatal("31 de abril")
	}
	m, c = mk(almacen.AutoAusencia, "x")
	c.Cuando = Cuando{Vacaciones: []Vacacion{{Desde: "2026-12-20", Hasta: "2026-12-01"}}}
	if Validar(m, c) == nil {
		t.Fatal("vacaciones al revés")
	}
	m, c = mk(almacen.AutoRecibido, "x")
	c.Repeticion = Repeticion{Cada: 5, Unidad: "minutos"}
	if Validar(m, c) == nil {
		t.Fatal("recibido necesita al menos una hora entre envíos")
	}
	m, c = mk(almacen.AutoRecibido, "x")
	c.Quien.Privados, c.Quien.Grupos = false, false
	if Validar(m, c) == nil {
		t.Fatal("ni privados ni grupos")
	}
	// atajos
	m = &almacen.AutoMensaje{Tipo: almacen.AutoAtajo, Texto: "Gracias", Nombre: " /Gracias "}
	cc := Config{}
	if err := Validar(m, &cc); err != nil || m.Nombre != "gracias" {
		t.Fatalf("atajo: %v %q", err, m.Nombre)
	}
	for _, malo := range []string{"", "con espacio", "a/b", "no.vale", "muuuuuuuuuuuuuuuuuuuuuuuy-largo"} {
		m = &almacen.AutoMensaje{Tipo: almacen.AutoAtajo, Texto: "x", Nombre: malo}
		if Validar(m, &cc) == nil {
			t.Fatalf("nombre de atajo no válido: %q", malo)
		}
	}
	// adjuntos: máximo 3, con id válido y los documentos con nombre
	ok := almacen.AdjuntoRef{ID: strings.Repeat("a", 24), Tipo: almacen.AdjImagen}
	m, c = mk(almacen.AutoRecibido, "x")
	m.Adjuntos = []almacen.AdjuntoRef{ok, ok, ok, ok}
	if Validar(m, c) == nil {
		t.Fatal("máximo 3 archivos")
	}
	m, c = mk(almacen.AutoRecibido, "x")
	m.Adjuntos = []almacen.AdjuntoRef{{ID: strings.Repeat("a", 24), Tipo: almacen.AdjDocumento}}
	if Validar(m, c) == nil {
		t.Fatal("documento sin nombre")
	}
	m, c = mk(almacen.AutoRecibido, "x")
	m.Adjuntos = []almacen.AdjuntoRef{{ID: "../../etc/passwd", Tipo: almacen.AdjImagen}}
	if Validar(m, c) == nil {
		t.Fatal("id de archivo no válido")
	}
}

func TestDescribirSemana(t *testing.T) {
	got := DescribirSemana(append(SemanaLaboral(), DiaHorario{Dia: 6, Rangos: []Rango{{"09:00", "13:00"}}}))
	if !strings.Contains(got, "de lunes a viernes de 09:00 a 13:00 y de 15:00 a 18:00") || !strings.Contains(got, "sábado de 09:00 a 13:00") {
		t.Fatal(got)
	}
	if DescribirSemana(nil) != "todos los días" {
		t.Fatal("sin horario")
	}
}

func TestPlantillasSonValidas(t *testing.T) {
	ps := Plantillas(time.Date(2026, 11, 10, 0, 0, 0, 0, time.UTC))
	vistos := map[string]int{}
	for _, p := range ps {
		vistos[p.Tipo]++
		m := &almacen.AutoMensaje{Tipo: p.Tipo, Texto: p.Texto, Nombre: NombreSugerido(p)}
		c := p.Config
		if err := Validar(m, &c); err != nil {
			t.Errorf("la plantilla %q no valida: %v", p.Nombre, err)
		}
	}
	for _, tipo := range []string{almacen.AutoBienvenida, almacen.AutoAusencia, almacen.AutoRecibido, almacen.AutoCierre, almacen.AutoAtajo} {
		if vistos[tipo] == 0 {
			t.Errorf("faltan plantillas de %s", tipo)
		}
	}
}
