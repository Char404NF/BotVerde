package modulos

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"botchat/internal/comandos"
)

type descFalso struct {
	reqs []PeticionHTTP
	resp func(p PeticionHTTP) (*RespuestaHTTP, error)
}

func (d *descFalso) Hacer(_ context.Context, p PeticionHTTP) (*RespuestaHTTP, error) {
	d.reqs = append(d.reqs, p)
	if p.Permitido != nil {
		u := strings.TrimPrefix(strings.TrimPrefix(p.URL, "https://"), "http://")
		host := strings.SplitN(strings.SplitN(u, "/", 2)[0], ":", 2)[0]
		if !p.Permitido(host) {
			return nil, ErrDominio
		}
	}
	return d.resp(p)
}

func cuerpo(mime, nombre, datos string) *RespuestaHTTP {
	return &RespuestaHTTP{Estado: 200, Mime: mime, Nombre: nombre, Largo: int64(len(datos)), Cuerpo: io.NopCloser(strings.NewReader(datos))}
}

func descargasLista(t *testing.T) (*ent, *descFalso) {
	e, _ := entH(t)
	d := &descFalso{}
	e.g.Descarga = d
	e.activa(IDDescargas)
	e.config(IDDescargas, map[string]any{"dominios": "archivos.ejemplo.com\nmiservidor.net"})
	return e, d
}

func cmdDe(t *testing.T, e *ent, nombre string) comandos.Comando {
	t.Helper()
	m, _ := Obtener(IDDescargas)
	for _, c := range m.(ComandosDeGestor).ComandosG(e.g) {
		if c.Nombre == nombre {
			return c
		}
	}
	t.Fatalf("no hay el comando %s", nombre)
	return comandos.Comando{}
}

func TestDescargasNoEsIniciativaYNaceApagado(t *testing.T) {
	e, _ := entH(t)
	i, _ := e.g.InfoDe(IDDescargas)
	if i.Activo || i.Iniciativa || i.Riesgo != RiesgoMedio {
		t.Fatalf("info: %+v", i)
	}
	if strings.Contains(strings.ToLower(i.Descripcion), "youtube") && !strings.Contains(strings.ToLower(i.Descripcion), "no incluye") {
		t.Fatal("la descripción debe aclarar que no hay descargadores de YouTube")
	}
}

func TestBajarEnviaDocumentoDeDominioPermitido(t *testing.T) {
	e, d := descargasLista(t)
	d.resp = func(p PeticionHTTP) (*RespuestaHTTP, error) {
		return cuerpo("application/pdf", "catalogo.pdf", "%PDF-1.4 hola"), nil
	}
	cx := &comandos.Contexto{Resto: "https://archivos.ejemplo.com/x/catalogo.pdf", Chat: pnAna}
	c := cmdDe(t, e, "bajar")
	if _, err := c.Ejecutar(cx); err != nil {
		t.Fatal(err)
	}
	if cx.Archivo == nil || cx.Archivo.Tipo != "documento" || cx.Archivo.Nombre != "catalogo.pdf" || cx.Archivo.Mime != "application/pdf" {
		t.Fatalf("archivo: %+v", cx.Archivo)
	}
	b, err := e.alm.LeerAdjuntoMenu(1, cx.Archivo.ID)
	if err != nil || string(b) != "%PDF-1.4 hola" {
		t.Fatalf("contenido cifrado: %q %v", b, err)
	}
	if d.reqs[0].Max != 16<<20 || d.reqs[0].Timeout != 30*time.Second || d.reqs[0].Metodo != "GET" {
		t.Fatalf("límites: %+v", d.reqs[0])
	}
}

