package modulos

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"botchat/internal/almacen"
)

// ---------- apoyo común de las pruebas H ----------

type accFalsa struct {
	menus       []string
	mu          sync.Mutex
	admin       bool
	expulsados  []string
	agregados   []string
	resAgregar  string
	errAgregar  error
	enlace      string
	errEnlace   error
	errExpulsar error
	rechazadas  []string
	errRechazar error
	encuestas   []string
	textos      []string
	fijados     []string
	borrados    []string
	botones     []string
	conBotones  bool
}

func (a *accFalsa) EsAdmin(_ context.Context, _ string) (bool, error) { return a.admin, nil }
func (a *accFalsa) Expulsar(_ context.Context, chat, p string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.errExpulsar != nil {
		return a.errExpulsar
	}
	a.expulsados = append(a.expulsados, chat+"|"+p)
	return nil
}
func (a *accFalsa) EnviarMenu(_ context.Context, chat, persona, titulo, texto string, ops []string, citaID string) error {
	a.menus = append(a.menus, texto)
	return nil
}
func (a *accFalsa) RechazarLlamada(_ context.Context, de, id string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.errRechazar != nil {
		return a.errRechazar
	}
	a.rechazadas = append(a.rechazadas, de+"|"+id)
	return nil
}
func (a *accFalsa) Enlace(_ context.Context, chat string) (string, error) {
	return a.enlace, a.errEnlace
}
func (a *accFalsa) Agregar(_ context.Context, chat, p string) (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.errAgregar != nil {
		return "", a.errAgregar
	}
	a.agregados = append(a.agregados, chat+"|"+p)
	if a.resAgregar == "" {
		return AgregOK, nil
	}
	return a.resAgregar, nil
}

func (a *accFalsa) EnviarEncuesta(_ context.Context, chat, titulo string, ops []string, multiple bool) (string, []byte, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.encuestas = append(a.encuestas, chat+"|"+titulo+"|"+strings.Join(ops, ","))
	id := "POLL" + strconv.Itoa(len(a.encuestas))
	return id, []byte("secreto-" + id), nil
}
func (a *accFalsa) EnviarTextoID(_ context.Context, chat, texto string, menc []string) (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.textos = append(a.textos, chat+"|"+texto)
	return "TXT" + strconv.Itoa(len(a.textos)), nil
}
func (a *accFalsa) FijarMensaje(_ context.Context, chat, id string, dias int, fijar bool) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.fijados = append(a.fijados, fmt.Sprintf("%s|%s|%d|%v", chat, id, dias, fijar))
	return nil
}
func (a *accFalsa) EnviarBotones(_ context.Context, chat, persona, texto string, bs []BotonCmd) (bool, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.conBotones {
		return false, nil
	}
	var t []string
	for _, b := range bs {
		t = append(t, b.Texto+"="+b.Comando)
	}
	a.botones = append(a.botones, chat+"|"+texto+"|"+strings.Join(t, ","))
	return true, nil
}
func (a *accFalsa) BorrarMensaje(_ context.Context, chat, id string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.borrados = append(a.borrados, chat+"|"+id)
	return nil
}

// entH arma el bot de prueba con los módulos H registrados (apagados).
func entH(t *testing.T) (*ent, *accFalsa) {
	t.Helper()
	e := nuevoEnt(t)
	e.g.SinPausaArchivos = true
	if err := RegistrarH(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(QuitarH)
	acc := &accFalsa{admin: true, enlace: "https://chat.whatsapp.com/ABCDEF"}
	e.g.Acc = acc
	return e, acc
}

func (e *ent) activa(id string) {
	e.t.Helper()
	if err := e.g.Activar(id); err != nil {
		e.t.Fatal(err)
	}
	e.fija("maestro", "1")
}

func (e *ent) config(id string, cambios map[string]any) {
	e.t.Helper()
	c := map[string]any{}
	for k, v := range e.g.Config(id) {
		c[k] = v
	}
	for k, v := range cambios {
		c[k] = v
	}
	b, _ := jsonDe(c)
	if err := e.g.GuardarConfig(id, b); err != nil {
		e.t.Fatal(err)
	}
}

func (e *ent) conChat(jid string, esGrupo bool, nombre string) {
	e.t.Helper()
	if _, err := e.alm.ActualizarChat(1, jid, esGrupo, func(c *almacen.Chat) { c.Guardado = nombre }); err != nil {
		e.t.Fatal(err)
	}
}

func (e *ent) enviados() []string {
	e.env.mu.Lock()
	defer e.env.mu.Unlock()
	return append([]string(nil), e.env.enviados...)
}

func (e *ent) hay(sub string) bool {
	for _, x := range e.enviados() {
		if strings.Contains(x, sub) {
			return true
		}
	}
	return false
}

func (e *ent) tick() { e.g.Evento(Evento{Tipo: EvTick}) }

func esperaErr(t *testing.T, err error, sub string) {
	t.Helper()
	if err == nil || !strings.Contains(err.Error(), sub) {
		t.Fatalf("se esperaba un error con %q, llegó %v", sub, err)
	}
}

var _ = errors.New
var _ = time.Second

func jsonDe(v any) (string, error) {
	b, err := jsonMarshal(v)
	return string(b), err
}
