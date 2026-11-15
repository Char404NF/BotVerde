package modulos

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"botchat/internal/comandos"
	"botchat/internal/permisos"
)

// ---------- Descargas de video y audio (módulo opcional) ----------
//
// "!descargar <enlace>" (o "!audio <enlace>"): el bot revisa el enlace SIN bajarlo, avisa si es muy largo o muy pesado, deja elegir
// formato (video o audio) y calidad, y si lo que sale pasa de lo que WhatsApp deja mandar de una vez, deja elegir entre un zip o
// varias partes. Las opciones salen como botones o encuesta si el Laboratorio está encendido, y siempre con el menú de números.
//
// Quien baja el video es yt-dlp (Python embebido en la app, ver MotorVideo). Nada se queda: el archivo se borra apenas se envía y el
// historial (en la app) guarda enlace, quién lo pidió, miniatura y datos para volver a bajarlo.
// Es de INICIATIVA: lo que manda sale por la cola (pausas, topes, "escribiendo…", modo seguro, pausa nocturna).

// IDVideo es el id del módulo.
const IDVideo = "descargas_video"

// MotorVideo es lo que la app (Kotlin + Python con yt-dlp) ofrece al motor. Todo viaja como texto JSON para no atar el motor a yt-dlp.
type MotorVideo interface {
	// Version es la versión de yt-dlp que está en uso.
	Version() (string, error)
	// Info revisa el enlace sin descargar (JSON: título, duración, miniatura, tamaños por calidad).
	Info(url string) (string, error)
	// Descargar baja el video ("video") o el audio ("audio") y deja UN archivo (JSON: {ruta, tam, dur, tipo}).
	Descargar(url, carpeta, modo string, altura, abr int) (string, error)
	// Convertir pasa un audio a otro formato ("mp3") y devuelve la ruta nueva.
	Convertir(ruta, formato string) (string, error)
	// Partir parte un archivo en n pedazos que se reproducen solos (JSON: lista de rutas).
	Partir(ruta string, n int, carpeta, base string) (string, error)
	// Zip mete los archivos (JSON: lista de rutas) en un zip.
	Zip(rutasJSON, salida string) error
	// Instalar pone en uso un yt-dlp descargado (verificado antes por el motor Go) y devuelve su versión; si falla, deja el anterior.
	Instalar(ruta string) (string, error)
	// Carpeta es dónde se guardan el yt-dlp actualizado y los archivos de trabajo.
	Carpeta() string
	// Borrar quita un archivo o una carpeta de trabajo.
	Borrar(ruta string) error
}

type infoAlt struct {
	Altura  int   `json:"altura"`
	Tam     int64 `json:"tam"`
	Directo bool  `json:"directo"`
}

type infoVideo struct {
	Version   string    `json:"version"`
	Titulo    string    `json:"titulo"`
	Subidor   string    `json:"subidor"`
	Duracion  int       `json:"duracion"`
	Miniatura string    `json:"miniatura"`
	Extractor string    `json:"extractor"`
	EnVivo    bool      `json:"en_vivo"`
	Edad      int       `json:"edad"`
	ID        string    `json:"id"`
	URL       string    `json:"url"`
	Alturas   []infoAlt `json:"alturas"`
	Audio     struct {
		Abr  int   `json:"abr"`
		Tam  int64 `json:"tam"`
		Abrs []int `json:"abrs"`
	} `json:"audio"`
}

// opVid es una opción del diálogo: lo que se muestra y lo que vale.
type opVid struct {
	Texto  string
	Altura int
	Abr    int
	Tam    int64
	Valor  string // para las etapas de tipo y de entrega
}

// sesVideo es una descarga en proceso de elegirse (vive en memoria, vence sola).
type sesVideo struct {
	Chat, Persona, Nombre, CitaID, URL string
	EsGrupo, Dueno                     bool
	Info                               infoVideo
	Etapa                              string // tipo | calidad | entrega
	Modo                               string // video | audio
	Altura, Abr                        int
	TamEst                             int64
	Opciones                           []opVid
	Entrega                            string // directo | zip | partes
	Vence                              time.Time
}

type estadoVideo struct {
	mu           sync.Mutex
	ses          map[string]*sesVideo
	sem          chan struct{}
	esperando    int
	enCurso      int
	actualizando bool
	ultimoError  string
}