func TestBajarRechazos(t *testing.T) {
	e, d := descargasLista(t)
	c := cmdDe(t, e, "bajar")
	corre := func(enlace string) error {
		_, err := c.Ejecutar(&comandos.Contexto{Resto: enlace})
		return err
	}
	d.resp = func(p PeticionHTTP) (*RespuestaHTTP, error) { return cuerpo("application/pdf", "a.pdf", "x"), nil }
	esperaErr(t, corre(""), "enlace")
	esperaErr(t, corre("hola"), "enlace directo")
	esperaErr(t, corre("http://archivos.ejemplo.com/a.pdf"), "https")
	esperaErr(t, corre("https://otro.com/a.pdf"), "no está en la lista")
	// sin dominios configurados no se baja nada
	e.config(IDDescargas, map[string]any{"dominios": ""})
	esperaErr(t, corre("https://archivos.ejemplo.com/a.pdf"), "dominios permitidos")
	e.config(IDDescargas, map[string]any{"dominios": "archivos.ejemplo.com"})
	// el servidor responde mal
	d.resp = func(p PeticionHTTP) (*RespuestaHTTP, error) {
		r := cuerpo("text/plain", "a.txt", "no")
		r.Estado = 404
		return r, nil
	}
	esperaErr(t, corre("https://archivos.ejemplo.com/a.txt"), "404")
	// una página web
	d.resp = func(p PeticionHTTP) (*RespuestaHTTP, error) { return cuerpo("text/html", "watch", "<html>"), nil }
	esperaErr(t, corre("https://archivos.ejemplo.com/watch?v=1"), "página web")
	// tipo no permitido y programas
	d.resp = func(p PeticionHTTP) (*RespuestaHTTP, error) {
		return cuerpo("application/octet-stream", "setup.exe", "MZ"), nil
	}
	esperaErr(t, corre("https://archivos.ejemplo.com/setup.exe"), "no está permitido")
	e.config(IDDescargas, map[string]any{"tipos": "pdf,exe"})
	esperaErr(t, corre("https://archivos.ejemplo.com/setup.exe"), "no está permitido") // un programa nunca, ni en la lista
	// demasiado grande
	d.resp = func(p PeticionHTTP) (*RespuestaHTTP, error) { return nil, ErrMuyGrande }
	esperaErr(t, corre("https://archivos.ejemplo.com/a.pdf"), "pesa más")
	// vacío
	d.resp = func(p PeticionHTTP) (*RespuestaHTTP, error) { return cuerpo("application/pdf", "a.pdf", ""), nil }
	esperaErr(t, corre("https://archivos.ejemplo.com/a.pdf"), "vacío")
	// redirección a otro dominio
	d.resp = func(p PeticionHTTP) (*RespuestaHTTP, error) { return nil, ErrDominio }
	esperaErr(t, corre("https://archivos.ejemplo.com/a.pdf"), "redirige")
	// no hay extractores: un enlace de YouTube con el dominio permitido solo trae una página web y se rechaza
	e.config(IDDescargas, map[string]any{"dominios": "youtube.com"})
	d.resp = func(p PeticionHTTP) (*RespuestaHTTP, error) {
		return cuerpo("text/html", "watch", "<html>video</html>"), nil
	}
	esperaErr(t, corre("https://www.youtube.com/watch?v=abc"), "página web")
}

func TestBajarTopePorHora(t *testing.T) {
	e, d := descargasLista(t)
	e.config(IDDescargas, map[string]any{"usos_hora": 2.0})
	d.resp = func(p PeticionHTTP) (*RespuestaHTTP, error) { return cuerpo("application/pdf", "a.pdf", "x"), nil }
	c := cmdDe(t, e, "bajar")
	for i := 0; i < 2; i++ {
		if _, err := c.Ejecutar(&comandos.Contexto{Resto: "https://archivos.ejemplo.com/a.pdf"}); err != nil {
			t.Fatal(err)
		}
	}
	_, err := c.Ejecutar(&comandos.Contexto{Resto: "https://archivos.ejemplo.com/a.pdf"})
	esperaErr(t, err, "muchas descargas")
	e.hora = e.hora.Add(2 * time.Hour)
	if _, err := c.Ejecutar(&comandos.Contexto{Resto: "https://archivos.ejemplo.com/a.pdf"}); err != nil {
		t.Fatalf("pasada la hora vuelve a andar: %v", err)
	}
}

func conectorBasico() Args {
	return Args{"nombre": "clima", "metodo": "GET", "url": "https://api.miservidor.net/clima?ciudad={arg}", "cabeceras": "Authorization: Bearer SECRETO123\nX-Otro: 1",
		"salida": "texto", "plantilla": "En {ciudad} hay {main.temp} grados y {clima.0.texto}"}
}

