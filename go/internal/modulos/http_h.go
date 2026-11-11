package modulos

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"
)

// Cliente JSON común de H6 (traductor) y H7 (IA). Usa el mismo descargador de H5: sin proxys, sin redes privadas (salvo que el dueño lo
// permita), sin seguir redirecciones a otros dominios, con tiempo y tamaño máximos. La llave va en la cabecera Authorization y nunca se
// escribe en ningún registro ni mensaje de error.

// destinoJSON es a dónde y cómo se hace la llamada.
type destinoJSON struct {
	URL      string
	Llave    string
	Tiempo   time.Duration
	RedLocal bool
}

func (g *Gestor) postJSON(ctx context.Context, d destinoJSON, cuerpo any, salida any) error {
	u, err := url.Parse(strings.TrimSpace(d.URL))
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return errors.New("la dirección del servidor no es válida (debe empezar con http:// o https://)")
	}
	b, _ := json.Marshal(cuerpo)
	cab := map[string]string{"Content-Type": "application/json", "Accept": "application/json"}
	if d.Llave != "" {
		cab["Authorization"] = "Bearer " + d.Llave
	}
	host := strings.ToLower(u.Hostname())
	resp, err := g.descargador().Hacer(ctx, PeticionHTTP{Metodo: "POST", URL: u.String(), Cabeceras: cab, Cuerpo: b, Timeout: d.Tiempo, Max: 1 << 20,
		RedLocal: d.RedLocal, Permitido: func(h string) bool { return strings.EqualFold(h, host) }})
	if err != nil {
		return explicarHTTP(err, 1<<20)
	}
	defer resp.Cuerpo.Close()
	datos, err := io.ReadAll(resp.Cuerpo)
	if err != nil {
		return explicarHTTP(err, 1<<20)
	}
	if resp.Estado < 200 || resp.Estado > 299 {
		var e struct {
			Error any `json:"error"`
		}
		msg := ""
		if json.Unmarshal(datos, &e) == nil {
			switch x := e.Error.(type) {
			case string:
				msg = x
			case map[string]any:
				msg, _ = x["message"].(string)
			}
		}
		if len([]rune(msg)) > 140 {
			msg = string([]rune(msg)[:140]) + "…"
		}
		switch resp.Estado {
		case 401, 403:
			return errors.New("el servidor rechazó la llave (código " + fmt.Sprint(resp.Estado) + "): revísala")
		case 429:
			return errors.New("el servidor dice que hay demasiadas peticiones o se acabó el saldo (429)")
		}
		if msg != "" {
			return fmt.Errorf("el servidor respondió con el código %d: %s", resp.Estado, msg)
		}
		return fmt.Errorf("el servidor respondió con el código %d", resp.Estado)
	}
	if err := json.NewDecoder(bytes.NewReader(datos)).Decode(salida); err != nil {
		return errors.New("el servidor respondió algo que no entendí")
	}
	return nil
}

// MensajeIA es un mensaje de una conversación con un modelo compatible con OpenAI.
type MensajeIA struct {
	Rol   string `json:"role"`
	Texto string `json:"content"`
}

// chatIA pide una respuesta a un endpoint compatible con OpenAI (/chat/completions).
func (g *Gestor) chatIA(ctx context.Context, d destinoJSON, modelo string, maxTokens int, msgs []MensajeIA) (string, error) {
	base := strings.TrimRight(strings.TrimSpace(d.URL), "/")
	if !strings.HasSuffix(base, "/chat/completions") {
		base += "/chat/completions"
	}
	d.URL = base
	req := map[string]any{"model": modelo, "messages": msgs, "temperature": 0.7}
	if maxTokens > 0 {
		req["max_tokens"] = maxTokens
	}
	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := g.postJSON(ctx, d, req, &out); err != nil {
		return "", err
	}
	if len(out.Choices) == 0 || strings.TrimSpace(out.Choices[0].Message.Content) == "" {
		return "", errors.New("el modelo no devolvió respuesta")
	}
	return strings.TrimSpace(out.Choices[0].Message.Content), nil
}
