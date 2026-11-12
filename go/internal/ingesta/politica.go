package ingesta

import "botchat/internal/almacen"

// Red es el tipo de conexión del teléfono.
type Red int

const (
	RedDesconocida Red = iota
	RedWifi
	RedDatos
)

// Regla dice por qué red se descarga solo un tipo de archivo. Apagada es el interruptor del tipo: apagado, nada de ese tipo baja solo.
type Regla struct {
	Wifi    bool `json:"wifi"`
	Datos   bool `json:"datos"`
	Apagada bool `json:"apagada,omitempty"`
}

// Claves de tipo que usa la política (el sticker tiene la suya, aunque en el espacio cuente como foto).
const (
	TipoFoto      = "foto"
	TipoAudio     = "audio"
	TipoDocumento = "documento"
	TipoVideo     = "video"
	TipoSticker   = "sticker"
)

// Tipos son las claves de tipo de la política, en orden.
var Tipos = []string{TipoFoto, TipoVideo, TipoAudio, TipoDocumento, TipoSticker}

// ClaveTipo traduce el tipo de mensaje ("imagen", "sticker", ...) a la clave de la política ("" si no es multimedia).
func ClaveTipo(tipoMensaje string) string {
	switch tipoMensaje {
	case almacen.TipoSticker:
		return TipoSticker
	case almacen.TipoImagen:
		return TipoFoto
	case almacen.TipoVideo:
		return TipoVideo
	case almacen.TipoAudio:
		return TipoAudio
	case almacen.TipoDocumento:
		return TipoDocumento
	}
	return ""
}

// Politica son las reglas de descarga automática (se cambian en Avanzado).
type Politica struct {
	Foto      Regla `json:"foto"`
	Audio     Regla `json:"audio"`
	Documento Regla `json:"documento"`
	Video     Regla `json:"video"`
	Sticker   Regla `json:"sticker"`
	// MaxBytes y VideoMaxBytes son el tope por archivo de antes (0 = sin tope, que es lo de fábrica desde la 0.6.0).
	MaxBytes      int64 `json:"maxBytes"`
	VideoMaxBytes int64 `json:"videoMaxBytes"`
	// TopeArchivo es un tope opcional por archivo y por tipo (clave de tipo → bytes; ausente o 0 = sin tope).
	TopeArchivo map[string]int64 `json:"topeArchivo,omitempty"`
	// TopeTipoTotal es un tope opcional para todo lo descargado de un tipo (ausente o 0 = sin tope).
	TopeTipoTotal map[string]int64 `json:"topeTipoTotal,omitempty"`
	// PisoLibre: con menos espacio libre que esto se pausan las descargas automáticas (500 MB por defecto).
	PisoLibre int64 `json:"pisoLibre"`
	// SoloWifiGrandes: los archivos de más de UmbralWifi (50 MB) solo bajan solos con Wi-Fi (encendido de fábrica).
	SoloWifiGrandes bool  `json:"soloWifiGrandes"`
	UmbralWifi      int64 `json:"umbralWifi"`
	// GrandePct: un archivo de más de este % del tope total no se baja solo (25 de fábrica; 0 = sin esta regla).
	GrandePct int `json:"grandePct"`
	// PausaBateria: con la batería por debajo de BateriaMin % y sin cargar, las descargas automáticas esperan (encendido, 15 %).
	PausaBateria bool `json:"pausaBateria"`
	BateriaMin   int  `json:"bateriaMin"`
}

const mb = 1024 * 1024

// PoliticaPorDefecto: todo se baja solo y sin tope por archivo; videos solo con Wi-Fi; lo de más de 50 MB solo con Wi-Fi;
// un archivo de más del 25 % del tope total no baja solo; piso de 500 MB libres; pausa con batería baja.
func PoliticaPorDefecto() Politica {
	todo := Regla{Wifi: true, Datos: true}
	return Politica{Foto: todo, Audio: todo, Documento: todo, Sticker: todo, Video: Regla{Wifi: true}, PisoLibre: 500 * mb,
		SoloWifiGrandes: true, UmbralWifi: 50 * mb, GrandePct: 25, PausaBateria: true, BateriaMin: 15}
}

