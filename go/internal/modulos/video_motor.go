package modulos

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/draw"
	"image/jpeg"
	_ "image/png"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"botchat/internal/almacen"
	"botchat/internal/envio"
)

// ---------- la descarga: bajar, preparar y mandar ----------

type resDescarga struct {
	Ruta string `json:"ruta"`
	Tam  int64  `json:"tam"`
	Dur  int    `json:"dur"`
	Tipo string `json:"tipo"`
}

// videoIniciar mete la descarga a la fila (una a la vez) y la corre. Corre en su propia goroutine.
func (g *Gestor) videoIniciar(s *sesVideo, cfg Config) {
	st := g.vid()
	st.mu.Lock()
	delete(st.ses, claveVideo(s.Chat, s.Persona))
	st.esperando++
	lugar := st.esperando + st.enCurso
	st.mu.Unlock()
	if lugar > 1 {
		g.videoTexto(s, fmt.Sprintf("⏳ Hay %d descargas antes que la tuya; te aviso cuando empiece.", lugar-1))
	} else {
		g.videoTexto(s, strings.NewReplacer("{titulo}", cortaVideo(s.Info.Titulo, 60)).Replace(cfg.Texto("msg_bajando")))
	}
	go func() {
		st.sem <- struct{}{} // una descarga a la vez: el teléfono no aguanta varias
		st.mu.Lock()
		st.esperando--
		st.enCurso++
		st.mu.Unlock()
		g.Cambio("descargas_video")
		defer func() {
			<-st.sem
			st.mu.Lock()
			st.enCurso--
			st.mu.Unlock()
			g.Cambio("descargas_video")
		}()
		g.videoEjecutar(s)
	}()
}

func formatoDe(modo, audioFmt string) string {
	if modo == "audio" {
		return audioFmt
	}
	return "mp4"
}

