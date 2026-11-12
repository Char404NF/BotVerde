// Package utilidades es el bloque F: notas y pendientes propios por comando, y convertir una imagen en sticker. Son comandos normales
// del despachador (con su casilla de permisos, su límite por minuto y sus ámbitos); el bot solo RESPONDE.
//
// Las notas y los pendientes son de quien los escribe: el Dueño los usa desde "Mensajes guardados" o la app, y los demás rangos en su
// chat privado con el bot. En un grupo NO funcionan (la respuesta la vería todo el grupo).
package utilidades

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"botchat/internal/almacen"
	"botchat/internal/comandos"
)

// Servicio guarda y lista las notas y los pendientes de cada persona.
type Servicio struct {
	Alm   *almacen.Almacen
	BotID int64
	Ahora func() time.Time
}

// Nuevo crea el servicio.
func Nuevo(alm *almacen.Almacen, botID int64) *Servicio {
	return &Servicio{Alm: alm, BotID: botID, Ahora: time.Now}
}

func (s *Servicio) ahora() time.Time {
	if s.Ahora != nil {
		return s.Ahora()
	}
	return time.Now()
}

// Agregar guarda una nota o un pendiente y devuelve su número en la lista.
func (s *Servicio) Agregar(persona, tipo, texto string) (int, error) {
	if _, err := s.Alm.AgregarItemUtil(s.BotID, persona, tipo, texto, s.ahora()); err != nil {
		return 0, err
	}
	return len(s.Alm.ListarItemsUtil(s.BotID, persona, tipo, true)), nil
}

// ErrNumero es el error cuando el número no existe en la lista.
var ErrNumero = errors.New("ese número no está en la lista")

// idDe da el id del elemento número n (desde 1) de la lista de ese tipo.
func (s *Servicio) idDe(persona, tipo string, n int) (int64, error) {
	l := s.Alm.ListarItemsUtil(s.BotID, persona, tipo, true)
	if n < 1 || n > len(l) {
		return 0, ErrNumero
	}
	return l[n-1].ID, nil
}

// Borrar quita el elemento número n de la lista.
func (s *Servicio) Borrar(persona, tipo string, n int) error {
	id, err := s.idDe(persona, tipo, n)
	if err != nil {
		return err
	}
	_, err = s.Alm.BorrarItemUtil(s.BotID, persona, id)
	return err
}

// Marcar marca (o desmarca) el pendiente número n.
func (s *Servicio) Marcar(persona string, n int, hecho bool) error {
	id, err := s.idDe(persona, almacen.UtilPendiente, n)
	if err != nil {
		return err
	}
	_, err = s.Alm.MarcarItemUtil(s.BotID, persona, id, hecho, s.ahora())
	return err
}

// Texto arma la lista legible de notas o de pendientes.
func (s *Servicio) Texto(persona, tipo string) string {
	l := s.Alm.ListarItemsUtil(s.BotID, persona, tipo, true)
	if len(l) == 0 {
		if tipo == almacen.UtilNota {
			return "No tienes notas. Guarda una con !anotar y el texto."
		}
		return "No tienes pendientes. Agrega uno con !tarea y el texto."
	}
	var b strings.Builder
	if tipo == almacen.UtilNota {
		b.WriteString("*Tus notas*")
	} else {
		b.WriteString("*Tus pendientes*")
	}
	hechos := 0
	for i, x := range l {
		marca := ""
		if tipo == almacen.UtilPendiente {
			marca = "[ ] "
			if x.Hecho {
				marca = "[x] "
				hechos++
			}
		}
		fmt.Fprintf(&b, "\n%d. %s%s", i+1, marca, x.Texto)
	}
	if tipo == almacen.UtilPendiente {
		fmt.Fprintf(&b, "\n\n%d por hacer, %d hechos.", len(l)-hechos, hechos)
	}
	return b.String()
}

func numero(cx *comandos.Contexto, uso string) (int, error) {
	if len(cx.Args) < 1 {
		return 0, fmt.Errorf("dime el número, por ejemplo %s%s", cx.D.Prefijo(), uso)
	}
	n, err := strconv.Atoi(strings.TrimRight(cx.Args[0], ".)"))
	if err != nil || n < 1 {
		return 0, fmt.Errorf("dime el número de la lista, por ejemplo %s%s", cx.D.Prefijo(), uso)
	}
	return n, nil
}

