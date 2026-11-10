package almacen

import (
	"fmt"
	"testing"
	"time"
)

func chatGrande(t testing.TB, a *Almacen, chat string, n int) {
	if _, err := a.ChatSintetico(1, n); err != nil {
		t.Fatal(err)
	}
}

func media(veces int, f func()) (media, peor time.Duration) {
	var suma time.Duration
	for i := 0; i < veces; i++ {
		t := time.Now()
		f()
		d := time.Since(t)
		suma += d
		if d > peor {
			peor = d
		}
	}
	return suma / time.Duration(veces), peor
}

// Mide abrir un chat de 20,000 mensajes: antes (ruta de la 0.4.0) y después (nombres por lote con caché).
func TestMedirAbrirChatGrande(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	a := nuevo(t)
	chat := JIDPrueba
	chatGrande(t, a, chat, 20000)
	defer CargaAntigua(false)
	for _, lim := range []int{40, 60, 200} {
		CargaAntigua(true)
		var antes, antesPeor time.Duration
		if lim <= 60 {
			antes, antesPeor = media(5, func() { _, _ = a.ListarMensajes(1, chat, 0, "", lim) })
		}
		CargaAntigua(false)
		a.nombres.limpiar()
		frio := time.Now()
		if vs, err := a.ListarMensajes(1, chat, 0, "", lim); err != nil || len(vs) != lim {
			t.Fatal(err, len(vs))
		}
		d1 := time.Since(frio)
		m, p := media(15, func() { _, _ = a.ListarMensajes(1, chat, 0, "", lim) })
		t.Logf("página de %3d: ANTES media %v (peor %v) · DESPUÉS en frío %v, con caché media %v (peor %v)", lim, antes, antesPeor, d1, m, p)
		if lim == 40 && d1 > 300*time.Millisecond {
			t.Errorf("la primera página tarda %v (meta < 300 ms)", d1)
		}
	}
	vs, _ := a.ListarMensajes(1, chat, 0, "", 40)
	ids := make([]string, 0, 300)
	for i := 0; i < 300; i++ {
		ids = append(ids, fmt.Sprintf("S%06d", 19000+i))
	}
	mm, pp := media(10, func() { _, _ = a.MensajesPorID(1, chat, ids) })
	t.Logf("MensajesPorID(300 ids): media %v, peor %v (primera página tenía %d)", mm, pp, len(vs))
}
