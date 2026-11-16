package modulos

import (
	"errors"
	"strings"
	"testing"
	"time"

	"botchat/internal/comandos"
)

func extrasLista(t *testing.T) (*ent, *descFalso) {
	e, _ := entH(t)
	d := &descFalso{}
	e.g.Descarga = d
	e.activa(IDExtras)
	return e, d
}

func cmdExtra(t *testing.T, e *ent, nombre string) comandos.Comando {
	t.Helper()
	return *comandoDe(t, e, IDExtras, nombre)
}

func ejecuta(t *testing.T, e *ent, nombre, resto string) (string, error) {
	t.Helper()
	c := cmdExtra(t, e, nombre)
	return c.Ejecutar(&comandos.Contexto{Persona: pnAna, Chat: pnAna, Resto: resto, Args: strings.Fields(resto), D: e.cmd})
}

func TestCalculadoraResuelveYRechazaLoQueNoEsUnaCuenta(t *testing.T) {
	buenas := map[string]string{
		"2+2": "4", "12*(3+4)": "84", "10/4": "2.5", "2^10": "1024", "-3+5": "2", "7%3": "1", "sqrt(144)+1": "13", "1,5*2": "3", "3x4": "12",
		"abs(-8)": "8", "round(2.6)": "3", "((2+3)*(4-1))^2": "225", "100-1-1-1": "97", "2^3^2": "512", "0.1+0.2": "0.3",
	}
	for cuenta, esperado := range buenas {
		if r, err := Calcula(cuenta); err != nil || r != esperado {
			t.Errorf("%q = %q (%v), esperaba %q", cuenta, r, err, esperado)
		}
	}
	malas := map[string]string{"": "escribe una cuenta", "5/0": "dividir entre cero", "(2+3": "paréntesis", "2+": "no entendí", "hola": "no entendí", "system(1)": "no conozco",
		"sqrt(-4)": "negativo", "9^9^9": "demasiado grande", "2 3": "no entendí", strings.Repeat("1+", 60) + "1": "muy larga", "((((((((((((((((((((((((((((((((1))))))))))))))))))))))))))))))))": "paréntesis"}
	for cuenta, sub := range malas {
		if _, err := Calcula(cuenta); err == nil || !strings.Contains(err.Error(), sub) {
			t.Errorf("%q: esperaba un error con %q, llegó %v", cuenta, sub, err)
		}
	}
}

func TestExtrasSinInternetFuncionanYSeVenBonitos(t *testing.T) {
	e, d := extrasLista(t)
	d.resp = func(PeticionHTTP) (*RespuestaHTTP, error) { return nil, errors.New("sin red") }
	for _, c := range []string{"calc", "hora", "chiste", "dato", "frase"} {
		r, err := ejecuta(t, e, c, map[string]string{"calc": "2+2"}[c])
		if err != nil || !strings.Contains(r, "*") || !strings.Contains(r, "━") || !strings.Contains(r, "💡") {
			t.Errorf("%s: %q %v", c, r, err)
		}
	}
	if len(d.reqs) != 3 { // chiste, dato y frase intentan internet; sin red usan los de fábrica
		t.Fatalf("solo chiste, dato y frase deben intentar internet: %v", d.reqs)
	}
	r, _ := ejecuta(t, e, "calc", "12*(3+4)")
	if !strings.Contains(r, "= *84*") {
		t.Fatalf("calc: %q", r)
	}
	// hora de otra ciudad, con y sin acentos, y una zona escrita completa
	for _, ciudad := range []string{"Tokio", "bogotá", "Nueva York", "America/Lima"} {
		if r, err := ejecuta(t, e, "hora", ciudad); err != nil || !strings.Contains(r, "Hora en") {
			t.Errorf("hora %s: %q %v", ciudad, r, err)
		}
	}
	if _, err := ejecuta(t, e, "hora", "Narnia"); err == nil || !strings.Contains(err.Error(), "no conozco esa ciudad") {
		t.Fatalf("ciudad inventada: %v", err)
	}
}

const jsonGeo = `{"results":[{"name":"Monterrey","country":"México","latitude":25.67,"longitude":-100.31}]}`
const jsonClima = `{"current":{"temperature_2m":28.4,"apparent_temperature":30.1,"relative_humidity_2m":55,"weather_code":2,"wind_speed_10m":12.5},"daily":{"temperature_2m_max":[33.2],"temperature_2m_min":[21.0],"precipitation_probability_max":[20]}}`

