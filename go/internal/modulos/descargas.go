package modulos

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	"botchat/internal/almacen"
	"botchat/internal/comandos"
	"botchat/internal/permisos"
)

// ---------- H5 · Descargas y conectores HTTP ----------
//
// (a) "!bajar <enlace directo>" manda el archivo como documento. Solo de dominios que yo permití, con tamaño máximo y tipos de
//     archivo elegidos. Es una RESPUESTA a un comando: no escribe por iniciativa.
// (b) Conectores HTTP: acciones con nombre (método, URL, cabeceras y llaves cifradas, cuerpo con variables, respuesta mapeada a
//     texto, imagen o archivo) que se usan con "!api <nombre> [texto]" y, en menús, reglas y comandos propios, con {api_<nombre>}.
//
// NO hay extractores ni descargadores de YouTube, Spotify ni de sitios de contenido protegido. Quien quiera conectar su propio
// servicio lo hace con un conector, bajo su responsabilidad.

// IDDescargas es el id del módulo.
const IDDescargas = "descargas"

// PeticionHTTP es lo que se pide al descargador.
type PeticionHTTP struct {
	Metodo    string
	URL       string
	Cabeceras map[string]string
	Cuerpo    []byte
	Timeout   time.Duration
	Max       int64                  // bytes máximos de la respuesta
	RedLocal  bool                   // dejar llegar a direcciones privadas y locales
	Permitido func(host string) bool // nil = cualquiera; se vuelve a preguntar en cada redirección
}

// RespuestaHTTP es lo que volvió. El cuerpo se lee hasta Max bytes; si lo pasa, la lectura falla con ErrMuyGrande.
type RespuestaHTTP struct {
	Estado   int
	Mime     string
	Nombre   string // nombre de archivo sugerido por el servidor o la URL
	Largo    int64  // Content-Length (-1 si no lo dice)
	Cuerpo   io.ReadCloser
	URLFinal string
}

// Descargador hace las peticiones HTTP de los módulos H5.
type Descargador interface {
	Hacer(ctx context.Context, p PeticionHTTP) (*RespuestaHTTP, error)
}

// Errores en español de las descargas.
var (
	ErrMuyGrande               = errors.New("el archivo pesa más del máximo permitido")
	ErrDominio                 = errors.New("ese dominio no está permitido")
	ErrRedPrivada              = errors.New("esa dirección es de una red privada o local y no está permitida")
	ErrEsquema                 = errors.New("solo se aceptan enlaces http o https")
	ErrDemasiadasRedirecciones = errors.New("el enlace redirige demasiadas veces")
)

// ClienteHTTP es el descargador real: no usa proxys del sistema, no acepta direcciones privadas (a menos que se permita), limita las
// redirecciones, el tiempo y el tamaño.
type ClienteHTTP struct{}

func esDireccionPrivada(ip net.IP) bool {
	if ip4 := ip.To4(); ip4 != nil {
		ip = ip4
		if ip4[0] == 100 && ip4[1]&0xC0 == 64 { // 100.64.0.0/10 (CGNAT)
			return true
		}
		if ip4[0] == 0 {
			return true
		}
	}
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() || ip.IsMulticast()
}

// Hacer hace la petición.
func (ClienteHTTP) Hacer(ctx context.Context, p PeticionHTTP) (*RespuestaHTTP, error) {
	u, err := url.Parse(strings.TrimSpace(p.URL))
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return nil, ErrEsquema
	}
	if p.Permitido != nil && !p.Permitido(u.Hostname()) {
		return nil, ErrDominio
	}
	if p.Timeout <= 0 {
		p.Timeout = 30 * time.Second
	}
	if p.Max <= 0 {
		p.Max = 16 << 20
	}
	metodo := strings.ToUpper(strings.TrimSpace(p.Metodo))
	if metodo == "" {
		metodo = http.MethodGet
	}
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	tr := &http.Transport{
		Proxy: nil,
		DialContext: func(ctx context.Context, red, dir string) (net.Conn, error) {
			host, puerto, err := net.SplitHostPort(dir)
			if err != nil {
				return nil, err
			}
			ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
			if err != nil {
				return nil, err
			}
			for _, ip := range ips {
				if !p.RedLocal && esDireccionPrivada(ip.IP) {
					return nil, ErrRedPrivada
				}
			}
			var ultimo error
			for _, ip := range ips {
				c, err := dialer.DialContext(ctx, red, net.JoinHostPort(ip.IP.String(), puerto))
				if err == nil {
					return c, nil
				}
				ultimo = err
			}
			if ultimo == nil {
				ultimo = errors.New("no se pudo resolver la dirección")
			}
			return nil, ultimo
		},
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: p.Timeout,
		DisableKeepAlives:     true,
	}
	cli := &http.Client{
		Transport: tr,
		Timeout:   p.Timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 3 {
				return ErrDemasiadasRedirecciones
			}
			if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
				return ErrEsquema
			}
			if p.Permitido != nil && !p.Permitido(req.URL.Hostname()) {
				return ErrDominio
			}
			return nil
		},
	}
	var cuerpo io.Reader
	if len(p.Cuerpo) > 0 {
		cuerpo = bytes.NewReader(p.Cuerpo)
	}
	req, err := http.NewRequestWithContext(ctx, metodo, u.String(), cuerpo)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "BotChat/0.5")
	for k, v := range p.Cabeceras {
		req.Header.Set(k, v)
	}
	resp, err := cli.Do(req)
	if err != nil {
		var ue *url.Error
		if errors.As(err, &ue) {
			err = ue.Err
		}
		return nil, err
	}
	out := &RespuestaHTTP{Estado: resp.StatusCode, Largo: resp.ContentLength, URLFinal: resp.Request.URL.String()}
	out.Mime, _, _ = mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if _, par, err := mime.ParseMediaType(resp.Header.Get("Content-Disposition")); err == nil {
		out.Nombre = par["filename"]
	}
	if out.Nombre == "" {
		out.Nombre = path.Base(resp.Request.URL.Path)
		if out.Nombre == "." || out.Nombre == "/" {
			out.Nombre = ""
		}
	}
	if resp.ContentLength > p.Max {
		resp.Body.Close()
		return nil, ErrMuyGrande
	}
	out.Cuerpo = &lectorLimitado{r: resp.Body, resta: p.Max}
	return out, nil
}

