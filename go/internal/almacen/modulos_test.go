package almacen

import (
	"bytes"
	"testing"
	"time"
)

func TestMigracion11Existe(t *testing.T) {
	a := nuevo(t)
	if a.Version() < 11 {
		t.Fatalf("falta la migración 11: versión %d", a.Version())
	}
	for _, tabla := range []string{"modulos", "modulos_registro", "no_molestar"} {
		var n int
		if err := a.db.QueryRow(`SELECT COUNT(*) FROM ` + tabla).Scan(&n); err != nil {
			t.Fatalf("tabla %s: %v", tabla, err)
		}
	}
}

func TestModuloConfigCifradaYTopes(t *testing.T) {
	a := nuevo(t)
	if f, _ := a.ObtenerModulo(1, "x"); f != nil {
		t.Fatal("no hay fila todavía")
	}
	if err := a.GuardarModuloConfig(1, "x", `{"frase":"SECRETO-PLANO"}`); err != nil {
		t.Fatal(err)
	}
	var crudo []byte
	if err := a.db.QueryRow(`SELECT config FROM modulos WHERE bot_id=1 AND id='x'`).Scan(&crudo); err != nil {
		t.Fatal(err)
	}
	if len(crudo) == 0 || bytes.Contains(crudo, []byte("SECRETO")) || bytes.Contains(crudo, []byte("frase")) {
		t.Fatalf("la configuración debe ir cifrada: %q", crudo)
	}
	f, _ := a.ObtenerModulo(1, "x")
	if f == nil || f.Config != `{"frase":"SECRETO-PLANO"}` || f.Activo {
		t.Fatalf("fila: %+v", f)
	}
	ahora := time.Now()
	if err := a.FijarModuloActivo(1, "x", true, ahora); err != nil {
		t.Fatal(err)
	}
	_ = a.FijarTopesModulo(1, "x", 3, 9)
	f, _ = a.ObtenerModulo(1, "x")
	if !f.Activo || f.ActivadoTS != ahora.Unix() || f.TopeHora != 3 || f.TopeDia != 9 || f.Config == "" {
		t.Fatalf("fila: %+v", f)
	}
	_ = a.FijarModuloActivo(1, "x", false, ahora)
	f, _ = a.ObtenerModulo(1, "x")
	if f.Activo || f.Config == "" {
		t.Fatal("apagar conserva la configuración")
	}
	// Otro bot no ve esta fila.
	if g, _ := a.ObtenerModulo(2, "x"); g != nil {
		t.Fatal("aislamiento por bot")
	}
}

func TestRegistroDeModulosNoTieneColumnaDeTexto(t *testing.T) {
	a := nuevo(t)
	filas, err := a.db.Query(`PRAGMA table_info(modulos_registro)`)
	if err != nil {
		t.Fatal(err)
	}
	defer filas.Close()
	cols := map[string]bool{}
	for filas.Next() {
		var cid, nn, pk int
		var nombre, tipo string
		var def any
		if err := filas.Scan(&cid, &nombre, &tipo, &nn, &def, &pk); err != nil {
			t.Fatal(err)
		}
		cols[nombre] = true
	}
	for _, c := range []string{"texto", "mensaje", "cuerpo", "contenido"} {
		if cols[c] {
			t.Fatalf("el registro no puede tener la columna %s", c)
		}
	}
	for _, c := range []string{"modulo", "que", "ts", "resultado"} {
		if !cols[c] {
			t.Fatalf("falta la columna %s", c)
		}
	}
	now := time.Now()
	a.RegistrarModulo(1, "m", "aviso", "enviado", "c@s.whatsapp.net", now.Add(-2*time.Hour))
	a.RegistrarModulo(1, "m", "aviso", "fallido", "c@s.whatsapp.net", now.Add(-30*time.Minute))
	a.RegistrarModulo(1, "n", "aviso", "descartado:maestro", "c@s.whatsapp.net", now)
	l, _ := a.ListarRegistroModulos(1, "", 10, 0)
	if len(l) != 3 || l[0].Resultado != "descartado:maestro" {
		t.Fatalf("lista: %+v", l)
	}
	if p, _ := a.ListarRegistroModulos(1, "", 10, l[0].ID); len(p) != 2 {
		t.Fatalf("página siguiente: %+v", p)
	}
	if m, _ := a.ListarRegistroModulos(1, "m", 10, 0); len(m) != 2 {
		t.Fatalf("por módulo: %+v", m)
	}
	if n := a.ContarRegistroModulos(1, "", "enviado", now.Add(-3*time.Hour)); n != 1 {
		t.Fatalf("contar: %d", n)
	}
	if e := a.EnviosRecientesModulos(1, now.Add(-time.Hour)); len(e) != 1 || e[0].Modulo != "m" {
		t.Fatalf("envíos recientes (solo enviado/fallido): %+v", e)
	}
	a.LimpiarRegistroModulos(1, 1, now.Add(72*time.Hour))
	if l, _ := a.ListarRegistroModulos(1, "", 10, 0); len(l) != 0 {
		t.Fatal("la limpieza borra lo viejo")
	}
}

