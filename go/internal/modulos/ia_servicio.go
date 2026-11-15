package modulos

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

// ModeloIA es un modelo que se sugiere de un proveedor.
type ModeloIA struct {
	ID     string `json:"id"`
	Nombre string `json:"nombre"`
	Nota   string `json:"nota,omitempty"` // "rápido y barato", "el más capaz"…
}

// ProveedorIA es un servicio de IA que se puede elegir con un toque: dirección, formato y modelos ya puestos.
type ProveedorIA struct {
	ID       string     `json:"id"`
	Nombre   string     `json:"nombre"`
	Detalle  string     `json:"detalle"`
	URL      string     `json:"url"`
	Formato  string     `json:"formato"` // openai | anthropic
	Modelos  []ModeloIA `json:"modelos"`
	Defecto  string     `json:"defecto"`
	UrlLlave string     `json:"url_llave,omitempty"` // dónde se crea la llave
	Gratis   bool       `json:"gratis,omitempty"`
	SinLlave bool       `json:"sin_llave,omitempty"` // Ollama
	Local    bool       `json:"local,omitempty"`     // red de casa
	Color    string     `json:"color"`
	Inicial  string     `json:"inicial"`
	Icono    string     `json:"icono,omitempty"` // nombre del recurso del logo
	Pista    string     `json:"pista,omitempty"` // cómo se ve la llave: "sk-ant-…"
}

// ProveedoresIA devuelve el catálogo (JSON).
func ProveedoresIA() string {
	b, _ := json.Marshal(proveedoresIA)
	return string(b)
}

func proveedorIA(id string) (ProveedorIA, bool) {
	for _, p := range proveedoresIA {
		if p.ID == id {
			return p, true
		}
	}
	return ProveedorIA{}, false
}

// destinoIA arma a dónde y cómo hablarle al servicio según la configuración.
func (g *Gestor) destinoIA(cfg Config) destinoJSON {
	prov := cfg.Texto("proveedor")
	d := destinoJSON{URL: cfg.Texto("url"), Llave: g.Llave(IDIA, "llave"), Tiempo: time.Duration(cfg.Int("tiempo_s")) * time.Second,
		RedLocal: cfg.Bool("red_local"), Formato: formatoOpenAI, Proveedor: prov}
	if p, ok := proveedorIA(prov); ok {
		d.Formato = p.Formato
		if p.Local {
			d.RedLocal = true
		}
	}
	return d
}

// ProbarIA hace una consulta corta para ver si el servicio, el modelo y la llave funcionan. No cuenta como consulta de nadie.
func (g *Gestor) ProbarIA() (string, error) {
	cfg := g.Config(IDIA)
	if strings.TrimSpace(cfg.Texto("url")) == "" || strings.TrimSpace(cfg.Texto("modelo")) == "" {
		return "", errors.New("elige un servicio y un modelo primero")
	}
	p, ok := proveedorIA(cfg.Texto("proveedor"))
	if !(ok && p.SinLlave) && !TieneLlave(g.BotID, IDIA, "llave") {
		return "", errors.New("falta guardar la llave del servicio")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.Int("tiempo_s")+5)*time.Second)
	defer cancel()
	r, err := g.chatIA(ctx, g.destinoIA(cfg), cfg.Texto("modelo"), 40, []MensajeIA{
		{"system", "Responde con una sola frase corta en español."}, {"user", "Di que la conexión funciona."}})
	if err != nil {
		return "", err
	}
	return corta(r, 160), nil
}
