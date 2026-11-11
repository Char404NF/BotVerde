package lab

import (
	"crypto/rand"
	"encoding/hex"
	"io"
	"strconv"
	"time"

	waBinary "go.mau.fi/whatsmeow/binary"
)

// NodoBiz arma el nodo <biz> que propone el PR #1235 de whatsmeow (https://github.com/tulir/whatsmeow/pull/1235) para que un
// InteractiveMessage (native_flow) se muestre. EXPERIMENTAL: lo pidió el dueño de la cuenta como prueba, solo para el Laboratorio
// («Interactivo + biz»), solo al chat de prueba y con interruptor propio apagado y casilla de riesgo. La librería no lo añade sola
// para InteractiveMessage, así que se pasa por SendRequestExtra.AdditionalNodes (sin tocar el módulo).
//
// Estructura:
//
//	<biz actual_actors="2" host_storage="2" privacy_mode_ts="<unix>">
//	  <interactive type="native_flow" v="1"><native_flow v="9" name="mixed"/></interactive>
//	  <quality_control decision_id="<20 bytes al azar en hex>" source_type="third_party"><decision_source value="df"/></quality_control>
//	</biz>
//
// `azar` es de dónde salen los 20 bytes (nil = crypto/rand).
func NodoBiz(ahora time.Time, azar io.Reader) waBinary.Node {
	if azar == nil {
		azar = rand.Reader
	}
	b := make([]byte, 20)
	_, _ = io.ReadFull(azar, b)
	return waBinary.Node{
		Tag:   "biz",
		Attrs: waBinary.Attrs{"actual_actors": "2", "host_storage": "2", "privacy_mode_ts": strconv.FormatInt(ahora.Unix(), 10)},
		Content: []waBinary.Node{
			{Tag: "interactive", Attrs: waBinary.Attrs{"type": "native_flow", "v": "1"}, Content: []waBinary.Node{
				{Tag: "native_flow", Attrs: waBinary.Attrs{"v": "9", "name": "mixed"}},
			}},
			{Tag: "quality_control", Attrs: waBinary.Attrs{"decision_id": hex.EncodeToString(b), "source_type": "third_party"}, Content: []waBinary.Node{
				{Tag: "decision_source", Attrs: waBinary.Attrs{"value": "df"}},
			}},
		},
	}
}
