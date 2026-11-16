package wabot

import (
	"testing"

	"botchat/internal/ingesta"
	"botchat/internal/modulos"
)

// Cierre v0.6.0 (E10): lo que sale encendido y apagado en una instalación nueva.
func TestReglasDeFabricaV060(t *testing.T) {
	_, a := abreSolo(t)
	// Apagados: todos los módulos registrados (difusión, invitar, programados, bienvenida por privado, descargas, traductor, IA, juegos, H1–H9...).
	lista := decodifica[[]modulos.Info](t, func() (string, error) { return ListarModulos(1) })
	if len(lista) < 9 {
		t.Fatalf("faltan módulos registrados: %d", len(lista))
	}
	for _, i := range lista {
		if i.Activo {
			t.Fatalf("el módulo %s sale encendido de fábrica", i.ID)
		}
	}
	aj := a.AjustesBot(1)
	for _, k := range []string{"lab_activo", "lab_grupos", "sil_expulsion_auto"} {
		if aj[k] != "0" {
			t.Fatalf("%s debe salir apagado, vale %q", k, aj[k])
		}
	}
	// Encendidos: auditoría, silenciar con avisos, advertencias, alertas, automáticos, utilidades del asistente.
	for _, k := range []string{"audit_defecto", "sil_activo", "sil_avisar_inicio", "sil_avisar_fin", "alertas_activas", "auto_activo", "util_notas", "util_recordatorios", "conservar_portada", "tope_confirmado"} {
		if aj[k] != "1" {
			t.Fatalf("%s debe salir encendido, vale %q", k, aj[k])
		}
	}
	if aj["adv_tope"] != "3" || aj["audit_ret_dias"] != "90" {
		t.Fatalf("advertencias/retención: %v %v", aj["adv_tope"], aj["audit_ret_dias"])
	}
	// Descarga multimedia encendida para todos los tipos.
	p := ingesta.PoliticaPorDefecto()
	for _, r := range []ingesta.Regla{p.Foto, p.Audio, p.Documento, p.Video, p.Sticker} {
		if r.Apagada || (!r.Wifi && !r.Datos) {
			t.Fatalf("la descarga multimedia debe salir encendida: %+v", p)
		}
	}
}
