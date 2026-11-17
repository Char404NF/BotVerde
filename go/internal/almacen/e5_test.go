package almacen

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func clave5() []byte {
	k := make([]byte, 32)
	for i := range k {
		k[i] = byte(i + 9)
	}
	return k
}

// Una base en la versión 21 con advertencias sube a la 22 sin perdonar a nadie: las que existían no caducan.
func TestMigracion22ConservaAdvertenciasSinCaducidad(t *testing.T) {
	raiz := t.TempDir()
	antes := migraciones
	migraciones = antes[:21]
	a, err := Abrir(raiz, clave5())
	if err != nil {
		migraciones = antes
		t.Fatal(err)
	}
	if a.Version() != 21 {
		migraciones = antes
		t.Fatalf("versión %d", a.Version())
	}
	if _, err := a.db.Exec(`INSERT INTO advertencias(bot_id,persona,chat,ts,por) VALUES(1,'5215511111111@s.whatsapp.net','g@g.us',100,'yo')`); err != nil {
		migraciones = antes
		t.Fatal(err)
	}
	_ = a.Cerrar()
	migraciones = antes
	b, err := Abrir(raiz, clave5())
	if err != nil {
		t.Fatal(err)
	}
	defer b.Cerrar()
	if b.Version() != len(migraciones) {
		t.Fatalf("versión %d", b.Version())
	}
	// ts = 100 (1970) y aun así vale: caduca_ts = 0 = no caduca
	if n := b.ContarAdvertenciasEn(1, "5215511111111@s.whatsapp.net", time.Now()); n != 1 {
		t.Fatalf("la vieja debe seguir vigente: %d", n)
	}
	xs, _ := b.ListarAdvertencias(1, "5215511111111@s.whatsapp.net", 10)
	if len(xs) != 1 || xs[0].CaducaTS != 0 || !xs[0].Vigente || xs[0].TipoID != 0 {
		t.Fatalf("%+v", xs)
	}
	for _, tb := range []string{"silencio_grupo", "silencio_grupo_chats", "silencio_conteo", "silencio_textos", "adv_tipos", "adv_escalera", "grupo_regla_items", "sancion_pendiente"} {
		var n int
		_ = b.db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE name=?", tb).Scan(&n)
		if n != 1 {
			t.Fatalf("falta la tabla %s", tb)
		}
	}
}

func TestCaducidadDeAdvertencias(t *testing.T) {
	a := nuevo(t)
	ahora := time.Date(2026, 11, 1, 10, 0, 0, 0, time.UTC)
	p := "5215511111111@s.whatsapp.net"
	if _, _, err := a.AgregarAdvertencia(1, p, "g@g.us", "uno", "yo", ahora); err != nil {
		t.Fatal(err)
	}
	if n := a.ContarAdvertenciasEn(1, p, ahora.Add(29*24*time.Hour)); n != 1 {
		t.Fatalf("a los 29 días vale: %d", n)
	}
	if n := a.ContarAdvertenciasEn(1, p, ahora.Add(31*24*time.Hour)); n != 0 {
		t.Fatalf("a los 31 días caducó: %d", n)
	}
	xs, _ := a.ListarAdvertenciasEn(1, p, 10, ahora.Add(31*24*time.Hour))
	if len(xs) != 1 || xs[0].Vigente {
		t.Fatalf("sigue en el historial pero vencida: %+v", xs)
	}
	// editable: 0 = no caduca; 7 días
	_ = a.FijarAjusteBot(1, "adv_caducidad_dias", "0")
	_, _, _ = a.AgregarAdvertencia(1, p, "g@g.us", "dos", "yo", ahora)
	if n := a.ContarAdvertenciasEn(1, p, ahora.Add(400*24*time.Hour)); n != 1 {
		t.Fatalf("la de caducidad 0 no caduca: %d", n)
	}
	_ = a.FijarAjusteBot(1, "adv_caducidad_dias", "7")
	_, _, _ = a.AgregarAdvertencia(1, p, "g@g.us", "tres", "yo", ahora)
	if n := a.ContarAdvertenciasEn(1, p, ahora.Add(8*24*time.Hour)); n != 2 {
		t.Fatalf("uno (30 días) y dos (nunca): %d", n)
	}
	// quitar la última quita la última VIGENTE
	quedan, ok, _ := a.QuitarAdvertenciaEn(1, p, 0, ahora.Add(8*24*time.Hour))
	if !ok || quedan != 1 {
		t.Fatalf("%d %v", quedan, ok)
	}
	// borrado de vencidas con gracia
	if n := a.BorrarAdvertenciasVencidas(1, ahora.Add(100*24*time.Hour), 30*24*time.Hour); n != 2 {
		t.Fatalf("borró %d", n)
	}
	// el resumen cuenta solo vigentes
	a.Reloj = func() time.Time { return ahora.Add(500 * 24 * time.Hour) }
	_ = a.FijarAjusteBot(1, "adv_caducidad_dias", "0")
	_, _, _ = a.AgregarAdvertencia(1, p, "g@g.us", "cuatro", "yo", ahora.Add(500*24*time.Hour))
	rs, _ := a.ListarPersonasAdvertidas(1)
	if len(rs) != 1 || rs[0].Total != 1 {
		t.Fatalf("%+v", rs)
	}
}

