package wabot

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"botchat/internal/almacen"
	"botchat/internal/envio"
	"botchat/internal/flujo"
	"botchat/internal/motor"
)

// API de los menús ampliados (etapa C): ajustes generales, formularios recibidos, lo que el menú no entendió, adjuntos y exportar/importar.

var clavesAjustesMenus = []string{
	"menu_err_max", "menu_txt_n1", "menu_txt_n2", "menu_txt_n3", "menu_txt_notexto", "menu_txt_ambiguo", "menu_txt_salir", "menu_txt_cancelar",
	"menu_saludos", "menu_volver", "menu_retoma_horas", "menu_humano_dentro", "menu_humano_fuera", "menu_horario", "menu_region",
	"noentendio_activo", "noentendio_dias",
	"grupo_max_sesiones", "grupo_expira_min", "grupo_msgs_min", "grupo_cualquiera", "grupo_cola_min", "grupo_txt_turno", "grupo_txt_vencido", "grupo_txt_privado",
}

var (
	rangosAjustesMenus = map[string][2]int{
		"menu_err_max": {1, 10}, "menu_retoma_horas": {1, 168}, "noentendio_dias": {0, 365},
		"grupo_max_sesiones": {1, 100}, "grupo_expira_min": {1, 1440}, "grupo_msgs_min": {1, 60}, "grupo_cola_min": {1, 120},
	}
	reRegion = regexp.MustCompile(`^[A-Z]{2}$`)
	reHora   = regexp.MustCompile(`^([01]?\d|2[0-3]):[0-5]\d$`)
)

// AjustesDeMenus devuelve (JSON) los ajustes generales de los menús (los que cada menú puede reemplazar) con sus valores actuales.
func AjustesDeMenus(botID int64) (string, error) {
	a, err := almacenAbierto()
	if err != nil {
		return "", err
	}
	out := map[string]string{}
	for _, k := range clavesAjustesMenus {
		out[k] = a.AjusteBot(botID, k)
	}
	return js(out, nil)
}

// FijarAjusteDeMenus cambia un ajuste general de los menús, ya validado.
func FijarAjusteDeMenus(botID int64, clave, valor string) error {
	a, err := almacenAbierto()
	if err != nil {
		return err
	}
	ok := false
	for _, k := range clavesAjustesMenus {
		if k == clave {
			ok = true
		}
	}
	if !ok {
		return errors.New("Ese ajuste no existe.")
	}
	valor = strings.TrimSpace(valor)
	switch {
	case clave == "noentendio_activo" || clave == "grupo_cualquiera":
		if valor != "0" && valor != "1" {
			return errors.New("Ese ajuste es de sí o no.")
		}
	case clave == "menu_volver":
		if valor != "retomar" && valor != "reiniciar" {
			return errors.New("Elige retomar o reiniciar.")
		}
	case clave == "menu_region":
		valor = strings.ToUpper(valor)
		if !reRegion.MatchString(valor) {
			return errors.New("Escribe el país con 2 letras, por ejemplo MX.")
		}
	case clave == "menu_horario":
		if valor != "" {
			var h flujo.Horario
			if json.Unmarshal([]byte(valor), &h) != nil {
				return errors.New("El horario no es válido.")
			}
			for _, d := range h.Dias {
				if d < 1 || d > 7 {
					return errors.New("Los días del horario van de 1 (lunes) a 7 (domingo).")
				}
			}
			for _, x := range []string{h.Desde, h.Hasta} {
				if x != "" && !reHora.MatchString(x) {
					return errors.New("Las horas se escriben como 09:00.")
				}
			}
		}
	case strings.HasPrefix(clave, "menu_txt_") || strings.HasPrefix(clave, "menu_humano_") || strings.HasPrefix(clave, "grupo_txt_") || clave == "menu_saludos":
		limite := 600
		if clave == "menu_saludos" {
			limite = 1500
		}
		if len([]rune(valor)) > limite {
			return fmt.Errorf("El texto es muy largo (máximo %d letras).", limite)
		}
	default:
		if r, hay := rangosAjustesMenus[clave]; hay {
			n, err := strconv.Atoi(valor)
			if err != nil || n < r[0] || n > r[1] {
				return fmt.Errorf("Escribe un número de %d a %d.", r[0], r[1])
			}
		}
	}
	return a.FijarAjusteBot(botID, clave, valor)
}

