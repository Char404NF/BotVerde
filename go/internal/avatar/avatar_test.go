package avatar

import (
	"strings"
	"testing"

	"botchat/internal/almacen"
)

func nuevo(t *testing.T) *Servicio {
	t.Helper()
	a, err := almacen.Abrir(t.TempDir(), []byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { a.Cerrar() })
	return Nuevo(a, 1)
}

func TestDeFabricaSaleEnMenuAyudaBienvenidaYAvisos(t *testing.T) {
	s := nuevo(t)
	const priv, grupo = "5215511111111@s.whatsapp.net", "120363000000000001@g.us"
	for _, m := range []string{Menu, Ayuda, Bienvenida, Aviso} {
		if !s.Corresponde(m, priv, "hola") {
			t.Errorf("%s en privado debía llevar avatar", m)
		}
	}
	if s.Corresponde(Error, priv, "x") || s.Corresponde("", priv, "x") || s.Corresponde("inventado", priv, "x") {
		t.Fatal("el error está apagado de fábrica y lo desconocido no lleva")
	}
	if !s.Corresponde(Menu, grupo, "x") || !s.Corresponde(Bienvenida, grupo, "x") || s.Corresponde(Ayuda, grupo, "x") || s.Corresponde(Aviso, grupo, "x") {
		t.Fatal("en grupos solo menú y bienvenida")
	}
	if s.Corresponde(Menu, priv, strings.Repeat("a", MaxPie+1)) || !s.Corresponde(Menu, priv, strings.Repeat("a", MaxPie)) {
		t.Fatal("el pie de foto tiene tope")
	}
}

func TestConfigurarYApagar(t *testing.T) {
	s := nuevo(t)
	if err := s.Configurar(true, []string{Error, "inventado"}); err != nil {
		t.Fatal(err)
	}
	if !s.Corresponde(Error, "5215511111111@s.whatsapp.net", "x") || s.Corresponde(Menu, "5215511111111@s.whatsapp.net", "x") {
		t.Fatal("solo los elegidos, ignorando los desconocidos")
	}
	if err := s.Configurar(false, []string{Menu}); err != nil || s.Corresponde(Menu, "5215511111111@s.whatsapp.net", "x") {
		t.Fatalf("apagado no lleva: %v", err)
	}
	// «ninguno elegido» se respeta (no vuelve a los de fábrica)
	_ = s.Configurar(true, nil)
	if s.Corresponde(Menu, "5215511111111@s.whatsapp.net", "x") {
		t.Fatal("ninguno elegido = ninguno")
	}
}

func TestCadaMomentoTieneSuCaraYLaImagenPropiaLasReemplazaTodas(t *testing.T) {
	s := nuevo(t)
	vistas := map[string]bool{}
	for _, m := range Momentos {
		id, mime, nombre, ok := s.Imagen(m.ID)
		if !ok || id == "" || mime != "image/png" || !strings.HasPrefix(nombre, "avatar_") {
			t.Fatalf("%s: %q %q %q %v", m.ID, id, mime, nombre, ok)
		}
		vistas[nombre] = true
		if id2, _, _, _ := s.Imagen(m.ID); id2 != id {
			t.Fatalf("la cara se guarda una sola vez: %s", m.ID)
		}
	}
	if len(vistas) < 4 {
		t.Fatalf("las caras cambian según el momento: %v", vistas)
	}
	propio, _, err := s.Alm.GuardarAdjuntoMenu(1, strings.NewReader("PNGDATA"))
	if err != nil {
		t.Fatal(err)
	}
	if err := s.FijarPropio(propio, "image/jpeg"); err != nil {
		t.Fatal(err)
	}
	for _, m := range Momentos {
		if id, mime, _, ok := s.Imagen(m.ID); !ok || id != propio || mime != "image/jpeg" {
			t.Fatalf("la propia reemplaza a todas: %s %q", m.ID, id)
		}
	}
	// si la imagen propia desaparece del disco, vuelven las caras
	s.Alm.BorrarAdjuntoMenu(1, propio)
	if id, _, nombre, ok := s.Imagen(Menu); !ok || id == propio || !strings.HasPrefix(nombre, "avatar_") {
		t.Fatalf("sin imagen propia usa la cara: %q", nombre)
	}
}

func TestImagenPropiaDeUnMomentoGanaALaGeneral(t *testing.T) {
	s := nuevo(t)
	general, _, _ := s.Alm.GuardarAdjuntoMenu(1, strings.NewReader("GENERAL"))
	soloMenu, _, _ := s.Alm.GuardarAdjuntoMenu(1, strings.NewReader("MENU"))
	if err := s.FijarPropio(general, "image/png"); err != nil {
		t.Fatal(err)
	}
	if err := s.FijarPropioDe(Menu, soloMenu, "image/jpeg"); err != nil {
		t.Fatal(err)
	}
	if id, mime, _, ok := s.Imagen(Menu); !ok || id != soloMenu || mime != "image/jpeg" {
		t.Fatalf("el menú usa la suya: %q %q", id, mime)
	}
	if id, _, _, ok := s.Imagen(Ayuda); !ok || id != general {
		t.Fatalf("los demás usan la general: %q", id)
	}
	if err := s.FijarPropioDe("inventado", soloMenu, "image/png"); err == nil {
		t.Fatal("un momento que no existe debe fallar")
	}
	// quitar la del menú: vuelve a la general
	if err := s.FijarPropioDe(Menu, "", ""); err != nil {
		t.Fatal(err)
	}
	if id, _, _, _ := s.Imagen(Menu); id != general || s.PropioDe(Menu) != "" {
		t.Fatalf("sin la del menú usa la general: %q", id)
	}
}
