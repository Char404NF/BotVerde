package lab

import (
	"context"
	"fmt"
	"time"

	"go.mau.fi/whatsmeow/proto/waE2E"

	"botchat/internal/almacen"
	"botchat/internal/envio"
)

// Prueba es una prueba de la matriz que se puede mandar: el tipo y cuántas opciones lleva.
type Prueba struct {
	Tipo     string `json:"tipo"`
	Variante int    `json:"variante"`
	Nombre   string `json:"nombre"`
}

// Plan son las pruebas que ofrece la pantalla. Las encuestas de 2 a 20 opciones sirven para medir el límite (la librería no lo valida);
// los botones, las listas y los mensajes interactivos son lo que la librería envía por defecto.
func Plan() []Prueba {
	var out []Prueba
	for _, n := range []int{2, 4, 8, 12, 13, 16, 20} {
		out = append(out, Prueba{almacen.LabEncuesta, n, fmt.Sprintf("Encuesta de %d opciones", n)})
	}
	for _, n := range []int{1, 2, 3} {
		out = append(out, Prueba{almacen.LabBotones, n, fmt.Sprintf("Botones (%d)", n)})
	}
	for _, n := range []int{3, 10, 11} {
		out = append(out, Prueba{almacen.LabLista, n, fmt.Sprintf("Lista de %d filas", n)})
	}
	for _, n := range []int{1, 3} {
		out = append(out, Prueba{almacen.LabInteractivo, n, fmt.Sprintf("Interactivo quick_reply (%d)", n)})
	}
	return out
}

func existe(tipo string, n int) bool {
	if n < 1 || n > 32 {
		return false
	}
	switch tipo {
	case almacen.LabEncuesta:
		return n >= 2
	case almacen.LabBotones, almacen.LabInteractivo, almacen.LabLista:
		return true
	}
	return false
}

// EnviarPrueba manda UNA prueba al chat de prueba (mi otro número o mi grupo de prueba) y la anota en la matriz. Respeta el tope por
// hora y por día. Devuelve el renglón de la matriz; si el envío falló, el renglón trae el error y la función también lo devuelve.
func (s *Servicio) EnviarPrueba(ctx context.Context, tipo string, variante int) (almacen.ResultadoLab, error) {
	if err := s.Habilitado(); err != nil {
		return almacen.ResultadoLab{}, err
	}
	if !existe(tipo, variante) {
		return almacen.ResultadoLab{}, ErrTipo
	}
	chat := s.aj("lab_prueba_chat")
	if chat == "" {
		return almacen.ResultadoLab{}, ErrSinDestino
	}
	ahora := s.ahora()
	if h := s.ajInt("lab_prueba_hora"); h > 0 && s.Alm.ContarEnviosLab(s.BotID, ahora.Add(-time.Hour)) >= h {
		return almacen.ResultadoLab{}, ErrTope
	}
	if d := s.ajInt("lab_prueba_dia"); d > 0 && s.Alm.ContarEnviosLab(s.BotID, ahora.Add(-24*time.Hour)) >= d {
		return almacen.ResultadoLab{}, ErrTope
	}
	rid, err := s.Alm.NuevoResultadoLab(s.BotID, tipo, fmt.Sprint(variante), chat, s.aj("lab_version_wa"), s.Cli.VersionCliente(), ahora)
	if err != nil {
		return almacen.ResultadoLab{}, err
	}
	sp := spec{Tipo: tipo, Titulo: fmt.Sprintf("Prueba del Laboratorio (%s, %d). Elige cualquier opción.", tipo, variante)}
	for i := 1; i <= variante; i++ {
		sp.Opciones = append(sp.Opciones, almacen.OpcionLab{Num: i, Texto: fmt.Sprintf("Opción %d", i)})
	}
	exp := ahora.Add(time.Duration(maxInt(s.ajInt("lab_expira_min"), 1)) * time.Minute)
	var sesID string
	r, err := s.Env.EnviarInteractivo(ctx, chat, resumenTexto(&sp, ""), "", "",
		func(id string, ci *waE2E.ContextInfo) *waE2E.Message {
			m := s.armar(&sp, id)
			ponerContexto(m, ci)
			return m
		},
		func(id string, m *waE2E.Message) error {
			ses := almacen.SesionLab{ID: id, Chat: chat, Persona: chat, Tipo: tipo, Origen: almacen.LabOrigenPrueba, PruebaID: rid,
				TS: ahora.Unix(), ExpiraTS: exp.Unix(), Opciones: sp.Opciones, Secreto: m.GetMessageContextInfo().GetMessageSecret()}
			if err := s.Alm.GuardarSesionLab(s.BotID, ses); err != nil {
				return err
			}
			sesID = id
			return nil
		})
	if err != nil {
		if sesID != "" {
			s.Alm.BorrarSesionLab(s.BotID, sesID)
		}
		motivo := err.Error()
		if err == envio.ErrChatDesconoci {
			motivo = "El chat de prueba no existe todavía: escríbele algo desde el teléfono o abre su chat primero."
		}
		s.Alm.ResultadoLabEnviado(s.BotID, rid, "", false, motivo)
		s.Log("Prueba del Laboratorio sin enviar: " + motivo)
	} else {
		s.Alm.ResultadoLabEnviado(s.BotID, rid, r.ID, true, "")
		s.Log(fmt.Sprintf("Prueba del Laboratorio enviada (%s, %d).", tipo, variante))
	}
	for _, x := range s.Alm.ListarResultadosLab(s.BotID, 20) {
		if x.ID == rid {
			return x, err
		}
	}
	return almacen.ResultadoLab{ID: rid}, err
}

// LimiteMedido es el mayor número de opciones de una encuesta que, según lo que marcaste en la matriz, se vio y se pudo tocar.
// 0 si todavía no hay ninguna prueba marcada como visible.
func LimiteMedido(rs []almacen.ResultadoLab) int {
	max := 0
	for _, r := range rs {
		if r.Tipo != almacen.LabEncuesta || r.Visible != 1 || r.Tocable == 0 {
			continue
		}
		var n int
		if _, err := fmt.Sscanf(r.Variante, "%d", &n); err == nil && n > max {
			max = n
		}
	}
	return max
}