func (g *Gestor) vid() *estadoVideo {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.video == nil {
		g.video = &estadoVideo{ses: map[string]*sesVideo{}, sem: make(chan struct{}, 1)}
	}
	return g.video
}

func claveVideo(chat, persona string) string { return chat + "|" + persona }

// ---------- definición del módulo ----------

const avisoRiesgoVideo = "Bajar y mandar videos o música puede violar derechos de autor y mandar archivos grandes a muchas personas puede hacer que WhatsApp limite o bloquee tu número. Tú eres responsable de lo que se pida con tu bot."

func descargasVideo() Modulo {
	return Nuevo(Def{
		ID: IDVideo, Nombre: "Descargas de video y audio", Icono: "descarga", Riesgo: RiesgoAlto, Iniciativa: true,
		Descripcion: "«!descargar <enlace>» baja un video o su audio (YouTube, TikTok, Instagram, Facebook, X, SoundCloud y más), deja elegir formato y calidad, " +
			"avisa si es largo o pesado y lo manda dividido si hace falta. Al encenderlo baja la última versión de yt-dlp y se mantiene al día. " + avisoRiesgoVideo,
		Casillas: []permisos.Casilla{
			{ID: "video.pedir", Nombre: "Pedir descargas de video y audio", Descripcion: "Puede pedir «!descargar» y «!audio». Usa datos, batería y el número de WhatsApp.", Grupo: "Descargas", Riesgo: permisos.RiesgoMedio, Privado: true, Grupos: true},
			{ID: "video.admin", Nombre: "Administrar las descargas", Descripcion: "Ver el estado y actualizar yt-dlp con «!descargas» y «!actualizarytdlp».", Grupo: "Descargas", Riesgo: permisos.RiesgoMedio, Privado: true},
		},
		Esquema: []Campo{
			{Clave: "acepto_riesgos", Titulo: "Entiendo los riesgos", Ayuda: avisoRiesgoVideo + " Hace falta marcarlo para que descargue.", Tipo: CampoInterruptor, Defecto: false},
			{Clave: "max_min", Titulo: "Duración máxima (minutos)", Ayuda: "Un video más largo se rechaza antes de bajarlo.", Tipo: CampoNumero, Defecto: 60.0, Min: 1, Max: 600},
			{Clave: "max_total_mb", Titulo: "Peso máximo total (MB)", Ayuda: "Si aun con la calidad más baja pesa más, se rechaza antes de bajarlo.", Tipo: CampoNumero, Defecto: 1500.0, Min: 5, Max: 1900},
			{Clave: "max_video_mb", Titulo: "Hasta qué peso se manda como video (MB)", Ayuda: "WhatsApp recomprime los videos y su ayuda oficial habla de unos 100 MB. Lo que pese más se manda como ARCHIVO (documento): llega completo y sin comprimir, hasta 2 000 MB.", Tipo: CampoNumero, Defecto: 90.0, Min: 5, Max: 100},
			{Clave: "max_envio_mb", Titulo: "Peso máximo por archivo que se manda (MB)", Ayuda: "WhatsApp deja mandar archivos de hasta 2 000 MB. Lo que pase de esto se manda en un zip o en partes.", Tipo: CampoNumero, Defecto: 1000.0, Min: 5, Max: 1900},
			{Clave: "max_zip_mb", Titulo: "Peso máximo para mandarlo en un solo zip (MB)", Ayuda: "WhatsApp deja hasta unos 2 000 MB por documento.", Tipo: CampoNumero, Defecto: 1900.0, Min: 20, Max: 1900},
			{Clave: "formato_partes", Titulo: "Si es muy pesado", Ayuda: "Qué hacer cuando pasa del peso por archivo: preguntar a quien lo pide, o elegir siempre una forma.", Tipo: CampoLista, Defecto: "preguntar",
				Opciones: []Opcion{{"preguntar", "Preguntar a la persona"}, {"zip", "Siempre en un zip"}, {"partes", "Siempre en partes (mp4)"}}},
			{Clave: "alturas", Titulo: "Calidades de video que se ofrecen", Ayuda: "Alturas separadas por coma, de menor a mayor (por ejemplo 360,480,720,1080).", Tipo: CampoTexto, Largo: 60, Defecto: "360,480,720,1080"},
			{Clave: "audio_formato", Titulo: "Formato del audio", Ayuda: "mp3 lo convierte (más compatible); m4a lo manda como viene (más rápido).", Tipo: CampoLista, Defecto: "mp3",
				Opciones: []Opcion{{"mp3", "MP3"}, {"m4a", "M4A (AAC)"}}},
			{Clave: "por_persona_hora", Titulo: "Descargas por persona por hora", Ayuda: "0 = sin límite. El Dueño no tiene límite.", Tipo: CampoNumero, Defecto: 3.0, Min: 0, Max: 100},
			{Clave: "por_persona_dia", Titulo: "Descargas por persona por día", Ayuda: "0 = sin límite.", Tipo: CampoNumero, Defecto: 10.0, Min: 0, Max: 500},
			{Clave: "sitios", Titulo: "Sitios permitidos", Ayuda: "Un dominio por línea (youtube.com, tiktok.com…). Vacío = todos los que reconoce yt-dlp, menos los de adultos.", Tipo: CampoTexto, Largo: 1500, Varias: true, Defecto: ""},
			{Clave: "espera_min", Titulo: "Minutos para contestar las opciones", Tipo: CampoNumero, Defecto: 5.0, Min: 1, Max: 60},
			{Clave: "actualizar_auto", Titulo: "Actualizar yt-dlp solo", Ayuda: "Revisa cada día si hay una versión nueva y la instala (los sitios cambian seguido y la versión vieja deja de funcionar).", Tipo: CampoInterruptor, Defecto: true},
			{Clave: "hora_actualizar", Titulo: "Hora de la revisión diaria", Tipo: CampoHora, Defecto: "04:00"},
			{Clave: "msg_buscando", Titulo: "Mensaje: revisando el enlace", Tipo: CampoTexto, Largo: 200, Defecto: "🔎 Revisando el enlace…"},
			{Clave: "msg_bajando", Titulo: "Mensaje: empezó la descarga", Tipo: CampoTexto, Largo: 200, Defecto: "⬇️ Descargando «{titulo}». Te lo mando en cuanto esté listo."},
			{Clave: "msg_listo", Titulo: "Mensaje: listo", Tipo: CampoTexto, Largo: 200, Defecto: "✅ Listo."},
			{Clave: "msg_error", Titulo: "Mensaje: no se pudo", Ayuda: "{motivo} se cambia por la causa.", Tipo: CampoTexto, Largo: 200, Defecto: "😕 No pude descargarlo: {motivo}"},
		},
		TopeHora: 40, TopeDia: 250,
		Validar:   validarConfigVideo,
		ComandosG: comandosVideo,
		AlActivar: func(g *Gestor) error {
			g.Alm.MarcarDescargasVideoHuerfanas(g.BotID)
			g.videoLimpiarTrabajo()
			go func() { _, _ = g.VideoActualizar(false) }()
			return nil
		},
		AlEvento: eventoVideo,
		AlEstado: func(g *Gestor) string {
			cfg := g.Config(IDVideo)
			if !cfg.Bool("acepto_riesgos") {
				return "Encendido, falta aceptar los riesgos"
			}
			if g.Video == nil {
				return "Encendido, el motor de descargas no está disponible aquí"
			}
			ver := g.Alm.AjusteBot(g.BotID, "video_ytdlp_ver")
			n := g.Alm.ContarDescargasVideo(g.BotID, "", g.ahora().Add(-24*time.Hour).Unix())
			s := "Encendido"
			if ver != "" {
				s += " · yt-dlp " + ver
			}
			return fmt.Sprintf("%s · %d descargas en 24 h", s, n)
		},
	})
}