func TestClimaDolarDefinirAcortarYQR(t *testing.T) {
	e, d := extrasLista(t)
	d.resp = func(p PeticionHTTP) (*RespuestaHTTP, error) {
		switch {
		case strings.Contains(p.URL, "geocoding-api.open-meteo.com") && strings.Contains(p.URL, "Narnia"):
			return cuerpo("application/json", "", `{}`), nil
		case strings.Contains(p.URL, "geocoding-api.open-meteo.com"):
			return cuerpo("application/json", "", jsonGeo), nil
		case strings.Contains(p.URL, "api.open-meteo.com"):
			return cuerpo("application/json", "", jsonClima), nil
		case strings.Contains(p.URL, "open.er-api.com"):
			return cuerpo("application/json", "", `{"result":"success","rates":{"MXN":17.5,"EUR":0.92}}`), nil
		case strings.Contains(p.URL, "es.wikipedia.org") && strings.Contains(p.URL, "Inventada"):
			r := cuerpo("application/json", "", `{}`)
			r.Estado = 404
			return r, nil
		case strings.Contains(p.URL, "es.wikipedia.org"):
			return cuerpo("application/json", "", `{"title":"Fotosíntesis","extract":"La fotosíntesis es el proceso por el que las plantas fabrican su alimento. Usa luz, agua y dióxido de carbono.","type":"standard"}`), nil
		case strings.Contains(p.URL, "is.gd"):
			return cuerpo("text/plain", "", "https://is.gd/abc123\n"), nil
		case strings.Contains(p.URL, "qrserver.com"):
			return cuerpo("image/png", "qr.png", strings.Repeat("P", 400)), nil
		}
		return nil, errors.New("inesperado: " + p.URL)
	}
	r, err := ejecuta(t, e, "clima", "Monterrey")
	if err != nil || !strings.Contains(r, "Clima en Monterrey, México") || !strings.Contains(r, "28 °C") || !strings.Contains(r, "poco nublado") || !strings.Contains(r, "máx 33") || !strings.Contains(r, "lluvia 20") {
		t.Fatalf("clima: %q %v", r, err)
	}
	if _, err := ejecuta(t, e, "clima", "Narnia"); err == nil || !strings.Contains(err.Error(), "no encontré esa ciudad") {
		t.Fatalf("ciudad que no existe: %v", err)
	}
	r, err = ejecuta(t, e, "clima", "")
	if err != nil || !strings.Contains(r, "Clima en") { // usa la ciudad por defecto de la configuración
		t.Fatalf("clima sin ciudad: %q %v", r, err)
	}
	r, err = ejecuta(t, e, "dolar", "100")
	if err != nil || !strings.Contains(r, "100 USD") || !strings.Contains(r, "1750 MXN") || !strings.Contains(r, "92 EUR") {
		t.Fatalf("dolar: %q %v", r, err)
	}
	if _, err := ejecuta(t, e, "dolar", "mucho"); err == nil {
		t.Fatal("una cantidad inválida se rechaza")
	}
	r, err = ejecuta(t, e, "definir", "fotosíntesis")
	if err != nil || !strings.Contains(r, "Fotosíntesis") || !strings.Contains(r, "Wikipedia") {
		t.Fatalf("definir: %q %v", r, err)
	}
	if _, err := ejecuta(t, e, "definir", "Inventada"); err == nil || !strings.Contains(err.Error(), "no encontré nada") {
		t.Fatalf("definir lo que no existe: %v", err)
	}
	r, err = ejecuta(t, e, "acortar", "https://ejemplo.com/una/pagina/larga")
	if err != nil || !strings.Contains(r, "https://is.gd/abc123") {
		t.Fatalf("acortar: %q %v", r, err)
	}
	for _, mal := range []string{"", "ejemplo.com", "ftp://x.com/a", "https://a.com/ b"} {
		if _, err := ejecuta(t, e, "acortar", mal); err == nil {
			t.Errorf("%q no es un enlace válido", mal)
		}
	}
	// qr: devuelve una imagen guardada para mandar
	c := cmdExtra(t, e, "qr")
	cx := &comandos.Contexto{Persona: pnAna, Chat: pnAna, Resto: "https://ejemplo.com", D: e.cmd}
	if r, err := c.Ejecutar(cx); err != nil || cx.Archivo == nil || cx.Archivo.Mime != "image/png" || !strings.Contains(r, "código QR") {
		t.Fatalf("qr: %q %v %+v", r, err, cx.Archivo)
	}
	// solo llega a los servicios de la lista
	for _, p := range d.reqs {
		if p.Permitido != nil && p.Permitido("evil.example.com") {
			t.Fatalf("un host fuera de la lista no debe pasar: %s", p.URL)
		}
	}
}