// ---------- formularios recibidos ----------

type formularioVista struct {
	almacen.Formulario
	Menu    string `json:"menu"`
	Cliente string `json:"cliente"` // nombre con el que se ve a la persona
}

func (s *estado) vistaForm(a *almacen.Almacen, botID int64, f almacen.Formulario, nombresMenu map[int64]string) formularioVista {
	v := formularioVista{Formulario: f}
	if n, ok := nombresMenu[f.MenuID]; ok {
		v.Menu = n
	} else {
		v.Menu = f.Titulo
	}
	v.Cliente = f.Nombre
	if v.Cliente == "" {
		v.Cliente = a.NombreConocido(botID, f.Chat, f.Persona)
	}
	return v
}

func nombresDeMenus(a *almacen.Almacen, botID int64) map[int64]string {
	out := map[int64]string{}
	ms, _ := a.ListarMenus(botID)
	for _, m := range ms {
		if d, err := flujo.Parse(m.Def); err == nil {
			out[m.ID] = d.Nombre
		}
	}
	return out
}

// ListarFormularios devuelve (JSON) los formularios recibidos, el más nuevo primero. menuID 0 = de todos los menús.
func ListarFormularios(botID, menuID int64, limite int) (string, error) {
	a, err := almacenAbierto()
	if err != nil {
		return "", err
	}
	fs, err := a.ListarFormularios(botID, menuID, limite)
	if err != nil {
		return "", err
	}
	nm := nombresDeMenus(a, botID)
	out := []formularioVista{}
	for _, f := range fs {
		out = append(out, (&estado{}).vistaForm(a, botID, f, nm))
	}
	return js(out, nil)
}

// ObtenerFormulario devuelve (JSON) un formulario recibido con todos sus datos.
func ObtenerFormulario(botID, id int64) (string, error) {
	a, err := almacenAbierto()
	if err != nil {
		return "", err
	}
	f, err := a.ObtenerFormulario(botID, id)
	if err != nil || f == nil {
		return "", errors.New("Ese formulario ya no existe.")
	}
	return js((&estado{}).vistaForm(a, botID, *f, nombresDeMenus(a, botID)), nil)
}

// ContarFormularios cuenta los formularios recibidos (menuID 0 = todos).
func ContarFormularios(botID, menuID int64) (int, error) {
	a, err := almacenAbierto()
	if err != nil {
		return 0, err
	}
	return a.ContarFormularios(botID, menuID), nil
}

// BorrarFormulario borra un formulario recibido.
func BorrarFormulario(botID, id int64) error {
	a, err := almacenAbierto()
	if err != nil {
		return err
	}
	a.RegistrarAccion(botID, almacen.PersonaDueno, "formulario.borrar", "", "", "ok", "")
	return a.BorrarFormulario(botID, id)
}

// BorrarFormularios borra todos los de un menú (0 = todos los menús). Devuelve cuántos.
func BorrarFormularios(botID, menuID int64) (int, error) {
	a, err := almacenAbierto()
	if err != nil {
		return 0, err
	}
	n := a.BorrarFormularios(botID, menuID)
	a.RegistrarAccion(botID, almacen.PersonaDueno, "formulario.borrar_todos", "", "", "ok", fmt.Sprintf("n=%d", n))
	return n, nil
}