type lectorLimitado struct {
	r     io.ReadCloser
	resta int64
}

func (l *lectorLimitado) Read(b []byte) (int, error) {
	if l.resta < 0 {
		return 0, ErrMuyGrande
	}
	if int64(len(b)) > l.resta+1 {
		b = b[:l.resta+1]
	}
	n, err := l.r.Read(b)
	l.resta -= int64(n)
	if l.resta < 0 {
		return n, ErrMuyGrande
	}
	return n, err
}
func (l *lectorLimitado) Close() error { return l.r.Close() }

// ---------- configuración ----------

func (g *Gestor) descargador() Descargador {
	if g.Descarga != nil {
		return g.Descarga
	}
	return ClienteHTTP{}
}

// dominioPermitido dice si el host coincide con alguno de la lista (el dominio exacto o un subdominio suyo).
func dominioPermitido(lista []string, host string) bool {
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	for _, d := range lista {
		d = strings.ToLower(strings.TrimSpace(d))
		if d == "" {
			continue
		}
		if host == d || strings.HasSuffix(host, "."+d) {
			return true
		}
	}
	return false
}

func lineas(s string) []string {
	var out []string
	for _, l := range strings.FieldsFunc(s, func(r rune) bool { return r == '\n' || r == ',' || r == ';' || r == ' ' }) {
		if l = strings.TrimSpace(l); l != "" {
			out = append(out, strings.ToLower(strings.TrimPrefix(strings.TrimPrefix(l, "https://"), "http://")))
		}
	}
	return out
}

func extensiones(s string) map[string]bool {
	m := map[string]bool{}
	for _, e := range strings.FieldsFunc(strings.ToLower(s), func(r rune) bool { return r == ',' || r == ';' || r == ' ' || r == '\n' }) {
		m[strings.TrimPrefix(strings.TrimSpace(e), ".")] = true
	}
	return m
}

// extensionesPeligrosas nunca se mandan, ni aunque se pongan en la lista.
var extensionesPeligrosas = map[string]bool{"exe": true, "apk": true, "bat": true, "cmd": true, "com": true, "scr": true, "msi": true, "sh": true, "js": true, "jar": true, "vbs": true, "ps1": true, "dll": true}

const tiposDescargaDefecto = "pdf,jpg,jpeg,png,gif,webp,mp4,mp3,ogg,txt,docx,xlsx,pptx,zip,csv"

var reNombreArchivo = regexp.MustCompile(`[^\p{L}\p{N}._ -]+`)

func nombreSeguro(n string) string {
	n = reNombreArchivo.ReplaceAllString(path.Base(strings.ReplaceAll(n, "\\", "/")), "_")
	n = strings.Trim(n, " .")
	if r := []rune(n); len(r) > 80 {
		ext := path.Ext(n)
		n = string(r[:80-len([]rune(ext))]) + ext
	}
	return n
}

func (g *Gestor) usoDescarga(cfg Config) bool {
	// tope por hora de usos de !bajar y !api (en memoria; reinicia al reiniciar)
	g.h.mu.Lock()
	defer g.h.mu.Unlock()
	tope := cfg.Int("usos_hora")
	if tope <= 0 {
		return true
	}
	ahora := g.ahora()
	var vivos []time.Time
	for _, t := range g.h.usos {
		if ahora.Sub(t) < time.Hour {
			vivos = append(vivos, t)
		}
	}
	if len(vivos) >= tope {
		g.h.usos = vivos
		return false
	}
	g.h.usos = append(vivos, ahora)
	return true
}

// ---------- !bajar ----------

