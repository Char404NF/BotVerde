package modulos

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"botchat/internal/comandos"
	"botchat/internal/permisos"
)

// ---------- H6 · Traductor ----------
//
// "!traducir <idioma>" citando un mensaje (o "!traducir <idioma> <texto>"). El servidor lo eliges tú: un LibreTranslate propio o un
// endpoint compatible con OpenAI. PRIVACIDAD: el texto a traducir SALE del teléfono hacia ese servidor. Es una respuesta a un comando:
// no escribe por iniciativa.

// IDTraductor es el id del módulo.
const IDTraductor = "traductor"

type idioma struct {
	codigo, nombre string
	alias          []string
}

var idiomas = []idioma{
	{"es", "español", []string{"espanol", "castellano", "spanish"}}, {"en", "inglés", []string{"ingles", "english"}},
	{"fr", "francés", []string{"frances", "french"}}, {"de", "alemán", []string{"aleman", "german"}},
	{"it", "italiano", []string{"italian"}}, {"pt", "portugués", []string{"portugues", "portuguese"}},
	{"ru", "ruso", []string{"russian"}}, {"zh", "chino", []string{"chinese", "mandarin"}},
	{"ja", "japonés", []string{"japones", "japanese"}}, {"ko", "coreano", []string{"korean"}},
	{"ar", "árabe", []string{"arabe", "arabic"}}, {"hi", "hindi", nil}, {"nl", "neerlandés", []string{"holandes", "neerlandes", "dutch"}},
	{"pl", "polaco", []string{"polish"}}, {"tr", "turco", []string{"turkish"}}, {"sv", "sueco", []string{"swedish"}},
	{"ca", "catalán", []string{"catalan"}}, {"uk", "ucraniano", []string{"ukrainian"}}, {"el", "griego", []string{"greek"}},
	{"he", "hebreo", []string{"hebrew"}}, {"id", "indonesio", []string{"indonesian"}}, {"vi", "vietnamita", []string{"vietnamese"}},
}

// buscaIdioma reconoce un código ("en") o un nombre en español o inglés ("inglés", "english").
func buscaIdioma(s string) (idioma, bool) {
	k := llave(s)
	if k == "" {
		return idioma{}, false
	}
	for _, i := range idiomas {
		if k == i.codigo || k == llave(i.nombre) {
			return i, true
		}
		for _, a := range i.alias {
			if k == a {
				return i, true
			}
		}
	}
	return idioma{}, false
}

func traductor() Modulo {
	return Nuevo(Def{
		ID: IDTraductor, Nombre: "Traductor", Icono: "traductor", Riesgo: RiesgoBajo, Iniciativa: false,
		Descripcion: "«!traducir inglés» citando un mensaje lo traduce. Usa el servidor que elijas (LibreTranslate propio o un endpoint compatible con IA).",
		Casillas: []permisos.Casilla{
			{ID: "traductor.usar", Nombre: "Pedir «!traducir»", Descripcion: "Puede pedir traducciones. El texto sale del teléfono hacia el servidor de traducción.", Grupo: "Traductor", Riesgo: permisos.RiesgoBajo},
		},
		Esquema: []Campo{
			{Clave: "servidor", Titulo: "Tipo de servidor", Tipo: CampoLista, Defecto: "libretranslate",
				Opciones: []Opcion{{"libretranslate", "LibreTranslate"}, {"openai", "Endpoint compatible con IA (OpenAI)"}}},
			{Clave: "url", Titulo: "Dirección del servidor", Ayuda: "LibreTranslate: https://traductor.tuservidor.com  ·  IA: https://api.openai.com/v1", Tipo: CampoTexto, Largo: 300, Defecto: ""},
			{Clave: "llave", Titulo: "Llave (opcional)", Ayuda: "Se guarda en el Keystore de Android, nunca en la base ni en el repositorio.", Tipo: CampoSecreto},
			{Clave: "modelo", Titulo: "Modelo (solo IA)", Tipo: CampoTexto, Largo: 80, Defecto: "gpt-4o-mini"},
			{Clave: "idioma", Titulo: "Idioma por defecto", Ayuda: "Código o nombre (es, en, francés…). Se usa si no dicen a cuál.", Tipo: CampoTexto, Largo: 20, Defecto: "es"},
			{Clave: "max_chars", Titulo: "Máximo de letras por traducción", Tipo: CampoNumero, Defecto: 1000.0, Min: 20, Max: 4000},
			{Clave: "usos_hora", Titulo: "Traducciones por hora", Ayuda: "0 = sin límite.", Tipo: CampoNumero, Defecto: 30.0, Min: 0, Max: 1000},
			{Clave: "tiempo_s", Titulo: "Tiempo máximo (segundos)", Tipo: CampoNumero, Defecto: 20.0, Min: 3, Max: 120},
			{Clave: "red_local", Titulo: "Servidor en mi red de casa", Ayuda: "Permite direcciones privadas o locales (192.168…, localhost). Apagado, el bot no llega a ellas.", Tipo: CampoInterruptor, Defecto: false},
		},
		Validar: func(c map[string]any) error {
			cfg := Config(c)
			if _, ok := buscaIdioma(cfg.Texto("idioma")); !ok {
				return errors.New("no conozco el idioma por defecto; usa un código como es, en, fr o su nombre")
			}
			return nil
		},
		ComandosG: comandosTraductor,
		AlEstado: func(g *Gestor) string {
			cfg := g.Config(IDTraductor)
			switch {
			case strings.TrimSpace(cfg.Texto("url")) == "":
				return "Encendido, falta la dirección del servidor"
			}
			n := g.Alm.ContarRegistroModulo(g.BotID, IDTraductor, "traducir", ResEnviado, g.ahora().Add(-24*time.Hour))
			return fmt.Sprintf("Encendido · %d traducciones en 24 h", n)
		},
	})
}