func validarConfigVideo(c map[string]any) error {
	cfg := Config(c)
	if len(alturasVideo(cfg.Texto("alturas"))) == 0 {
		return errors.New("escribe al menos una calidad de video, por ejemplo 360,720")
	}
	if cfg.Int("max_envio_mb") > cfg.Int("max_zip_mb") && cfg.Int("max_zip_mb") > 0 && cfg.Int("max_envio_mb") > cfg.Int("max_total_mb") {
		return errors.New("el peso por archivo no puede pasar del peso total")
	}
	return nil
}

func alturasVideo(s string) []int {
	var out []int
	seen := map[int]bool{}
	for _, p := range strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == ' ' || r == ';' }) {
		if n, err := strconv.Atoi(strings.TrimSuffix(strings.TrimSpace(p), "p")); err == nil && n >= 144 && n <= 4320 && !seen[n] {
			seen[n] = true
			out = append(out, n)
		}
	}
	sort.Ints(out)
	return out
}

// ---------- comandos ----------

func comandosVideo(g *Gestor) []comandos.Comando {
	pedir := func(pre string) func(cx *comandos.Contexto) (string, error) {
		return func(cx *comandos.Contexto) (string, error) { return g.videoPedir(cx, pre) }
	}
	return []comandos.Comando{
		{Nombre: "descargar", Alias: []string{"bajarvideo", "video"}, Resumen: "Bajar un video o su audio", Uso: "descargar <enlace>", Casilla: "video.pedir", Ejecutar: pedir("")},
		{Nombre: "audio", Alias: []string{"mp3"}, Resumen: "Bajar solo el audio de un enlace", Uso: "audio <enlace>", Casilla: "video.pedir", Ejecutar: pedir("audio")},
		{Nombre: "descargas", Resumen: "Estado de las descargas", Casilla: "video.admin", Ambito: comandos.Privado, Ejecutar: func(cx *comandos.Contexto) (string, error) {
			st := g.vid()
			st.mu.Lock()
			defer st.mu.Unlock()
			n := g.Alm.ContarDescargasVideo(g.BotID, "", g.ahora().Add(-24*time.Hour).Unix())
			return fmt.Sprintf("📥 yt-dlp %s · %d descargas en 24 h · %d en curso · %d esperando", g.Alm.AjusteBot(g.BotID, "video_ytdlp_ver"), n, st.enCurso, st.esperando), nil
		}},
		{Nombre: "actualizarytdlp", Resumen: "Revisar e instalar la última versión de yt-dlp", Casilla: "video.admin", Ambito: comandos.Privado, Ejecutar: func(cx *comandos.Contexto) (string, error) {
			return g.VideoActualizar(true)
		}},
	}
}