// Bajar descarga un enlace directo y lo guarda cifrado como documento listo para mandar. Devuelve el adjunto y un texto corto.
func (g *Gestor) Bajar(ctx context.Context, enlace string) (AdjRef, error) {
	cfg := g.Config(IDDescargas)
	u, err := url.Parse(strings.TrimSpace(enlace))
	if err != nil || u.Host == "" {
		return AdjRef{}, errors.New("escribe un enlace directo al archivo, por ejemplo https://sitio.com/archivo.pdf")
	}
	if u.Scheme != "https" && !(u.Scheme == "http" && cfg.Bool("permitir_http")) {
		return AdjRef{}, errors.New("solo se aceptan enlaces https")
	}
	dominios := lineas(cfg.Texto("dominios"))
	if len(dominios) == 0 {
		return AdjRef{}, errors.New("todavía no hay dominios permitidos: agrégalos en la configuración del módulo Descargas")
	}
	permitido := func(h string) bool { return dominioPermitido(dominios, h) }
	if !permitido(u.Hostname()) {
		return AdjRef{}, errors.New("ese dominio no está en la lista de permitidos")
	}
	if !g.usoDescarga(cfg) {
		return AdjRef{}, errors.New("ya hice muchas descargas en esta hora; intenta más tarde")
	}
	max := int64(cfg.Int("max_mb")) << 20
	if max <= 0 {
		max = 16 << 20
	}
	resp, err := g.descargador().Hacer(ctx, PeticionHTTP{Metodo: "GET", URL: u.String(), Timeout: time.Duration(cfg.Int("tiempo_s")) * time.Second,
		Max: max, RedLocal: cfg.Bool("red_local"), Permitido: permitido})
	if err != nil {
		g.Alm.RegistrarModulo(g.BotID, IDDescargas, "bajar", "fallido", "", g.ahora())
		return AdjRef{}, explicarHTTP(err, max)
	}
	defer resp.Cuerpo.Close()
	if resp.Estado < 200 || resp.Estado > 299 {
		g.Alm.RegistrarModulo(g.BotID, IDDescargas, "bajar", "fallido", "", g.ahora())
		return AdjRef{}, fmt.Errorf("el sitio respondió con el código %d", resp.Estado)
	}
	if strings.HasPrefix(resp.Mime, "text/html") || resp.Mime == "application/xhtml+xml" {
		return AdjRef{}, errors.New("ese enlace lleva a una página web, no a un archivo; usa un enlace directo al archivo")
	}
	nombre := nombreSeguro(resp.Nombre)
	if nombre == "" {
		nombre = "archivo"
	}
	ext := strings.TrimPrefix(strings.ToLower(path.Ext(nombre)), ".")
	if ext == "" {
		if exts, _ := mime.ExtensionsByType(resp.Mime); len(exts) > 0 {
			ext = strings.TrimPrefix(exts[0], ".")
			nombre += "." + ext
		}
	}
	permitidas := extensiones(cfg.Texto("tipos"))
	if ext == "" || extensionesPeligrosas[ext] || !permitidas[ext] {
		g.Alm.RegistrarModulo(g.BotID, IDDescargas, "bajar", "descartado:tipo", "", g.ahora())
		return AdjRef{}, errors.New("ese tipo de archivo no está permitido")
	}
	id, n, err := g.Alm.GuardarAdjuntoMenu(g.BotID, resp.Cuerpo)
	if err != nil {
		g.Alm.BorrarAdjuntoMenu(g.BotID, id)
		g.Alm.RegistrarModulo(g.BotID, IDDescargas, "bajar", "fallido", "", g.ahora())
		return AdjRef{}, explicarHTTP(err, max)
	}
	if n == 0 {
		g.Alm.BorrarAdjuntoMenu(g.BotID, id)
		return AdjRef{}, errors.New("el archivo llegó vacío")
	}
	g.Alm.RegistrarModulo(g.BotID, IDDescargas, "bajar", "enviado", "", g.ahora())
	mm := resp.Mime
	if mm == "" {
		mm = mime.TypeByExtension("." + ext)
	}
	if mm == "" {
		mm = "application/octet-stream"
	}
	return AdjRef{ID: id, Tipo: almacen.TipoDocumento, Mime: mm, Nombre: nombre, Bytes: n}, nil
}

func explicarHTTP(err error, max int64) error {
	switch {
	case errors.Is(err, ErrMuyGrande):
		return fmt.Errorf("el archivo pesa más del máximo permitido (%d MB)", max>>20)
	case errors.Is(err, ErrDominio):
		return errors.New("el enlace redirige a un dominio que no está permitido")
	case errors.Is(err, ErrRedPrivada):
		return errors.New("esa dirección es de una red privada o local y no está permitida")
	case errors.Is(err, ErrEsquema):
		return errors.New("solo se aceptan enlaces http o https")
	case errors.Is(err, ErrDemasiadasRedirecciones):
		return ErrDemasiadasRedirecciones
	case errors.Is(err, context.DeadlineExceeded):
		return errors.New("el sitio tardó demasiado en responder")
	}
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		return errors.New("el sitio tardó demasiado en responder")
	}
	return errors.New("no se pudo descargar (revisa el enlace y la conexión)")
}

