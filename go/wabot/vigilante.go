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
	// Aviso de desconexión larga: tras `avisoTras()` sin conexión se llama alLargo(minutos); al volver, alVolver(minutos que duró).
	avisoTras func() time.Duration
	alLargo   func(min int)
	alVolver  func(min int)
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
	avisado := false
	var caido time.Time // desde cuándo no hay conexión (no se reinicia al reintentar)
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		if !v.vinculado() || v.conectado() {
			if avisado && v.alVolver != nil && !caido.IsZero() {
				v.alVolver(int(v.ahora().Sub(caido) / time.Minute))
			}
			avisado = false
			caido = time.Time{}
			desde = time.Time{}
			continue
		}
		ahora := v.ahora()
		if desde.IsZero() {
			desde, caido = ahora, ahora
			continue
		}
		if !avisado && v.alLargo != nil && v.avisoTras != nil {
			if t := v.avisoTras(); t > 0 && ahora.Sub(caido) >= t {
				avisado = true
				v.alLargo(int(ahora.Sub(caido) / time.Minute))
			}
		}
		if ahora.Sub(desde) >= v.limite {
			v.log("Más de 3 minutos sin conexión: se intenta conectar de nuevo.")
			v.conectar()
			desde = ahora
		}
	}
}
