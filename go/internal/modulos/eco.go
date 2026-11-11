package modulos

import (
	"errors"
	"strconv"
	"strings"
	"sync/atomic"

	"botchat/internal/comandos"
	"botchat/internal/permisos"
)

// IDEco es el id del módulo de ejemplo.
const IDEco = "eco_prueba"

// Eco devuelve el módulo "Eco de prueba": el ejemplo más pequeño que usa todo el marco (casilla, comando, esquema con los cinco tipos de
// campo, eventos e iniciativa por la cola). NO se registra solo: solo las pruebas llaman a RegistrarEco, así que no aparece en la app.
func Eco() Modulo {
	var vistos atomic.Int64
	return Nuevo(Def{
		ID: IDEco, Nombre: "Eco de prueba", Icono: "eco", Riesgo: RiesgoMedio, Iniciativa: true,
		Descripcion: "Módulo de ejemplo: repite una frase y saluda a quien entra a un grupo. Solo sirve para probar.",
		Casillas: []permisos.Casilla{
			{ID: "eco.usar", Nombre: "Usar el eco", Descripcion: "Puede pedirle una frase al eco de prueba.", Grupo: "Eco de prueba"},
		},
		Esquema: []Campo{
			{Clave: "frase", Titulo: "Frase", Tipo: CampoTexto, Defecto: "eco", Largo: 40, Obligatorio: true},
			{Clave: "veces", Titulo: "Veces", Tipo: CampoNumero, Defecto: 1.0, Min: 1, Max: 5},
			{Clave: "gritar", Titulo: "En mayúsculas", Tipo: CampoInterruptor, Defecto: false},
			{Clave: "estilo", Titulo: "Estilo", Tipo: CampoLista, Defecto: "normal", Opciones: []Opcion{{"normal", "Normal"}, {"corto", "Corto"}}},
			{Clave: "hora_aviso", Titulo: "Hora del aviso", Tipo: CampoHora, Defecto: "09:00"},
			{Clave: "chat_aviso", Titulo: "Chat del aviso", Tipo: CampoTexto, Defecto: ""},
		},
		TopeHora: 5, TopeDia: 20,
		Comandos: []comandos.Comando{{
			Nombre: "eco", Resumen: "Repite la frase del eco", Casilla: "eco.usar",
			Ejecutar: func(cx *comandos.Contexto) (string, error) {
				if strings.TrimSpace(cx.Resto) == "" {
					return "", errors.New("escribe algo después del comando")
				}
				return cx.Resto, nil
			},
		}},
		AlEvento: func(g *Gestor, e Evento) {
			vistos.Add(1)
			cfg := g.Config(IDEco)
			texto := cfg.Texto("frase")
			if cfg.Bool("gritar") {
				texto = strings.ToUpper(texto)
			}
			switch e.Tipo {
			case EvMiembroEntra:
				_, _ = g.Enviar(Iniciativa{Modulo: IDEco, Que: "bienvenida", Chat: e.Chat, Personas: e.Personas, Texto: texto})
			case EvTick:
				if c := cfg.Texto("chat_aviso"); c != "" {
					_, _ = g.Enviar(Iniciativa{Modulo: IDEco, Que: "aviso", Chat: c, Texto: texto})
				}
			}
		},
		AlEstado: func(g *Gestor) string {
			if n := vistos.Load(); n > 0 {
				return "Encendido · " + plural(n)
			}
			return ""
		},
	})
}

func plural(n int64) string {
	if n == 1 {
		return "1 evento visto"
	}
	return strconv.FormatInt(n, 10) + " eventos vistos"
}

// RegistrarEco agrega el módulo de ejemplo al registro (solo para pruebas).
func RegistrarEco() error { return Registrar(Eco()) }
