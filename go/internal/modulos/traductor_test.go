package modulos

import (
	"encoding/json"
	"strings"
	"testing"

	"botchat/internal/almacen"
	"botchat/internal/comandos"
)

func tradLista(t *testing.T) (*ent, *descFalso) {
	e, _ := entH(t)
	d := &descFalso{}
	e.g.Descarga = d
	e.activa(IDTraductor)
	e.config(IDTraductor, map[string]any{"url": "https://trad.ejemplo.com", "acepto_privacidad": true})
	return e, d
}

func cmdTrad(t *testing.T, e *ent) comandos.Comando {
	t.Helper()
	m, _ := Obtener(IDTraductor)
	return m.(ComandosDeGestor).ComandosG(e.g)[0]
}

func guardaMsg(t *testing.T, e *ent, chat, id, texto string) {
	t.Helper()
	if !e.g.hayChat(chat) {
		e.conChat(chat, strings.HasSuffix(chat, "@g.us"), "X")
	}
	if _, err := e.alm.GuardarMensaje(&almacen.Mensaje{ID: id, ChatJID: chat, BotID: 1, RemitenteJID: pnAna, Tipo: almacen.TipoTexto, Texto: texto, TS: e.hora.Unix()}); err != nil {
		t.Fatal(err)
	}
}

func TestTraductorNaceApagadoYBajoRiesgo(t *testing.T) {
	e, _ := entH(t)
	i, _ := e.g.InfoDe(IDTraductor)
	if i.Activo || i.Iniciativa || i.Riesgo != RiesgoBajo {
		t.Fatalf("info: %+v", i)
	}
	if !strings.Contains(i.Descripcion, "SALE de tu teléfono") {
		t.Fatal("falta el aviso de privacidad")
	}
}

func TestTraducirCitandoConLibreTranslate(t *testing.T) {
	e, d := tradLista(t)
	FijarLlave(1, IDTraductor, "llave", "SECRETA-123")
	t.Cleanup(func() { FijarLlave(1, IDTraductor, "llave", "") })
	d.resp = func(p PeticionHTTP) (*RespuestaHTTP, error) {
		return cuerpo("application/json", "", `{"translatedText":"Hello, how are you?"}`), nil
	}
	guardaMsg(t, e, pnAna, "M1", "Hola, ¿cómo estás?")
	cx := &comandos.Contexto{Chat: pnAna, CitaID: "M1", Args: []string{"inglés"}, Resto: "inglés"}
	r, err := cmdTrad(t, e).Ejecutar(cx)
	if err != nil || !strings.Contains(r, "Hello, how are you?") {
		t.Fatalf("%q %v", r, err)
	}
	p := d.reqs[0]
	if !strings.HasSuffix(p.URL, "/translate") || p.Metodo != "POST" {
		t.Fatalf("petición: %+v", p)
	}
	var cu map[string]any
	_ = json.Unmarshal(p.Cuerpo, &cu)
	if cu["q"] != "Hola, ¿cómo estás?" || cu["target"] != "en" || cu["api_key"] != "SECRETA-123" {
		t.Fatalf("cuerpo: %v", cu)
	}
	// la llave nunca queda en la configuración, el registro ni el resultado
	cj, _ := e.g.ConfigJSON(IDTraductor)
	if strings.Contains(cj, "SECRETA-123") || strings.Contains(r, "SECRETA") {
		t.Fatal("la llave se coló")
	}
	for _, x := range e.registro() {
		if strings.Contains(x.Que+x.Resultado+x.Chat, "SECRETA") {
			t.Fatal("la llave llegó al registro")
		}
	}
}