func TestConectorTextoConPlantillaVariablesYCabeceras(t *testing.T) {
	e, d := descargasLista(t)
	d.resp = func(p PeticionHTTP) (*RespuestaHTTP, error) {
		return cuerpo("application/json", "", `{"ciudad":"Lima","main":{"temp":21.5},"clima":[{"texto":"nublado"}]}`), nil
	}
	if _, err := e.g.AccionModulo(IDDescargas, "conectores.agregar", conectorBasico()); err != nil {
		t.Fatal(err)
	}
	// la respuesta del servidor no trae "ciudad" arriba: la plantilla usa rutas
	c := cmdDe(t, e, "api")
	cx := &comandos.Contexto{Args: []string{"clima", "Lima"}, Resto: "clima Lima", Nombre: "Ana"}
	r, err := c.Ejecutar(cx)
	if err != nil || r != "En Lima hay 21.5 grados y nublado" {
		t.Fatalf("respuesta: %q %v", r, err)
	}
	p := d.reqs[len(d.reqs)-1]
	if p.URL != "https://api.miservidor.net/clima?ciudad=Lima" || p.Cabeceras["Authorization"] != "Bearer SECRETO123" || p.Metodo != "GET" {
		t.Fatalf("petición: %+v", p)
	}
	// las llaves no se muestran nunca
	filas, _ := e.g.FilasPanel(IDDescargas, "conectores")
	if len(filas) != 1 || strings.Contains(filas[0].Titulo+filas[0].Detalle, "SECRETO") {
		t.Fatalf("filas: %+v", filas)
	}
	cfg, _ := e.g.ConfigJSON(IDDescargas)
	if strings.Contains(cfg, "SECRETO") {
		t.Fatal("la llave no va en la configuración del módulo")
	}
	// variables en la URL se escapan
	_, _ = c.Ejecutar(&comandos.Contexto{Args: []string{"clima", "a", "b&c"}, Resto: "clima a b&c"})
	if u := d.reqs[len(d.reqs)-1].URL; !strings.HasSuffix(u, "ciudad=a+b%26c") {
		t.Fatalf("url: %s", u)
	}
}

