package wabot

import (
	"context"
	"strings"
	"testing"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/types"
	waLog "go.mau.fi/whatsmeow/util/log"

	"botchat/internal/almacen"
	"botchat/internal/envio"
	"botchat/internal/estilo"
	"botchat/internal/lab"
	"botchat/internal/motor"
)

func avatarEntorno(t *testing.T) (enviadorMotor, *clienteFalso, *almacen.Almacen) {
	_, a := abreSolo(t)
	cli := &clienteFalso{}
	s := &estado{botID: 1, l: &falso{}}
	s.envio = envio.Nuevo(1, a, cli, nil)
	pn := types.NewJID("5215500000000", types.DefaultUserServer)
	s.lab = lab.Nuevo(1, a, lab.Real{Client: whatsmeow.NewClient(&store.Device{ID: &pn}, waLog.Noop)}, s.envio, nil)
	_, _ = a.ActualizarChat(1, avPriv, false, func(c *almacen.Chat) { c.Publico = "Ana" })
	_, _ = a.ActualizarChat(1, avGrupo, true, func(c *almacen.Chat) { c.Publico = "Grupo" })
	return enviadorMotor{s}, cli, a
}

const (
	avPriv  = "5215511111111@s.whatsapp.net"
	avGrupo = "120363433659966902@g.us"
)

func TestAvatarUnTextoSalePorImagenConElTextoDePie(t *testing.T) {
	e, cli, _ := avatarEntorno(t)
	ctx := context.Background()
	if _, err := e.EnviarSalida(ctx, avPriv, motor.Salida{Texto: "🤖 *Esto puedes usar*", Avatar: "ayuda"}, "", "", ""); err != nil {
		t.Fatal(err)
	}
	img := cli.enviados[0].GetImageMessage()
	if img == nil || img.GetCaption() != "🤖 *Esto puedes usar*" {
		t.Fatalf("debía ser imagen con pie de foto: %+v", cli.enviados[0])
	}
	// sin momento no hay imagen
	if _, err := e.EnviarSalida(ctx, avPriv, motor.Salida{Texto: "hola"}, "", "", ""); err != nil || cli.enviados[1].GetConversation() != "hola" {
		t.Fatalf("sin avatar sale texto: %v %+v", err, cli.enviados[1])
	}
	// un texto largo que no es tarjeta no lleva imagen (el pie de foto tiene tope)
	largo := strings.Repeat("a", 901)
	if _, err := e.EnviarSalida(ctx, avPriv, motor.Salida{Texto: largo, Avatar: "ayuda"}, "", "", ""); err != nil || cli.enviados[2].GetConversation() != largo {
		t.Fatalf("largo: %v", err)
	}
	// una tarjeta larga (la lista de comandos): la imagen con el título y debajo el resto, sin repetir el título
	tarjeta := "🤖 *Esto puedes usar*\n━━━━━━━━━━━━\n" + strings.Repeat("• *!algo* — algo\n", 80)
	if _, err := e.EnviarSalida(ctx, avPriv, motor.Salida{Texto: tarjeta, Avatar: "ayuda"}, "", "", ""); err != nil {
		t.Fatal(err)
	}
	if im := cli.enviados[3].GetImageMessage(); im == nil || im.GetCaption() != "🤖 *Esto puedes usar*" || strings.Contains(cli.enviados[4].GetConversation(), "Esto puedes usar") || !strings.HasPrefix(cli.enviados[4].GetConversation(), "• *!algo*") {
		t.Fatalf("tarjeta larga: %+v / %+v", cli.enviados[3], cli.enviados[4])
	}
	cli.enviados = cli.enviados[:3]
	// en un grupo la ayuda sale sin imagen; el menú sí lleva
	if _, err := e.EnviarSalida(ctx, avGrupo, motor.Salida{Texto: "ayuda", Avatar: "ayuda"}, "", "", ""); err != nil || cli.enviados[3].GetConversation() != "ayuda" {
		t.Fatalf("grupo ayuda: %v", err)
	}
	if _, err := e.EnviarSalida(ctx, avGrupo, motor.Salida{Texto: "1. Uno\n2. Dos", Avatar: "menu", Menu: &motor.MenuLab{Titulo: "Menú", Opciones: []motor.OpcionMenu{{Num: 1, Texto: "Uno"}, {Num: 2, Texto: "Dos"}}}}, "", "", ""); err != nil {
		t.Fatal(err)
	}
	if im := cli.enviados[4].GetImageMessage(); im == nil || im.GetCaption() != "1. Uno\n2. Dos" {
		t.Fatalf("el menú de grupo va con la imagen y el menú numerado de pie: %+v", cli.enviados[4])
	}
}

