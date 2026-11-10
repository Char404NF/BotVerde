package almacen

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"math/rand"
	"time"
)

// JIDPrueba es el chat sintético que crea "Medir rendimiento → Abrir chat" (se borra al terminar).
const JIDPrueba = "120363999999999999@g.us"

func jpegPrueba(ancho, alto, semilla int, calidad int) []byte {
	r := rand.New(rand.NewSource(int64(semilla)))
	img := image.NewRGBA(image.Rect(0, 0, ancho, alto))
	for y := 0; y < alto; y++ {
		for x := 0; x < ancho; x++ {
			ruido := uint8(r.Intn(40))
			img.Set(x, y, color.RGBA{uint8(x*255/ancho) + ruido, uint8(y*255/alto) + uint8(semilla*30), uint8((x+y)*255/(ancho+alto)) + ruido, 255})
		}
	}
	var b bytes.Buffer
	_ = jpeg.Encode(&b, img, &jpeg.Options{Quality: calidad})
	return b.Bytes()
}

// ChatSintetico crea un grupo con n mensajes (texto, fotos reales cifradas, citas, menciones, reacciones, recibos y eliminados)
// para medir cuánto tarda en abrirse. Solo se usa desde Avanzado → Medir rendimiento; no pasa por el bot ni por WhatsApp.
func (a *Almacen) ChatSintetico(botID int64, n int) (string, error) {
	chat := JIDPrueba
	if _, err := a.ActualizarChat(botID, chat, true, func(c *Chat) { c.Guardado = "Chat de prueba (se borra solo)" }); err != nil {
		return "", err
	}
	fotos := make([]string, 8)
	minis := make([]string, 8)
	for k := range fotos {
		idf := fmt.Sprintf("sint%d", k)
		fotos[k], minis[k] = RutaMedio(botID, chat, idf), RutaMiniatura(botID, chat, idf)
		if err := a.EscribirCifrado(fotos[k], jpegPrueba(1280, 960, k, 85)); err != nil {
			return "", err
		}
		if err := a.EscribirCifrado(minis[k], jpegPrueba(160, 120, k, 60)); err != nil {
			return "", err
		}
	}
	base := time.Now().Unix() - int64(n)*60
	lote := make([]*Mensaje, 0, 1000)
	vuelca := func() error {
		_, err := a.GuardarMensajes(lote)
		lote = lote[:0]
		return err
	}
	for i := 0; i < n; i++ {
		quien := fmt.Sprintf("52155%07d@s.whatsapp.net", i%25)
		m := &Mensaje{ID: fmt.Sprintf("S%06d", i), ChatJID: chat, BotID: botID, RemitenteJID: quien, RemitenteNombre: fmt.Sprintf("Persona %d", i%25),
			EsMio: i%7 == 0, Tipo: TipoTexto, Texto: fmt.Sprintf("Mensaje de prueba número %d con *algo* de texto, un enlace https://ejemplo.mx y _formato_ para medir", i), TS: base + int64(i)*60}
		if i%7 == 0 {
			m.RemitenteJID, m.RemitenteNombre = QuienYo, ""
		}
		if i%10 == 3 {
			k := (i / 10) % 8
			m.Tipo, m.ArchivoBytes, m.ArchivoMime, m.ArchivoEstado, m.ArchivoRuta, m.MiniaturaRuta, m.Ancho, m.Alto = TipoImagen, 150000, "image/jpeg", EstadoDescargado, fotos[k], minis[k], 1280, 960
		}
		if i%6 == 1 && i > 10 {
			m.CitaID, m.CitaAutor, m.CitaTexto = fmt.Sprintf("S%06d", i-5), fmt.Sprintf("52155%07d@s.whatsapp.net", (i-5)%25), "texto citado"
		}
		if i%9 == 2 {
			m.Menciones = []string{"5215500000001@s.whatsapp.net", "5215500000002@s.whatsapp.net"}
		}
		lote = append(lote, m)
		if len(lote) == 1000 {
			if err := vuelca(); err != nil {
				return "", err
			}
		}
	}
	if err := vuelca(); err != nil {
		return "", err
	}
	for i := max(0, n-400); i < n; i++ {
		id := fmt.Sprintf("S%06d", i)
		if i%3 == 0 {
			_ = a.GuardarReaccion(botID, chat, id, fmt.Sprintf("52155%07d@s.whatsapp.net", i%25), "👍", base+int64(i)*60+5)
		}
		if i%7 == 0 {
			_, _ = a.GuardarAcuse(botID, chat, id, "5215500000001@s.whatsapp.net", AcuseEntregado, base+int64(i)*60+2)
			_, _ = a.GuardarAcuse(botID, chat, id, "5215500000001@s.whatsapp.net", AcuseLeido, base+int64(i)*60+3)
		}
		if i%11 == 5 {
			_, _ = a.MarcarEliminado(botID, chat, id, "5215500000004@s.whatsapp.net", base+int64(i)*60+9)
		}
	}
	return chat, nil
}