func TestTraducirConEndpointDeIAYIdiomaPorDefecto(t *testing.T) {
	e, d := tradLista(t)
	e.config(IDTraductor, map[string]any{"servidor": "openai", "url": "https://api.ejemplo.com/v1", "modelo": "m1"})
	FijarLlave(1, IDTraductor, "llave", "K")
	t.Cleanup(func() { FijarLlave(1, IDTraductor, "llave", "") })
	d.resp = func(p PeticionHTTP) (*RespuestaHTTP, error) {
		return cuerpo("application/json", "", `{"choices":[{"message":{"content":"Hola amigo"}}]}`), nil
	}
	r, err := cmdTrad(t, e).Ejecutar(&comandos.Contexto{Chat: pnAna, Resto: "hello friend"})
	if err != nil || !strings.Contains(r, "Hola amigo") {
		t.Fatalf("%q %v", r, err)
	}
	p := d.reqs[0]
	if !strings.HasSuffix(p.URL, "/v1/chat/completions") || p.Cabeceras["Authorization"] != "Bearer K" {
		t.Fatalf("petición: %+v", p)
	}
	if !strings.Contains(string(p.Cuerpo), "español") {
		t.Fatalf("debe pedir el idioma por defecto: %s", p.Cuerpo)
	}
}

func TestTraducirPideAvisoUrlYValida(t *testing.T) {
	e, d := tradLista(t)
	e.config(IDTraductor, map[string]any{"acepto_privacidad": false})
	_, err := cmdTrad(t, e).Ejecutar(&comandos.Contexto{Chat: pnAna, Resto: "en hola"})
	esperaErr(t, err, "aviso de privacidad")
	e.config(IDTraductor, map[string]any{"acepto_privacidad": true, "url": ""})
	_, err = cmdTrad(t, e).Ejecutar(&comandos.Contexto{Chat: pnAna, Resto: "en hola"})
	esperaErr(t, err, "servidor")
	e.config(IDTraductor, map[string]any{"url": "https://trad.ejemplo.com"})
	_, err = cmdTrad(t, e).Ejecutar(&comandos.Contexto{Chat: pnAna, CitaID: "NO", Args: []string{"klingon"}, Resto: "klingon"})
	esperaErr(t, err, "idioma")
	_, err = cmdTrad(t, e).Ejecutar(&comandos.Contexto{Chat: pnAna, CitaID: "NO", Args: []string{"en"}, Resto: "en"})
	esperaErr(t, err, "no tiene texto")
	_, err = cmdTrad(t, e).Ejecutar(&comandos.Contexto{Chat: pnAna, Args: []string{"en"}, Resto: "en"})
	esperaErr(t, err, "cita el mensaje")
	if len(d.reqs) != 0 {
		t.Fatal("no debió salir nada")
	}
	// una llave en la configuración se rechaza
	b, _ := jsonDe(map[string]any{"llave": "x"})
	esperaErr(t, e.g.GuardarConfig(IDTraductor, b), "Keystore")
}

func TestTraducirLimitesYErroresDelServidor(t *testing.T) {
	e, d := tradLista(t)
	e.config(IDTraductor, map[string]any{"usos_hora": 1.0, "max_chars": 20.0})
	d.resp = func(p PeticionHTTP) (*RespuestaHTTP, error) {
		return &RespuestaHTTP{Estado: 403, Mime: "application/json", Cuerpo: cuerpo("", "", `{"error":"x"}`).Cuerpo}, nil
	}
	_, err := cmdTrad(t, e).Ejecutar(&comandos.Contexto{Chat: pnAna, Resto: "en esto es un texto demasiado largo para el tope"})
	esperaErr(t, err, "muy largo")
	_, err = cmdTrad(t, e).Ejecutar(&comandos.Contexto{Chat: pnAna, Resto: "en hola"})
	esperaErr(t, err, "rechazó la llave")
	_, err = cmdTrad(t, e).Ejecutar(&comandos.Contexto{Chat: pnAna, Resto: "en hola"})
	esperaErr(t, err, "muchas veces")
}

func TestTraducirNoLlegaARedPrivadaPorDefecto(t *testing.T) {
	e, d := tradLista(t)
	d.resp = func(p PeticionHTTP) (*RespuestaHTTP, error) {
		return cuerpo("application/json", "", `{"translatedText":"x"}`), nil
	}
	_, _ = cmdTrad(t, e).Ejecutar(&comandos.Contexto{Chat: pnAna, Resto: "en hola"})
	if d.reqs[0].RedLocal {
		t.Fatal("red local debe estar apagada")
	}
	e.config(IDTraductor, map[string]any{"red_local": true})
	_, _ = cmdTrad(t, e).Ejecutar(&comandos.Contexto{Chat: pnAna, Resto: "en hola"})
	if !d.reqs[1].RedLocal {
		t.Fatal("red local debía pasar")
	}
}