// ---------- conectores ----------

// Conector es la definición de un conector HTTP (se guarda cifrada completa: puede traer llaves en las cabeceras).
type Conector struct {
	Nombre    string `json:"nombre"`
	Metodo    string `json:"metodo"`
	URL       string `json:"url"`
	Cabeceras string `json:"cabeceras"` // una por línea: "Nombre: valor"
	Cuerpo    string `json:"cuerpo"`
	Salida    string `json:"salida"`    // texto | imagen | archivo
	Ruta      string `json:"ruta"`      // dónde está el dato en la respuesta JSON (a.b.0.c); vacío = toda la respuesta
	Plantilla string `json:"plantilla"` // texto con {ruta.json} (solo salida texto)
}

var reNombreConector = regexp.MustCompile(`^[a-z][a-z0-9_]{0,23}$`)

// ValidarConector revisa un conector antes de guardarlo.
func ValidarConector(c *Conector) error {
	c.Nombre = strings.ToLower(strings.TrimSpace(c.Nombre))
	if !reNombreConector.MatchString(c.Nombre) {
		return errors.New("el nombre del conector va en minúsculas, sin espacios ni acentos (por ejemplo clima o precio_dolar)")
	}
	c.Metodo = strings.ToUpper(strings.TrimSpace(c.Metodo))
	switch c.Metodo {
	case "":
		c.Metodo = "GET"
	case "GET", "POST", "PUT", "DELETE":
	default:
		return errors.New("el método debe ser GET, POST, PUT o DELETE")
	}
	c.URL = strings.TrimSpace(c.URL)
	if u, err := url.Parse(strings.ReplaceAll(strings.ReplaceAll(c.URL, "{arg}", "x"), "{nombre}", "x")); err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return errors.New("la URL debe empezar con http:// o https://")
	}
	if len(c.URL) > 600 || len(c.Cabeceras) > 2000 || len(c.Cuerpo) > 4000 || len(c.Plantilla) > 1000 || len(c.Ruta) > 120 {
		return errors.New("algún campo del conector es demasiado largo")
	}
	switch c.Salida {
	case "":
		c.Salida = "texto"
	case "texto", "imagen", "archivo":
	default:
		return errors.New("la salida debe ser texto, imagen o archivo")
	}
	if _, err := cabecerasDe(c.Cabeceras); err != nil {
		return err
	}
	return nil
}

func cabecerasDe(s string) (map[string]string, error) {
	out := map[string]string{}
	for _, l := range strings.Split(s, "\n") {
		l = strings.TrimSpace(l)
		if l == "" {
			continue
		}
		k, v, ok := strings.Cut(l, ":")
		k, v = strings.TrimSpace(k), strings.TrimSpace(v)
		if !ok || k == "" || strings.ContainsAny(k, " \t") {
			return nil, errors.New("cada cabecera va en su línea como «Nombre: valor»")
		}
		switch strings.ToLower(k) {
		case "host", "content-length", "transfer-encoding", "connection":
			return nil, fmt.Errorf("la cabecera %s no se puede cambiar", k)
		}
		out[k] = v
	}
	return out, nil
}

// ResultadoConector es lo que sale de ejecutar un conector.
type ResultadoConector struct {
	Texto   string
	Archivo *AdjRef // imagen o archivo ya guardado (cifrado)
}

func (g *Gestor) conector(nombre string) (*Conector, error) {
	f, err := g.Alm.ObtenerConector(g.BotID, nombre)
	if err != nil || f == nil {
		return nil, errors.New("ese conector no existe")
	}
	var c Conector
	if json.Unmarshal([]byte(f.Config), &c) != nil {
		return nil, errors.New("ese conector está dañado: vuelve a guardarlo")
	}
	c.Nombre = f.Nombre
	return &c, nil
}

// jsonEn busca un valor en un JSON con una ruta como "a.b.0.c".
func jsonEn(v any, ruta string) (any, bool) {
	ruta = strings.TrimSpace(ruta)
	if ruta == "" {
		return v, true
	}
	for _, p := range strings.Split(ruta, ".") {
		switch x := v.(type) {
		case map[string]any:
			n, ok := x[p]
			if !ok {
				return nil, false
			}
			v = n
		case []any:
			i, err := strconv.Atoi(p)
			if err != nil || i < 0 || i >= len(x) {
				return nil, false
			}
			v = x[i]
		default:
			return nil, false
		}
	}
	return v, true
}

func textoDe(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case float64:
		if x == float64(int64(x)) {
			return strconv.FormatInt(int64(x), 10)
		}
		return strconv.FormatFloat(x, 'f', -1, 64)
	case bool:
		if x {
			return "sí"
		}
		return "no"
	default:
		b, _ := json.Marshal(x)
		return string(b)
	}
}

