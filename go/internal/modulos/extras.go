package modulos

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"math"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
	_ "time/tzdata" // las zonas horarias viajan dentro de la app (Android no siempre las trae completas)
	"unicode"

	"botchat/internal/almacen"
	"botchat/internal/comandos"
	"botchat/internal/estilo"
	"botchat/internal/permisos"
	"botchat/internal/respuestas"
)

// ---------- Extras de todos los días ----------
//
// Comandos útiles y simples para cualquier persona, en privado y en grupos: calculadora, hora de otras ciudades, chistes, datos y frases
// (sin internet), y clima, dólar, definiciones, acortar enlaces y códigos QR (con servicios gratuitos que no piden cuenta). Solo
// RESPONDEN a un comando: no escriben por su cuenta. Cada uno tiene su tope por persona y por hora.

// IDExtras es el id del módulo.
const IDExtras = "extras"

// hostsExtras son los únicos servicios a los que llegan estos comandos.
var hostsExtras = []string{"geocoding-api.open-meteo.com", "api.open-meteo.com", "open.er-api.com", "es.wikipedia.org", "is.gd", "api.qrserver.com", "v2.jokeapi.dev", "es.wikiquote.org"}

func extras() Modulo {
	return Nuevo(Def{
		ID: IDExtras, Nombre: "Extras de todos los días", Icono: "info", Riesgo: RiesgoBajo,
		Descripcion: "Comandos útiles para todos: !calc, !hora, !chiste, !dato, !frase, !clima, !dolar, !definir, !buscar, !acortar y !qr. Chistes, datos, clima, dólar, búsquedas, acortar y QR salen de servicios gratuitos de internet que no piden cuenta; si fallan o los apagas, el bot usa tus listas (o las de fábrica). Los títulos de cada tarjeta se cambian en «Respuestas del bot».",
		Casillas: []permisos.Casilla{
			{ID: "extras.usar", Nombre: "Usar los extras", Descripcion: "Pedir !calc, !hora, !chiste, !dato, !frase, !clima, !dolar, !definir, !buscar, !acortar y !qr.", Grupo: "Extras", Riesgo: permisos.RiesgoBajo, Privado: true, Grupos: true, Base: true},
		},
		Esquema: []Campo{
			{Clave: "ciudad", Titulo: "Ciudad por defecto del clima", Ayuda: "La que usa !clima cuando no dicen ninguna.", Tipo: CampoTexto, Largo: 60, Defecto: "Ciudad de México"},
			{Clave: "usos_hora", Titulo: "Usos por persona y hora", Ayuda: "Tope de comandos de este módulo por persona cada hora. 0 = sin tope.", Tipo: CampoNumero, Defecto: 40.0, Min: 0, Max: 1000},
			{Clave: "internet", Titulo: "Permitir los que usan internet", Ayuda: "Clima, dólar, definiciones, búsquedas, acortar, QR y chistes o datos nuevos cada vez. Apagado, solo funcionan los que no necesitan conexión.", Tipo: CampoInterruptor, Defecto: true},
			{Clave: "mis_chistes", Titulo: "Mis chistes", Ayuda: "Uno por línea. Si escribes alguno, !chiste usa solo los tuyos (sin internet). Vacío = chistes nuevos de internet, o los de fábrica si falla.", Tipo: CampoTexto, Varias: true, Largo: 6000},
			{Clave: "mis_datos", Titulo: "Mis datos curiosos", Ayuda: "Uno por línea. Si escribes alguno, !dato usa solo los tuyos. Vacío = un hecho de «un día como hoy» de Wikipedia, o los de fábrica si falla.", Tipo: CampoTexto, Varias: true, Largo: 6000},
			{Clave: "mis_frases", Titulo: "Mis frases del día", Ayuda: "Una por línea. Si escribes alguna, !frase usa solo las tuyas. Vacío = la «cita del día» de Wikiquote, o las de fábrica si falla.", Tipo: CampoTexto, Varias: true, Largo: 6000},
		},
		ComandosG: comandosExtras,
	})
}

// ---------- ayudas de red ----------

func (g *Gestor) extrasPeticion(ctx context.Context, direccion string, max int64) (io.ReadCloser, error) {
	if !g.Config(IDExtras).Bool("internet") {
		return nil, errors.New("los comandos que usan internet están apagados en la configuración de Extras")
	}
	r, err := g.descargador().Hacer(ctx, PeticionHTTP{Metodo: "GET", URL: direccion, Timeout: 8 * time.Second, Max: max,
		Permitido: func(h string) bool { return dominioPermitido(hostsExtras, h) }, Cabeceras: map[string]string{"User-Agent": "BotChat/0.6 (+comandos de todos los dias)"}})
	if err != nil {
		return nil, errors.New("no pude consultar el servicio ahora; intenta de nuevo en un momento")
	}
	if r.Estado == 404 {
		_ = r.Cuerpo.Close()
		return nil, errNoHallado
	}
	if r.Estado < 200 || r.Estado > 299 {
		_ = r.Cuerpo.Close()
		return nil, errors.New("el servicio no respondió bien; intenta de nuevo en un momento")
	}
	return r.Cuerpo, nil
}