// ---------- pedir una descarga ----------

var sitiosAdultos = []string{"pornhub.com", "xvideos.com", "xnxx.com", "xhamster.com", "redtube.com", "youporn.com", "spankbang.com", "onlyfans.com", "rule34.xxx", "hentaihaven", "nhentai.net", "eporner.com", "tnaflix.com"}

func hostPermitidoVideo(host string, sitios []string) bool {
	host = strings.ToLower(strings.TrimPrefix(host, "www."))
	for _, a := range sitiosAdultos {
		if host == a || strings.HasSuffix(host, "."+a) {
			return false
		}
	}
	if len(sitios) == 0 {
		return true
	}
	return dominioPermitido(sitios, host)
}

func validarURLVideo(raw string, sitios []string) (string, error) {
	raw = strings.TrimSpace(strings.Trim(raw, "<>"))
	if raw == "" {
		return "", errors.New("escribe el enlace después del comando, por ejemplo: !descargar https://…")
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") {
		return "", errors.New("ese enlace no parece válido")
	}
	h := strings.ToLower(u.Hostname())
	if h == "localhost" || net.ParseIP(h) != nil || !strings.Contains(h, ".") {
		return "", errors.New("solo acepto enlaces de sitios de internet")
	}
	if !hostPermitidoVideo(h, sitios) {
		return "", errors.New("ese sitio no está permitido")
	}
	return u.String(), nil
}