var rePlantilla = regexp.MustCompile(`\{([a-zA-Z0-9_.\-]+)\}`)

// aplicaVars cambia {arg}, {nombre} y {chat} en un texto. En la URL los valores van escapados.
func aplicaVars(s string, vars map[string]string, escapar bool) string {
	for k, v := range vars {
		if escapar {
			v = url.QueryEscape(v)
		}
		s = strings.ReplaceAll(s, "{"+k+"}", v)
	}
	return s
}

// EjecutarConector corre un conector con las variables dadas (arg, nombre). Respeta los límites del módulo (tiempo, tamaño, red).
func (g *Gestor) EjecutarConector(ctx context.Context, c *Conector, vars map[string]string) (ResultadoConector, error) {
	cfg := g.Config(IDDescargas)
	cab, err := cabecerasDe(c.Cabeceras)
	if err != nil {
		return ResultadoConector{}, err
	}
	for k, v := range cab {
		cab[k] = aplicaVars(v, vars, false)
	}
	var cuerpo []byte
	if c.Metodo == "POST" || c.Metodo == "PUT" {
		cuerpo = []byte(aplicaVars(c.Cuerpo, vars, false))
		if len(cuerpo) > 0 {
			ok := false
			for k := range cab {
				if strings.EqualFold(k, "Content-Type") {
					ok = true
				}
			}
			if !ok {
				if json.Valid(cuerpo) {
					cab["Content-Type"] = "application/json"
				} else {
					cab["Content-Type"] = "text/plain; charset=utf-8"
				}
			}
		}
	}
	u, _ := url.Parse(aplicaVars(c.URL, vars, true))
	host := ""
	if u != nil {
		host = strings.ToLower(u.Hostname())
	}
	extra := lineas(cfg.Texto("dominios"))
	permitido := func(h string) bool { return strings.EqualFold(h, host) || dominioPermitido(extra, h) }
	max := int64(cfg.Int("max_mb")) << 20
	if max <= 0 {
		max = 16 << 20
	}
	tiempo := time.Duration(cfg.Int("tiempo_s")) * time.Second
	pedir := func(metodo, direccion string, cab map[string]string, cuerpo []byte, tope int64) (*RespuestaHTTP, error) {
		return g.descargador().Hacer(ctx, PeticionHTTP{Metodo: metodo, URL: direccion, Cabeceras: cab, Cuerpo: cuerpo, Timeout: tiempo, Max: tope,
			RedLocal: cfg.Bool("red_local"), Permitido: permitido})
	}
	topeTexto := int64(256 << 10)
	if c.Salida != "texto" {
		topeTexto = max
	}
	resp, err := pedir(c.Metodo, u.String(), cab, cuerpo, topeTexto)
	if err != nil {
		return ResultadoConector{}, explicarHTTP(err, topeTexto)
	}
	defer resp.Cuerpo.Close()
	if resp.Estado < 200 || resp.Estado > 299 {
		return ResultadoConector{}, fmt.Errorf("el servicio respondió con el código %d", resp.Estado)
	}
	guarda := func(r *RespuestaHTTP, tipo string) (ResultadoConector, error) {
		id, n, err := g.Alm.GuardarAdjuntoMenu(g.BotID, r.Cuerpo)
		if err != nil {
			g.Alm.BorrarAdjuntoMenu(g.BotID, id)
			return ResultadoConector{}, explicarHTTP(err, max)
		}
		if n == 0 {
			g.Alm.BorrarAdjuntoMenu(g.BotID, id)
			return ResultadoConector{}, errors.New("el servicio devolvió un archivo vacío")
		}
		nombre := nombreSeguro(r.Nombre)
		mm := r.Mime
		if tipo == almacen.TipoImagen {
			if !strings.HasPrefix(mm, "image/") {
				mm = "image/jpeg"
			}
			if nombre == "" {
				nombre = "imagen.jpg"
			}
		} else {
			if mm == "" {
				mm = "application/octet-stream"
			}
			if nombre == "" {
				nombre = "archivo"
			}
			if ext := strings.TrimPrefix(strings.ToLower(path.Ext(nombre)), "."); extensionesPeligrosas[ext] {
				g.Alm.BorrarAdjuntoMenu(g.BotID, id)
				return ResultadoConector{}, errors.New("ese tipo de archivo no está permitido")
			}
		}
		return ResultadoConector{Archivo: &AdjRef{ID: id, Tipo: tipo, Mime: mm, Nombre: nombre, Bytes: n}}, nil
	}
	esJSON := strings.Contains(resp.Mime, "json")
	switch c.Salida {
	case "imagen", "archivo":
		tipo := almacen.TipoDocumento
		if c.Salida == "imagen" {
			tipo = almacen.TipoImagen
		}
		if esJSON && strings.TrimSpace(c.Ruta) != "" {
			// la respuesta trae la dirección del archivo; se baja con las mismas protecciones
			b, err := io.ReadAll(resp.Cuerpo)
			if err != nil {
				return ResultadoConector{}, explicarHTTP(err, topeTexto)
			}
			var v any
			if json.Unmarshal(b, &v) != nil {
				return ResultadoConector{}, errors.New("la respuesta no es un JSON válido")
			}
			d, ok := jsonEn(v, c.Ruta)
			dir := textoDe(d)
			if !ok || dir == "" {
				return ResultadoConector{}, errors.New("no encontré la dirección del archivo en la respuesta")
			}
			r2, err := pedir("GET", dir, nil, nil, max)
			if err != nil {
				return ResultadoConector{}, explicarHTTP(err, max)
			}
			defer r2.Cuerpo.Close()
			if r2.Estado < 200 || r2.Estado > 299 {
				return ResultadoConector{}, fmt.Errorf("el archivo respondió con el código %d", r2.Estado)
			}
			return guarda(r2, tipo)
		}
		return guarda(resp, tipo)
	}
	b, err := io.ReadAll(resp.Cuerpo)
	if err != nil {
		return ResultadoConector{}, explicarHTTP(err, topeTexto)
	}
	var v any
	if esJSON || json.Valid(b) {
		if json.Unmarshal(b, &v) == nil {
			if t := strings.TrimSpace(c.Plantilla); t != "" {
				out := rePlantilla.ReplaceAllStringFunc(t, func(x string) string {
					d, ok := jsonEn(v, x[1:len(x)-1])
					if !ok {
						return ""
					}
					return textoDe(d)
				})
				return ResultadoConector{Texto: limitaTexto(out)}, nil
			}
			d, ok := jsonEn(v, c.Ruta)
			if !ok {
				return ResultadoConector{}, errors.New("no encontré ese dato en la respuesta")
			}
			return ResultadoConector{Texto: limitaTexto(textoDe(d))}, nil
		}
	}
	return ResultadoConector{Texto: limitaTexto(string(b))}, nil
}