func TestConectorPostCuerpoRutaYErrores(t *testing.T) {
	e, d := descargasLista(t)
	var visto PeticionHTTP
	d.resp = func(p PeticionHTTP) (*RespuestaHTTP, error) {
		visto = p
		return cuerpo("application/json", "", `{"datos":[{"texto":"hola desde el servicio"}]}`), nil
	}
	_, err := e.g.AccionModulo(IDDescargas, "conectores.agregar", Args{"nombre": "eco", "metodo": "POST", "url": "https://api.miservidor.net/eco",
		"cuerpo": `{"q":"{arg}","de":"{nombre}"}`, "salida": "texto", "ruta": "datos.0.texto"})
	if err != nil {
		t.Fatal(err)
	}
	r, err := cmdDe(t, e, "api").Ejecutar(&comandos.Contexto{Args: []string{"eco", "hola"}, Resto: "eco hola", Nombre: "Ana"})
	if err != nil || r != "hola desde el servicio" {
		t.Fatalf("%q %v", r, err)
	}
	if visto.Metodo != "POST" || string(visto.Cuerpo) != `{"q":"hola","de":"Ana"}` || visto.Cabeceras["Content-Type"] != "application/json" {
		t.Fatalf("petición: %+v / %s", visto, visto.Cuerpo)
	}
	// ruta que no existe
	_, err = e.g.AccionModulo(IDDescargas, "conectores.agregar", Args{"nombre": "mal", "url": "https://api.miservidor.net/x", "ruta": "no.existe"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = cmdDe(t, e, "api").Ejecutar(&comandos.Contexto{Args: []string{"mal"}, Resto: "mal"})
	esperaErr(t, err, "no encontré")
	// el servicio falla / no existe el conector / tiempo
	d.resp = func(p PeticionHTTP) (*RespuestaHTTP, error) {
		r := cuerpo("text/plain", "", "x")
		r.Estado = 500
		return r, nil
	}
	_, err = cmdDe(t, e, "api").Ejecutar(&comandos.Contexto{Args: []string{"eco"}, Resto: "eco"})
	esperaErr(t, err, "500")
	_, err = cmdDe(t, e, "api").Ejecutar(&comandos.Contexto{Args: []string{"nada"}, Resto: "nada"})
	esperaErr(t, err, "no existe")
	_, err = cmdDe(t, e, "api").Ejecutar(&comandos.Contexto{})
	esperaErr(t, err, "nombre del conector")
	// el dominio del conector no se puede cambiar por una redirección a otro host
	d.resp = func(p PeticionHTTP) (*RespuestaHTTP, error) { return nil, ErrDominio }
	_, err = cmdDe(t, e, "api").Ejecutar(&comandos.Contexto{Args: []string{"eco"}, Resto: "eco"})
	esperaErr(t, err, "redirige")
}

func TestConectorValidacion(t *testing.T) {
	e, _ := descargasLista(t)
	casos := map[string]Args{
		"nombre":   {"nombre": "Mal Nombre", "url": "https://a.com"},
		"URL":      {"nombre": "x", "url": "ftp://a.com"},
		"método":   {"nombre": "x", "url": "https://a.com", "metodo": "TRACE"},
		"salida":   {"nombre": "x", "url": "https://a.com", "salida": "video"},
		"cabecera": {"nombre": "x", "url": "https://a.com", "cabeceras": "sin dos puntos"},
		"Host":     {"nombre": "x", "url": "https://a.com", "cabeceras": "Host: otro.com"},
	}
	for sub, a := range casos {
		_, err := e.g.AccionModulo(IDDescargas, "conectores.agregar", a)
		if err == nil {
			t.Fatalf("%s debió fallar", sub)
		}
	}
	if _, err := e.g.AccionModulo(IDDescargas, "conectores.borrar", Args{"id": "x"}); err != nil {
		t.Fatal(err)
	}
}

func TestConectorImagenPorRutaYProbar(t *testing.T) {
	e, d := descargasLista(t)
	d.resp = func(p PeticionHTTP) (*RespuestaHTTP, error) {
		if strings.HasSuffix(p.URL, "/api") {
			return cuerpo("application/json", "", `{"foto":"https://miservidor.net/img/1.jpg"}`), nil
		}
		return cuerpo("image/jpeg", "1.jpg", "JPEGDATA"), nil
	}
	_, err := e.g.AccionModulo(IDDescargas, "conectores.agregar", Args{"nombre": "foto", "url": "https://miservidor.net/api", "salida": "imagen", "ruta": "foto"})
	if err != nil {
		t.Fatal(err)
	}
	cx := &comandos.Contexto{Args: []string{"foto"}, Resto: "foto"}
	if _, err := cmdDe(t, e, "api").Ejecutar(cx); err != nil {
		t.Fatal(err)
	}
	if cx.Archivo == nil || cx.Archivo.Tipo != "imagen" {
		t.Fatalf("archivo: %+v", cx.Archivo)
	}
	b, _ := e.alm.LeerAdjuntoMenu(1, cx.Archivo.ID)
	if string(b) != "JPEGDATA" {
		t.Fatal("la imagen se guarda cifrada")
	}
	// una dirección secundaria de otro dominio no se baja
	d.resp = func(p PeticionHTTP) (*RespuestaHTTP, error) {
		if strings.HasSuffix(p.URL, "/api") {
			return cuerpo("application/json", "", `{"foto":"https://malo.com/1.jpg"}`), nil
		}
		return cuerpo("image/jpeg", "1.jpg", "X"), nil
	}
	_, err = cmdDe(t, e, "api").Ejecutar(&comandos.Contexto{Args: []string{"foto"}, Resto: "foto"})
	esperaErr(t, err, "redirige")
	// probar no deja el archivo
	d.resp = func(p PeticionHTTP) (*RespuestaHTTP, error) {
		if strings.HasSuffix(p.URL, "/api") {
			return cuerpo("application/json", "", `{"foto":"https://miservidor.net/img/1.jpg"}`), nil
		}
		return cuerpo("image/jpeg", "1.jpg", "JPEGDATA"), nil
	}
	m, err := e.g.AccionModulo(IDDescargas, "conectores.probar_fila", Args{"id": "foto"})
	if err != nil || !strings.Contains(m, "Funcionó") {
		t.Fatalf("probar: %q %v", m, err)
	}
	m, err = e.g.AccionModulo(IDDescargas, "conectores.probar", Args{"nombre": "t", "url": "https://miservidor.net/api", "salida": "imagen", "ruta": "foto"})
	if err != nil || !strings.Contains(m, "Funcionó") {
		t.Fatalf("probar con el formulario: %q %v", m, err)
	}
}

func TestConectorEnMenusComoVariable(t *testing.T) {
	e, d := descargasLista(t)
	d.resp = func(p PeticionHTTP) (*RespuestaHTTP, error) { return cuerpo("text/plain", "", "Dólar: 18.20"), nil }
	_, _ = e.g.AccionModulo(IDDescargas, "conectores.agregar", Args{"nombre": "dolar", "url": "https://miservidor.net/dolar", "salida": "texto"})
	if t1 := e.g.TextoConector("dolar", pnAna, pnAna); t1 != "Dólar: 18.20" {
		t.Fatalf("variable: %q", t1)
	}
	if e.g.TextoConector("no_existe", pnAna, pnAna) != "" {
		t.Fatal("conector desconocido = vacío, sin romper el menú")
	}
	d.resp = func(p PeticionHTTP) (*RespuestaHTTP, error) { return nil, errors.New("x") }
	if e.g.TextoConector("dolar", pnAna, pnAna) != "" {
		t.Fatal("si falla, vacío")
	}
	_ = e.g.Desactivar(IDDescargas)
	if e.g.TextoConector("dolar", pnAna, pnAna) != "" {
		t.Fatal("apagado, vacío")
	}
}

// ---------- el cliente HTTP real ----------

func TestClienteHTTPBloqueaRedesPrivadasYRespetaLimites(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/grande":
			w.Write(make([]byte, 5000))
		case "/redir":
			http.Redirect(w, r, "http://localhost:"+strings.Split(srv2Port(r), ":")[0]+"/ok", http.StatusFound)
		case "/lento":
			time.Sleep(600 * time.Millisecond)
			w.Write([]byte("tarde"))
		case "/cab":
			w.Write([]byte(r.Header.Get("X-Prueba") + "|" + r.Method))
		default:
			w.Write([]byte("hola"))
		}
	}))
	defer srv.Close()
	ctx := context.Background()
	var c ClienteHTTP
	// por defecto una dirección local no se alcanza
	_, err := c.Hacer(ctx, PeticionHTTP{URL: srv.URL + "/ok"})
	if !errors.Is(err, ErrRedPrivada) {
		t.Fatalf("debía bloquearse: %v", err)
	}
	_, err = c.Hacer(ctx, PeticionHTTP{URL: "file:///etc/passwd"})
	if !errors.Is(err, ErrEsquema) {
		t.Fatalf("esquema: %v", err)
	}
	_, err = c.Hacer(ctx, PeticionHTTP{URL: srv.URL, Permitido: func(string) bool { return false }, RedLocal: true})
	if !errors.Is(err, ErrDominio) {
		t.Fatalf("dominio: %v", err)
	}
	r, err := c.Hacer(ctx, PeticionHTTP{URL: srv.URL + "/ok", RedLocal: true})
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(r.Cuerpo)
	r.Cuerpo.Close()
	if string(b) != "hola" || r.Estado != 200 {
		t.Fatalf("respuesta: %q", b)
	}
	// tamaño máximo
	r, err = c.Hacer(ctx, PeticionHTTP{URL: srv.URL + "/grande", RedLocal: true, Max: 1000})
	if !errors.Is(err, ErrMuyGrande) {
		if err == nil {
			_, err = io.ReadAll(r.Cuerpo)
		}
		if !errors.Is(err, ErrMuyGrande) {
			t.Fatalf("tamaño: %v", err)
		}
	}
	// tiempo
	_, err = c.Hacer(ctx, PeticionHTTP{URL: srv.URL + "/lento", RedLocal: true, Timeout: 100 * time.Millisecond})
	if err == nil {
		t.Fatal("el tiempo máximo debe cortar")
	}
	// método y cabeceras
	r, err = c.Hacer(ctx, PeticionHTTP{URL: srv.URL + "/cab", RedLocal: true, Metodo: "POST", Cabeceras: map[string]string{"X-Prueba": "v"}, Cuerpo: []byte("{}")})
	if err != nil {
		t.Fatal(err)
	}
	b, _ = io.ReadAll(r.Cuerpo)
	if string(b) != "v|POST" {
		t.Fatalf("cabeceras: %q", b)
	}
}

