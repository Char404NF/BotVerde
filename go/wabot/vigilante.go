package wabot

import (
	"context"
	"time"
)

// vigilante reconecta si el bot lleva más de `limite` desconectado (3 minutos). Vive dentro del servicio y se detiene con Stop.
type vigilante struct {
	vinculado func() bool
	conectado func() bool
	conectar  func()
	log       func(string)
	limite    time.Duration
	paso      time.Duration
	ahora     func() time.Time
}

func (v *vigilante) correr(ctx context.Context) {
	if v.limite <= 0 {
		v.limite = 3 * time.Minute
	}
	if v.paso <= 0 {
		v.paso = 30 * time.Second
	}
	if v.ahora == nil {
		v.ahora = time.Now
	}
	t := time.NewTicker(v.paso)
	defer t.Stop()
	var desde time.Time
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		if !v.vinculado() || v.conectado() {
			desde = time.Time{}
			continue
		}
		ahora := v.ahora()
		if desde.IsZero() {
			desde = ahora
			continue
		}
		if ahora.Sub(desde) >= v.limite {
			v.log("Más de 3 minutos sin conexión: se intenta conectar de nuevo.")
			v.conectar()
			desde = ahora
		}
	}
}