func limitaTexto(s string) string {
	s = strings.TrimSpace(s)
	if r := []rune(s); len(r) > 1500 {
		return string(r[:1500]) + "…"
	}
	return s
}

// TextoConector corre un conector por nombre y devuelve solo el texto (para {api_nombre} en menús, reglas y comandos propios). Si algo
// falla devuelve "" sin romper la conversación; el motivo queda en el registro.
func (g *Gestor) TextoConector(nombre, persona, chat string) string {
	if !g.Activo(IDDescargas) {
		return ""
	}
	c, err := g.conector(nombre)
	if err != nil || c.Salida != "texto" {
		return ""
	}
	cx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if !g.usoDescarga(g.Config(IDDescargas)) {
		return ""
	}
	r, err := g.EjecutarConector(cx, c, map[string]string{"arg": "", "nombre": g.nombreDe(persona), "chat": ""})
	if err != nil {
		g.Alm.RegistrarModulo(g.BotID, IDDescargas, "conector", "fallido", "", g.ahora())
		return ""
	}
	g.Alm.RegistrarModulo(g.BotID, IDDescargas, "conector", "enviado", "", g.ahora())
	return r.Texto
}

// ---------- el módulo ----------

func descargas() Modulo {
	return Nuevo(Def{
		ID: IDDescargas, Nombre: "Descargas y conectores", Icono: "descarga", Riesgo: RiesgoMedio, Iniciativa: false,
		Descripcion: "«!bajar» manda un archivo desde un enlace directo (solo de dominios que tú permitas) y los conectores HTTP traen datos, imágenes o archivos de tu propio servicio. No incluye descargadores de YouTube, Spotify ni de sitios con contenido protegido.",
		Casillas: []permisos.Casilla{
			{ID: "descargas.bajar", Nombre: "Pedir «!bajar»", Descripcion: "Puede pedirle al bot que mande un archivo desde un enlace directo de un dominio permitido.", Grupo: "Descargas y conectores", Riesgo: permisos.RiesgoMedio},
			{ID: "descargas.conectar", Nombre: "Usar conectores («!api»)", Descripcion: "Puede ejecutar los conectores HTTP que tú configuraste.", Grupo: "Descargas y conectores", Riesgo: permisos.RiesgoMedio},
		},
		Esquema: []Campo{
			{Clave: "dominios", Titulo: "Dominios permitidos", Ayuda: "Uno por línea (ejemplo: miservidor.com). Incluye sus subdominios. Vacío = no se baja nada.", Tipo: CampoTexto, Defecto: "", Largo: 1000, Varias: true},
			{Clave: "max_mb", Titulo: "Tamaño máximo (MB)", Ayuda: "Más grande que esto se rechaza.", Tipo: CampoNumero, Defecto: 16.0, Min: 1, Max: 100},
			{Clave: "tipos", Titulo: "Tipos de archivo permitidos", Ayuda: "Extensiones separadas por coma. Los programas (exe, apk, sh, js…) nunca se mandan.", Tipo: CampoTexto, Defecto: tiposDescargaDefecto, Largo: 300},
			{Clave: "tiempo_s", Titulo: "Tiempo máximo (segundos)", Tipo: CampoNumero, Defecto: 30.0, Min: 5, Max: 120},
			{Clave: "usos_hora", Titulo: "Usos por hora", Ayuda: "Cuántas veces entre «!bajar» y «!api» por hora (0 = sin límite).", Tipo: CampoNumero, Defecto: 20.0, Min: 0, Max: 500},
			{Clave: "permitir_http", Titulo: "Aceptar enlaces http (sin candado)", Tipo: CampoInterruptor, Defecto: false},
			{Clave: "red_local", Titulo: "Permitir redes privadas y locales", Ayuda: "Solo si tu servicio está en tu red de casa. Apagado, el bot no llega a direcciones privadas.", Tipo: CampoInterruptor, Defecto: false},
		},
		ComandosG: comandosDescargas,
		Paneles:   panelesDescargas,
		AlFilas:   filasDescargas,
		AlAccion:  accionDescargas,
	})
}