func TestNoMolestarAliasYConfirmacionUnica(t *testing.T) {
	a := nuevo(t)
	ahora := time.Now()
	nuevoP, conf, err := a.AgregarNoMolestar(1, pnAna, "baja", ahora)
	if err != nil || !nuevoP || conf {
		t.Fatalf("primera vez: %v %v %v", nuevoP, conf, err)
	}
	if !a.EstaNoMolestar(1, pnAna) || a.EstaNoMolestar(1, pnLuis) || a.EstaNoMolestar(1, "") {
		t.Fatal("pertenencia")
	}
	a.MarcarNoMolestarConfirmado(1, pnAna)
	nuevoP, conf, _ = a.AgregarNoMolestar(1, pnAna, "baja", ahora)
	if nuevoP || !conf {
		t.Fatalf("segunda vez: %v %v", nuevoP, conf)
	}
	// El LID de la misma persona también cuenta.
	a.AprenderAlias(1, pnAna, lidAna)
	if !a.EstaNoMolestar(1, lidAna) {
		t.Fatal("por LID")
	}
	if _, _, err := a.AgregarNoMolestar(1, PersonaDueno, "x", ahora); err == nil {
		t.Fatal("el dueño no puede estar en la lista")
	}
	l, _ := a.ListarNoMolestar(1)
	if len(l) != 1 || !l[0].Confirmado {
		t.Fatalf("lista: %+v", l)
	}
	if err := a.QuitarNoMolestar(1, lidAna); err != nil {
		t.Fatal(err)
	}
	if a.EstaNoMolestar(1, pnAna) {
		t.Fatal("quitada")
	}
}

func TestAjustesDeIniciativaYEliminarBot(t *testing.T) {
	a := nuevo(t)
	if a.AjusteIni(1, "ini_maestro") != "0" || a.AjusteIniInt(1, "ini_tope_hora") != 10 || a.AjusteIni(1, "ini_noche_desde") != "23:00" {
		t.Fatal("valores por defecto: maestro apagado")
	}
	if err := a.FijarAjusteIni(1, "ini_inventado", "1"); err == nil {
		t.Fatal("clave desconocida")
	}
	_ = a.FijarAjusteIni(1, "ini_tope_hora", "4")
	if a.AjusteIniInt(1, "ini_tope_hora") != 4 {
		t.Fatal("no se guardó")
	}
	// Los ajustes de iniciativa no se cuelan en los ajustes de permisos.
	if _, ok := a.AjustesBot(1)["ini_tope_hora"]; ok {
		t.Fatal("AjustesBot no debe traer los ini_")
	}
	_ = a.GuardarModuloConfig(1, "x", `{}`)
	a.RegistrarModulo(1, "x", "q", "enviado", "", time.Now())
	if _, _, err := a.AgregarNoMolestar(1, pnAna, "", time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := a.EliminarBot(1); err != nil {
		t.Fatal(err)
	}
	for _, tabla := range []string{"modulos", "modulos_registro", "no_molestar"} {
		var n int
		_ = a.db.QueryRow(`SELECT COUNT(*) FROM ` + tabla).Scan(&n)
		if n != 0 {
			t.Fatalf("%s quedó con %d filas", tabla, n)
		}
	}
	if a.AjusteIni(1, "ini_tope_hora") != "10" {
		t.Fatal("los ajustes se borran con el bot")
	}
}
