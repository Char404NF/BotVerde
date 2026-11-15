package wabot

import (
	"strings"
	"testing"
	"time"

	"botchat/internal/almacen"
)

func TestCopiaDeConfiguracionCifradaYRestaurable(t *testing.T) {
	reinicia(t)
	if err := AbrirAlmacen(t.TempDir(), clavePrueba(), &obsFalso{}); err != nil {
		t.Fatal(err)
	}
	a, _ := almacenAbierto()
	g := "120363000000000066@g.us"
	_, _ = a.ActualizarChat(1, g, true, func(c *almacen.Chat) { c.Guardado = "Club" })
	_ = a.FijarAjusteBot(1, "alerta_wa", "1")
	_ = a.FijarRespuesta(1, "grupo.baneada", "Fuera de aquí.")
	_ = a.FijarReglasGrupo(1, g, "Respeto ante todo", time.Now())
	_ = a.GuardarFiltroGrupo(1, almacen.FiltroGrupo{Chat: g, Filtro: "flood", Activo: true, Accion: "borrar", Config: `{"max":9}`}, time.Now())
	_ = a.GuardarHorario(1, almacen.HorarioGrupo{Grupo: g, Activo: true, Cierra: "23:00", Abre: "07:00", Dias: 127})
	_ = a.GuardarEntrada(1, almacen.ProteccionEntrada{Grupo: g, RaidActivo: true, RaidMax: 5, RaidMinutos: 3, Prefijos: []string{"212"}})
	_ = a.Banear(1, g, "34600000009@s.whatsapp.net", "yo", "spam", time.Now())

	if _, err := RespaldoConfigExportar(1, "corta"); err == nil {
		t.Fatal("una contraseña corta no sirve")
	}
	datos, err := RespaldoConfigExportar(1, "una-contraseña-larga")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(datos), "Respeto") || strings.Contains(string(datos), "alerta_wa") || strings.Contains(string(datos), "34600000009") {
		t.Fatal("la copia debe ir cifrada")
	}
	if _, err := RespaldoConfigRestaurar(1, "otra-contraseña-larga", datos); err == nil || !strings.Contains(err.Error(), "contraseña") {
		t.Fatalf("contraseña mala: %v", err)
	}
	if _, err := RespaldoConfigRestaurar(1, "una-contraseña-larga", []byte("no soy una copia de seguridad")); err == nil {
		t.Fatal("un archivo cualquiera no se restaura")
	}

	// Borro todo y restauro (como en un teléfono nuevo).
	_ = a.FijarAjusteBot(1, "alerta_wa", "0")
	_ = a.FijarRespuesta(1, "grupo.baneada", "")
	_ = a.FijarReglasGrupo(1, g, "", time.Now())
	_ = a.BorrarFiltroGrupo(1, g, "flood")
	_, _ = a.Desbanear(1, g, "34600000009@s.whatsapp.net")
	_ = a.GuardarHorario(1, almacen.HorarioGrupo{Grupo: g, Cierra: "00:00", Abre: "00:00"})
	_ = a.GuardarEntrada(1, almacen.ProteccionEntrada{Grupo: g, RaidMax: 8, RaidMinutos: 2})

	j, err := RespaldoConfigRestaurar(1, "una-contraseña-larga", datos)
	if err != nil || !strings.Contains(j, `"grupos":1`) || !strings.Contains(j, `"filtros":1`) || !strings.Contains(j, `"baneados":1`) {
		t.Fatalf("restaurar: %s %v", j, err)
	}
	if a.AjusteBot(1, "alerta_wa") != "1" || a.Respuesta(1, "grupo.baneada") != "Fuera de aquí." || a.ReglasGrupo(1, g) != "Respeto ante todo" {
		t.Fatal("no se restauraron ajustes, respuestas o reglas")
	}
	fs, _ := a.FiltrosDeGrupo(1, g)
	if len(fs) != 1 || !fs[0].Activo || fs[0].Accion != "borrar" || !strings.Contains(fs[0].Config, "9") {
		t.Fatalf("filtros: %+v", fs)
	}
	if h := a.HorarioDe(1, g); !h.Activo || h.Cierra != "23:00" {
		t.Fatalf("horario: %+v", h)
	}
	if e := a.EntradaDe(1, g); !e.RaidActivo || e.RaidMax != 5 || len(e.Prefijos) != 1 {
		t.Fatalf("entrada: %+v", e)
	}
	if !a.EstaBaneado(1, g, "34600000009@s.whatsapp.net") {
		t.Fatal("el baneado debía volver")
	}
}