func TestTiposDeFabricaEscaleraYCaducidadPropia(t *testing.T) {
	a := nuevo(t)
	ts, err := a.ListarTiposAdv(1, false)
	if err != nil || len(ts) != 4 || ts[0].Nombre != "Spam" || ts[1].Nombre != "Insultos" || ts[2].Nombre != "Enlaces" || ts[3].Nombre != "Otro" {
		t.Fatalf("%v %+v", err, ts)
	}
	if len(ts[0].Escalera) != 4 || ts[0].Escalera[3].Accion != EscExpulsar {
		t.Fatalf("%+v", ts[0].Escalera)
	}
	// sembrado una sola vez: si se borra Spam no vuelve
	if err := a.BorrarTipoAdv(1, ts[0].ID); err != nil {
		t.Fatal(err)
	}
	ts, _ = a.ListarTiposAdv(1, false)
	if len(ts) != 3 {
		t.Fatalf("%d", len(ts))
	}
	// crear / editar
	id, err := a.GuardarTipoAdv(1, TipoAdv{Nombre: "Foto fea", Emoji: "📷", Color: "#112233", CaducidadDias: 3, Activo: true,
		Escalera: []PasoEsc{{Nivel: 2, Accion: EscSilenciar, Minutos: 30}, {Nivel: 1, Accion: EscAvisar, Aviso: "Cuidado con las fotos"}}})
	if err != nil {
		t.Fatal(err)
	}
	t1 := a.TipoAdvPorID(1, id)
	if t1.Escalera[0].Nivel != 1 || t1.Escalera[0].Aviso != "Cuidado con las fotos" || t1.Escalera[1].Alcance != EscAlcanceGrupo {
		t.Fatalf("%+v", t1.Escalera)
	}
	// nombre repetido y datos malos
	if _, err := a.GuardarTipoAdv(1, TipoAdv{Nombre: "foto FEA", Activo: true}); err == nil {
		t.Fatal("repetido")
	}
	for _, mal := range []TipoAdv{
		{Nombre: "", Activo: true},
		{Nombre: "X", Color: "rojo", Activo: true},
		{Nombre: "X", Activo: true, Escalera: []PasoEsc{{Nivel: 1, Accion: "matar"}}},
		{Nombre: "X", Activo: true, Escalera: []PasoEsc{{Nivel: 1, Accion: EscSilenciar}}},
		{Nombre: "X", Activo: true, Escalera: []PasoEsc{{Nivel: 1, Accion: EscAvisar}, {Nivel: 1, Accion: EscAvisar}}},
	} {
		if _, err := a.GuardarTipoAdv(1, mal); err == nil {
			t.Fatalf("debía rechazar %+v", mal)
		}
	}
	// el aviso de la escalera va cifrado
	_, _ = a.db.Exec("PRAGMA wal_checkpoint(TRUNCATE)")
	crudo, _ := os.ReadFile(filepath.Join(a.raiz, "messages.db"))
	if bytes.Contains(crudo, []byte("Cuidado con las fotos")) {
		t.Fatal("el aviso de la escalera está en claro")
	}
	// caducidad propia del tipo (3 días) manda sobre la general (30)
	ahora := time.Date(2026, 11, 1, 10, 0, 0, 0, time.UTC)
	p := "5215511111111@s.whatsapp.net"
	if _, _, err := a.AgregarAdvertenciaTipo(1, p, "g@g.us", "", "yo", id, 0, ahora); err != nil {
		t.Fatal(err)
	}
	if a.ContarAdvertenciasTipo(1, p, id, ahora.Add(2*24*time.Hour)) != 1 || a.ContarAdvertenciasTipo(1, p, id, ahora.Add(4*24*time.Hour)) != 0 {
		t.Fatal("caducidad propia de 3 días")
	}
	// borrar el tipo deja la advertencia sin tipo
	_ = a.BorrarTipoAdv(1, id)
	xs, _ := a.ListarAdvertenciasEn(1, p, 10, ahora)
	if len(xs) != 1 || xs[0].TipoID != 0 {
		t.Fatalf("%+v", xs)
	}
}