func TestExtrasInternetApagadoTopeYServicioCaido(t *testing.T) {
	e, d := extrasLista(t)
	d.resp = func(p PeticionHTTP) (*RespuestaHTTP, error) { return nil, errors.New("sin red") }
	if _, err := ejecuta(t, e, "clima", "Lima"); err == nil || !strings.Contains(err.Error(), "no pude consultar el servicio") {
		t.Fatalf("servicio caído: %v", err)
	}
	e.config(IDExtras, map[string]any{"internet": false})
	n := len(d.reqs)
	if _, err := ejecuta(t, e, "dolar", ""); err == nil || !strings.Contains(err.Error(), "apagados") || len(d.reqs) != n {
		t.Fatalf("internet apagado: %v", err)
	}
	if r, err := ejecuta(t, e, "calc", "1+1"); err != nil || !strings.Contains(r, "2") {
		t.Fatalf("lo que no usa internet sigue: %q %v", r, err)
	}
	e.hora = e.hora.Add(61 * time.Minute) // se vacía el conteo de lo que ya se usó en esta prueba
	e.config(IDExtras, map[string]any{"usos_hora": 3.0})
	for i := 0; i < 3; i++ {
		if _, err := ejecuta(t, e, "chiste", ""); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := ejecuta(t, e, "chiste", ""); err == nil || !strings.Contains(err.Error(), "muchos comandos") {
		t.Fatalf("tope por hora: %v", err)
	}
	e.hora = e.hora.Add(61 * time.Minute)
	if _, err := ejecuta(t, e, "chiste", ""); err != nil {
		t.Fatalf("pasada la hora vuelve a funcionar: %v", err)
	}
}

func TestExtrasEsDeTodosYNaceApagado(t *testing.T) {
	e, _ := entH(t)
	i, _ := e.g.InfoDe(IDExtras)
	if i.Activo || i.Riesgo != RiesgoBajo || i.Iniciativa {
		t.Fatalf("info: %+v", i)
	}
	if len(chistes) < 15 || len(datos) < 15 || len(frases) < 10 {
		t.Fatal("faltan textos de ejemplo")
	}
}

func TestExtrasChisteYDatoDeInternetListasPropiasYTitulos(t *testing.T) {
	e, d := extrasLista(t)
	d.resp = func(p PeticionHTTP) (*RespuestaHTTP, error) {
		switch {
		case strings.Contains(p.URL, "jokeapi"):
			return cuerpo("application/json", "", `{"error":false,"type":"twopart","setup":"¿Qué hace una abeja?","delivery":"Zum."}`), nil
		case strings.Contains(p.URL, "onthisday"):
			return cuerpo("application/json", "", `{"selected":[{"text":"Pasó algo importante.","year":1969}]}`), nil
		case strings.Contains(p.URL, "wikiquote"):
			return cuerpo("application/json", "", `{"parse":{"text":"<div id=\"mf-FDD\"><a title=\"Jean d’Alembert\"><img></a><td>«&#160;<p>La guerra es   el arte</p>&#160;»</td></div>"}}`), nil
		case strings.Contains(p.URL, "list=search"):
			return cuerpo("application/json", "", `{"query":{"search":[{"title":"Fotosíntesis oxigénica","snippet":"La <span class=\"searchmatch\">fotosíntesis</span> es &quot;un proceso&quot;"}]}}`), nil
		}
		return nil, errors.New("no esperado: " + p.URL)
	}
	if r, err := ejecuta(t, e, "chiste", ""); err != nil || !strings.Contains(r, "¿Qué hace una abeja?\nZum.") {
		t.Fatalf("chiste de internet: %q %v", r, err)
	}
	if r, err := ejecuta(t, e, "dato", ""); err != nil || !strings.Contains(r, "*1969:* Pasó algo importante.") || !strings.Contains(r, "Un día como hoy") {
		t.Fatalf("dato de internet: %q %v", r, err)
	}
	if r, err := ejecuta(t, e, "frase", ""); err != nil || !strings.Contains(r, "«La guerra es el arte»\n— Jean d’Alembert") {
		t.Fatalf("frase de internet: %q %v", r, err)
	}
	r, err := ejecuta(t, e, "buscar", "fotosíntesis")
	if err != nil || !strings.Contains(r, "*Fotosíntesis oxigénica*") || !strings.Contains(r, "es \"un proceso\"") || strings.Contains(r, "<span") || !strings.Contains(r, "https://es.wikipedia.org/wiki/") {
		t.Fatalf("buscar: %q %v", r, err)
	}
	// con listas propias no se usa internet y salen solo las del dueño
	n := len(d.reqs)
	e.config(IDExtras, map[string]any{"mis_chistes": "Mi chiste uno\nMi chiste dos", "mis_datos": "Mi dato", "mis_frases": "Mi frase"})
	for cmd, esperado := range map[string]string{"chiste": "Mi chiste ", "dato": "Mi dato", "frase": "Mi frase"} {
		if r, err := ejecuta(t, e, cmd, ""); err != nil || !strings.Contains(r, esperado) {
			t.Errorf("%s propio: %q %v", cmd, r, err)
		}
	}
	if len(d.reqs) != n {
		t.Fatalf("con listas propias no debe ir a internet: %v", d.reqs[n:])
	}
	// el título se cambia desde «Respuestas del bot»
	if err := e.g.Alm.FijarRespuesta(e.g.BotID, "extras.t_chiste", "Me río de"); err != nil {
		t.Fatal(err)
	}
	if r, _ := ejecuta(t, e, "chiste", ""); !strings.Contains(r, "*Me río de*") {
		t.Fatalf("título propio: %q", r)
	}
}
