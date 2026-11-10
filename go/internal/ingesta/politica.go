package ingesta

import "botchat/internal/almacen"

// Red es el tipo de conexión del teléfono.
type Red int

const (
	RedDesconocida Red = iota
	RedWifi
	RedDatos
)

// Regla dice por qué red se descarga solo un tipo de archivo.
type Regla struct {
	Wifi  bool `json:"wifi"`
	Datos bool `json:"datos"`
}

// Politica son las reglas de descarga automática (se cambian en Avanzado).
type Politica struct {
	Foto      Regla `json:"foto"`
	Audio     Regla `json:"audio"`
	Documento Regla `json:"documento"`
	Video     Regla `json:"video"`
	// MaxBytes: tope para fotos, audios y documentos.
	MaxBytes int64 `json:"maxBytes"`
	// VideoMaxBytes: tope para videos (50 MB por defecto).
	VideoMaxBytes int64 `json:"videoMaxBytes"`
	// PisoLibre: con menos espacio libre que esto se pausan las descargas automáticas (500 MB por defecto).
	PisoLibre int64 `json:"pisoLibre"`
}

const mb = 1024 * 1024

// PoliticaPorDefecto: fotos, audios y documentos hasta 50 MB solos; videos solo con Wi-Fi y hasta 50 MB; piso de 500 MB libres.
func PoliticaPorDefecto() Politica {
	todo := Regla{Wifi: true, Datos: true}
	return Politica{Foto: todo, Audio: todo, Documento: todo, Video: Regla{Wifi: true}, MaxBytes: 50 * mb, VideoMaxBytes: 50 * mb, PisoLibre: 500 * mb}
}

// Decidir responde si un archivo se descarga solo y, si no, por qué.
func (p Politica) Decidir(cat string, bytes int64, red Red, libre int64) (bool, string) {
	var r Regla
	tope := p.MaxBytes
	switch cat {
	case almacen.CatFoto:
		r = p.Foto
	case almacen.CatAudio:
		r = p.Audio
	case almacen.CatDocumento:
		r = p.Documento
	case almacen.CatVideo:
		r = p.Video
		tope = p.VideoMaxBytes
	default:
		return false, "no es multimedia"
	}
	if p.PisoLibre > 0 && libre > 0 && libre < p.PisoLibre {
		return false, "poco espacio libre"
	}
	if tope > 0 && bytes > tope {
		return false, "archivo grande"
	}
	switch red {
	case RedWifi:
		if !r.Wifi {
			return false, "solo con datos"
		}
	default: // datos móviles o desconocida: se trata como datos
		if !r.Datos {
			return false, "solo con Wi-Fi"
		}
	}
	return true, ""
}
