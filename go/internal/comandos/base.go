package comandos

import (
	"botchat/internal/estilo"
	"errors"
	"fmt"
	"strings"
	"time"

	"botchat/internal/almacen"
	"botchat/internal/permisos"
)

func (d *Despachador) registrarBase() {
	_ = d.Registrar(Comando{Nombre: "ayuda", Alias: []string{"help", "comandos"}, Resumen: "Ver lo que puedes usar", Casilla: "cmd.ayuda", Avatar: "ayuda", Ejecutar: cmdAyuda})
	_ = d.Registrar(Comando{Nombre: "rango", Resumen: "Ver cuál es tu rango", Casilla: "cmd.rango", Ejecutar: cmdRango})
	_ = d.Registrar(Comando{Nombre: "asignar", Alias: []string{"darrango"}, Resumen: "Dar un rango a una persona", Uso: "asignar <persona> <rango> [24h|7d|permanente]",
		Casilla: "rangos.asignar", Ejecutar: cmdAsignar})
	_ = d.Registrar(Comando{Nombre: "quitarrango", Alias: []string{"quitar"}, Resumen: "Quitar el rango a una persona", Uso: "quitarrango <persona>",
		Casilla: "rangos.asignar", Ejecutar: cmdQuitarRango})
	_ = d.Registrar(Comando{Nombre: "confirmar", Resumen: "Confirmar con un código", Uso: "confirmar <código>", Casilla: "cmd.confirmar", Ejecutar: cmdConfirmar})
}

func cmdAyuda(cx *Contexto) (string, error) {
	cs := cx.D.Lista(cx)
	pref := cx.D.Prefijo()
	rangoTxt := ""
	if cx.Rol != nil {
		rangoTxt = " (rango " + cx.Rol.Nombre + ")"
	}
	titulo := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(cx.D.R("cmd.ayuda_titulo", "rango", rangoTxt)), ":"))
	var lineas []string
	for _, c := range cs {
		l := "• *" + pref + c.Nombre + "*"
		if c.Resumen != "" {
			l += " — " + c.Resumen
		}
		lineas = append(lineas, l)
	}
	if cx.D.Extra != nil {
		lineas = append(lineas, cx.D.Extra(cx)...)
	}
	return estilo.Tarjeta("🤖", titulo, lineas, cx.D.R(map[bool]string{false: "cmd.pie_menu", true: "cmd.pie_menu_grupo"}[cx.EsGrupo], "prefijo", pref)), nil
}

func textoCaduca(as *almacen.Asignacion, ahora time.Time) string {
	if as == nil || as.CaducaEn == 0 {
		return ""
	}
	resta := time.Unix(as.CaducaEn, 0).Sub(ahora)
	switch {
	case resta > 48*time.Hour:
		return fmt.Sprintf(" (vence en %d días)", int(resta.Hours()/24))
	case resta > time.Hour:
		return fmt.Sprintf(" (vence en %d horas)", int(resta.Hours()))
	}
	return " (vence en menos de una hora)"
}

func cmdRango(cx *Contexto) (string, error) {
	if cx.Dueno {
		return cx.D.R("cmd.eres_dueno"), nil
	}
	r, as, _ := cx.D.Alm.RolDePersona(cx.D.BotID, cx.Persona, cx.D.ahora())
	if r == nil {
		return cx.D.R("cmd.sin_rango", "prefijo", cx.D.Prefijo()), nil
	}
	return cx.D.R("cmd.tu_rango", "rango", r.Nombre, "vence", textoCaduca(as, cx.D.ahora())), nil
}

// duraciones que se reconocen al final de !asignar
func esDuracion(s string) bool {
	_, err := permisos.CaducidadDe(s, time.Now())
	return err == nil && strings.TrimSpace(s) != ""
}

func cmdAsignar(cx *Contexto) (string, error) {
	obj, resto, ok := cx.Objetivo()
	if !ok {
		return "", errors.New("dime a quién: menciónalo, cita su mensaje o escribe su número")
	}
	dur := "permanente"
	if n := len(resto); n > 0 && esDuracion(resto[n-1]) {
		dur, resto = resto[n-1], resto[:n-1]
	}
	nombre := strings.TrimSpace(strings.Join(resto, " "))
	if nombre == "" {
		return "", errors.New("falta el rango")
	}
	r, err := cx.D.Alm.RolPorNombre(cx.D.BotID, nombre)
	if err != nil || r == nil {
		return "", errors.New("no existe ese rango")
	}
	cad, err := permisos.CaducidadDe(dur, cx.D.ahora())
	if err != nil {
		return "", err
	}
	if err := cx.D.Perm.AsignarComo(cx.Persona, obj, r.ID, cad); err != nil {
		return "", err
	}
	fin := ""
	if cad > 0 {
		fin = " por " + map[bool]string{true: "24 horas", false: "7 días"}[time.Unix(cad, 0).Sub(cx.D.ahora()) < 48*time.Hour]
	}
	return cx.D.R("cmd.rango_dado", "rango", r.Nombre, "duracion", fin), nil
}

func cmdQuitarRango(cx *Contexto) (string, error) {
	obj, _, ok := cx.Objetivo()
	if !ok {
		return "", errors.New("dime a quién: menciónalo, cita su mensaje o escribe su número")
	}
	if err := cx.D.Perm.QuitarComo(cx.Persona, obj); err != nil {
		return "", err
	}
	return cx.D.R("cmd.rango_quitado"), nil
}

func cmdConfirmar(cx *Contexto) (string, error) {
	if len(cx.Args) == 0 {
		return "", errors.New("escribe el código, por ejemplo " + cx.D.Prefijo() + "confirmar AB12CD")
	}
	t, err := cx.D.Confirmar(cx.Args[0], cx.Persona)
	if err != nil {
		return "", err
	}
	if t == "" {
		t = "Listo."
	}
	return t, nil
}