func TestReglasComoListaYTextoParaReglas(t *testing.T) {
	a := nuevo(t)
	g := "grupo@g.us"
	ahora := time.Now()
	id1, err := a.GuardarReglaItem(1, ReglaItem{Grupo: g, Titulo: "Respeto", Texto: "Nada de insultos"}, ahora)
	if err != nil {
		t.Fatal(err)
	}
	id2, _ := a.GuardarReglaItem(1, ReglaItem{Grupo: g, Titulo: "Sin spam"}, ahora)
	if a.NumeroDeRegla(1, id2) != 2 {
		t.Fatal("número 2")
	}
	if got := a.ReglasGrupo(1, g); got != "1. Respeto: Nada de insultos\n2. Sin spam" {
		t.Fatalf("%q", got)
	}
	if err := a.MoverReglaItem(1, id2, -1, ahora); err != nil {
		t.Fatal(err)
	}
	if got := a.ReglasGrupo(1, g); !strings.HasPrefix(got, "1. Sin spam\n2. Respeto") {
		t.Fatalf("%q", got)
	}
	// una advertencia ligada a la regla; al borrar la regla queda sin regla
	p := "5215511111111@s.whatsapp.net"
	_, _, _ = a.AgregarAdvertenciaTipo(1, p, g, "", "yo", 0, id1, ahora)
	if err := a.BorrarReglaItem(1, id1, ahora); err != nil {
		t.Fatal(err)
	}
	xs, _ := a.ListarAdvertencias(1, p, 5)
	if xs[0].ReglaID != 0 {
		t.Fatalf("%+v", xs[0])
	}
	if got := a.ReglasGrupo(1, g); got != "1. Sin spam" {
		t.Fatalf("%q", got)
	}
	_ = a.BorrarReglaItem(1, id2, ahora)
	if got := a.ReglasGrupo(1, g); got != "" {
		t.Fatalf("lista vacía, texto vacío: %q", got)
	}
	// globales: no tocan !reglas de ningún grupo; vacío y grupo malo se rechazan
	if _, err := a.GuardarReglaItem(1, ReglaItem{Grupo: "", Titulo: "Global"}, ahora); err != nil {
		t.Fatal(err)
	}
	if _, err := a.GuardarReglaItem(1, ReglaItem{Grupo: "pepe"}, ahora); err == nil {
		t.Fatal("vacía/grupo malo")
	}
	// cifradas
	_, _ = a.GuardarReglaItem(1, ReglaItem{Grupo: g, Titulo: "Secretísima", Texto: "regla confidencial xyz"}, ahora)
	_, _ = a.db.Exec("PRAGMA wal_checkpoint(TRUNCATE)")
	crudo, _ := os.ReadFile(filepath.Join(a.raiz, "messages.db"))
	// el texto de !reglas (grupo_reglas) y la lista van cifrados
	if bytes.Contains(crudo, []byte("confidencial xyz")) {
		t.Fatal("regla en claro")
	}
}

func TestSilencioOlvidarYFusionDePersonas(t *testing.T) {
	a := nuevo(t)
	ahora := time.Now()
	pn, lid := "5215511111111@s.whatsapp.net", "99887766554433@lid"
	if _, err := a.CrearSilencioGrupo(1, lid, SilTodos, nil, 0, "yo", "manual", "", true, ahora); err != nil {
		t.Fatal(err)
	}
	_, _, _ = a.SumarIntento(1, lid, "g@g.us", "2026-11-10")
	a.AprenderAlias(1, pn, lid)
	canon := a.ResolverPersona(1, lid)
	xs, _ := a.SilenciosActivos(1, canon, ahora)
	if len(xs) != 1 {
		t.Fatalf("el silencio sigue a la persona canónica: %+v", xs)
	}
	if c := a.ConteoDe(1, canon, "", "2026-11-10"); c.N != 1 {
		t.Fatalf("%+v", c)
	}
	if _, err := a.OlvidarPersona(1, canon); err != nil {
		t.Fatal(err)
	}
	if n := a.NumSilenciosActivos(1); n != 0 {
		t.Fatal("olvidar borra el silencio")
	}
	if c := a.ConteoDe(1, canon, "", "2026-11-10"); c.N != 0 {
		t.Fatal("y el contador")
	}
}
