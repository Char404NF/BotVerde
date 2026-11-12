package silencio

import (
	"errors"
	"fmt"
	"strings"
)

// ValidarAjuste revisa un ajuste sil_*. conocido = false si la clave no es de este paquete.
func ValidarAjuste(clave, valor string) (conocido bool, err error) {
	num := func(min, max int) error {
		var n int
		if _, e := fmt.Sscanf(valor, "%d", &n); e != nil || n < min || n > max || fmt.Sprint(n) != valor {
			return fmt.Errorf("el valor debe estar entre %d y %d", min, max)
		}
		return nil
	}
	switch clave {
	case "sil_activo", "sil_expulsion_auto", "sil_conteo_por_grupo", "sil_avisar_inicio", "sil_avisar_fin", "sil_aviso_noadmin":
		return true, num(0, 1)
	case "sil_avisos_hora":
		return true, num(0, 1000)
	case "sil_avisos_dia":
		return true, num(0, 10000)
	case "sil_avisos_persona_dia":
		return true, num(0, 100)
	case "sil_borrados_min":
		return true, num(0, 600)
	case "sil_aviso_cada":
		return true, num(0, 1000)
	case "sil_tope":
		return true, num(0, 10000)
	case "sil_largo_min":
		return true, num(1, 525600)
	case "sil_sancion":
		switch valor {
		case SancConfirmar, SancAvisar, SancSilencioLargo, SancExpulsar:
			return true, nil
		}
		return true, errors.New("la sanción debe ser confirmar, avisar, silencio_largo o expulsar")
	case "sil_reinicio_hora":
		var h, m int
		if _, e := fmt.Sscanf(valor, "%d:%d", &h, &m); e != nil || h < 0 || h > 23 || m < 0 || m > 59 || len(valor) != 5 || !strings.Contains(valor, ":") {
			return true, errors.New("la hora debe ser como 00:00")
		}
		return true, nil
	case "adv_caducidad_dias":
		return true, num(0, 3650)
	}
	return false, nil
}