func (g *Gestor) videoPedir(cx *comandos.Contexto, pre string) (string, error) {
	cfg := g.Config(IDVideo)
	if !cfg.Bool("acepto_riesgos") {
		return "", errors.New("el dueño todavía no aceptó los riesgos de las descargas en la configuración del módulo")
	}
	if g.Video == nil {
		return "", errors.New("las descargas no están disponibles en este momento")
	}
	enlace, err := validarURLVideo(cx.Resto, lineas(cfg.Texto("sitios")))
	if err != nil {
		return "", err
	}
	if !cx.Dueno {
		if t := cfg.Int("por_persona_hora"); t > 0 && g.Alm.ContarDescargasVideo(g.BotID, cx.Persona, g.ahora().Add(-time.Hour).Unix()) >= t {
			return "", fmt.Errorf("ya pediste %d descargas en esta hora; intenta más tarde", t)
		}
		if t := cfg.Int("por_persona_dia"); t > 0 && g.Alm.ContarDescargasVideo(g.BotID, cx.Persona, g.ahora().Add(-24*time.Hour).Unix()) >= t {
			return "", fmt.Errorf("ya llegaste a las %d descargas de hoy; mañana puedes seguir", t)
		}
	}
	st := g.vid()
	st.mu.Lock()
	k := claveVideo(cx.Chat, cx.Persona)
	if _, hay := st.ses[k]; hay {
		delete(st.ses, k) // lo nuevo reemplaza a lo que estaba esperando
	}
	st.mu.Unlock()
	s := &sesVideo{Chat: cx.Chat, Persona: cx.Persona, Nombre: cx.Nombre, CitaID: cx.MensajeID, URL: enlace, EsGrupo: cx.EsGrupo, Dueno: cx.Dueno, Modo: pre}
	go g.videoRevisar(s)
	return cfg.Texto("msg_buscando"), nil
}

// videoRevisar mira el enlace (sin bajarlo), valida los límites y arma la primera pregunta.
func (g *Gestor) videoRevisar(s *sesVideo) {
	cfg := g.Config(IDVideo)
	raw, err := g.Video.Info(s.URL)
	if err != nil {
		g.videoFalla(s, cfg, motivoVideo(err))
		return
	}
	var inf infoVideo
	if err := json.Unmarshal([]byte(raw), &inf); err != nil {
		g.videoFalla(s, cfg, "no entendí la respuesta del descargador")
		return
	}
	s.Info = inf
	switch {
	case inf.EnVivo:
		g.videoFalla(s, cfg, "ese video es una transmisión en vivo y no se puede bajar")
		return
	case inf.Edad >= 18:
		g.videoFalla(s, cfg, "ese contenido es para adultos y no se descarga")
		return
	case inf.Duracion > cfg.Int("max_min")*60:
		g.videoFalla(s, cfg, fmt.Sprintf("dura %s y el máximo es %d min", duracionTxt(inf.Duracion), cfg.Int("max_min")))
		return
	}
	hayVideo := len(g.opcionesVideo(s, cfg)) > 0
	switch {
	case s.Modo == "audio" || (!hayVideo && s.Modo == ""):
		s.Modo = "audio"
	case !hayVideo && s.Modo == "video":
		g.videoFalla(s, cfg, "ese enlace no tiene video que pueda bajar; prueba con «!audio»")
		return
	}
	s.Vence = g.ahora().Add(time.Duration(cfg.Int("espera_min")) * time.Minute)
	g.vid().mu.Lock()
	g.vid().ses[claveVideo(s.Chat, s.Persona)] = s
	g.vid().mu.Unlock()
	if s.Modo == "" {
		g.videoPreguntarTipo(s, cfg)
		return
	}
	g.videoPreguntarCalidad(s, cfg)
}

func motivoVideo(err error) string {
	m := err.Error()
	l := strings.ToLower(m)
	switch {
	case strings.Contains(l, "private") || strings.Contains(l, "sign in") || strings.Contains(l, "login") || strings.Contains(l, "logged-in"):
		return "ese video es privado o pide iniciar sesión"
	case strings.Contains(l, "unavailable") || strings.Contains(l, "not available") || strings.Contains(l, "removed"):
		return "ese video no está disponible"
	case strings.Contains(l, "unsupported url"):
		return "no reconozco ese sitio"
	case strings.Contains(l, "no address") || strings.Contains(l, "network") || strings.Contains(l, "timed out") || strings.Contains(l, "connection"):
		return "no hubo conexión con el sitio, intenta de nuevo"
	case strings.Contains(l, "geo") || strings.Contains(l, "country"):
		return "ese video no está disponible en este país"
	}
	if i := strings.LastIndex(m, "ERROR:"); i >= 0 {
		m = m[i+6:]
	}
	m = strings.TrimSpace(m)
	if len([]rune(m)) > 140 {
		m = string([]rune(m)[:140]) + "…"
	}
	return m
}

