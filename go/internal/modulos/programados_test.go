package modulos

import (
	"strings"
	"testing"
	"time"
)

func programadosListo(t *testing.T) *ent {
	e, _ := entH(t)
	e.conChat(pnAna, false, "Ana")
	e.activa(IDProgramados)
	e.fija("tope_hora", "100")
	e.fija("tope_dia", "100")
	return e
}

func programa(t *testing.T, e *ent, dest, texto string, cuando time.Time, rep string) int64 {
	t.Helper()
	_, err := e.g.AccionModulo(IDProgramados, "programados.agregar", Args{"nombre": "Aviso", "destino": dest, "texto": texto,
		"fecha": cuando.Format("02/01/2006 15:04"), "repeticion": rep})
	if err != nil {
		t.Fatal(err)
	}
	fs, _ := e.alm.ListarAgenda(1, IDProgramados)
	var mayor int64
	for _, f := range fs {
		if f.ID > mayor {
			mayor = f.ID
		}
	}
	return mayor
}

func TestProgramadosSaleALaHora(t *testing.T) {
	e := programadosListo(t)
	programa(t, e, pnAna, "Hola {nombre}, son las {hora}", e.hora.Add(10*time.Minute), "")
	e.simula(5)
	if e.cuantos() != 0 {
		t.Fatal("antes de la hora no sale")
	}
	e.simula(8)
	if e.cuantos() != 1 || !e.hay("Hola Ana, son las 12:1") {
		t.Fatalf("enviados: %v", e.enviados())
	}
	e.simula(10)
	if e.cuantos() != 1 {
		t.Fatal("una sola vez")
	}
	fs, _ := e.alm.ListarAgenda(1, IDProgramados)
	if fs[0].Estado != pgTerminado {
		t.Fatalf("estado: %s", fs[0].Estado)
	}
	for _, r := range e.registro() {
		if strings.Contains(r.Que+r.Resultado, "Hola") {
			t.Fatal("registro con texto")
		}
	}
}

func TestProgramadosRepeticiones(t *testing.T) {
	base := time.Date(2026, 11, 10, 9, 0, 0, 0, time.UTC)
	ahora := time.Date(2026, 11, 12, 10, 0, 0, 0, time.UTC)
	if s := siguienteFecha(base, repDiaria, ahora); !s.Equal(time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC)) {
		t.Fatalf("diaria: %v", s)
	}
	if s := siguienteFecha(base, repSemanal, ahora); !s.Equal(time.Date(2026, 11, 17, 9, 0, 0, 0, time.UTC)) {
		t.Fatalf("semanal: %v", s)
	}
	cumple := time.Date(2020, 11, 12, 8, 0, 0, 0, time.UTC)
	if s := siguienteFecha(cumple, repAnual, ahora); !s.Equal(time.Date(2027, 11, 12, 8, 0, 0, 0, time.UTC)) {
		t.Fatalf("anual: %v", s)
	}
	if !siguienteFecha(base, repNinguna, ahora).IsZero() {
		t.Fatal("sin repetición no hay siguiente")
	}

	e := programadosListo(t)
	id := programa(t, e, pnAna, "Feliz cumple", e.hora.Add(5*time.Minute), repAnual)
	e.simula(8)
	if e.cuantos() != 1 {
		t.Fatalf("enviados: %v", e.enviados())
	}
	f, _ := e.alm.ObtenerAgenda(1, id)
	if f.Estado != pgActivo || time.Unix(f.ProximoTS, 0).UTC().Year() != 2027 {
		t.Fatalf("debía quedar para el año que viene: %+v", f)
	}
	// diaria: sale cada día
	e2 := programadosListo(t)
	programa(t, e2, pnAna, "Buen día", e2.hora.Add(5*time.Minute), repDiaria)
	for d := 0; d < 3; d++ {
		e2.simula(8)
		e2.hora = e2.hora.Add(24*time.Hour - 8*time.Minute)
	}
	if e2.cuantos() != 3 {
		t.Fatalf("tres días, tres mensajes: %d", e2.cuantos())
	}
}

