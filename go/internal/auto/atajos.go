package auto

import (
	"context"
	"math/rand"
	"strings"
	"time"

	"botchat/internal/almacen"
	"botchat/internal/motor"
)

// EnvioAtajo es lo que los atajos necesitan para hablar con WhatsApp.
type EnvioAtajo interface {
	// Editar cambia el texto de un mensaje mío (el "/atajo" que escribí).
	Editar(ctx context.Context, chat, id, texto string) error
	// EnviarTexto manda un texto al chat como si lo escribiera yo y devuelve el id del mensaje.
	EnviarTexto(ctx context.Context, chat, texto string) (string, error)
	// EnviarMedio manda un archivo guardado al chat. Si el archivo no se puede leer o subir devuelve un error.
	EnviarMedio(ctx context.Context, chat string, m motor.Medio) error
}

// AtajosConfig son las piezas de las respuestas rápidas.
type AtajosConfig struct {
	BotID  int64
	Alm    *almacen.Almacen
	Dec    *Decisor // para poner las variables del texto
	Envio  EnvioAtajo
	Ahora  func() time.Time
	Dormir func(ctx context.Context, d time.Duration) bool
	// Aleatorio da un número entre 0 y n-1.
	Aleatorio func(n int64) int64
	Log       func(string)
}

// Atajos son las respuestas rápidas: escribo "/gracias" en un chat y el bot manda ahí el texto guardado como si lo escribiera yo.
// Implementa motor.Atajador.
//
// Comportamiento (camino del teléfono): el mensaje tiene que ser EXACTAMENTE "/nombre" de un atajo que existe y que yo escribí (el
// motor ignora lo demás). Con atajo_modo=editar (por defecto) mi mensaje "/gracias" se EDITA y queda con el texto del atajo; si
// WhatsApp no deja editarlo (pasó el plazo) se manda el texto aparte. Con atajo_modo=enviar el texto sale aparte y mi "/gracias" se
// queda. Después salen los archivos (hasta 3, con 1 a 3 s entre uno y otro). Nunca se manda a otro chat y nunca escribe primero: es
// algo que yo pedí. En la app, el mismo atajo se elige al escribir "/" en la caja de mensaje (selector) y se manda como cliente normal.
type Atajos struct{ c AtajosConfig }

// NuevosAtajos crea las respuestas rápidas.
func NuevosAtajos(c AtajosConfig) *Atajos {
	if c.Ahora == nil {
		c.Ahora = time.Now
	}
	if c.Dormir == nil {
		c.Dormir = func(ctx context.Context, d time.Duration) bool {
			t := time.NewTimer(d)
			defer t.Stop()
			select {
			case <-t.C:
				return true
			case <-ctx.Done():
				return false
			}
		}
	}
	if c.Aleatorio == nil {
		c.Aleatorio = rand.Int63n
	}
	if c.Log == nil {
		c.Log = func(string) {}
	}
	return &Atajos{c: c}
}

func (a *Atajos) activos() bool { return a.c.Alm.AjusteBot(a.c.BotID, "atajos_activo") != "0" }

// Existe dice si hay un atajo activo con ese nombre y la función está encendida.
func (a *Atajos) Existe(nombre string) bool {
	nombre = strings.ToLower(strings.TrimSpace(nombre))
	if nombre == "" || !a.activos() {
		return false
	}
	return a.c.Alm.ExisteAtajo(a.c.BotID, nombre)
}

// Texto da el texto del atajo con las variables puestas para ese chat (lo usa el selector de la app).
func (a *Atajos) Texto(m almacen.AutoMensaje, chat string) string {
	return a.c.Dec.Render(m.Texto, Config{}, chat, chat, a.c.Ahora())
}

// Ejecutar manda la respuesta rápida en el chat donde la pedí.
func (a *Atajos) Ejecutar(ctx context.Context, msg motor.Mensaje) {
	nombre := strings.ToLower(strings.TrimPrefix(strings.TrimSpace(msg.Texto), "/"))
	m := a.c.Alm.AutoPorNombre(a.c.BotID, nombre)
	if m == nil || !a.activos() {
		return
	}
	persona := msg.Chat
	if msg.EsGrupo {
		persona = ""
	}
	texto := a.c.Dec.Render(m.Texto, Config{}, msg.Chat, persona, a.c.Ahora())
	info := &motor.AutoInfo{ID: m.ID, Tipo: almacen.AutoAtajo, Chat: msg.Chat}
	fin := func(res string) {
		a.c.Alm.RegistrarAutoEnvio(a.c.BotID, almacen.AutoEnvio{TS: a.c.Ahora().Unix(), AutoID: m.ID, Tipo: almacen.AutoAtajo, Chat: msg.Chat, Resultado: res, AdjOK: info.AdjOK, AdjFallo: info.AdjFallo, Motivo: info.Motivo})
	}
	hecho := false
	if a.c.Alm.AjusteBot(a.c.BotID, "atajo_modo") != "enviar" {
		if err := a.c.Envio.Editar(ctx, msg.Chat, msg.ID, texto); err == nil {
			hecho = true
			a.c.Alm.MarcarAutomatico(a.c.BotID, msg.Chat, msg.ID, almacen.AutoAtajo)
		} else {
			a.c.Log("No se pudo editar el «/atajo» (" + err.Error() + "); se manda el texto aparte.")
		}
	}
	if !hecho {
		ctxA := almacen.ConEtiqueta(ctx, almacen.AutoAtajo)
		if _, err := a.c.Envio.EnviarTexto(ctxA, msg.Chat, texto); err != nil {
			info.Motivo = "texto_fallo"
			fin("fallo")
			a.c.Log("No se pudo mandar el atajo: " + err.Error())
			return
		}
	}
	for i, ad := range m.Adjuntos {
		if i >= almacen.MaxAdjuntos {
			break
		}
		span := int64(2 * time.Second)
		if !a.c.Dormir(ctx, time.Second+time.Duration(a.c.Aleatorio(span))) {
			return
		}
		ctxA := almacen.ConEtiqueta(ctx, almacen.AutoAtajo)
		if err := a.c.Envio.EnviarMedio(ctxA, msg.Chat, motor.Medio{ID: ad.ID, Tipo: ad.Tipo, Mime: ad.Mime, Nombre: ad.Nombre}); err != nil {
			info.Fallo("archivo_fallo")
		} else {
			info.AdjOK++
		}
	}
	if info.AdjFallo > 0 {
		fin(almacen.AutoParcial)
	} else {
		fin(almacen.AutoOK)
	}
}

var _ motor.Atajador = (*Atajos)(nil)