func (g *Gestor) videoFalla(s *sesVideo, cfg Config, motivo string) {
	g.Alm.RegistrarModulo(g.BotID, IDVideo, "revisar", ResFallido, "", g.ahora())
	g.videoTexto(s, strings.NewReplacer("{motivo}", motivo).Replace(cfg.Texto("msg_error")))
}

// ---------- el diálogo ----------

func (g *Gestor) videoTexto(s *sesVideo, texto string) {
	_, _ = g.Enviar(Iniciativa{Modulo: IDVideo, Que: "respuesta", Chat: s.Chat, Personas: []string{s.Persona}, Texto: texto, CitaID: s.CitaID,
		RetrasoMin: 1500 * time.Millisecond, RetrasoMax: 3500 * time.Millisecond})
}

// videoMenu manda las opciones como botones o encuesta (si el Laboratorio está encendido) y siempre con el texto numerado de respaldo.
func (g *Gestor) videoMenu(s *sesVideo, titulo string, ops []string) {
	var b strings.Builder
	b.WriteString(titulo)
	b.WriteString("\n")
	for i, o := range ops {
		fmt.Fprintf(&b, "\n%d. %s", i+1, o)
	}
	b.WriteString("\n\n_Responde con el número._")
	texto := b.String()
	if g.Acc == nil {
		g.videoTexto(s, texto)
		return
	}
	acc := g.Acc
	_, _ = g.Enviar(Iniciativa{Modulo: IDVideo, Que: "opciones", Chat: s.Chat, Personas: []string{s.Persona}, CitaID: s.CitaID,
		RetrasoMin: 1500 * time.Millisecond, RetrasoMax: 3500 * time.Millisecond,
		Accion: func(ctx context.Context) error {
			return acc.EnviarMenu(ctx, s.Chat, s.Persona, titulo, texto, ops, s.CitaID)
		}})
}

func (g *Gestor) videoPreguntarTipo(s *sesVideo, cfg Config) {
	s.Etapa = "tipo"
	s.Opciones = []opVid{{Texto: "🎬 Video (mp4)", Valor: "video"}, {Texto: "🎵 Solo el audio", Valor: "audio"}}
	g.videoMenu(s, fmt.Sprintf("*%s*\n⏱ %s · ¿Qué quieres?", cortaVideo(s.Info.Titulo, 80), duracionTxt(s.Info.Duracion)), textosOps(s.Opciones))
}

// opcionesVideo son las calidades de video que se ofrecen: las pedidas en la configuración que de verdad existen en ese video.
func (g *Gestor) opcionesVideo(s *sesVideo, cfg Config) []opVid {
	disp := s.Info.Alturas
	if len(disp) == 0 {
		return nil
	}
	sort.Slice(disp, func(i, j int) bool { return disp[i].Altura < disp[j].Altura })
	var out []opVid
	visto := map[int]bool{}
	for _, p := range alturasVideo(cfg.Texto("alturas")) {
		var mejor *infoAlt
		for i := range disp {
			if disp[i].Altura <= p {
				mejor = &disp[i]
			}
		}
		if mejor == nil && len(disp) > 0 && p == alturasVideo(cfg.Texto("alturas"))[0] {
			mejor = &disp[0]
		}
		if mejor == nil || visto[mejor.Altura] {
			continue
		}
		visto[mejor.Altura] = true
		out = append(out, opVid{Altura: mejor.Altura, Tam: mejor.Tam})
	}
	return out
}

func (g *Gestor) opcionesAudio(s *sesVideo, cfg Config) []opVid {
	abrs := append([]int(nil), s.Info.Audio.Abrs...)
	sort.Ints(abrs)
	alto := s.Info.Audio.Abr
	tam := s.Info.Audio.Tam
	if alto == 0 && len(abrs) > 0 {
		alto = abrs[len(abrs)-1]
	}
	var out []opVid
	if len(abrs) >= 2 {
		bajo := abrs[0]
		for _, a := range abrs {
			if a <= 128 {
				bajo = a
			}
		}
		if bajo != alto {
			t := tam
			if alto > 0 {
				t = tam * int64(bajo) / int64(alto)
			}
			out = append(out, opVid{Abr: bajo, Tam: t})
		}
	}
	out = append(out, opVid{Abr: alto, Tam: tam})
	return out
}