func TestProgramadosTardeDentroYFueraDeLaTolerancia(t *testing.T) {
	e := programadosListo(t)
	e.config(IDProgramados, map[string]any{"tolerancia_min": 30.0})
	programa(t, e, pnAna, "Uno", e.hora.Add(5*time.Minute), "")
	programa(t, e, pnAna, "Dos", e.hora.Add(6*time.Minute), "")
	// el teléfono estuvo apagado: vuelve a las 12:20 (15 min tarde)
	e.hora = e.hora.Add(20 * time.Minute)
	e.simula(3)
	if e.cuantos() != 2 {
		t.Fatalf("dentro de la tolerancia salen tarde: %v", e.enviados())
	}
	// ahora uno que vuelve 2 horas tarde
	programa(t, e, pnAna, "Tres", e.hora.Add(5*time.Minute), "")
	e.hora = e.hora.Add(2 * time.Hour)
	e.simula(3)
	if e.cuantos() != 2 {
		t.Fatalf("fuera de la tolerancia se descarta: %v", e.enviados())
	}
	if !tiene(e.avisos, "Mensaje programado sin enviar") {
		t.Fatalf("avisos: %v", e.avisos)
	}
	if !tiene(e.resultados(), "descartado:tarde") {
		t.Fatalf("registro: %v", e.resultados())
	}
	// avisar_perdido apagado: no avisa
	e.avisos = nil
	e.config(IDProgramados, map[string]any{"avisar_perdido": false})
	programa(t, e, pnAna, "Cuatro", e.hora.Add(5*time.Minute), "")
	e.hora = e.hora.Add(3 * time.Hour)
	e.simula(2)
	if len(e.avisos) != 0 {
		t.Fatal("sin aviso")
	}
}

func TestProgramadosRepetidoTardeSaltaALaSiguiente(t *testing.T) {
	e := programadosListo(t)
	id := programa(t, e, pnAna, "Diario", e.hora.Add(5*time.Minute), repDiaria)
	e.hora = e.hora.Add(30 * time.Hour) // pasó un día entero apagado
	e.simula(2)
	if e.cuantos() != 0 {
		t.Fatal("muy tarde: no sale")
	}
	f, _ := e.alm.ObtenerAgenda(1, id)
	if f.Estado != pgActivo || f.ProximoTS <= e.hora.Unix() {
		t.Fatalf("debe quedar la siguiente fecha futura: %+v", f)
	}
}

func TestProgramadosMaestroApagadoReintentaDentroDeLaTolerancia(t *testing.T) {
	e := programadosListo(t)
	programa(t, e, pnAna, "Hola", e.hora.Add(2*time.Minute), "")
	e.fija("maestro", "0")
	e.simula(5)
	if e.cuantos() != 0 {
		t.Fatal("maestro apagado")
	}
	e.fija("maestro", "1")
	e.simula(3)
	if e.cuantos() != 1 {
		t.Fatalf("al encender el maestro dentro de la tolerancia sale: %d", e.cuantos())
	}
}

func TestProgramadosAListaNoMolestarYNoche(t *testing.T) {
	e := programadosListo(t)
	e.conChat(pnLuis, false, "Luis")
	for _, p := range []string{pnAna, pnLuis} {
		_, _ = e.alm.AgregarALista(1, "amigos", p, "a mano", e.hora)
	}
	_ = e.g.AgregarNoMolestar(pnLuis, "a mano")
	programa(t, e, "lista:amigos", "Fiesta", e.hora.Add(2*time.Minute), "")
	e.simula(5)
	if e.cuantos() != 1 || !e.hay(pnAna+"|Fiesta") {
		t.Fatalf("a la lista, menos quien pidió no recibir: %v", e.enviados())
	}
	// cae en la pausa nocturna (23:00–07:00): se descarta y avisa
	e.avisos = nil
	e.hora = time.Date(2026, 11, 10, 22, 50, 0, 0, time.UTC)
	programa(t, e, pnAna, "Tarde", time.Date(2026, 11, 10, 23, 30, 0, 0, time.UTC), "")
	e.hora = time.Date(2026, 11, 10, 23, 31, 0, 0, time.UTC)
	e.simula(3)
	if e.cuantos() != 1 {
		t.Fatal("de noche no sale")
	}
	if !tiene(e.avisos, "Mensaje programado sin enviar") {
		t.Fatalf("avisar: %v", e.avisos)
	}
}