// ExportarFormularios guarda los formularios recibidos (CSV con acentos para Excel) dentro de un archivo cifrado BCF1 en la carpeta de
// exportes y devuelve su ruta relativa. Se borra solo en 24 horas.
func ExportarFormularios(botID, menuID int64) (string, error) {
	a, err := almacenAbierto()
	if err != nil {
		return "", err
	}
	fs, err := a.ListarFormularios(botID, menuID, 1000)
	if err != nil {
		return "", err
	}
	if len(fs) == 0 {
		return "", errors.New("Todavía no hay formularios para exportar.")
	}
	nm := nombresDeMenus(a, botID)
	var buf bytes.Buffer
	buf.Write([]byte{0xEF, 0xBB, 0xBF}) // BOM para que Excel lea bien los acentos
	w := csv.NewWriter(&buf)
	_ = w.Write([]string{"Fecha", "Menú", "Persona", "Dato", "Respuesta"})
	zona := a.Zona()
	for _, f := range fs {
		v := (&estado{}).vistaForm(a, botID, f, nm)
		fecha := time.Unix(f.TS, 0).In(zona).Format("2006-01-02 15:04")
		for _, c := range f.Campos {
			_ = w.Write([]string{fecha, v.Menu, v.Cliente, c.Etiqueta, csvSeguro(c.Valor)})
		}
	}
	w.Flush()
	rel := filepath.Join("exportes", fmt.Sprint(botID), fmt.Sprintf("formularios_%d.bcf", time.Now().Unix()))
	if err := a.EscribirCifrado(rel, buf.Bytes()); err != nil {
		return "", err
	}
	a.LimpiarExportes(botID, 24, time.Now())
	a.RegistrarAccion(botID, almacen.PersonaDueno, "formulario.exportar", "", "", "ok", fmt.Sprintf("n=%d", len(fs)))
	return rel, nil
}

// csvSeguro evita que una hoja de cálculo tome una respuesta como fórmula (=, +, -, @).
func csvSeguro(s string) string {
	if s != "" && strings.ContainsRune("=+-@\t\r", rune(s[0])) {
		return "'" + s
	}
	return s
}

// ---------- lo que el menú no entendió ----------

// ListarNoEntendio devuelve (JSON) los mensajes que el menú no entendió (el más nuevo primero). menuID 0 = de todos.
func ListarNoEntendio(botID, menuID int64, limite int) (string, error) {
	a, err := almacenAbierto()
	if err != nil {
		return "", err
	}
	l, err := a.ListarNoEntendio(botID, menuID, limite)
	if err != nil {
		return "", err
	}
	nm := nombresDeMenus(a, botID)
	type fila struct {
		almacen.NoEntendio
		Menu string `json:"menu"`
	}
	out := []fila{}
	for _, x := range l {
		out = append(out, fila{x, nm[x.MenuID]})
	}
	return js(out, nil)
}

// BorrarNoEntendio borra la lista de lo que no entendió un menú (0 = de todos). Devuelve cuántos mensajes borró.
func BorrarNoEntendio(botID, menuID int64) (int, error) {
	a, err := almacenAbierto()
	if err != nil {
		return 0, err
	}
	n := a.BorrarNoEntendio(botID, menuID)
	a.RegistrarAccion(botID, almacen.PersonaDueno, "menu.borrar_noentendio", "", "", "ok", fmt.Sprintf("n=%d", n))
	return n, nil
}

// ---------- adjuntos de los menús ----------

// SubirAdjuntoMenu guarda cifrado un archivo para mandarlo desde un menú o un comando propio. Es el mismo servicio de adjuntos de los
// mensajes automáticos (SubirAdjunto): mismas reglas de tipo y peso, y las notas de voz se validan (.ogg, Opus, mono).
func SubirAdjuntoMenu(botID int64, ruta, tipo, mime, nombre string) (string, error) {
	return SubirAdjunto(botID, ruta, tipo, mime, nombre)
}