func (g *Gestor) videoEjecutar(s *sesVideo) {
	cfg := g.Config(IDVideo)
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Minute)
	defer cancel()
	audioFmt := cfg.Texto("audio_formato")
	if audioFmt == "" {
		audioFmt = "mp3"
	}
	calidad := fmt.Sprintf("%dp", s.Altura)
	if s.Modo == "audio" {
		calidad = "mejor"
		if s.Abr > 0 {
			calidad = fmt.Sprintf("%d kbps", s.Abr)
		}
	}
	mini := g.videoMiniatura(ctx, s.Info.Miniatura)
	id, _ := g.Alm.NuevaDescargaVideo(g.BotID, almacen.DescargaVideo{Persona: s.Persona, Nombre: s.Nombre, Chat: s.Chat, EsGrupo: s.EsGrupo,
		URL: s.URL, Titulo: s.Info.Titulo, Subidor: s.Info.Subidor, Mini: mini, Sitio: s.Info.Extractor, Duracion: s.Info.Duracion, Peso: s.TamEst,
		Formato: formatoDe(s.Modo, audioFmt), Calidad: calidad, Entrega: s.Entrega})
	fallo := func(err error) {
		motivo := motivoVideo(err)
		g.Alm.RegistrarModulo(g.BotID, IDVideo, "descargar", ResFallido, "", g.ahora())
		_ = g.Alm.CerrarDescargaVideo(g.BotID, id, "fallido", motivo, 0, 0, "")
		g.videoTexto(s, strings.NewReplacer("{motivo}", motivo).Replace(cfg.Texto("msg_error")))
	}
	trabajo := filepath.Join(g.Video.Carpeta(), "trabajo", fmt.Sprintf("d%d", id))
	limpiar := func() { _ = g.Video.Borrar(trabajo) }
	raw, err := g.Video.Descargar(s.URL, trabajo, s.Modo, s.Altura, s.Abr)
	if err != nil {
		limpiar()
		fallo(err)
		return
	}
	var r resDescarga
	if err := json.Unmarshal([]byte(raw), &r); err != nil || r.Ruta == "" {
		limpiar()
		fallo(errors.New("no entendí lo que devolvió el descargador"))
		return
	}
	if s.Modo == "audio" && audioFmt == "mp3" {
		if nueva, err := g.Video.Convertir(r.Ruta, formatoMp3(s.Abr)); err == nil && nueva != "" {
			_ = g.Video.Borrar(r.Ruta)
			r.Ruta = nueva
			if st, e := os.Stat(nueva); e == nil {
				r.Tam = st.Size()
			}
		} else {
			audioFmt = "m4a" // no se pudo convertir: se manda como vino
			g.Alm.FormatoDescargaVideo(g.BotID, id, "m4a")
			g.Log("Descargas: no se pudo pasar a mp3, sale en m4a.")
		}
	}
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(r.Ruta)), ".")
	dur := s.Info.Duracion
	if dur == 0 {
		dur = r.Dur
	}
	envioMax := int64(cfg.Int("max_envio_mb")) << 20
	archivos := []string{r.Ruta}
	entrega := "directo"
	base := nombreSeguro(s.Info.Titulo)
	if r.Tam > envioMax {
		entrega = s.Entrega
		if entrega == "" || entrega == "directo" {
			entrega = "partes"
		}
		if entrega == "zip" {
			z := filepath.Join(trabajo, base+".zip")
			j, _ := json.Marshal([]string{r.Ruta})
			if err := g.Video.Zip(string(j), z); err != nil {
				limpiar()
				fallo(err)
				return
			}
			_ = g.Video.Borrar(r.Ruta)
			archivos = []string{z}
		} else {
			n := partesNecesarias(r.Tam, envioMax)
			rawP, err := g.Video.Partir(r.Ruta, n, topeParte(envioMax), trabajo, base)
			var ps []string
			if err == nil {
				err = json.Unmarshal([]byte(rawP), &ps)
			}
			if err != nil || len(ps) == 0 {
				// no se pudo partir bien: si cabe, se manda en un zip en vez de fallar
				if r.Tam <= int64(cfg.Int("max_zip_mb"))<<20 {
					z := filepath.Join(trabajo, base+".zip")
					j, _ := json.Marshal([]string{r.Ruta})
					if e2 := g.Video.Zip(string(j), z); e2 == nil {
						g.Log("Descargas: no se pudo partir, sale en zip.")
						g.videoTexto(s, "No pude dividirlo en partes, así que te lo mando en un solo archivo ZIP.")
						_ = g.Video.Borrar(r.Ruta)
						entrega, archivos = "zip", []string{z}
						g.videoEnviar(ctx, s, cfg, id, archivos, entrega, ext, dur, r.Tam, mini, trabajo)
						return
					}
				}
				limpiar()
				fallo(fmt.Errorf("no pude dividirlo: %v", err))
				return
			}
			if len(ps) > 1 {
				_ = g.Video.Borrar(r.Ruta)
			}
			archivos = ps
			if anunciadas := partesNecesarias(s.TamEst, envioMax); s.TamEst > 0 && len(ps) != anunciadas {
				g.videoTexto(s, fmt.Sprintf("Al final pesó %s, así que van %d partes.", mbTxt(r.Tam), len(ps)))
			}
		}
	}
	g.videoEnviar(ctx, s, cfg, id, archivos, entrega, ext, dur, r.Tam, mini, trabajo)
}

func mimeDe(ext string) string {
	switch ext {
	case "mp4", "m4v":
		return "video/mp4"
	case "mp3":
		return "audio/mpeg"
	case "m4a":
		return "audio/mp4"
	case "zip":
		return "application/zip"
	}
	return "application/octet-stream"
}

