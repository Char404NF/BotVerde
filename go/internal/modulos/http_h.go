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
	URL       string
	Llave     string
	Tiempo    time.Duration
	RedLocal  bool
	Formato   string // "" u "openai" (Bearer, /chat/completions) o "anthropic" (x-api-key, /v1/messages)
	Proveedor string
}

func (g *Gestor) postJSON(ctx context.Context, d destinoJSON, cuerpo any, salida any) error {
	u, err := url.Parse(strings.TrimSpace(d.URL))
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return errors.New("la dirección del servidor no es válida (debe empezar con http:// o https://)")
	}
	b, _ := json.Marshal(cuerpo)
	cab := map[string]string{"Content-Type": "application/json", "Accept": "application/json"}
	if d.Llave != "" {
		if d.Formato == formatoAnthropic {
			cab["x-api-key"] = d.Llave
		} else {
			cab["Authorization"] = "Bearer " + d.Llave
		}
	}
	if d.Formato == formatoAnthropic {
		cab["anthropic-version"] = versionAnthropic
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

const (
	formatoOpenAI    = "openai"
	formatoAnthropic = "anthropic"
	versionAnthropic = "2023-06-01"
)

// chatIA pide una respuesta al servicio elegido: compatible con OpenAI (/chat/completions) o Claude (/v1/messages).
func (g *Gestor) chatIA(ctx context.Context, d destinoJSON, modelo string, maxTokens int, msgs []MensajeIA) (string, error) {
	if d.Formato == formatoAnthropic {
		return g.chatAnthropic(ctx, d, modelo, maxTokens, msgs)
	}
	base := strings.TrimRight(strings.TrimSpace(d.URL), "/")
	if !strings.HasSuffix(base, "/chat/completions") {
		base += "/chat/completions"
	}
	d.URL = base
	req := map[string]any{"model": modelo, "messages": msgs}
	if maxTokens > 0 {
		// OpenAI renombró el parámetro (los modelos nuevos rechazan max_tokens); el resto de servicios siguen con el nombre de siempre.
		if d.Proveedor == "openai" {
			req["max_completion_tokens"] = maxTokens
		} else {
			req["max_tokens"] = maxTokens
		}
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

// chatAnthropic usa la API de Mensajes de Claude: la instrucción va aparte ("system"), max_tokens es obligatorio y la respuesta viene en
// bloques de texto.
func (g *Gestor) chatAnthropic(ctx context.Context, d destinoJSON, modelo string, maxTokens int, msgs []MensajeIA) (string, error) {
	base := strings.TrimRight(strings.TrimSpace(d.URL), "/")
	if !strings.HasSuffix(base, "/messages") {
		if !strings.HasSuffix(base, "/v1") {
			base += "/v1"
		}
		base += "/messages"
	}
	d.URL = base
	if maxTokens <= 0 {
		maxTokens = 1024
	}
	var sis []string
	var conv []MensajeIA
	for _, m := range msgs {
		if m.Rol == "system" {
			sis = append(sis, m.Texto)
			continue
		}
		conv = append(conv, m)
	}
	req := map[string]any{"model": modelo, "max_tokens": maxTokens, "messages": conv}
	if len(sis) > 0 {
		req["system"] = strings.Join(sis, "\n")
	}
	var out struct {
		Contenido []struct {
			Tipo  string `json:"type"`
			Texto string `json:"text"`
		} `json:"content"`
	}
	if err := g.postJSON(ctx, d, req, &out); err != nil {
		return "", err
	}
	var sb strings.Builder
	for _, c := range out.Contenido {
		if c.Tipo == "text" {
			sb.WriteString(c.Texto)
		}
	}
	if strings.TrimSpace(sb.String()) == "" {
		return "", errors.New("el modelo no devolvió respuesta")
	}
	return strings.TrimSpace(sb.String()), nil
}