var errNoHallado = errors.New("no encontré eso")

func (g *Gestor) extrasJSON(ctx context.Context, direccion string, salida any) error {
	c, err := g.extrasPeticion(ctx, direccion, 256<<10)
	if err != nil {
		return err
	}
	defer c.Close()
	b, err := io.ReadAll(c)
	if err != nil || json.Unmarshal(b, salida) != nil {
		return errors.New("el servicio contestó algo que no entendí; intenta más tarde")
	}
	return nil
}

func (g *Gestor) extrasTope(cx *comandos.Contexto) error {
	if !g.limite("extras:"+cx.Persona, g.Config(IDExtras).Int("usos_hora"), time.Hour) {
		return errors.New("ya usaste muchos comandos en esta hora; intenta un poco más tarde")
	}
	return nil
}

func ctxExtras() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 20*time.Second)
}

// ---------- los comandos ----------

func comandosExtras(g *Gestor) []comandos.Comando {
	pie := func(cx *comandos.Contexto) string { return g.tx("extras.pie", "prefijo", cx.D.Prefijo()) }
	cmd := func(nombre, resumen, uso string, alias []string, f func(cx *comandos.Contexto) (string, error)) comandos.Comando {
		return comandos.Comando{Nombre: nombre, Alias: alias, Resumen: resumen, Uso: uso, Casilla: "extras.usar", Ejecutar: func(cx *comandos.Contexto) (string, error) {
			if err := g.extrasTope(cx); err != nil {
				return "", err
			}
			return f(cx)
		}}
	}
	return []comandos.Comando{
		cmd("calc", "Hacer una cuenta", "calc 12*(3+4)", []string{"calcular"}, func(cx *comandos.Contexto) (string, error) {
			r, err := Calcula(cx.Resto)
			if err != nil {
				return "", err
			}
			return estilo.Tarjeta("🧮", g.tx("extras.t_calc"), []string{"`" + strings.TrimSpace(cx.Resto) + "`", "= *" + r + "*"}, pie(cx)), nil
		}),
		cmd("hora", "Ver la hora en otra ciudad", "hora Tokio", nil, func(cx *comandos.Contexto) (string, error) {
			return g.extrasHora(cx, pie(cx))
		}),
		cmd("chiste", "Un chiste", "chiste", nil, func(cx *comandos.Contexto) (string, error) {
			return estilo.Tarjeta("😄", g.tx("extras.t_chiste"), []string{g.chiste()}, pie(cx)), nil
		}),
		cmd("dato", "Un dato curioso", "dato", []string{"curiosidad"}, func(cx *comandos.Contexto) (string, error) {
			t, hecho := g.dato()
			if hecho {
				return estilo.Tarjeta("📅", g.tx("extras.t_efemeride"), []string{t}, pie(cx)), nil
			}
			return estilo.Tarjeta("🤓", g.tx("extras.t_dato"), []string{t}, pie(cx)), nil
		}),
		cmd("frase", "Una frase para el día", "frase", nil, func(cx *comandos.Contexto) (string, error) {
			return estilo.Tarjeta("✨", g.tx("extras.t_frase"), []string{g.frase()}, pie(cx)), nil
		}),
		cmd("clima", "El clima de una ciudad", "clima Monterrey", nil, func(cx *comandos.Contexto) (string, error) {
			ciudad := strings.TrimSpace(cx.Resto)
			if ciudad == "" {
				ciudad = g.Config(IDExtras).Texto("ciudad")
			}
			return g.extrasClima(ciudad, pie(cx))
		}),
		cmd("dolar", "Cuánto vale el dólar", "dolar 100", []string{"dólar", "divisas"}, func(cx *comandos.Contexto) (string, error) {
			return g.extrasDolar(cx.Resto, pie(cx))
		}),
		cmd("definir", "Qué significa algo", "definir fotosíntesis", []string{"significado", "wiki"}, func(cx *comandos.Contexto) (string, error) {
			return g.extrasDefinir(cx.Resto, pie(cx))
		}),
		cmd("buscar", "Buscar algo en internet", "buscar historia de México", []string{"busca", "google"}, func(cx *comandos.Contexto) (string, error) {
			return g.extrasBuscar(cx.Resto, pie(cx))
		}),
		cmd("acortar", "Acortar un enlace", "acortar https://…", []string{"corto"}, func(cx *comandos.Contexto) (string, error) {
			return g.extrasAcortar(cx.Resto, pie(cx))
		}),
		cmd("qr", "Hacer un código QR", "qr https://…", nil, func(cx *comandos.Contexto) (string, error) {
			t := strings.TrimSpace(cx.Resto)
			if t == "" || len([]rune(t)) > 300 {
				return "", errors.New("escribe el texto o enlace del QR (hasta 300 letras), por ejemplo: qr https://ejemplo.com")
			}
			ctx, cancel := ctxExtras()
			defer cancel()
			c, err := g.extrasPeticion(ctx, "https://api.qrserver.com/v1/create-qr-code/?size=400x400&margin=12&format=png&data="+url.QueryEscape(t), 200<<10)
			if err != nil {
				return "", err
			}
			defer c.Close()
			png, err := io.ReadAll(c)
			if err != nil || len(png) < 100 {
				return "", errors.New("no pude armar el QR ahora; intenta de nuevo")
			}
			ref, _, err := g.Alm.GuardarAdjuntoMenu(g.BotID, bytes.NewReader(png))
			if err != nil {
				return "", errors.New("no pude preparar el QR")
			}
			cx.Archivo = &comandos.Archivo{ID: ref, Tipo: almacen.TipoImagen, Mime: "image/png", Nombre: "qr.png"}
			return estilo.Tarjeta("🔳", g.tx("extras.t_qr"), []string{g.tx("extras.qr_linea")}, ""), nil
		}),
	}
}