func (g *Gestor) videoPreguntarCalidad(s *sesVideo, cfg Config) {
	var ops []opVid
	max := int64(cfg.Int("max_total_mb")) << 20
	if s.Modo == "audio" {
		ops = g.opcionesAudio(s, cfg)
	} else {
		for _, o := range g.opcionesVideo(s, cfg) {
			if o.Tam == 0 || o.Tam <= max {
				ops = append(ops, o)
			}
		}
		if len(ops) == 0 && len(g.opcionesVideo(s, cfg)) > 0 {
			g.videoFalla(s, cfg, fmt.Sprintf("pesa más de %d MB aun con la calidad más baja", cfg.Int("max_total_mb")))
			g.vid().mu.Lock()
			delete(g.vid().ses, claveVideo(s.Chat, s.Persona))
			g.vid().mu.Unlock()
			return
		}
	}
	if len(ops) == 1 { // una sola calidad: no hay nada que preguntar
		g.videoElegirCalidad(s, cfg, ops[0])
		return
	}
	s.Etapa = "calidad"
	for i := range ops {
		if s.Modo == "audio" {
			ops[i].Texto = fmt.Sprintf("%s%s", etiquetaAbr(ops[i].Abr, i, len(ops)), pesoTxt(ops[i].Tam))
		} else {
			ops[i].Texto = fmt.Sprintf("%dp%s", ops[i].Altura, pesoTxt(ops[i].Tam))
		}
	}
	s.Opciones = ops
	g.videoMenu(s, fmt.Sprintf("*%s*\n⏱ %s · ¿En qué calidad?", cortaVideo(s.Info.Titulo, 80), duracionTxt(s.Info.Duracion)), textosOps(ops))
}

func etiquetaAbr(abr, i, n int) string {
	if n >= 2 && i == 0 {
		return fmt.Sprintf("Normal (%d kbps)", abr)
	}
	if abr > 0 {
		return fmt.Sprintf("Alta (%d kbps)", abr)
	}
	return "Mejor calidad"
}

func pesoTxt(b int64) string {
	if b <= 0 {
		return ""
	}
	return " · ~" + mbTxt(b)
}

func mbTxt(b int64) string {
	if b >= 1<<30 {
		return fmt.Sprintf("%.1f GB", float64(b)/float64(1<<30))
	}
	return fmt.Sprintf("%d MB", int(math.Max(1, math.Round(float64(b)/float64(1<<20)))))
}

func duracionTxt(s int) string {
	if s >= 3600 {
		return fmt.Sprintf("%d h %02d min", s/3600, (s%3600)/60)
	}
	return fmt.Sprintf("%d:%02d", s/60, s%60)
}

func cortaVideo(s string, n int) string {
	r := []rune(strings.TrimSpace(s))
	if len(r) > n {
		return string(r[:n]) + "…"
	}
	return string(r)
}

func textosOps(o []opVid) []string {
	out := make([]string, len(o))
	for i, x := range o {
		out[i] = x.Texto
	}
	return out
}