// Bateria es el estado de la batería que pasa la app (Nivel -1 = no se sabe).
type Bateria struct {
	Nivel    int
	Cargando bool
}

// Entorno es lo que cambia con el tiempo y hace falta para decidir una descarga.
type Entorno struct {
	Red       Red
	Libre     int64
	Bateria   Bateria
	TopeTotal int64              // tope total del almacenamiento (0 = sin límite)
	UsoTipo   func(string) int64 // bytes ya descargados de un tipo (puede ser nil)
}

// Motivos por los que no se baja solo un archivo. Los dos primeros son pasajeros: la descarga espera y se vuelve a ver.
const (
	MotivoBateria   = "batería baja"
	MotivoSoloWifi  = "solo con Wi-Fi"
	MotivoSoloDatos = "solo con datos"
	MotivoEspacio   = "poco espacio libre"
	MotivoGrande    = "archivo grande"
	MotivoApagado   = "tipo apagado"
	MotivoTopeTipo  = "tope del tipo lleno"
	MotivoTopeTotal = "muy grande para el tope total"
)

// Pasajero dice si el motivo es temporal (red o batería) y conviene esperar en vez de dejar el archivo con botón.
func Pasajero(motivo string) bool { return motivo == MotivoBateria || motivo == MotivoSoloWifi }

// tope por archivo que aplica a un tipo.
func (p Politica) topeArchivo(tipo string) int64 {
	var t int64
	if tipo == TipoVideo {
		t = p.VideoMaxBytes
	} else {
		t = p.MaxBytes
	}
	if x := p.TopeArchivo[tipo]; x > 0 && (t <= 0 || x < t) {
		t = x
	}
	return t
}

func (p Politica) regla(tipo string) (Regla, bool) {
	switch tipo {
	case TipoFoto:
		return p.Foto, true
	case TipoAudio:
		return p.Audio, true
	case TipoDocumento:
		return p.Documento, true
	case TipoVideo:
		return p.Video, true
	case TipoSticker:
		return p.Sticker, true
	}
	return Regla{}, false
}

// Evaluar responde si un archivo del tipo dado se descarga solo y, si no, por qué.
func (p Politica) Evaluar(tipo string, bytes int64, e Entorno) (bool, string) {
	r, ok := p.regla(tipo)
	if !ok {
		return false, "no es multimedia"
	}
	if r.Apagada {
		return false, MotivoApagado
	}
	if p.PisoLibre > 0 && e.Libre > 0 && e.Libre < p.PisoLibre {
		return false, MotivoEspacio
	}
	if t := p.topeArchivo(tipo); t > 0 && bytes > t {
		return false, MotivoGrande
	}
	if e.TopeTotal > 0 && p.GrandePct > 0 && bytes > e.TopeTotal/100*int64(p.GrandePct) {
		return false, MotivoTopeTotal
	}
	if t := p.TopeTipoTotal[tipo]; t > 0 && e.UsoTipo != nil && e.UsoTipo(tipo)+bytes > t {
		return false, MotivoTopeTipo
	}
	if e.Red == RedWifi {
		if !r.Wifi {
			return false, MotivoSoloDatos
		}
	} else { // datos móviles o desconocida: se trata como datos
		if !r.Datos {
			return false, MotivoSoloWifi
		}
		if p.SoloWifiGrandes && p.UmbralWifi > 0 && bytes > p.UmbralWifi {
			return false, MotivoSoloWifi
		}
	}
	if p.PausaBateria && e.Bateria.Nivel >= 0 && !e.Bateria.Cargando && e.Bateria.Nivel < p.BateriaMin {
		return false, MotivoBateria
	}
	return true, ""
}

// Decidir es Evaluar sin tope total, tope por tipo ni batería (se queda por compatibilidad). `cat` es la categoría del espacio
// (foto, video, audio, documento); un sticker entra como foto.
func (p Politica) Decidir(cat string, bytes int64, red Red, libre int64) (bool, string) {
	return p.Evaluar(cat, bytes, Entorno{Red: red, Libre: libre, Bateria: Bateria{Nivel: -1}})
}