func (g *Gestor) elige(l []string) string { return l[g.azar(len(l))] }

// tx da un texto de «Respuestas del bot» (el del dueño si lo cambió, o el de fábrica).
func (g *Gestor) tx(id string, vars ...string) string {
	return respuestas.Texto(g.Alm, g.BotID, id, vars...)
}

// propias da las líneas que el dueño escribió en un campo de «mis …»; si no escribió ninguna, las de fábrica.
func (g *Gestor) propias(clave string, fabrica []string) []string {
	var out []string
	for _, l := range strings.Split(g.Config(IDExtras).Texto(clave), "\n") {
		if l = strings.TrimSpace(l); l != "" {
			out = append(out, l)
		}
	}
	if len(out) == 0 {
		return fabrica
	}
	return out
}

func (g *Gestor) tieneLista(clave string) bool {
	return strings.TrimSpace(g.Config(IDExtras).Texto(clave)) != ""
}

// chiste: los del dueño si escribió; si no, uno nuevo de internet y, si falla o está apagado, de fábrica.
func (g *Gestor) chiste() string {
	if !g.tieneLista("mis_chistes") && g.Config(IDExtras).Bool("internet") {
		ctx, cancel := ctxExtras()
		defer cancel()
		var r struct {
			Error    bool   `json:"error"`
			Tipo     string `json:"type"`
			Chiste   string `json:"joke"`
			Pregunta string `json:"setup"`
			Remate   string `json:"delivery"`
		}
		if g.extrasJSON(ctx, "https://v2.jokeapi.dev/joke/Any?lang=es&safe-mode&blacklistFlags=nsfw,religious,political,racist,sexist,explicit", &r) == nil && !r.Error {
			t := strings.TrimSpace(r.Chiste)
			if r.Tipo == "twopart" {
				t = strings.TrimSpace(r.Pregunta) + "\n" + strings.TrimSpace(r.Remate)
			}
			if t != "" && len([]rune(t)) <= 500 {
				return t
			}
		}
	}
	return g.elige(g.propias("mis_chistes", chistes))
}

// dato: los del dueño si escribió; si no, un hecho de «un día como hoy» de Wikipedia (true) y, si falla, uno de fábrica.
func (g *Gestor) dato() (string, bool) {
	if !g.tieneLista("mis_datos") && g.Config(IDExtras).Bool("internet") {
		ctx, cancel := ctxExtras()
		defer cancel()
		var r struct {
			Hechos []struct {
				Texto string `json:"text"`
				Anio  int    `json:"year"`
			} `json:"selected"`
		}
		ahora := g.ahora()
		if g.extrasJSON(ctx, fmt.Sprintf("https://es.wikipedia.org/api/rest_v1/feed/onthisday/selected/%02d/%02d", int(ahora.Month()), ahora.Day()), &r) == nil && len(r.Hechos) > 0 {
			h := r.Hechos[g.azar(len(r.Hechos))]
			if t := strings.TrimSpace(h.Texto); t != "" && len([]rune(t)) <= 500 {
				if h.Anio > 0 {
					return fmt.Sprintf("*%d:* %s", h.Anio, t), true
				}
				return t, true
			}
		}
	}
	return g.elige(g.propias("mis_datos", datos)), false
}

var reCita = regexp.MustCompile(`(?s)«\s*(.+?)\s*»`)
var reTitulo = regexp.MustCompile(`title="([^"]{2,80})"`)