func (g *Gestor) videoEnviar(ctx context.Context, s *sesVideo, cfg Config, id int64, archivos []string, entrega, ext string, dur int, tam int64, mini []byte, trabajo string) {
	topeEnvio := int64(cfg.Int("max_envio_mb"))
	topeZip := int64(cfg.Int("max_zip_mb"))
	if topeZip > topeEnvio {
		topeEnvio = topeZip
	}
	n := len(archivos)
	var mu sync.Mutex
	pendientes, fallidos := n, 0
	cierra := func() {
		_ = g.Video.Borrar(trabajo)
		estado, msg := "enviado", ""
		if fallidos > 0 {
			estado, msg = "fallido", fmt.Sprintf("No se pudieron mandar %d de %d archivos", fallidos, n)
		}
		_ = g.Alm.CerrarDescargaVideo(g.BotID, id, estado, msg, tam, n, entrega)
		if fallidos == 0 {
			g.Alm.RegistrarModulo(g.BotID, IDVideo, "descargar", ResEnviado, "", g.ahora())
			g.videoTexto(s, cfg.Texto("msg_listo"))
		} else {
			g.Alm.RegistrarModulo(g.BotID, IDVideo, "descargar", ResFallido, "", g.ahora())
			g.videoTexto(s, strings.NewReplacer("{motivo}", msg).Replace(cfg.Texto("msg_error")))
		}
	}
	var mandar func(i int)
	mandar = func(i int) {
		ruta := archivos[i]
		e := strings.TrimPrefix(strings.ToLower(filepath.Ext(ruta)), ".")
		a := envio.Adjunto{Ruta: ruta, Mime: mimeDe(e), Nombre: filepath.Base(ruta), TopeBytes: topeEnvio << 20, SinCopia: true, DurS: dur}
		var pesoArch int64
		if st, err := os.Stat(ruta); err == nil {
			pesoArch = st.Size()
		}
		// Lo que pasa del peso de video se manda como ARCHIVO (documento): WhatsApp no lo recomprime y deja hasta 2 000 MB.
		comoArchivo := pesoArch > int64(cfg.Int("max_video_mb"))<<20
		switch {
		case e == "zip":
			a.Tipo = "documento"
		case comoArchivo:
			a.Tipo, a.Miniatura = "documento", mini
		case s.Modo == "audio":
			a.Tipo = "audio"
		default:
			a.Tipo, a.Miniatura = "video", mini
			if n > 1 {
				a.DurS = dur / n
			}
		}
		pie := ""
		if i == 0 {
			pie = "🎬 " + cortaVideo(s.Info.Titulo, 100)
			if s.Modo == "audio" {
				pie = "🎵 " + cortaVideo(s.Info.Titulo, 100)
			}
		}
		if n > 1 {
			if pie != "" {
				pie += "\n"
			}
			pie += fmt.Sprintf("Parte %d de %d", i+1, n)
		}
		a.Pie = pie
		ini := Iniciativa{Modulo: IDVideo, Que: "archivo", Chat: s.Chat, Personas: []string{s.Persona}, Medio: &a, CitaID: s.CitaID,
			EsperaMax: 90 * time.Minute,
			AlResultado: func(res string) {
				mu.Lock()
				defer mu.Unlock()
				if res != ResEnviado {
					fallidos++
				}
				pendientes--
				if pendientes == 0 {
					cierra()
					return
				}
				// las partes salen una por una y en orden: la siguiente se encola hasta que la anterior ya se mandó
				if res != ResEnviado {
					fallidos += pendientes
					pendientes = 0
					cierra()
					return
				}
				go mandar(i + 1)
			}}
		if i > 0 {
			ini.RetrasoMin, ini.RetrasoMax = 4*time.Second, 12*time.Second
		}
		if _, err := g.Enviar(ini); err != nil {
			mu.Lock()
			fallidos += pendientes
			pendientes = 0
			mu.Unlock()
			cierra()
		}
	}
	mandar(0)
}

// ---------- miniatura ----------

// videoMiniatura baja la portada y la deja como un JPEG chico (de 160 px de ancho). nil si no se pudo (por ejemplo, si viene en webp).
func (g *Gestor) videoMiniatura(ctx context.Context, enlace string) []byte {
	if enlace == "" || !strings.HasPrefix(enlace, "https://") {
		return nil
	}
	resp, err := g.descargador().Hacer(ctx, PeticionHTTP{Metodo: "GET", URL: enlace, Timeout: 15 * time.Second, Max: 3 << 20})
	if err != nil {
		return nil
	}
	defer resp.Cuerpo.Close()
	b, err := io.ReadAll(io.LimitReader(resp.Cuerpo, 3<<20))
	if err != nil {
		return nil
	}
	return reducirJPEG(b, 160)
}