// RegistrarComandos agrega los comandos de notas y pendientes a un despachador. El sticker lo registra su propio servicio.
func (s *Servicio) RegistrarComandos(d *comandos.Despachador) {
	reg := func(c comandos.Comando) {
		c.Ambito = comandos.Privado
		// v0.6.0 E9: el asistente «Crea tu bot» puede apagar las notas y los pendientes (util_notas = 0).
		if ej := c.Ejecutar; ej != nil {
			c.Ejecutar = func(cx *comandos.Contexto) (string, error) {
				if s.Alm.AjusteBot(s.BotID, "util_notas") == "0" {
					return "", fmt.Errorf("las notas y los pendientes están apagados")
				}
				return ej(cx)
			}
		}
		_ = d.Registrar(c)
	}
	ag := func(tipo, ok string) func(cx *comandos.Contexto) (string, error) {
		return func(cx *comandos.Contexto) (string, error) {
			if strings.TrimSpace(cx.Resto) == "" {
				return "", fmt.Errorf("escribe el texto después del comando")
			}
			n, err := s.Agregar(cx.Persona, tipo, cx.Resto)
			if err != nil {
				return "", err
			}
			return fmt.Sprintf("%s (número %d).", ok, n), nil
		}
	}
	lista := func(tipo string) func(cx *comandos.Contexto) (string, error) {
		return func(cx *comandos.Contexto) (string, error) { return s.Texto(cx.Persona, tipo), nil }
	}
	reg(comandos.Comando{Nombre: "anotar", Alias: []string{"apuntar"}, Resumen: "Guardar una nota tuya", Uso: "anotar <texto>", Casilla: "util.notas",
		Ejecutar: ag(almacen.UtilNota, "Nota guardada")})
	reg(comandos.Comando{Nombre: "notas", Alias: []string{"misnotas"}, Resumen: "Ver tus notas", Uso: "notas", Casilla: "util.notas",
		Ejecutar: lista(almacen.UtilNota)})
	reg(comandos.Comando{Nombre: "borrarnota", Alias: []string{"quitarnota"}, Resumen: "Borrar una nota por su número", Uso: "borrarnota <número>", Casilla: "util.notas",
		Ejecutar: func(cx *comandos.Contexto) (string, error) {
			n, err := numero(cx, "borrarnota 2")
			if err != nil {
				return "", err
			}
			if err := s.Borrar(cx.Persona, almacen.UtilNota, n); err != nil {
				return "", err
			}
			return "Nota borrada.", nil
		}})
	reg(comandos.Comando{Nombre: "tarea", Alias: []string{"porhacer"}, Resumen: "Agregar un pendiente a tu lista", Uso: "tarea <texto>", Casilla: "util.pendientes",
		Ejecutar: ag(almacen.UtilPendiente, "Pendiente agregado")})
	reg(comandos.Comando{Nombre: "tareas", Alias: []string{"pendientes"}, Resumen: "Ver tu lista de pendientes", Uso: "tareas", Casilla: "util.pendientes",
		Ejecutar: lista(almacen.UtilPendiente)})
	reg(comandos.Comando{Nombre: "hecho", Alias: []string{"listo"}, Resumen: "Marcar un pendiente como hecho", Uso: "hecho <número>", Casilla: "util.pendientes",
		Ejecutar: func(cx *comandos.Contexto) (string, error) {
			n, err := numero(cx, "hecho 1")
			if err != nil {
				return "", err
			}
			if err := s.Marcar(cx.Persona, n, true); err != nil {
				return "", err
			}
			return "Marcado como hecho.", nil
		}})
	reg(comandos.Comando{Nombre: "deshacer", Resumen: "Volver a poner un pendiente por hacer", Uso: "deshacer <número>", Casilla: "util.pendientes",
		Ejecutar: func(cx *comandos.Contexto) (string, error) {
			n, err := numero(cx, "deshacer 1")
			if err != nil {
				return "", err
			}
			if err := s.Marcar(cx.Persona, n, false); err != nil {
				return "", err
			}
			return "Vuelve a estar por hacer.", nil
		}})
	reg(comandos.Comando{Nombre: "borrartarea", Alias: []string{"quitartarea"}, Resumen: "Borrar un pendiente por su número", Uso: "borrartarea <número>", Casilla: "util.pendientes",
		Ejecutar: func(cx *comandos.Contexto) (string, error) {
			n, err := numero(cx, "borrartarea 1")
			if err != nil {
				return "", err
			}
			if err := s.Borrar(cx.Persona, almacen.UtilPendiente, n); err != nil {
				return "", err
			}
			return "Pendiente borrado.", nil
		}})
	reg(comandos.Comando{Nombre: "limpiartareas", Resumen: "Borrar los pendientes que ya están hechos", Uso: "limpiartareas", Casilla: "util.pendientes",
		Ejecutar: func(cx *comandos.Contexto) (string, error) {
			n, err := s.Alm.BorrarHechosUtil(s.BotID, cx.Persona)
			if err != nil {
				return "", err
			}
			if n == 0 {
				return "No había pendientes hechos.", nil
			}
			return fmt.Sprintf("Listo: borré %d pendientes hechos.", n), nil
		}})
}