// BorrarAdjuntoMenu quita el archivo guardado de un adjunto (cuando ya no lo usa ningún menú).
func BorrarAdjuntoMenu(botID int64, id string) error {
	a, err := almacenAbierto()
	if err != nil {
		return err
	}
	if id == "" || strings.ContainsAny(id, "/\\.") {
		return errors.New("adjunto no válido")
	}
	a.BorrarAdjuntoMenu(botID, id)
	return nil
}

// copiaAdjunto descifra un adjunto de menú en una copia temporal en claro (la borra quien la manda, al terminar).
func (s *estado) copiaAdjunto(id string) (string, error) {
	if id == "" || strings.ContainsAny(id, "/\\.") {
		return "", errors.New("adjunto no válido")
	}
	d, err := alm.AbrirAdjuntoMenu(s.botID, id)
	if err != nil {
		return "", err
	}
	defer d.Close()
	dir := filepath.Join(alm.Raiz(), "tmp")
	_ = os.MkdirAll(dir, 0o700)
	f, err := os.CreateTemp(dir, "menu-*")
	if err != nil {
		return "", err
	}
	if _, err := f.ReadFrom(d); err != nil {
		f.Close()
		_ = os.Remove(f.Name())
		return "", err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(f.Name())
		return "", err
	}
	return f.Name(), nil
}

// ---------- exportar e importar menús ----------

// ExportarMenus crea un archivo cifrado con los menús pedidos (idsJSON: arreglo de ids) y devuelve su ruta relativa. clave vacía = con la
// llave de este teléfono (copia de seguridad); con contraseña (mínimo 6 caracteres) se puede abrir en otro teléfono. Se borra en 24 h.
func ExportarMenus(botID int64, idsJSON, clave string) (string, error) {
	a, err := almacenAbierto()
	if err != nil {
		return "", err
	}
	var ids []int64
	if err := json.Unmarshal([]byte(idsJSON), &ids); err != nil {
		return "", errors.New("Elige los menús que quieres exportar.")
	}
	b, err := flujo.ExportarMenus(a, botID, ids, clave)
	if err != nil {
		return "", err
	}
	rel := filepath.Join("exportes", fmt.Sprint(botID), fmt.Sprintf("menus_%d.bcm", time.Now().Unix()))
	abs, err := a.Abs(rel)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0o700); err != nil {
		return "", err
	}
	if err := os.WriteFile(abs, b, 0o600); err != nil {
		return "", err
	}
	a.LimpiarExportes(botID, 24, time.Now())
	a.RegistrarAccion(botID, almacen.PersonaDueno, "menu.exportar", "", "", "ok", fmt.Sprintf("menus=%d,clave=%t", len(ids), clave != ""))
	return rel, nil
}

// ImportarMenus lee un archivo exportado (la copia en `ruta` se borra al terminar) y crea los menús APAGADOS. Devuelve (JSON)
// {ids, nombres, avisos, omitidos}. No ejecuta nada del archivo.
func ImportarMenus(botID int64, ruta, clave string) (string, error) {
	defer func() { _ = os.Remove(ruta) }()
	a, err := almacenAbierto()
	if err != nil {
		return "", err
	}
	st, err := os.Stat(ruta)
	if err != nil || st.IsDir() {
		return "", errors.New("No se pudo leer el archivo.")
	}
	if st.Size() > 60<<20 {
		return "", errors.New("El archivo es demasiado grande.")
	}
	b, err := os.ReadFile(ruta)
	if err != nil {
		return "", errors.New("No se pudo leer el archivo.")
	}
	r, err := flujo.ImportarMenus(a, botID, b, clave)
	if err != nil {
		a.RegistrarAccion(botID, almacen.PersonaDueno, "menu.importar", "", "", "error", "")
		return "", err
	}
	a.RegistrarAccion(botID, almacen.PersonaDueno, "menu.importar", "", "", "ok", fmt.Sprintf("menus=%d", len(r.IDs)))
	return js(r, nil)
}