// Traducir traduce un texto con el servidor configurado. Devuelve el texto traducido.
func (g *Gestor) Traducir(ctx context.Context, texto string, destino idioma) (string, error) {
	cfg := g.Config(IDTraductor)
	if strings.TrimSpace(cfg.Texto("url")) == "" {
		return "", errors.New("todavía no se configuró el servidor de traducción")
	}
	d := destinoJSON{URL: cfg.Texto("url"), Llave: g.Llave(IDTraductor, "llave"), Tiempo: time.Duration(cfg.Int("tiempo_s")) * time.Second, RedLocal: cfg.Bool("red_local")}
	if cfg.Texto("servidor") == "openai" {
		sis := "Eres un traductor. Traduce el texto del usuario al idioma " + destino.nombre + ". Responde SOLO con la traducción, sin comentarios ni comillas. Si no hay nada que traducir, devuelve el texto igual."
		return g.chatIA(ctx, d, cfg.Texto("modelo"), 0, []MensajeIA{{"system", sis}, {"user", texto}})
	}
	u := strings.TrimRight(strings.TrimSpace(d.URL), "/")
	if !strings.HasSuffix(u, "/translate") {
		u += "/translate"
	}
	d.URL = u
	req := map[string]any{"q": texto, "source": "auto", "target": destino.codigo, "format": "text"}
	if d.Llave != "" {
		req["api_key"] = d.Llave
	}
	var out struct {
		Texto string `json:"translatedText"`
		Error string `json:"error"`
	}
	if err := g.postJSON(ctx, d, req, &out); err != nil {
		return "", err
	}
	if out.Error != "" {
		return "", errors.New("el traductor respondió: " + out.Error)
	}
	if strings.TrimSpace(out.Texto) == "" {
		return "", errors.New("el traductor no devolvió texto")
	}
	return strings.TrimSpace(out.Texto), nil
}

func comandosTraductor(g *Gestor) []comandos.Comando {
	return []comandos.Comando{{
		Nombre: "traducir", Alias: []string{"traduce"}, Resumen: "Traducir un mensaje (cítalo)", Uso: "traducir <idioma>  (citando un mensaje)", Casilla: "traductor.usar",
		Ejecutar: func(cx *comandos.Contexto) (string, error) {
			cfg := g.Config(IDTraductor)
			dest, _ := buscaIdioma(cfg.Texto("idioma"))
			resto := strings.TrimSpace(cx.Resto)
			if len(cx.Args) > 0 {
				if i, ok := buscaIdioma(cx.Args[0]); ok {
					dest = i
					resto = strings.TrimSpace(strings.TrimPrefix(resto, cx.Args[0]))
				} else if cx.CitaID != "" {
					return "", errors.New("no conozco ese idioma; prueba con un código (en, fr, de…) o su nombre")
				}
			}
			texto := resto
			if cx.CitaID != "" && texto == "" {
				m, _ := g.Alm.ObtenerMensaje(g.BotID, cx.Chat, cx.CitaID)
				if m == nil || strings.TrimSpace(m.Texto) == "" {
					return "", errors.New("ese mensaje no tiene texto que traducir")
				}
				texto = m.Texto
			}
			if texto == "" {
				return "", errors.New("cita el mensaje que quieres traducir y escribe el idioma, por ejemplo: traducir inglés")
			}
			if len([]rune(texto)) > cfg.Int("max_chars") {
				return "", fmt.Errorf("el texto es muy largo (máximo %d letras)", cfg.Int("max_chars"))
			}
			if !g.limite("traducir", cfg.Int("usos_hora"), time.Hour) {
				return "", errors.New("ya traduje muchas veces en esta hora; intenta más tarde")
			}
			ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.Int("tiempo_s")+10)*time.Second)
			defer cancel()
			r, err := g.Traducir(ctx, texto, dest)
			if err != nil {
				g.Alm.RegistrarModulo(g.BotID, IDTraductor, "traducir", ResFallido, "", g.ahora())
				return "", err
			}
			g.Alm.RegistrarModulo(g.BotID, IDTraductor, "traducir", ResEnviado, "", g.ahora())
			return "🌐 " + dest.nombre + ":\n" + r, nil
		},
	}}
}