// frase: las del dueño si escribió; si no, la «cita del día» de la portada de Wikiquote en español y, si falla, una de fábrica.
func (g *Gestor) frase() string {
	if !g.tieneLista("mis_frases") && g.Config(IDExtras).Bool("internet") {
		ctx, cancel := ctxExtras()
		defer cancel()
		var r struct {
			Parse struct {
				Texto string `json:"text"`
			} `json:"parse"`
		}
		if g.extrasJSON(ctx, "https://es.wikiquote.org/w/api.php?action=parse&page=Portada&prop=text&format=json&formatversion=2", &r) == nil {
			if i := strings.Index(r.Parse.Texto, `id="mf-FDD"`); i >= 0 {
				seg := r.Parse.Texto[i:]
				if len(seg) > 6000 {
					seg = seg[:6000]
				}
				autor := ""
				if m := reTitulo.FindStringSubmatch(seg); m != nil {
					autor = strings.TrimSpace(html.UnescapeString(m[1]))
				}
				txt := strings.ReplaceAll(html.UnescapeString(reEtiqueta.ReplaceAllString(seg, "")), "\u00a0", " ")
				if m := reCita.FindStringSubmatch(txt); m != nil {
					cita := strings.Join(strings.Fields(m[1]), " ")
					if n := len([]rune(cita)); n >= 8 && n <= 400 {
						if autor != "" {
							return "«" + cita + "»\n— " + autor
						}
						return "«" + cita + "»"
					}
				}
			}
		}
	}
	return g.elige(g.propias("mis_frases", frases))
}

var reEtiqueta = regexp.MustCompile(`<[^>]*>`)

// extrasBuscar trae los primeros resultados de Wikipedia (en español) para lo que escribieron.
func (g *Gestor) extrasBuscar(resto, pie string) (string, error) {
	t := strings.TrimSpace(resto)
	if t == "" || len([]rune(t)) > 100 {
		return "", errors.New("escribe qué quieres buscar, por ejemplo: buscar historia de México")
	}
	ctx, cancel := ctxExtras()
	defer cancel()
	var r struct {
		Consulta struct {
			Resultados []struct {
				Titulo  string `json:"title"`
				Resumen string `json:"snippet"`
			} `json:"search"`
		} `json:"query"`
	}
	if err := g.extrasJSON(ctx, "https://es.wikipedia.org/w/api.php?action=query&list=search&srlimit=3&format=json&utf8=1&srsearch="+url.QueryEscape(t), &r); err != nil {
		return "", err
	}
	if len(r.Consulta.Resultados) == 0 {
		return "", errors.New("no encontré nada de «" + t + "»; prueba con otras palabras")
	}
	var l []string
	for _, x := range r.Consulta.Resultados {
		resumen := strings.TrimSpace(html.UnescapeString(reEtiqueta.ReplaceAllString(x.Resumen, "")))
		if rs := []rune(resumen); len(rs) > 160 {
			resumen = strings.TrimSpace(string(rs[:160])) + "…"
		}
		l = append(l, "• *"+x.Titulo+"*\n"+resumen+"\n"+"https://es.wikipedia.org/wiki/"+url.PathEscape(strings.ReplaceAll(x.Titulo, " ", "_")))
	}
	return estilo.Tarjeta("🔎", g.tx("extras.t_buscar", "texto", t), l, "Fuente: Wikipedia · "+pie), nil
}

// ---------- hora ----------

var zonasHora = map[string]string{
	"ciudad de mexico": "America/Mexico_City", "cdmx": "America/Mexico_City", "mexico": "America/Mexico_City", "monterrey": "America/Monterrey", "guadalajara": "America/Mexico_City",
	"tijuana": "America/Tijuana", "cancun": "America/Cancun", "bogota": "America/Bogota", "lima": "America/Lima", "quito": "America/Guayaquil", "caracas": "America/Caracas",
	"santiago": "America/Santiago", "buenos aires": "America/Argentina/Buenos_Aires", "montevideo": "America/Montevideo", "sao paulo": "America/Sao_Paulo", "la paz": "America/La_Paz",
	"panama": "America/Panama", "san jose": "America/Costa_Rica", "habana": "America/Havana", "nueva york": "America/New_York", "new york": "America/New_York", "miami": "America/New_York",
	"chicago": "America/Chicago", "los angeles": "America/Los_Angeles", "madrid": "Europe/Madrid", "barcelona": "Europe/Madrid", "londres": "Europe/London", "paris": "Europe/Paris",
	"berlin": "Europe/Berlin", "roma": "Europe/Rome", "moscu": "Europe/Moscow", "dubai": "Asia/Dubai", "tokio": "Asia/Tokyo", "seul": "Asia/Seoul", "pekin": "Asia/Shanghai",
	"shanghai": "Asia/Shanghai", "sidney": "Australia/Sydney", "el cairo": "Africa/Cairo", "utc": "UTC",
}