func (g *Gestor) videoElegirCalidad(s *sesVideo, cfg Config, o opVid) {
	s.Altura, s.Abr, s.TamEst = o.Altura, o.Abr, o.Tam
	envio := int64(cfg.Int("max_envio_mb")) << 20
	if s.TamEst <= 0 || s.TamEst <= envio {
		s.Entrega = "directo"
		if v := int64(cfg.Int("max_video_mb")) << 20; s.TamEst > v {
			g.videoTexto(s, fmt.Sprintf("Pesa unos %s, así que te lo mando como archivo: llega completo y sin comprimir (lo abres desde el chat).", mbTxt(s.TamEst)))
		}
		g.videoIniciar(s, cfg)
		return
	}
	partes := partesNecesarias(s.TamEst, envio)
	zipMax := int64(cfg.Int("max_zip_mb")) << 20
	puedeZip := s.TamEst <= zipMax
	fija := cfg.Texto("formato_partes")
	if fija == "zip" && !puedeZip {
		fija = "partes"
	}
	if fija == "zip" || fija == "partes" {
		s.Entrega = fija
		g.videoTexto(s, avisoPartes(s, envio, partes, fija))
		g.videoIniciar(s, cfg)
		return
	}
	s.Etapa = "entrega"
	s.Opciones = nil
	if puedeZip {
		s.Opciones = append(s.Opciones, opVid{Texto: "📦 Un solo archivo ZIP (puede tardar más)", Valor: "zip"})
	}
	ext := "mp4"
	if s.Modo == "audio" {
		ext = "audio"
	}
	s.Opciones = append(s.Opciones, opVid{Texto: fmt.Sprintf("🎞 %d partes (%s) que se ven por separado", partes, ext), Valor: "partes"})
	g.videoMenu(s, fmt.Sprintf("*%s*\nPesa unos %s y WhatsApp deja mandar hasta %s por archivo, así que lo mando dividido. ¿Cómo lo prefieres?",
		cortaVideo(s.Info.Titulo, 80), mbTxt(s.TamEst), mbTxt(envio)), textosOps(s.Opciones))
}

func partesNecesarias(tam, maxEnvio int64) int {
	util := float64(maxEnvio) * 0.85
	return int(math.Ceil(float64(tam) / util))
}

func avisoPartes(s *sesVideo, envio int64, partes int, entrega string) string {
	if entrega == "zip" {
		return fmt.Sprintf("Pesa unos %s, más de lo que WhatsApp deja mandar de una vez (%s). Te lo mando en un solo archivo ZIP.", mbTxt(s.TamEst), mbTxt(envio))
	}
	return fmt.Sprintf("Pesa unos %s, más de lo que WhatsApp deja mandar de una vez (%s). Te lo mando en %d partes.", mbTxt(s.TamEst), mbTxt(envio), partes)
}

// ---------- respuestas de quien pidió ----------

// VideoResponder lo llama la app cuando alguien toca un botón o vota la encuesta del diálogo. Devuelve true si había una descarga esperando
// esa respuesta (si no, el número sigue su camino normal).
func (g *Gestor) VideoResponder(chat, persona string, num int) bool {
	st := g.vid()
	st.mu.Lock()
	s := st.ses[claveVideo(chat, persona)]
	if s == nil || g.ahora().After(s.Vence) {
		if s != nil {
			delete(st.ses, claveVideo(chat, persona))
		}
		st.mu.Unlock()
		return false
	}
	if num < 1 || num > len(s.Opciones) {
		st.mu.Unlock()
		return true
	}
	op := s.Opciones[num-1]
	etapa := s.Etapa
	delete(st.ses, claveVideo(chat, persona))
	st.mu.Unlock()
	cfg := g.Config(IDVideo)
	s.Vence = g.ahora().Add(time.Duration(cfg.Int("espera_min")) * time.Minute)
	switch etapa {
	case "tipo":
		s.Modo = op.Valor
		g.vid().mu.Lock()
		g.vid().ses[claveVideo(chat, persona)] = s
		g.vid().mu.Unlock()
		go g.videoPreguntarCalidad(s, cfg)
	case "calidad":
		g.vid().mu.Lock()
		g.vid().ses[claveVideo(chat, persona)] = s
		g.vid().mu.Unlock()
		go g.videoElegirCalidad(s, cfg, op)
	case "entrega":
		s.Entrega = op.Valor
		go func() {
			g.videoTexto(s, avisoPartes(s, int64(cfg.Int("max_envio_mb"))<<20, partesNecesarias(s.TamEst, int64(cfg.Int("max_envio_mb"))<<20), s.Entrega))
			g.videoIniciar(s, cfg)
		}()
	}
	return true
}

func eventoVideo(g *Gestor, e Evento) {
	switch e.Tipo {
	case EvMensaje:
		if e.EsMio || e.Chat == "" || e.Persona == "" {
			return
		}
		t := strings.TrimSpace(e.Texto)
		t = strings.TrimRight(t, ".)")
		n, err := strconv.Atoi(t)
		if err != nil || n < 1 || n > 9 {
			return
		}
		g.VideoResponder(e.Chat, e.Persona, n)
	case EvTick:
		g.videoRevisionDiaria(e.Hora)
	}
}