func reducirJPEG(b []byte, ancho int) []byte {
	img, _, err := image.Decode(bytes.NewReader(b))
	if err != nil {
		return nil
	}
	r := img.Bounds()
	if r.Dx() == 0 || r.Dy() == 0 {
		return nil
	}
	w := ancho
	if r.Dx() < w {
		w = r.Dx()
	}
	h := r.Dy() * w / r.Dx()
	if h == 0 {
		h = 1
	}
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	// promedio por bloques (suficiente para una miniatura)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			x0, x1 := r.Min.X+x*r.Dx()/w, r.Min.X+(x+1)*r.Dx()/w
			y0, y1 := r.Min.Y+y*r.Dy()/h, r.Min.Y+(y+1)*r.Dy()/h
			if x1 <= x0 {
				x1 = x0 + 1
			}
			if y1 <= y0 {
				y1 = y0 + 1
			}
			var sr, sg, sb, n uint32
			for yy := y0; yy < y1; yy += 1 + (y1-y0)/4 {
				for xx := x0; xx < x1; xx += 1 + (x1-x0)/4 {
					cr, cg, cb, _ := img.At(xx, yy).RGBA()
					sr, sg, sb, n = sr+cr>>8, sg+cg>>8, sb+cb>>8, n+1
				}
			}
			if n == 0 {
				n = 1
			}
			dst.Pix[(y*w+x)*4+0], dst.Pix[(y*w+x)*4+1], dst.Pix[(y*w+x)*4+2], dst.Pix[(y*w+x)*4+3] = uint8(sr/n), uint8(sg/n), uint8(sb/n), 255
		}
	}
	_ = draw.Draw
	var out bytes.Buffer
	if err := jpeg.Encode(&out, dst, &jpeg.Options{Quality: 62}); err != nil {
		return nil
	}
	return out.Bytes()
}

// ---------- limpieza ----------

// videoLimpiarTrabajo borra lo que haya quedado en la carpeta de trabajo (por ejemplo si la app se cerró a media descarga).
func (g *Gestor) videoLimpiarTrabajo() {
	if g.Video == nil {
		return
	}
	_ = g.Video.Borrar(filepath.Join(g.Video.Carpeta(), "trabajo"))
}

// ---------- yt-dlp al día ----------

var hostsYtdlp = map[string]bool{"api.github.com": true, "github.com": true, "objects.githubusercontent.com": true, "release-assets.githubusercontent.com": true}

type releaseGH struct {
	Tag    string `json:"tag_name"`
	Assets []struct {
		Nombre string `json:"name"`
		URL    string `json:"browser_download_url"`
	} `json:"assets"`
}

func (g *Gestor) leerGH(ctx context.Context, enlace string, max int64) ([]byte, error) {
	resp, err := g.descargador().Hacer(ctx, PeticionHTTP{Metodo: "GET", URL: enlace, Timeout: 90 * time.Second, Max: max,
		Cabeceras: map[string]string{"Accept": "application/vnd.github+json", "User-Agent": "BotChat"},
		Permitido: func(h string) bool { return hostsYtdlp[strings.ToLower(h)] }})
	if err != nil {
		return nil, err
	}
	defer resp.Cuerpo.Close()
	if resp.Estado != 200 {
		return nil, fmt.Errorf("GitHub respondió %d", resp.Estado)
	}
	return io.ReadAll(resp.Cuerpo)
}

// versionMayor dice si a es más nueva que b (las versiones de yt-dlp son fechas: 2026.10.26 o 2026.10.26.1).
func versionMayor(a, b string) bool {
	pa, pb := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < len(pa) || i < len(pb); i++ {
		var x, y int
		if i < len(pa) {
			fmt.Sscanf(pa[i], "%d", &x)
		}
		if i < len(pb) {
			fmt.Sscanf(pb[i], "%d", &y)
		}
		if x != y {
			return x > y
		}
	}
	return false
}