func TestAvatarElMenuConBotonesSaleDespuesDeLaImagenSinRepetirse(t *testing.T) {
	e, cli, _ := avatarEntorno(t)
	sal := motor.Salida{Texto: "¿Qué quieres?\n1. A\n2. B", Avatar: "menu", Menu: &motor.MenuLab{Persona: avPriv, Titulo: "¿Qué quieres?", Opciones: []motor.OpcionMenu{{Num: 1, Texto: "A"}, {Num: 2, Texto: "B"}}}}
	if _, err := e.EnviarSalida(context.Background(), avPriv, sal, "", "", ""); err != nil {
		t.Fatal(err)
	}
	if len(cli.enviados) != 2 {
		t.Fatalf("imagen y botones: %d", len(cli.enviados))
	}
	if im := cli.enviados[0].GetImageMessage(); im == nil || im.GetCaption() != "¿Qué quieres?" {
		t.Fatalf("primero la imagen con el título: %+v", cli.enviados[0])
	}
	nf := cli.enviados[1].GetInteractiveMessage()
	if nf == nil || strings.Contains(nf.GetBody().GetText(), "¿Qué quieres?") || !strings.Contains(nf.GetBody().GetText(), "Elige") {
		t.Fatalf("debajo salen los botones sin repetir el título: %+v", cli.enviados[1])
	}
}

func TestAvatarSeApagaYSeEligenLosMomentos(t *testing.T) {
	e, cli, a := avatarEntorno(t)
	ctx := context.Background()
	if err := AvatarConfigurar(1, false, "menu,ayuda"); err != nil {
		t.Fatal(err)
	}
	_, _ = e.EnviarSalida(ctx, avPriv, motor.Salida{Texto: "x", Avatar: "ayuda"}, "", "", "")
	if cli.enviados[0].GetConversation() != "x" {
		t.Fatalf("apagado: %+v", cli.enviados[0])
	}
	_ = AvatarConfigurar(1, true, "menu,inventado")
	_, _ = e.EnviarSalida(ctx, avPriv, motor.Salida{Texto: "y", Avatar: "ayuda"}, "", "", "")
	if cli.enviados[1].GetConversation() != "y" {
		t.Fatalf("ayuda no está elegida: %+v", cli.enviados[1])
	}
	est := decodifica[avatarEstado](t, func() (string, error) { return AvatarEstado(1) })
	activos := 0
	for _, m := range est.Momentos {
		if m.Activo {
			activos++
		}
	}
	if !est.Activo || activos != 1 || est.Propio || len(est.Caras) != 8 {
		t.Fatalf("estado: %+v", est)
	}
	// imagen propia: se usa en lugar de la cara y se puede quitar
	id, _, err := a.GuardarAdjuntoMenu(1, strings.NewReader(string(estilo.Cara("guino"))))
	if err != nil {
		t.Fatal(err)
	}
	if err := AvatarPropio(1, id, "text/plain"); err == nil {
		t.Fatal("solo imágenes")
	}
	if err := AvatarPropio(1, "../x", "image/png"); err == nil {
		t.Fatal("id no válido")
	}
	if err := AvatarPropio(1, id, "image/png"); err != nil {
		t.Fatal(err)
	}
	_ = AvatarConfigurar(1, true, "ayuda")
	_, _ = e.EnviarSalida(ctx, avPriv, motor.Salida{Texto: "z", Avatar: "ayuda"}, "", "", "")
	if cli.enviados[2].GetImageMessage() == nil {
		t.Fatalf("con imagen propia: %+v", cli.enviados[2])
	}
	if est = decodifica[avatarEstado](t, func() (string, error) { return AvatarEstado(1) }); !est.Propio || est.PropioID != id {
		t.Fatalf("propio: %+v", est)
	}
	if err := AvatarPropio(1, "", ""); err != nil || a.ExisteAdjunto(1, id) {
		t.Fatalf("al volver a las caras se borra la imagen propia: %v", err)
	}
}