func panelesDescargas(g *Gestor) []Panel {
	return []Panel{{
		ID: "conectores", Titulo: "Conectores HTTP",
		Ayuda: "Cada conector es una acción con nombre. Úsalo con «!api nombre texto» o escribe {api_nombre} en un menú, una regla o un comando propio (solo conectores de salida texto). Las cabeceras y llaves se guardan cifradas y no se vuelven a mostrar: para cambiarlas, guarda el conector otra vez con el mismo nombre. Variables: {arg} (lo que escribe la persona) y {nombre}.",
		Campos: []Campo{
			{Clave: "nombre", Titulo: "Nombre", Ayuda: "Minúsculas, sin espacios (clima, precio_dolar).", Tipo: CampoTexto, Largo: 24, Obligatorio: true},
			{Clave: "metodo", Titulo: "Método", Tipo: CampoLista, Defecto: "GET", Opciones: []Opcion{{"GET", "GET"}, {"POST", "POST"}, {"PUT", "PUT"}, {"DELETE", "DELETE"}}},
			{Clave: "url", Titulo: "URL", Ayuda: "Puede llevar {arg}. Ejemplo: https://api.miservidor.com/clima?ciudad={arg}", Tipo: CampoTexto, Largo: 600, Obligatorio: true},
			{Clave: "cabeceras", Titulo: "Cabeceras y llaves", Ayuda: "Una por línea: «Authorization: Bearer …». Se guardan cifradas.", Tipo: CampoTexto, Largo: 2000, Varias: true},
			{Clave: "cuerpo", Titulo: "Cuerpo (POST/PUT)", Ayuda: "Puede llevar {arg} y {nombre}.", Tipo: CampoTexto, Largo: 4000, Varias: true},
			{Clave: "salida", Titulo: "La respuesta es", Tipo: CampoLista, Defecto: "texto", Opciones: []Opcion{{"texto", "Texto"}, {"imagen", "Imagen"}, {"archivo", "Archivo"}}},
			{Clave: "ruta", Titulo: "Dato de la respuesta JSON", Ayuda: "Ruta como datos.0.texto. Vacío = toda la respuesta. Con imagen o archivo, apunta a la dirección del archivo.", Tipo: CampoTexto, Largo: 120},
			{Clave: "plantilla", Titulo: "Plantilla de texto", Ayuda: "Ejemplo: «Hoy hay {main.temp} grados». Si la llenas, manda sobre la ruta.", Tipo: CampoTexto, Largo: 1000, Varias: true},
			{Clave: "arg", Titulo: "Texto de prueba", Ayuda: "Solo para el botón Probar: reemplaza a {arg}.", Tipo: CampoTexto, Largo: 200},
		},
		Alta:    "Guardar conector",
		Extras:  []Boton{{Accion: "conectores.probar", Titulo: "Probar"}},
		PorFila: []Boton{{Accion: "conectores.probar_fila", Titulo: "Probar"}, {Accion: "conectores.borrar", Titulo: "Borrar", Confirma: "¿Borrar este conector?"}},
	}}
}

func filasDescargas(g *Gestor, panel string) []Fila {
	var out []Fila
	fs, _ := g.Alm.ListarConectores(g.BotID)
	for _, f := range fs {
		var c Conector
		_ = json.Unmarshal([]byte(f.Config), &c)
		host := ""
		if u, err := url.Parse(strings.ReplaceAll(c.URL, "{arg}", "x")); err == nil {
			host = u.Host
		}
		out = append(out, Fila{ID: f.Nombre, Titulo: f.Nombre, Detalle: c.Metodo + " " + host + " → " + c.Salida})
	}
	return out
}

func conectorDeArgs(a Args) (*Conector, error) {
	c := &Conector{Nombre: a.Texto("nombre"), Metodo: a.Texto("metodo"), URL: a.Texto("url"), Cabeceras: strings.TrimSpace(fmt.Sprint(valorTexto(a, "cabeceras"))),
		Cuerpo: valorTexto(a, "cuerpo"), Salida: a.Texto("salida"), Ruta: a.Texto("ruta"), Plantilla: valorTexto(a, "plantilla")}
	return c, ValidarConector(c)
}

