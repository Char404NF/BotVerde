package flujo

import (
	"strings"
	"testing"
)

func TestBotParaTodosEsValidoYNoSeActivaConPalabrasSueltas(t *testing.T) {
	d := PlantillaBotParaTodos()
	d.Normalizar()
	for _, a := range d.Validar() {
		if a.Grave {
			t.Fatalf("el menú de fábrica no debe tener problemas graves: %+v", a)
		}
	}
	if len(d.Disparadores.Palabras) != 0 || d.Disparadores.Comando != ".menu" || !d.Disparadores.Mencion || d.Disparadores.Cualquiera {
		t.Fatalf("solo se abre con el símbolo o mencionando: %+v", d.Disparadores)
	}
	if !d.Alcance.Privados || !d.Alcance.TodosGrupos {
		t.Fatalf("alcance: %+v", d.Alcance)
	}
	// ninguna opción lleva a un paso inexistente y todos los comandos que explica existen con ese nombre
	if m := d.Paso("m"); len(m.Opciones) != 7 || len(m.Opciones) > 10 {
		t.Fatalf("el menú principal cabe en una lista: %d opciones", len(m.Opciones))
	}
	for _, p := range d.Pasos {
		if p.Tipo == PasoMenu && (len(p.Opciones) < 2 || len(p.Opciones) > 9) {
			t.Fatalf("%s: %d opciones", p.ID, len(p.Opciones))
		}
		for _, o := range p.Opciones {
			if len([]rune(o.Texto)) > 24 {
				t.Fatalf("«%s» es largo para una fila de lista (máx. 24 letras)", o.Texto)
			}
		}
	}
	todo := ""
	for _, p := range d.Pasos {
		todo += p.Texto
	}
	for _, c := range []string{"!sticker", "!descargar", "!ia", "!anotar", "!tarea", "!clima", "!calc", "!hora", "!dolar", "!definir", "!acortar", "!qr", "!juegos", "!chiste", "!dato", "!frase", "!ayuda"} {
		if !strings.Contains(todo, c) {
			t.Errorf("el menú no explica %s", c)
		}
	}
}

func TestBotParaTodosEnLaPracticaConDuenoYSubmenus(t *testing.T) {
	e := nuevo(t)
	e.crea(t, PlantillaBotParaTodos(), true)
	if d := e.dice(t, ana, ana, "menu"); len(d.Respuestas) != 0 {
		t.Fatalf("la palabra suelta no abre el menú: %q", textos(d))
	}
	d := e.dice(t, ana, ana, "!menu")
	if m := d.Respuestas[0].Menu; m == nil || len(m.Opciones) != 7 || !strings.Contains(m.Titulo, "¿Qué quieres hacer") {
		t.Fatalf("menú principal: %+v", d.Respuestas)
	}
	d = e.dice(t, ana, ana, "4")                                                               // Utilidades
	if m := d.Respuestas[0].Menu; m == nil || len(m.Opciones) != 7 || m.Opciones[6].Num != 0 { // 6 opciones + volver
		t.Fatalf("submenú: %+v", d.Respuestas[0].Menu)
	}
	d = e.dice(t, ana, ana, "1") // Clima
	if !strings.Contains(textos(d), "!clima Monterrey") {
		t.Fatalf("respuesta: %q", textos(d))
	}
}