func sinAcentos(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(s)) {
		switch r {
		case 'á', 'à', 'ä':
			r = 'a'
		case 'é', 'è', 'ë':
			r = 'e'
		case 'í', 'ì', 'ï':
			r = 'i'
		case 'ó', 'ò', 'ö':
			r = 'o'
		case 'ú', 'ù', 'ü':
			r = 'u'
		}
		b.WriteRune(r)
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

func (g *Gestor) extrasHora(cx *comandos.Contexto, pie string) (string, error) {
	ahora := g.ahora()
	arg := strings.TrimSpace(cx.Resto)
	if arg == "" {
		l := []string{"🕒 Aquí: *" + g.Hora().Format("15:04") + "* · " + g.Hora().Format("02/01/2006")}
		for _, c := range []struct{ n, z string }{{"Madrid", "Europe/Madrid"}, {"Nueva York", "America/New_York"}, {"Tokio", "Asia/Tokyo"}} {
			if loc, err := time.LoadLocation(c.z); err == nil {
				l = append(l, "• "+c.n+": "+ahora.In(loc).Format("15:04"))
			}
		}
		return estilo.Tarjeta("🌎", g.tx("extras.t_hora_mundo"), l, "Prueba: "+cx.D.Prefijo()+"hora Tokio"), nil
	}
	zona, ok := zonasHora[sinAcentos(arg)]
	if !ok {
		zona = arg
	}
	loc, err := time.LoadLocation(zona)
	if err != nil {
		return "", errors.New("no conozco esa ciudad; prueba con una grande, por ejemplo: hora Madrid, hora Tokio o hora Bogotá")
	}
	t := ahora.In(loc)
	return estilo.Tarjeta("🕒", g.tx("extras.t_hora", "ciudad", strings.TrimSpace(arg)), []string{"*" + t.Format("15:04") + "* · " + t.Format("02/01/2006")}, pie), nil
}

// ---------- clima ----------

func climaTexto(codigo int) (string, string) {
	switch {
	case codigo == 0:
		return "☀️", "despejado"
	case codigo <= 2:
		return "🌤️", "poco nublado"
	case codigo == 3:
		return "☁️", "nublado"
	case codigo == 45 || codigo == 48:
		return "🌫️", "niebla"
	case codigo >= 51 && codigo <= 57:
		return "🌦️", "llovizna"
	case codigo >= 61 && codigo <= 67, codigo >= 80 && codigo <= 82:
		return "🌧️", "lluvia"
	case codigo >= 71 && codigo <= 77, codigo == 85, codigo == 86:
		return "❄️", "nieve"
	case codigo >= 95:
		return "⛈️", "tormenta"
	}
	return "🌡️", "variable"
}

func (g *Gestor) extrasClima(ciudad, pie string) (string, error) {
	ciudad = strings.TrimSpace(ciudad)
	if ciudad == "" || len([]rune(ciudad)) > 60 {
		return "", errors.New("escribe una ciudad, por ejemplo: clima Monterrey")
	}
	ctx, cancel := ctxExtras()
	defer cancel()
	var geo struct {
		Results []struct {
			Name    string  `json:"name"`
			Country string  `json:"country"`
			Lat     float64 `json:"latitude"`
			Lon     float64 `json:"longitude"`
		} `json:"results"`
	}
	if err := g.extrasJSON(ctx, "https://geocoding-api.open-meteo.com/v1/search?count=1&language=es&format=json&name="+url.QueryEscape(ciudad), &geo); err != nil {
		return "", err
	}
	if len(geo.Results) == 0 {
		return "", errors.New("no encontré esa ciudad; prueba con otra o con su nombre completo")
	}
	l := geo.Results[0]
	var w struct {
		Current struct {
			Temp    float64 `json:"temperature_2m"`
			Sens    float64 `json:"apparent_temperature"`
			Humedad float64 `json:"relative_humidity_2m"`
			Viento  float64 `json:"wind_speed_10m"`
			Codigo  int     `json:"weather_code"`
		} `json:"current"`
		Daily struct {
			Max  []float64 `json:"temperature_2m_max"`
			Min  []float64 `json:"temperature_2m_min"`
			Lluv []float64 `json:"precipitation_probability_max"`
		} `json:"daily"`
	}
	q := fmt.Sprintf("https://api.open-meteo.com/v1/forecast?latitude=%.4f&longitude=%.4f&current=temperature_2m,apparent_temperature,relative_humidity_2m,weather_code,wind_speed_10m&daily=temperature_2m_max,temperature_2m_min,precipitation_probability_max&timezone=auto&forecast_days=1", l.Lat, l.Lon)
	if err := g.extrasJSON(ctx, q, &w); err != nil {
		return "", err
	}
	emoji, desc := climaTexto(w.Current.Codigo)
	lineas := []string{fmt.Sprintf("%s *%.0f °C* · %s", emoji, w.Current.Temp, desc), fmt.Sprintf("🤒 Se siente como %.0f °C", w.Current.Sens),
		fmt.Sprintf("💧 Humedad %.0f %% · 💨 Viento %.0f km/h", w.Current.Humedad, w.Current.Viento)}
	if len(w.Daily.Max) > 0 && len(w.Daily.Min) > 0 {
		x := fmt.Sprintf("📈 Hoy: máx %.0f °C · mín %.0f °C", w.Daily.Max[0], w.Daily.Min[0])
		if len(w.Daily.Lluv) > 0 {
			x += fmt.Sprintf(" · 🌂 lluvia %.0f %%", w.Daily.Lluv[0])
		}
		lineas = append(lineas, x)
	}
	nombre := l.Name
	if l.Country != "" {
		nombre += ", " + l.Country
	}
	return estilo.Tarjeta("🌤️", g.tx("extras.t_clima", "ciudad", nombre), lineas, pie), nil
}

// ---------- dólar ----------

func (g *Gestor) extrasDolar(resto, pie string) (string, error) {
	monto := 1.0
	if t := strings.TrimSpace(strings.ReplaceAll(resto, ",", ".")); t != "" {
		m, err := strconv.ParseFloat(strings.Fields(t)[0], 64)
		if err != nil || m <= 0 || m > 1e12 {
			return "", errors.New("escribe una cantidad, por ejemplo: dolar 100")
		}
		monto = m
	}
	ctx, cancel := ctxExtras()
	defer cancel()
	var r struct {
		Resultado string             `json:"result"`
		Tasas     map[string]float64 `json:"rates"`
	}
	if err := g.extrasJSON(ctx, "https://open.er-api.com/v6/latest/USD", &r); err != nil {
		return "", err
	}
	mxn, eur := r.Tasas["MXN"], r.Tasas["EUR"]
	if r.Resultado != "success" || mxn <= 0 || eur <= 0 {
		return "", errors.New("no pude saber el precio del dólar ahora; intenta más tarde")
	}
	l := []string{fmt.Sprintf("🇺🇸 %s USD", num(monto)), fmt.Sprintf("🇲🇽 *%s MXN*", num(monto*mxn)), fmt.Sprintf("🇪🇺 %s EUR", num(monto*eur))}
	return estilo.Tarjeta("💵", g.tx("extras.t_dolar"), l, "Precio de referencia, puede variar en el banco · "+pie), nil
}

func num(f float64) string {
	s := strconv.FormatFloat(math.Round(f*100)/100, 'f', 2, 64)
	s = strings.TrimRight(strings.TrimRight(s, "0"), ".")
	return s
}

// ---------- definir y acortar ----------

func (g *Gestor) extrasDefinir(resto, pie string) (string, error) {
	t := strings.TrimSpace(resto)
	if t == "" || len([]rune(t)) > 80 {
		return "", errors.New("escribe qué quieres saber, por ejemplo: definir fotosíntesis")
	}
	ctx, cancel := ctxExtras()
	defer cancel()
	var r struct {
		Titulo   string `json:"title"`
		Extracto string `json:"extract"`
		Tipo     string `json:"type"`
	}
	err := g.extrasJSON(ctx, "https://es.wikipedia.org/api/rest_v1/page/summary/"+url.PathEscape(strings.ReplaceAll(t, " ", "_")), &r)
	if errors.Is(err, errNoHallado) || (err == nil && strings.TrimSpace(r.Extracto) == "") {
		return "", errors.New("no encontré nada de «" + t + "»; prueba con otra palabra")
	}
	if err != nil {
		return "", err
	}
	ex := strings.TrimSpace(r.Extracto)
	if r.Tipo == "disambiguation" {
		ex += "\n(Hay varios significados: sé más específico.)"
	}
	if rs := []rune(ex); len(rs) > 600 {
		ex = strings.TrimSpace(string(rs[:600]))
		if i := strings.LastIndexAny(ex, ".;"); i > 200 {
			ex = ex[:i+1]
		} else {
			ex += "…"
		}
	}
	return estilo.Tarjeta("📚", r.Titulo, []string{ex}, "Fuente: Wikipedia · "+pie), nil
}

func (g *Gestor) extrasAcortar(resto, pie string) (string, error) {
	t := strings.TrimSpace(resto)
	u, err := url.Parse(t)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || len(t) > 800 || strings.ContainsAny(t, " \n\t") {
		return "", errors.New("escribe un enlace completo que empiece con https://, por ejemplo: acortar https://ejemplo.com/pagina-larga")
	}
	ctx, cancel := ctxExtras()
	defer cancel()
	c, err := g.extrasPeticion(ctx, "https://is.gd/create.php?format=simple&url="+url.QueryEscape(t), 2<<10)
	if err != nil {
		return "", err
	}
	defer c.Close()
	b, _ := io.ReadAll(c)
	corto := strings.TrimSpace(string(b))
	if !strings.HasPrefix(corto, "https://is.gd/") || strings.ContainsAny(corto, " \n") {
		return "", errors.New("no pude acortar ese enlace; revisa que sea correcto")
	}
	return estilo.Tarjeta("🔗", g.tx("extras.t_acortar"), []string{corto}, pie), nil
}

// ---------- calculadora ----------

var numEspacio = regexp.MustCompile(`[0-9.,]\s+[0-9.,]`)

// Calcula resuelve una cuenta sencilla: + - * / % ^, paréntesis, decimales, pi y sqrt(), abs(), round(). No ejecuta nada más.
func Calcula(texto string) (string, error) {
	t := strings.TrimSpace(texto)
	if t == "" {
		return "", errors.New("escribe una cuenta, por ejemplo: calc 12*(3+4)")
	}
	if len([]rune(t)) > 100 {
		return "", errors.New("esa cuenta es muy larga")
	}
	if numEspacio.MatchString(t) {
		return "", errors.New("no entendí esa cuenta: falta un signo entre los números (+ - * /)")
	}
	t = strings.NewReplacer("×", "*", "÷", "/", "x", "*", "X", "*", ",", ".", "^", "^").Replace(t)
	p := &calc{s: []rune(strings.ReplaceAll(strings.ToLower(t), " ", ""))}
	v, err := p.expr(0)
	if err != nil {
		return "", err
	}
	if p.i < len(p.s) {
		return "", errors.New("no entendí esa cuenta; usa números, + - * / ^ y paréntesis")
	}
	if math.IsNaN(v) || math.IsInf(v, 0) || math.Abs(v) > 1e15 {
		return "", errors.New("ese resultado es demasiado grande o no existe")
	}
	return num10(v), nil
}

func num10(f float64) string {
	if f == math.Trunc(f) && math.Abs(f) < 1e15 {
		return strconv.FormatFloat(f, 'f', 0, 64)
	}
	s := strconv.FormatFloat(f, 'f', 8, 64)
	return strings.TrimRight(strings.TrimRight(s, "0"), ".")
}

type calc struct {
	s []rune
	i int
}

func (c *calc) peek() rune {
	if c.i < len(c.s) {
		return c.s[c.i]
	}
	return 0
}

func (c *calc) expr(prof int) (float64, error) {
	if prof > 30 {
		return 0, errors.New("esa cuenta tiene demasiados paréntesis")
	}
	v, err := c.term(prof)
	for err == nil && (c.peek() == '+' || c.peek() == '-') {
		op := c.s[c.i]
		c.i++
		var w float64
		if w, err = c.term(prof); err == nil {
			if op == '+' {
				v += w
			} else {
				v -= w
			}
		}
	}
	return v, err
}

func (c *calc) term(prof int) (float64, error) {
	v, err := c.pot(prof)
	for err == nil && (c.peek() == '*' || c.peek() == '/' || c.peek() == '%') {
		op := c.s[c.i]
		c.i++
		var w float64
		if w, err = c.pot(prof); err != nil {
			break
		}
		switch op {
		case '*':
			v *= w
		case '/':
			if w == 0 {
				return 0, errors.New("no se puede dividir entre cero")
			}
			v /= w
		default:
			if w == 0 {
				return 0, errors.New("no se puede dividir entre cero")
			}
			v = math.Mod(v, w)
		}
	}
	return v, err
}

func (c *calc) pot(prof int) (float64, error) {
	v, err := c.unario(prof)
	if err == nil && c.peek() == '^' {
		c.i++
		w, e := c.pot(prof)
		if e != nil {
			return 0, e
		}
		v = math.Pow(v, w)
	}
	return v, err
}

func (c *calc) unario(prof int) (float64, error) {
	switch c.peek() {
	case '-':
		c.i++
		v, err := c.unario(prof)
		return -v, err
	case '+':
		c.i++
		return c.unario(prof)
	}
	return c.primario(prof)
}

func (c *calc) primario(prof int) (float64, error) {
	r := c.peek()
	switch {
	case r == '(':
		c.i++
		v, err := c.expr(prof + 1)
		if err != nil {
			return 0, err
		}
		if c.peek() != ')' {
			return 0, errors.New("falta cerrar un paréntesis")
		}
		c.i++
		return v, nil
	case unicode.IsDigit(r) || r == '.':
		j := c.i
		for c.i < len(c.s) && (unicode.IsDigit(c.s[c.i]) || c.s[c.i] == '.') {
			c.i++
		}
		v, err := strconv.ParseFloat(string(c.s[j:c.i]), 64)
		if err != nil {
			return 0, errors.New("hay un número mal escrito")
		}
		return v, nil
	case unicode.IsLetter(r):
		j := c.i
		for c.i < len(c.s) && unicode.IsLetter(c.s[c.i]) {
			c.i++
		}
		nombre := string(c.s[j:c.i])
		if nombre == "pi" {
			return math.Pi, nil
		}
		if c.peek() != '(' {
			return 0, errors.New("no entendí «" + nombre + "»; puedo usar pi, sqrt(), abs() y round()")
		}
		c.i++
		v, err := c.expr(prof + 1)
		if err != nil {
			return 0, err
		}
		if c.peek() != ')' {
			return 0, errors.New("falta cerrar un paréntesis")
		}
		c.i++
		switch nombre {
		case "sqrt", "raiz":
			if v < 0 {
				return 0, errors.New("no existe la raíz de un número negativo")
			}
			return math.Sqrt(v), nil
		case "abs":
			return math.Abs(v), nil
		case "round", "redondear":
			return math.Round(v), nil
		}
		return 0, errors.New("no conozco «" + nombre + "»; puedo usar sqrt(), abs() y round()")
	}
	return 0, errors.New("no entendí esa cuenta; usa números, + - * / ^ y paréntesis")
}

// ---------- textos sin internet ----------

var chistes = []string{
	"¿Qué le dice un jardinero a otro? Disfrutemos mientras podamos.",
	"¿Por qué los pájaros vuelan hacia el sur? Porque caminando tardarían mucho.",
	"¿Qué hace una abeja en el gimnasio? ¡Zum-ba!",
	"¿Cómo se llama el campeón de buceo japonés? Tokofondo. ¿Y el subcampeón? Kasitoko.",
	"–Doctor, me siento invisible. –¿Quién dijo eso? –Nadie me ve.",
	"¿Qué le dice un zero a otro zero? Nada, es que no valemos nada.",
	"Mi vecino tiene tantos gatos que su casa es un ron-ron-erío.",
	"¿Por qué el libro de matemáticas estaba triste? Porque tenía muchos problemas.",
	"–Mamá, ¿qué es la eternidad? –Es lo que tardas en bajar la basura.",
	"¿Qué le dice una iguana a su hermana gemela? Somos iguanitas.",
	"¿Cuál es el colmo de un electricista? Que su esposa se llame Luz y lo deje en la oscuridad.",
	"¿Qué le dice el 1 al 10? Para ser como yo, tienes que ser sincero.",
	"–¿Cuál es el animal más antiguo? –La cebra, porque está en blanco y negro.",
	"¿Por qué las focas del circo miran siempre hacia arriba? Porque es donde están los focos.",
	"Fui a la tienda de mascotas por un pez y salí con un catálogo. Qué anzuelo.",
	"¿Qué hace un perro con un taladro? Taladrando.",
	"¿Cómo se despiden los químicos? Ácido un placer.",
	"–Camarero, ¿este plato es de pollo o de pescado? –¿Por qué lo pregunta? –Porque el tenedor no sabe qué hacer.",
	"¿Qué le dice una pared a otra? Nos vemos en la esquina.",
	"¿Por qué la computadora fue al médico? Porque tenía un virus.",
}

var datos = []string{
	"Los pulpos tienen tres corazones y su sangre es azul.",
	"La miel nunca se echa a perder: se han encontrado tarros comestibles de más de 3 000 años.",
	"Un día en Venus dura más que un año en Venus.",
	"Los plátanos son ligeramente radiactivos por su potasio, pero totalmente inofensivos.",
	"El corazón de una ballena azul pesa tanto como un automóvil pequeño.",
	"Las hormigas no duermen como nosotros: descansan en ratitos de unos minutos.",
	"La Torre Eiffel crece unos 15 cm en verano porque el metal se dilata con el calor.",
	"Los koalas tienen huellas dactilares casi idénticas a las humanas.",
	"El agua caliente puede congelarse más rápido que la fría en ciertas condiciones: se llama efecto Mpemba.",
	"Un rayo es cinco veces más caliente que la superficie del Sol.",
	"Las jirafas tienen la misma cantidad de huesos en el cuello que los humanos: siete.",
	"Los gatos pasan alrededor de dos tercios de su vida durmiendo.",
	"México tiene más de 60 lenguas indígenas reconocidas.",
	"El cerebro humano usa aproximadamente el 20 % de la energía del cuerpo.",
	"Las estrellas de mar no tienen cerebro ni sangre; usan agua de mar para moverse.",
	"En promedio, una persona camina el equivalente a dar varias vueltas a la Tierra a lo largo de su vida.",
	"El sonido viaja unas cuatro veces más rápido en el agua que en el aire.",
	"El corazón humano late cerca de 100 000 veces al día.",
	"Los delfines duermen con la mitad del cerebro a la vez para seguir respirando.",
	"La Luna se aleja de la Tierra unos 3,8 centímetros cada año.",
}

var frases = []string{
	"Lo que haces hoy puede mejorar todos tus mañanas.",
	"No tienes que ser perfecto para empezar, pero tienes que empezar para ser mejor.",
	"Cada pequeño paso cuenta; lo importante es no detenerse.",
	"La constancia vence al talento cuando el talento no se esfuerza.",
	"Hoy es un buen día para intentarlo otra vez.",
	"Quien sabe esperar con calma, llega más lejos que quien corre sin rumbo.",
	"Las dificultades son escalones, no muros.",
	"Sé amable: todos libran batallas que no se ven.",
	"Lo mejor está por venir; mientras tanto, disfruta el camino.",
	"Haz las cosas con ganas y saldrán mejor de lo que esperabas.",
	"Cuida tu energía: es el recurso más valioso que tienes.",
	"Una sonrisa a tiempo cambia el ritmo de un día entero.",
	"Aprender es el único tesoro que nadie te puede quitar.",
	"No compares tu capítulo uno con el capítulo veinte de otra persona.",
	"Respira hondo: lo estás haciendo mejor de lo que piensas.",
}