func TestProgramadosTopeDiario(t *testing.T) {
	e := programadosListo(t)
	_ = e.g.FijarTopesModulo(IDProgramados, 100, 2)
	for i := 0; i < 4; i++ {
		programa(t, e, pnAna, "m", e.hora.Add(time.Duration(2+i)*time.Minute), "")
	}
	e.simula(10)
	if e.cuantos() != 2 {
		t.Fatalf("tope diario 2: %d", e.cuantos())
	}
}

func TestProgramadosValidaYRecuperaAlArrancar(t *testing.T) {
	e := programadosListo(t)
	_, err := e.g.AccionModulo(IDProgramados, "programados.agregar", Args{"nombre": "x", "destino": pnAna, "texto": "t", "fecha": "mañana"})
	esperaErr(t, err, "día/mes/año")
	_, err = e.g.AccionModulo(IDProgramados, "programados.agregar", Args{"nombre": "x", "destino": pnAna, "texto": "t", "fecha": "01/01/2020 10:00"})
	esperaErr(t, err, "ya pasó")
	_, err = e.g.AccionModulo(IDProgramados, "programados.agregar", Args{"nombre": "x", "destino": "nadie@s.whatsapp.net", "texto": "t", "fecha": "01/01/2030 10:00"})
	esperaErr(t, err, "a dónde")
	_, err = e.g.AccionModulo(IDProgramados, "programados.agregar", Args{"nombre": "x", "destino": "lista:vacia", "texto": "t", "fecha": "01/01/2030 10:00"})
	esperaErr(t, err, "vacía")
	m, err := e.g.AccionModulo(IDProgramados, "programados.vista", Args{"nombre": "x", "destino": pnAna, "texto": "Hola {nombre}", "fecha": "01/01/2030 10:00", "repeticion": "anual"})
	if err != nil || !strings.Contains(m, "Hola María") || !strings.Contains(m, "cada año") {
		t.Fatalf("vista: %q %v", m, err)
	}

	// la app se cerró con un mensaje en la cola: al arrancar se vuelve a encolar
	id := programa(t, e, pnAna, "Se perdió", e.hora.Add(time.Minute), "")
	e.fija("maestro", "1")
	e.simula(1)
	e.g.Cola.Vaciar("", "cierre") // la cola en memoria se pierde
	f, _ := e.alm.ObtenerAgenda(1, id)
	if f.UltimoRes != "encolado" && f.UltimoRes != "enviado" {
		t.Fatalf("resultado: %q", f.UltimoRes)
	}
}

func TestProgramadosProximoAgendaYCancelar(t *testing.T) {
	e := programadosListo(t)
	if e.g.ProximoAgenda() != 0 {
		t.Fatal("sin nada programado no hay alarma")
	}
	id := programa(t, e, pnAna, "A", e.hora.Add(2*time.Hour), "")
	programa(t, e, pnAna, "B", e.hora.Add(time.Hour), "")
	if p := e.g.ProximoAgenda(); p != e.hora.Add(time.Hour).Unix() {
		t.Fatalf("la alarma es la más próxima: %d", p)
	}
	if _, err := e.g.AccionModulo(IDProgramados, "programados.cancelar", Args{"id": float64(id)}); err != nil {
		t.Fatal(err)
	}
	e.simula(1)
	f, _ := e.g.FilasPanel(IDProgramados, "programados")
	if len(f) != 2 {
		t.Fatalf("filas: %+v", f)
	}
	if err := e.g.Desactivar(IDProgramados); err != nil {
		t.Fatal(err)
	}
	if e.g.ProximoAgenda() != 0 {
		t.Fatal("apagado no hay alarma")
	}
}