func srv2Port(r *http.Request) string { return strings.Split(r.Host, ":")[1] }

func TestClienteHTTPRedireccionAOtroDominioSeRevisa(t *testing.T) {
	var puerto string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/redir" {
			http.Redirect(w, r, "http://localhost:"+puerto+"/ok", http.StatusFound)
			return
		}
		w.Write([]byte("hola"))
	}))
	defer srv.Close()
	puerto = strings.Split(strings.TrimPrefix(srv.URL, "http://"), ":")[1]
	var c ClienteHTTP
	solo127 := func(h string) bool { return h == "127.0.0.1" }
	_, err := c.Hacer(context.Background(), PeticionHTTP{URL: srv.URL + "/redir", RedLocal: true, Permitido: solo127})
	if !errors.Is(err, ErrDominio) {
		t.Fatalf("la redirección a otro dominio se bloquea: %v", err)
	}
	r, err := c.Hacer(context.Background(), PeticionHTTP{URL: srv.URL + "/redir", RedLocal: true, Permitido: func(h string) bool { return h == "127.0.0.1" || h == "localhost" }})
	if err != nil {
		t.Fatal(err)
	}
	r.Cuerpo.Close()
}

func TestEsDireccionPrivada(t *testing.T) {
	for ip, esperado := range map[string]bool{"127.0.0.1": true, "10.0.0.5": true, "192.168.1.10": true, "172.16.0.1": true, "169.254.169.254": true,
		"100.64.0.1": true, "0.0.0.0": true, "::1": true, "fe80::1": true, "8.8.8.8": false, "1.1.1.1": false, "2606:4700::1111": false} {
		if got := esDireccionPrivada(parseIP(ip)); got != esperado {
			t.Errorf("%s: %v", ip, got)
		}
	}
}