func accionDescargas(g *Gestor, accion string, a Args) (string, error) {
	prueba := func(c *Conector, arg string) (string, error) {
		cx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()
		r, err := g.EjecutarConector(cx, c, map[string]string{"arg": arg, "nombre": "Prueba", "chat": ""})
		if err != nil {
			return "", err
		}
		if r.Archivo != nil {
			g.Alm.BorrarAdjuntoMenu(g.BotID, r.Archivo.ID) // era una prueba: no se queda
			return fmt.Sprintf("Funcionó: llegó un archivo «%s» de %d KB (la prueba no lo guarda).", r.Archivo.Nombre, (r.Archivo.Bytes+1023)/1024), nil
		}
		if r.Texto == "" {
			return "Funcionó, pero la respuesta salió vacía.", nil
		}
		return "Funcionó. Respuesta:\n" + r.Texto, nil
	}
	switch accion {
	case "conectores.agregar":
		c, err := conectorDeArgs(a)
		if err != nil {
			return "", err
		}
		b, _ := json.Marshal(c)
		if _, err := g.Alm.GuardarConector(g.BotID, c.Nombre, string(b)); err != nil {
			return "", err
		}
		g.Alm.RegistrarAccion(g.BotID, almacen.PersonaDueno, "modulo.conector_guardar", "", "", "ok", "")
		return "Conector «" + c.Nombre + "» guardado.", nil
	case "conectores.probar":
		c, err := conectorDeArgs(a)
		if err != nil {
			return "", err
		}
		return prueba(c, a.Texto("arg"))
	case "conectores.probar_fila":
		c, err := g.conector(a.Texto("id"))
		if err != nil {
			return "", err
		}
		return prueba(c, "")
	case "conectores.borrar":
		if err := g.Alm.BorrarConector(g.BotID, a.Texto("id")); err != nil {
			return "", err
		}
		g.Alm.RegistrarAccion(g.BotID, almacen.PersonaDueno, "modulo.conector_borrar", "", "", "ok", "")
		return "Conector borrado.", nil
	}
	return "", errors.New("esa acción no existe")
}

// comandosDescargas son los comandos de H5. Se registran como parte del módulo.
func comandosDescargas(g *Gestor) []comandos.Comando {
	return []comandos.Comando{
		{
			Nombre: "bajar", Resumen: "Mandar un archivo desde un enlace directo", Uso: "bajar <enlace directo>", Casilla: "descargas.bajar",
			Ejecutar: func(cx *comandos.Contexto) (string, error) {
				if strings.TrimSpace(cx.Resto) == "" {
					return "", errors.New("escribe el enlace directo al archivo después del comando")
				}
				c, cancel := context.WithTimeout(context.Background(), time.Duration(g.Config(IDDescargas).Int("tiempo_s")+15)*time.Second)
				defer cancel()
				ref, err := g.Bajar(c, strings.Fields(cx.Resto)[0])
				if err != nil {
					return "", err
				}
				cx.Archivo = &comandos.Archivo{ID: ref.ID, Tipo: ref.Tipo, Mime: ref.Mime, Nombre: ref.Nombre}
				return "", nil
			},
		},
		{
			Nombre: "api", Alias: []string{"conector"}, Resumen: "Usar un conector HTTP", Uso: "api <nombre> [texto]", Casilla: "descargas.conectar",
			Ejecutar: func(cx *comandos.Contexto) (string, error) {
				if len(cx.Args) == 0 {
					return "", errors.New("dime el nombre del conector: api <nombre> [texto]")
				}
				c, err := g.conector(strings.ToLower(cx.Args[0]))
				if err != nil {
					return "", err
				}
				cfg := g.Config(IDDescargas)
				if !g.usoDescarga(cfg) {
					return "", errors.New("ya usé muchos conectores en esta hora; intenta más tarde")
				}
				arg := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(cx.Resto), cx.Args[0]))
				ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.Int("tiempo_s")+15)*time.Second)
				defer cancel()
				r, err := g.EjecutarConector(ctx, c, map[string]string{"arg": arg, "nombre": cx.Nombre, "chat": ""})
				if err != nil {
					g.Alm.RegistrarModulo(g.BotID, IDDescargas, "conector", "fallido", "", g.ahora())
					return "", err
				}
				g.Alm.RegistrarModulo(g.BotID, IDDescargas, "conector", "enviado", "", g.ahora())
				if r.Archivo != nil {
					cx.Archivo = &comandos.Archivo{ID: r.Archivo.ID, Tipo: r.Archivo.Tipo, Mime: r.Archivo.Mime, Nombre: r.Archivo.Nombre}
					return "", nil
				}
				if r.Texto == "" {
					return "(sin respuesta)", nil
				}
				return r.Texto, nil
			},
		},
	}
}