// VideoActualizar revisa si hay un yt-dlp más nuevo y lo instala (verificando su firma sha256 y que arranque; si falla, queda el anterior).
// forzar = instalarlo aunque la versión sea la misma. Devuelve una frase para mostrar.
func (g *Gestor) VideoActualizar(forzar bool) (string, error) {
	if g.Video == nil {
		return "", errors.New("las descargas no están disponibles en este momento")
	}
	st := g.vid()
	st.mu.Lock()
	if st.actualizando {
		st.mu.Unlock()
		return "Ya estoy revisando las actualizaciones.", nil
	}
	st.actualizando = true
	st.mu.Unlock()
	defer func() {
		st.mu.Lock()
		st.actualizando = false
		st.mu.Unlock()
		g.Cambio("descargas_video")
	}()
	g.Cambio("descargas_video")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	actual, _ := g.Video.Version()
	if actual != "" {
		_ = g.Alm.FijarAjusteBot(g.BotID, "video_ytdlp_ver", actual)
	}
	_ = g.Alm.FijarAjusteBot(g.BotID, "video_ytdlp_rev_ts", fmt.Sprint(g.ahora().Unix()))
	raw, err := g.leerGH(ctx, "https://api.github.com/repos/yt-dlp/yt-dlp/releases/latest", 2<<20)
	if err != nil {
		st.ultimoError = "no pude revisar: " + motivoVideo(err)
		return "", fmt.Errorf("no pude revisar las versiones nuevas (%s)", motivoVideo(err))
	}
	var rel releaseGH
	if err := json.Unmarshal(raw, &rel); err != nil || rel.Tag == "" {
		return "", errors.New("GitHub respondió algo que no entendí")
	}
	_ = g.Alm.FijarAjusteBot(g.BotID, "video_ytdlp_ultima", rel.Tag)
	if !forzar && actual != "" && !versionMayor(rel.Tag, actual) {
		st.ultimoError = ""
		return fmt.Sprintf("yt-dlp %s ya es la última versión.", actual), nil
	}
	var urlBin, urlSums string
	for _, a := range rel.Assets {
		switch a.Nombre {
		case "yt-dlp":
			urlBin = a.URL
		case "SHA2-256SUMS":
			urlSums = a.URL
		}
	}
	if urlBin == "" || urlSums == "" {
		return "", errors.New("la versión nueva no trae los archivos que espero")
	}
	bin, err := g.leerGH(ctx, urlBin, 20<<20)
	if err != nil {
		return "", fmt.Errorf("no pude bajar la versión nueva (%s)", motivoVideo(err))
	}
	sumas, err := g.leerGH(ctx, urlSums, 1<<20)
	if err != nil {
		return "", fmt.Errorf("no pude bajar la firma de la versión nueva (%s)", motivoVideo(err))
	}
	h := sha256.Sum256(bin)
	esperado := ""
	for _, l := range strings.Split(string(sumas), "\n") {
		f := strings.Fields(l)
		if len(f) == 2 && strings.TrimPrefix(f[1], "*") == "yt-dlp" {
			esperado = strings.ToLower(f[0])
		}
	}
	if esperado == "" || esperado != hex.EncodeToString(h[:]) {
		return "", errors.New("la versión que bajé no coincide con su firma, no la instalé")
	}
	dir := g.Video.Carpeta()
	_ = os.MkdirAll(dir, 0o700)
	nuevo := filepath.Join(dir, "yt-dlp.nuevo")
	if err := os.WriteFile(nuevo, bin, 0o600); err != nil {
		return "", err
	}
	ver, err := g.Video.Instalar(nuevo)
	_ = os.Remove(nuevo)
	if err != nil {
		st.ultimoError = "la versión nueva no arrancó, sigue la anterior"
		g.Alm.RegistrarModulo(g.BotID, IDVideo, "actualizar", ResFallido, "", g.ahora())
		return "", errors.New("la versión nueva no arrancó; dejé la anterior")
	}
	st.ultimoError = ""
	_ = g.Alm.FijarAjusteBot(g.BotID, "video_ytdlp_ver", ver)
	g.Alm.RegistrarModulo(g.BotID, IDVideo, "actualizar", ResEnviado, "", g.ahora())
	if actual == "" {
		return fmt.Sprintf("Listo: descargué el motor (yt-dlp %s). Ya puedes encender el módulo.", ver), nil
	}
	return fmt.Sprintf("yt-dlp actualizado a la versión %s.", ver), nil
}

// VideoQuitarMotor borra el motor descargado y apaga el módulo: queda bloqueado hasta que se baje otra vez.
func (g *Gestor) VideoQuitarMotor() error {
	if g.Video == nil {
		return errors.New("las descargas no están disponibles en este momento")
	}
	st := g.vid()
	st.mu.Lock()
	ocupado := st.enCurso > 0 || st.esperando > 0 || st.actualizando
	st.mu.Unlock()
	if ocupado {
		return errors.New("hay descargas en curso o una actualización; espera a que terminen")
	}
	if g.Activo(IDVideo) {
		if err := g.Desactivar(IDVideo); err != nil {
			return err
		}
	}
	if err := g.Video.Quitar(); err != nil {
		return err
	}
	for _, k := range []string{"video_ytdlp_ver", "video_ytdlp_ultima", "video_ytdlp_rev_ts"} {
		_ = g.Alm.FijarAjusteBot(g.BotID, k, "")
	}
	g.Alm.RegistrarModulo(g.BotID, IDVideo, "quitar_motor", ResEnviado, "", g.ahora())
	g.Cambio("descargas_video")
	return nil
}