// ---------- envío de lo que sale de un menú ----------

// EnviarSalida implementa motor.EnviadorSalida: manda texto, archivo, ubicación o contacto citando el mensaje exacto de la persona
// (con el autor de la cita tal como lo mandó WhatsApp) y devuelve el id del mensaje que salió.
//
// Si un archivo no se puede leer o subir, NO se pierde el mensaje: sale solo el texto (si lo había), se anota en el registro (sin
// textos ni nombres) y lo demás sigue. Un archivo sin texto que falla se omite (motor.ErrSaltado).
func (e enviadorMotor) EnviarSalida(ctx context.Context, chat string, sal motor.Salida, citaID, citaAutorWA, mencion string) (string, error) {
	var men []string
	if mencion != "" {
		men = []string{mencion}
	}
	var r envio.Resultado
	var err error
	switch {
	case sal.Medio != nil:
		ruta, errC := e.s.copiaAdjunto(sal.Medio.ID)
		if errC != nil {
			return e.sinArchivo(ctx, chat, sal, citaID, citaAutorWA, men, "archivo_ilegible")
		}
		pie := sal.Texto
		r, err = e.s.envio.EnviarMedioAutoCita(ctx, chat, adjuntoEnvio(ruta, sal.Medio, pie), citaID, citaAutorWA, men)
		if err != nil && !errors.Is(err, envio.ErrSinConexion) {
			return e.sinArchivo(ctx, chat, sal, citaID, citaAutorWA, men, "subida_fallo")
		}
		if err == nil && r.Estado == "enviado" && sal.Auto != nil {
			sal.Auto.AdjOK++
		}
	case sal.Ubicacion != nil:
		u := sal.Ubicacion
		r, err = e.s.envio.EnviarUbicacionAuto(ctx, chat, u.Lat, u.Lon, u.Nombre, u.Direccion, citaID, citaAutorWA)
	case sal.Contacto != nil:
		r, err = e.s.envio.EnviarContactoAuto(ctx, chat, sal.Contacto.Nombre, sal.Contacto.Telefono, citaID, citaAutorWA)
	default:
		r, err = e.s.envio.EnviarAutoCita(ctx, chat, sal.Texto, citaID, citaAutorWA, men)
	}
	if err != nil {
		return "", err
	}
	if r.Estado != "enviado" {
		return "", errNoEnviado
	}
	return r.ID, nil
}

// sinArchivo anota que un archivo no salió y manda solo el texto (si lo hay). Nunca escribe nombres de archivo en el registro.
func (e enviadorMotor) sinArchivo(ctx context.Context, chat string, sal motor.Salida, citaID, citaAutorWA string, men []string, motivo string) (string, error) {
	sal.Auto.Fallo(motivo)
	if sal.Auto == nil && alm != nil {
		alm.RegistrarActividad(e.s.botID, almacen.ActOmitido, "archivo no enviado", chat, motivo)
	}
	e.s.log("No se pudo mandar un archivo de una respuesta automática (" + motivo + "); se manda solo el texto.")
	if strings.TrimSpace(sal.Texto) == "" {
		return "", motor.ErrSaltado
	}
	r, err := e.s.envio.EnviarAutoCita(ctx, chat, sal.Texto, citaID, citaAutorWA, men)
	if err != nil {
		return "", err
	}
	if r.Estado != "enviado" {
		return "", errNoEnviado
	}
	return r.ID, nil
}

func adjuntoEnvio(ruta string, m *motor.Medio, pie string) envio.Adjunto {
	a := envio.Adjunto{Tipo: m.Tipo, Ruta: ruta, Mime: m.Mime, Nombre: m.Nombre, Pie: pie}
	if m.Tipo == "audio" && strings.Contains(strings.ToLower(m.Mime), "ogg") {
		a.NotaVoz = true
	}
	return a
}
