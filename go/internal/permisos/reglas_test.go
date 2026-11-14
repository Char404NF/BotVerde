package permisos

import (
	"testing"

	"botchat/internal/almacen"
)

func TestRolAdminDeFabrica(t *testing.T) {
	admin := RolAdmin()
	if err := ValidarRol(&admin); err != nil {
		t.Fatal(err)
	}
	dueno := &almacen.Rol{EsDueno: true}
	otro := almacen.Rol{Nombre: "Admin", Posicion: 1}
	if PuedeActuar(&admin, dueno) {
		t.Error("un Admin no debe poder actuar sobre el Dueño")
	}
	if PuedeActuar(&otro, &otro) {
		t.Error("un Admin no debe poder actuar sobre otro Admin")
	}
	usuario := (*almacen.Rol)(nil)
	if !PuedeActuar(&otro, usuario) {
		t.Error("un Admin sí debe poder actuar sobre un usuario sin rango")
	}
	for _, id := range []string{"grupos.expulsar", "silencio.silenciar", "personas.silenciar"} {
		if _, ok := admin.Casillas[id]; !ok {
			t.Errorf("al Admin le falta %s", id)
		}
	}
	for _, id := range []string{"rangos.asignar", "personas.olvidar", "grupos.ascender"} {
		if _, ok := admin.Casillas[id]; ok {
			t.Errorf("el Admin no debe tener %s", id)
		}
	}
}