// videoRevisionDiaria corre cada minuto (EvTick): a la hora elegida revisa una vez al día si hay yt-dlp nuevo.
func (g *Gestor) videoRevisionDiaria(hora time.Time) {
	cfg := g.Config(IDVideo)
	if !cfg.Bool("actualizar_auto") || g.Video == nil || !g.Video.Instalado() {
		return
	}
	loc := time.Local
	if g.Zona != nil {
		loc = g.Zona()
	}
	l := hora.In(loc)
	if l.Format("15:04") != cfg.Texto("hora_actualizar") {
		return
	}
	dia := l.Format("2006-01-02")
	if g.Alm.AjusteBot(g.BotID, "video_upd_dia") == dia {
		return
	}
	_ = g.Alm.FijarAjusteBot(g.BotID, "video_upd_dia", dia)
	go func() { _, _ = g.VideoActualizar(false) }()
}

// ---------- estado para la app ----------

// EstadoVideo es lo que la pantalla de descargas muestra arriba.
type EstadoVideo struct {
	Activo       bool   `json:"activo"`
	Motor        bool   `json:"motor"`
	Instalado    bool   `json:"instalado"` // el motor (yt-dlp) ya está descargado
	Version      string `json:"version"`
	Ultima       string `json:"ultima"`
	Hay          bool   `json:"hayNueva"`
	RevisionTS   int64  `json:"revisionTs"`
	Actualizando bool   `json:"actualizando"`
	EnCurso      int    `json:"enCurso"`
	Esperando    int    `json:"esperando"`
	Error        string `json:"error"`
	Hoy          int    `json:"hoy"`
}

func (g *Gestor) VideoEstado() EstadoVideo {
	st := g.vid()
	st.mu.Lock()
	defer st.mu.Unlock()
	e := EstadoVideo{Activo: g.Activo(IDVideo), Motor: g.Video != nil, Instalado: g.Video != nil && g.Video.Instalado(), Actualizando: st.actualizando, EnCurso: st.enCurso, Esperando: st.esperando, Error: st.ultimoError}
	e.Version = g.Alm.AjusteBot(g.BotID, "video_ytdlp_ver")
	e.Ultima = g.Alm.AjusteBot(g.BotID, "video_ytdlp_ultima")
	e.Hay = e.Ultima != "" && e.Version != "" && versionMayor(e.Ultima, e.Version)
	fmt.Sscanf(g.Alm.AjusteBot(g.BotID, "video_ytdlp_rev_ts"), "%d", &e.RevisionTS)
	e.Hoy = g.Alm.ContarDescargasVideo(g.BotID, "", g.ahora().Add(-24*time.Hour).Unix())
	return e
}

// VideoRepetir vuelve a pedir una descarga del historial con las mismas opciones, sin preguntar otra vez. El archivo ya no existe: se baja de nuevo.
func (g *Gestor) VideoRepetir(id int64) (string, error) {
	if g.Video == nil {
		return "", errors.New("las descargas no están disponibles en este momento")
	}
	d, err := g.Alm.DescargaVideoPorID(g.BotID, id)
	if err != nil {
		return "", err
	}
	s := &sesVideo{Chat: d.Chat, Persona: d.Persona, Nombre: d.Nombre, URL: d.URL, EsGrupo: d.EsGrupo, Dueno: true, TamEst: d.Peso, Entrega: d.Entrega}
	s.Modo = "video"
	if d.Formato != "mp4" {
		s.Modo = "audio"
	}
	fmt.Sscanf(d.Calidad, "%dp", &s.Altura)
	fmt.Sscanf(d.Calidad, "%d kbps", &s.Abr)
	go func() {
		cfg := g.Config(IDVideo)
		raw, err := g.Video.Info(s.URL)
		if err != nil {
			g.videoFalla(s, cfg, motivoVideo(err))
			return
		}
		if json.Unmarshal([]byte(raw), &s.Info) != nil {
			g.videoFalla(s, cfg, "no entendí la respuesta del descargador")
			return
		}
		if s.Entrega == "directo" || s.Entrega == "" {
			s.Entrega = "partes"
		}
		g.videoIniciar(s, cfg)
	}()
	return "Volviendo a descargar «" + cortaVideo(d.Titulo, 60) + "»…", nil
}

// formatoMp3 dice a qué bitrate pasar el audio: el que se eligió (96 o 160 kbps…), con un mínimo razonable.
func formatoMp3(abr int) string {
	if abr <= 0 {
		return "mp3"
	}
	if abr < 96 {
		abr = 96
	}
	return fmt.Sprintf("mp3:%d", abr)
}
