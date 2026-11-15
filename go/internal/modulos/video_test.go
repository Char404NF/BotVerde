package modulos

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"botchat/internal/almacen"
	"botchat/internal/comandos"
)

// motorFalso es un MotorVideo que no usa Python: crea archivos del tamaño que se le pida.
type motorFalso struct {
	mu         sync.Mutex
	dir        string
	info       string
	errInfo    error
	tam        int64 // bytes del archivo "bajado"
	errBajar   error
	bajadas    []string
	versionAct string
	nuevaVer   string
	errInst    error
	instalados int
	sinMp3     bool
	errPartir  error
}

func (m *motorFalso) Version() (string, error) { return m.versionAct, nil }
func (m *motorFalso) Info(url string) (string, error) {
	if m.errInfo != nil {
		return "", m.errInfo
	}
	return m.info, nil
}
func (m *motorFalso) Descargar(url, carpeta, modo string, altura, abr int) (string, error) {
	m.mu.Lock()
	m.bajadas = append(m.bajadas, fmt.Sprintf("%s|%s|%d|%d", url, modo, altura, abr))
	m.mu.Unlock()
	if m.errBajar != nil {
		return "", m.errBajar
	}
	_ = os.MkdirAll(carpeta, 0o700)
	ext := "mp4"
	if modo == "audio" {
		ext = "m4a"
	}
	r := filepath.Join(carpeta, "v_abc."+ext)
	if err := os.WriteFile(r, make([]byte, m.tam), 0o600); err != nil {
		return "", err
	}
	j, _ := json.Marshal(map[string]any{"ruta": r, "tam": m.tam, "dur": 120, "tipo": modo})
	return string(j), nil
}
func (m *motorFalso) Convertir(ruta, formato string) (string, error) {
	if m.sinMp3 {
		return "", fmt.Errorf("sin conversor")
	}
	n := strings.TrimSuffix(ruta, filepath.Ext(ruta)) + "." + formato
	return n, os.WriteFile(n, make([]byte, m.tam/2+1), 0o600)
}
func (m *motorFalso) Partir(ruta string, n int, tope int64, carpeta, base string) (string, error) {
	if m.errPartir != nil {
		return "", m.errPartir
	}
	var out []string
	for i := 0; i < n; i++ {
		p := filepath.Join(carpeta, fmt.Sprintf("%s_parte%02d.mp4", base, i+1))
		_ = os.WriteFile(p, make([]byte, m.tam/int64(n)), 0o600)
		out = append(out, p)
	}
	j, _ := json.Marshal(out)
	return string(j), nil
}
func (m *motorFalso) Zip(rutasJSON, salida string) error {
	return os.WriteFile(salida, []byte("zip"), 0o600)
}
func (m *motorFalso) Instalar(ruta string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.errInst != nil {
		return "", m.errInst
	}
	m.instalados++
	m.versionAct = m.nuevaVer
	return m.nuevaVer, nil
}
func (m *motorFalso) Carpeta() string          { return m.dir }
func (m *motorFalso) Borrar(ruta string) error { return os.RemoveAll(ruta) }

const infoCorta = `{"version":"2026.10.26","titulo":"Gatito feliz","subidor":"Canal","duracion":120,"miniatura":"","extractor":"Youtube","en_vivo":false,"edad":0,"id":"abc",
 "url":"https://youtu.be/abc","alturas":[{"altura":360,"tam":20971520,"directo":true},{"altura":720,"tam":62914560,"directo":false},{"altura":1080,"tam":209715200,"directo":false}],
 "audio":{"abr":160,"tam":3145728,"abrs":[96,160]}}`

func videoLista(t *testing.T, info string, tam int64) (*ent, *accFalsa, *motorFalso) {
	t.Helper()
	e, acc := entH(t)
	m := &motorFalso{dir: t.TempDir(), info: info, tam: tam, versionAct: "2026.10.26", nuevaVer: "2026.11.01"}
	e.g.Video = m
	e.activa(IDVideo)
	e.config(IDVideo, map[string]any{"acepto_riesgos": true, "max_envio_mb": 90.0, "max_video_mb": 90.0, "max_zip_mb": 1500.0, "max_total_mb": 800.0})
	return e, acc, m
}

func cmdVideo(t *testing.T, e *ent, nombre string) comandos.Comando {
	t.Helper()
	m, _ := Obtener(IDVideo)
	for _, c := range m.(ComandosDeGestor).ComandosG(e.g) {
		if c.Nombre == nombre {
			return c
		}
	}
	t.Fatalf("no hay el comando %s", nombre)
	return comandos.Comando{}
}

func (e *ent) esperaVideo(cond func() bool) {
	e.t.Helper()
	for i := 0; i < 600; i++ {
		e.avanza(6 * time.Second)
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	e.t.Fatalf("no pasó lo esperado a tiempo; enviado=%v", e.enviados())
}

func (a *accFalsa) ultimoMenu() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	if len(a.menus) == 0 {
		return ""
	}
	return a.menus[len(a.menus)-1]
}

func (a *accFalsa) nMenus() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return len(a.menus)
}

func pideVideo(t *testing.T, e *ent, cmd, enlace string) string {
	t.Helper()
	r, err := cmdVideo(t, e, cmd).Ejecutar(&comandos.Contexto{Chat: pnAna, Persona: pnAna, Nombre: "Ana", MensajeID: "M1", Resto: enlace, Args: strings.Fields(enlace)})
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestVideoNaceApagadoYDeRiesgoAlto(t *testing.T) {
	e, _ := entH(t)
	m, ok := Obtener(IDVideo)
	if !ok || m.Riesgo() != RiesgoAlto || !m.Iniciativa() {
		t.Fatal("el módulo de descargas debe existir, ser de riesgo alto y de iniciativa")
	}
	if e.g.Activo(IDVideo) {
		t.Fatal("tiene que nacer apagado")
	}
	if e.g.Config(IDVideo).Bool("acepto_riesgos") {
		t.Fatal("los riesgos no pueden venir aceptados")
	}
	if c := e.g.Config(IDVideo); c.Int("max_envio_mb") != 1000 || c.Int("max_video_mb") != 90 || c.Texto("audio_formato") != "mp3" || !c.Bool("actualizar_auto") {
		t.Fatalf("valores de fábrica: %v", c)
	}
}

func TestVideoPideAceptarRiesgosYValidaElEnlace(t *testing.T) {
	e, _, _ := videoLista(t, infoCorta, 1000)
	e.config(IDVideo, map[string]any{"acepto_riesgos": false})
	_, err := cmdVideo(t, e, "descargar").Ejecutar(&comandos.Contexto{Chat: pnAna, Persona: pnAna, Resto: "https://youtu.be/abc"})
	esperaErr(t, err, "aceptó los riesgos")
	e.config(IDVideo, map[string]any{"acepto_riesgos": true})
	for enlace, quiere := range map[string]string{
		"":                           "escribe el enlace",
		"https://pornhub.com/view":   "no está permitido",
		"http://127.0.0.1/x.mp4":     "sitios de internet",
		"https://localhost/video":    "sitios de internet",
		"ftp://sitio.com/video":      "no parece válido",
		"https://es.pornhub.com/abc": "no está permitido",
	} {
		_, err := cmdVideo(t, e, "descargar").Ejecutar(&comandos.Contexto{Chat: pnAna, Persona: pnAna, Resto: enlace})
		esperaErr(t, err, quiere)
	}
	e.config(IDVideo, map[string]any{"sitios": "tiktok.com"})
	_, err = cmdVideo(t, e, "descargar").Ejecutar(&comandos.Contexto{Chat: pnAna, Persona: pnAna, Resto: "https://youtu.be/abc"})
	esperaErr(t, err, "no está permitido")
}

func TestVideoFlujoCompletoDeUnVideoChico(t *testing.T) {
	e, acc, m := videoLista(t, infoCorta, 5<<20)
	if r := pideVideo(t, e, "descargar", "https://youtu.be/abc"); !strings.Contains(r, "Revisando") {
		t.Fatalf("respuesta inmediata: %q", r)
	}
	e.esperaVideo(func() bool { return acc.nMenus() == 1 })
	if mn := acc.ultimoMenu(); !strings.Contains(mn, "Gatito feliz") || !strings.Contains(mn, "1. 🎬 Video") || !strings.Contains(mn, "2. 🎵 Solo el audio") {
		t.Fatalf("primer menú: %q", mn)
	}
	if !e.g.VideoResponder(pnAna, pnAna, 1) {
		t.Fatal("la respuesta debía atenderse")
	}
	e.esperaVideo(func() bool { return acc.nMenus() == 2 })
	mn := acc.ultimoMenu()
	if !strings.Contains(mn, "360p") || !strings.Contains(mn, "720p") || !strings.Contains(mn, "1080p") || !strings.Contains(mn, "~20 MB") {
		t.Fatalf("menú de calidades: %q", mn)
	}
	e.g.VideoResponder(pnAna, pnAna, 2) // 720p
	e.esperaVideo(func() bool { return e.hay("[medio:video]") })
	if len(m.bajadas) != 1 || !strings.HasSuffix(m.bajadas[0], "|video|720|0") {
		t.Fatalf("bajadas: %v", m.bajadas)
	}
	e.esperaVideo(func() bool { return e.hay("Listo") })
	h, _ := e.alm.ListarDescargasVideo(1, 10, 0)
	if len(h) != 1 || h[0].Estado != "enviado" || h[0].Titulo != "Gatito feliz" || h[0].URL != "https://youtu.be/abc" || h[0].Persona != pnAna || h[0].Calidad != "720p" || h[0].Formato != "mp4" || h[0].Partes != 1 {
		t.Fatalf("historial: %+v", h)
	}
	if _, err := os.Stat(filepath.Join(m.dir, "trabajo")); err == nil {
		ents, _ := os.ReadDir(filepath.Join(m.dir, "trabajo"))
		if len(ents) != 0 {
			t.Fatalf("quedaron archivos de trabajo: %v", ents)
		}
	}
}

func TestVideoRechazaLoLargoLoEnVivoYLoDeAdultos(t *testing.T) {
	largo := strings.Replace(infoCorta, `"duracion":120`, `"duracion":7200`, 1)
	e, _, _ := videoLista(t, largo, 1000)
	pideVideo(t, e, "descargar", "https://youtu.be/abc")
	e.esperaVideo(func() bool { return e.hay("dura 2 h 00 min y el máximo es 60 min") })
	vivo := strings.Replace(infoCorta, `"en_vivo":false`, `"en_vivo":true`, 1)
	e2, _, _ := videoLista(t, vivo, 1000)
	pideVideo(t, e2, "descargar", "https://youtu.be/abc")
	e2.esperaVideo(func() bool { return e2.hay("transmisión en vivo") })
	adulto := strings.Replace(infoCorta, `"edad":0`, `"edad":18`, 1)
	e3, _, _ := videoLista(t, adulto, 1000)
	pideVideo(t, e3, "descargar", "https://youtu.be/abc")
	e3.esperaVideo(func() bool { return e3.hay("para adultos") })
}

func TestVideoAvisaQueEsPesadoYDejaElegirZipOPartes(t *testing.T) {
	e, acc, m := videoLista(t, infoCorta, 200<<20)
	pideVideo(t, e, "descargar", "https://youtu.be/abc")
	e.esperaVideo(func() bool { return acc.nMenus() == 1 })
	e.g.VideoResponder(pnAna, pnAna, 1)
	e.esperaVideo(func() bool { return acc.nMenus() == 2 })
	e.g.VideoResponder(pnAna, pnAna, 3) // 1080p ≈ 200 MB
	e.esperaVideo(func() bool { return acc.nMenus() == 3 })
	mn := acc.ultimoMenu()
	if !strings.Contains(mn, "Pesa unos 200 MB") || !strings.Contains(mn, "hasta 90 MB") || !strings.Contains(mn, "1. 📦 Un solo archivo ZIP") || !strings.Contains(mn, "2. 🎞 3 partes (mp4)") {
		t.Fatalf("menú de entrega: %q", mn)
	}
	e.g.VideoResponder(pnAna, pnAna, 2) // en partes
	e.esperaVideo(func() bool { return e.hay("3 partes") })
	e.esperaVideo(func() bool { return e.hay("Listo") })
	n := 0
	for _, x := range e.enviados() {
		if strings.Contains(x, "[medio:video]") {
			n++
		}
	}
	if n != 3 {
		t.Fatalf("debían salir 3 partes y salieron %d: %v", n, e.enviados())
	}
	h, _ := e.alm.ListarDescargasVideo(1, 10, 0)
	if len(h) != 1 || h[0].Partes != 3 || h[0].Entrega != "partes" || h[0].Estado != "enviado" {
		t.Fatalf("historial: %+v", h)
	}
	if ents, _ := os.ReadDir(filepath.Join(m.dir, "trabajo")); len(ents) != 0 {
		t.Fatalf("quedaron archivos de trabajo: %v", ents)
	}
}

func TestVideoEnUnZip(t *testing.T) {
	e, acc, _ := videoLista(t, infoCorta, 200<<20)
	pideVideo(t, e, "descargar", "https://youtu.be/abc")
	e.esperaVideo(func() bool { return acc.nMenus() == 1 })
	e.g.VideoResponder(pnAna, pnAna, 1)
	e.esperaVideo(func() bool { return acc.nMenus() == 2 })
	e.g.VideoResponder(pnAna, pnAna, 3)
	e.esperaVideo(func() bool { return acc.nMenus() == 3 })
	e.g.VideoResponder(pnAna, pnAna, 1)
	e.esperaVideo(func() bool { return e.hay("[medio:documento]") })
	e.esperaVideo(func() bool { return e.hay("Listo") })
	h, _ := e.alm.ListarDescargasVideo(1, 10, 0)
	if len(h) != 1 || h[0].Entrega != "zip" || h[0].Partes != 1 {
		t.Fatalf("historial: %+v", h)
	}
}

func TestVideoFormatoFijoNoPregunta(t *testing.T) {
	e, acc, _ := videoLista(t, infoCorta, 200<<20)
	e.config(IDVideo, map[string]any{"formato_partes": "partes"})
	pideVideo(t, e, "descargar", "https://youtu.be/abc")
	e.esperaVideo(func() bool { return acc.nMenus() == 1 })
	e.g.VideoResponder(pnAna, pnAna, 1)
	e.esperaVideo(func() bool { return acc.nMenus() == 2 })
	e.g.VideoResponder(pnAna, pnAna, 3)
	e.esperaVideo(func() bool { return e.hay("Listo") })
	if acc.nMenus() != 2 {
		t.Fatalf("no debía volver a preguntar: %d menús", acc.nMenus())
	}
}

func TestVideoAudioEnMp3ConDosCalidades(t *testing.T) {
	e, acc, m := videoLista(t, infoCorta, 4<<20)
	pideVideo(t, e, "audio", "https://youtu.be/abc")
	e.esperaVideo(func() bool { return acc.nMenus() == 1 })
	mn := acc.ultimoMenu()
	if !strings.Contains(mn, "1. Normal (96 kbps)") || !strings.Contains(mn, "2. Alta (160 kbps)") {
		t.Fatalf("calidades de audio: %q", mn)
	}
	e.g.VideoResponder(pnAna, pnAna, 2)
	e.esperaVideo(func() bool { return e.hay("[medio:audio]") })
	e.esperaVideo(func() bool { return e.hay("Listo") })
	if len(m.bajadas) != 1 || !strings.HasSuffix(m.bajadas[0], "|audio|0|160") {
		t.Fatalf("bajadas: %v", m.bajadas)
	}
	h, _ := e.alm.ListarDescargasVideo(1, 10, 0)
	if len(h) != 1 || h[0].Formato != "mp3" || h[0].Calidad != "160 kbps" {
		t.Fatalf("historial: %+v", h)
	}
}

func TestVideoSiNoSePuedeConvertirSaleEnM4a(t *testing.T) {
	e, acc, m := videoLista(t, infoCorta, 4<<20)
	m.sinMp3 = true
	pideVideo(t, e, "audio", "https://youtu.be/abc")
	e.esperaVideo(func() bool { return acc.nMenus() == 1 })
	e.g.VideoResponder(pnAna, pnAna, 1)
	e.esperaVideo(func() bool { return e.hay("[medio:audio]") })
	e.esperaVideo(func() bool { return e.hay("Listo") })
}

func TestVideoLimitesPorPersonaYDelDueno(t *testing.T) {
	e, _, _ := videoLista(t, infoCorta, 1000)
	e.config(IDVideo, map[string]any{"por_persona_hora": 2.0})
	for i := 0; i < 2; i++ {
		if _, err := e.alm.NuevaDescargaVideo(1, almacen.DescargaVideo{TS: e.hora.Unix(), Persona: pnAna, Chat: pnAna, URL: "https://youtu.be/abc", Titulo: "Gatito feliz", Formato: "mp4", Calidad: "360p", Estado: "enviado", Entrega: "directo", Peso: 20 << 20}); err != nil {
			t.Fatal(err)
		}
	}
	_, err := cmdVideo(t, e, "descargar").Ejecutar(&comandos.Contexto{Chat: pnAna, Persona: pnAna, Resto: "https://youtu.be/abc"})
	esperaErr(t, err, "ya pediste 2 descargas en esta hora")
	if _, err := cmdVideo(t, e, "descargar").Ejecutar(&comandos.Contexto{Chat: pnAna, Persona: pnAna, Resto: "https://youtu.be/abc", Dueno: true}); err != nil {
		t.Fatalf("el Dueño no tiene límite: %v", err)
	}
	// otra persona no se ve afectada
	if _, err := cmdVideo(t, e, "descargar").Ejecutar(&comandos.Contexto{Chat: pnAna, Persona: "otra", Resto: "https://youtu.be/abc"}); err != nil {
		t.Fatalf("otra persona: %v", err)
	}
}

func TestVideoErrorDelSitioSeExplicaEnEspanol(t *testing.T) {
	e, _, m := videoLista(t, infoCorta, 1000)
	m.errInfo = fmt.Errorf("DownloadError: ERROR: [youtube] abc: Sign in to confirm you're not a bot")
	pideVideo(t, e, "descargar", "https://youtu.be/abc")
	e.esperaVideo(func() bool { return e.hay("privado o pide iniciar sesión") })
}

func TestVideoUnaFallaDeDescargaQuedaEnElHistorial(t *testing.T) {
	e, acc, m := videoLista(t, infoCorta, 1000)
	m.errBajar = fmt.Errorf("HTTP Error 403: Forbidden")
	pideVideo(t, e, "descargar", "https://youtu.be/abc")
	e.esperaVideo(func() bool { return acc.nMenus() == 1 })
	e.g.VideoResponder(pnAna, pnAna, 1)
	e.esperaVideo(func() bool { return acc.nMenus() == 2 })
	e.g.VideoResponder(pnAna, pnAna, 1)
	e.esperaVideo(func() bool { return e.hay("No pude descargarlo") })
	h, _ := e.alm.ListarDescargasVideo(1, 10, 0)
	if len(h) != 1 || h[0].Estado != "fallido" || h[0].Error == "" {
		t.Fatalf("historial: %+v", h)
	}
}

func TestVideoLaRespuestaVenceYUnNumeroSueltoNoHaceNada(t *testing.T) {
	e, acc, _ := videoLista(t, infoCorta, 1000)
	if e.g.VideoResponder(pnAna, pnAna, 1) {
		t.Fatal("sin nada esperando no debe atender el número")
	}
	pideVideo(t, e, "descargar", "https://youtu.be/abc")
	e.esperaVideo(func() bool { return acc.nMenus() == 1 })
	e.hora = e.hora.Add(10 * time.Minute)
	if e.g.VideoResponder(pnAna, pnAna, 1) {
		t.Fatal("la respuesta ya había vencido")
	}
	// otra persona en el mismo chat no contesta por ella
	pideVideo(t, e, "descargar", "https://youtu.be/abc")
	e.esperaVideo(func() bool { return acc.nMenus() == 2 })
	if e.g.VideoResponder(pnAna, "intruso", 1) {
		t.Fatal("otra persona no puede elegir")
	}
}

func TestVideoRespuestaEscritaTambienFunciona(t *testing.T) {
	e, acc, _ := videoLista(t, infoCorta, 1000)
	pideVideo(t, e, "descargar", "https://youtu.be/abc")
	e.esperaVideo(func() bool { return acc.nMenus() == 1 })
	e.g.Evento(Evento{Tipo: EvMensaje, Chat: pnAna, Persona: pnAna, Texto: " 2. "})
	e.esperaVideo(func() bool { return acc.nMenus() == 2 })
	if !strings.Contains(acc.ultimoMenu(), "Normal (96 kbps)") {
		t.Fatalf("debía pasar a las calidades de audio: %q", acc.ultimoMenu())
	}
}

// ---------- yt-dlp al día ----------

func releaseFalso(tag string, bin []byte, sumaMala bool) func(p PeticionHTTP) (*RespuestaHTTP, error) {
	h := sha256.Sum256(bin)
	suma := hex.EncodeToString(h[:])
	if sumaMala {
		suma = strings.Repeat("0", 64)
	}
	return func(p PeticionHTTP) (*RespuestaHTTP, error) {
		switch {
		case strings.HasSuffix(p.URL, "/releases/latest"):
			return cuerpo("application/json", "", fmt.Sprintf(`{"tag_name":%q,"assets":[{"name":"yt-dlp","browser_download_url":"https://github.com/yt-dlp/yt-dlp/releases/download/%s/yt-dlp"},{"name":"SHA2-256SUMS","browser_download_url":"https://github.com/yt-dlp/yt-dlp/releases/download/%s/SHA2-256SUMS"}]}`, tag, tag, tag)), nil
		case strings.HasSuffix(p.URL, "/yt-dlp"):
			return cuerpo("application/octet-stream", "yt-dlp", string(bin)), nil
		case strings.HasSuffix(p.URL, "/SHA2-256SUMS"):
			return cuerpo("text/plain", "", suma+"  yt-dlp\n"+strings.Repeat("a", 64)+"  yt-dlp.exe\n"), nil
		}
		return nil, fmt.Errorf("url inesperada %s", p.URL)
	}
}

func TestVideoActualizaYtdlpVerificandoLaFirma(t *testing.T) {
	e, _, m := videoLista(t, infoCorta, 1000)
	d := &descFalso{}
	e.g.Descarga = d
	d.resp = releaseFalso("2026.11.01", []byte("contenido-de-yt-dlp"), false)
	r, err := e.g.VideoActualizar(false)
	if err != nil || !strings.Contains(r, "2026.11.01") || m.instalados != 1 {
		t.Fatalf("%q %v instalados=%d", r, err, m.instalados)
	}
	if e.g.Alm.AjusteBot(1, "video_ytdlp_ver") != "2026.11.01" {
		t.Fatal("no guardó la versión")
	}
	if _, err := os.Stat(filepath.Join(m.dir, "yt-dlp.nuevo")); err == nil {
		t.Fatal("quedó el archivo temporal")
	}
	// ya está al día: no vuelve a instalar
	r, err = e.g.VideoActualizar(false)
	if err != nil || !strings.Contains(r, "ya es la última") || m.instalados != 1 {
		t.Fatalf("%q %v", r, err)
	}
	est := e.g.VideoEstado()
	if est.Version != "2026.11.01" || est.Hay || !est.Motor || !est.Activo {
		t.Fatalf("estado: %+v", est)
	}
}

func TestVideoNoInstalaSiLaFirmaNoCoincide(t *testing.T) {
	e, _, m := videoLista(t, infoCorta, 1000)
	d := &descFalso{}
	e.g.Descarga = d
	d.resp = releaseFalso("2026.11.01", []byte("algo-raro"), true)
	_, err := e.g.VideoActualizar(false)
	esperaErr(t, err, "no coincide con su firma")
	if m.instalados != 0 {
		t.Fatal("no debía instalar")
	}
}

func TestVideoSiLaVersionNuevaNoArrancaQuedaLaAnterior(t *testing.T) {
	e, _, m := videoLista(t, infoCorta, 1000)
	m.errInst = fmt.Errorf("falló al importar")
	d := &descFalso{}
	e.g.Descarga = d
	d.resp = releaseFalso("2026.11.01", []byte("roto"), false)
	_, err := e.g.VideoActualizar(false)
	esperaErr(t, err, "dejé la anterior")
	if m.versionAct != "2026.10.26" {
		t.Fatal("tenía que quedar la versión anterior")
	}
}

func TestVideoVersionesSeComparanComoFechas(t *testing.T) {
	casos := []struct {
		a, b   string
		quiere bool
	}{{"2026.11.01", "2026.10.26", true}, {"2026.10.26", "2026.10.26", false}, {"2026.10.26.1", "2026.10.26", true}, {"2026.9.30", "2026.10.1", false}}
	for _, c := range casos {
		if versionMayor(c.a, c.b) != c.quiere {
			t.Fatalf("%s vs %s", c.a, c.b)
		}
	}
}

func TestVideoRevisionDiariaSoloUnaVezALaHora(t *testing.T) {
	e, _, m := videoLista(t, infoCorta, 1000)
	d := &descFalso{}
	e.g.Descarga = d
	d.resp = releaseFalso("2026.11.01", []byte("nuevo"), false)
	e.config(IDVideo, map[string]any{"hora_actualizar": "12:00"})
	e.tick()
	e.tick()
	for i := 0; i < 200 && m.instalados == 0; i++ {
		time.Sleep(10 * time.Millisecond)
	}
	if m.instalados != 1 {
		t.Fatalf("instalados=%d", m.instalados)
	}
	time.Sleep(100 * time.Millisecond)
	if m.instalados != 1 {
		t.Fatalf("la revisión de ese día se repitió: %d", m.instalados)
	}
}

func TestVideoRepetirUnaDescargaDelHistorial(t *testing.T) {
	e, _, m := videoLista(t, infoCorta, 5<<20)
	id, err := e.alm.NuevaDescargaVideo(1, almacen.DescargaVideo{TS: e.hora.Unix(), Persona: pnAna, Chat: pnAna, URL: "https://youtu.be/abc", Titulo: "Gatito feliz", Formato: "mp4", Calidad: "360p", Estado: "enviado", Entrega: "directo", Peso: 20 << 20})
	if err != nil {
		t.Fatal(err)
	}
	r, err := e.g.VideoRepetir(id)
	if err != nil || !strings.Contains(r, "Volviendo a descargar") {
		t.Fatalf("%q %v", r, err)
	}
	e.esperaVideo(func() bool { return e.hay("[medio:video]") })
	if len(m.bajadas) != 1 {
		t.Fatalf("bajadas: %v", m.bajadas)
	}
}

func TestVideoElHistorialSeCifraYSePuedeBorrar(t *testing.T) {
	e, _, _ := videoLista(t, infoCorta, 1000)
	id, _ := e.alm.NuevaDescargaVideo(1, almacen.DescargaVideo{TS: e.hora.Unix(), Persona: pnAna, Chat: pnAna, URL: "https://youtu.be/abc", Titulo: "Gatito feliz", Formato: "mp4", Calidad: "360p", Estado: "enviado", Entrega: "directo", Peso: 20 << 20})
	if e.alm.ContarDescargasVideo(1, pnAna, 0) != 1 {
		t.Fatal("debía contar 1")
	}
	if err := e.alm.BorrarDescargaVideo(1, id); err != nil {
		t.Fatal(err)
	}
	if e.alm.ContarDescargasVideo(1, "", 0) != 0 {
		t.Fatal("no se borró")
	}
}

func TestVideoSiNoSePuedePartirSaleEnZip(t *testing.T) {
	e, acc, m := videoLista(t, infoCorta, 200<<20)
	m.errPartir = fmt.Errorf("una de las partes no quedó bien")
	e.config(IDVideo, map[string]any{"formato_partes": "partes"})
	pideVideo(t, e, "descargar", "https://youtu.be/abc")
	e.esperaVideo(func() bool { return acc.nMenus() == 1 })
	e.g.VideoResponder(pnAna, pnAna, 1)
	e.esperaVideo(func() bool { return acc.nMenus() == 2 })
	e.g.VideoResponder(pnAna, pnAna, 3)
	e.esperaVideo(func() bool { return e.hay("[medio:documento]") })
	e.esperaVideo(func() bool { return e.hay("Listo") })
	if !e.hay("no pude dividirlo en partes, así que te lo mando en un solo archivo ZIP") && !e.hay("No pude dividirlo en partes") {
		t.Fatalf("debía avisar del cambio a zip: %v", e.enviados())
	}
	h, _ := e.alm.ListarDescargasVideo(1, 10, 0)
	if len(h) != 1 || h[0].Entrega != "zip" || h[0].Estado != "enviado" {
		t.Fatalf("historial: %+v", h)
	}
}

func TestVideoAvisaSiAlFinalPesoOtraCosa(t *testing.T) {
	// el menú dijo ~200 MB (3 partes) pero el archivo real pesó la mitad: se avisa cuántas partes salieron de verdad
	e, acc, m := videoLista(t, infoCorta, 100<<20)
	e.config(IDVideo, map[string]any{"formato_partes": "partes"})
	pideVideo(t, e, "descargar", "https://youtu.be/abc")
	e.esperaVideo(func() bool { return acc.nMenus() == 1 })
	e.g.VideoResponder(pnAna, pnAna, 1)
	e.esperaVideo(func() bool { return acc.nMenus() == 2 })
	e.g.VideoResponder(pnAna, pnAna, 3)
	e.esperaVideo(func() bool { return e.hay("Al final pesó") })
	e.esperaVideo(func() bool { return e.hay("Listo") })
	_ = m
}

func TestVideoLoQuePasaDelPesoDeVideoSaleComoArchivo(t *testing.T) {
	e, acc, _ := videoLista(t, infoCorta, 150<<20)
	e.config(IDVideo, map[string]any{"max_envio_mb": 1000.0, "max_video_mb": 90.0})
	pideVideo(t, e, "descargar", "https://youtu.be/abc")
	e.esperaVideo(func() bool { return acc.nMenus() == 1 })
	e.g.VideoResponder(pnAna, pnAna, 1)
	e.esperaVideo(func() bool { return acc.nMenus() == 2 })
	e.g.VideoResponder(pnAna, pnAna, 3) // 1080p ≈ 200 MB: cabe en un archivo (hasta 1000 MB) pero no como video (90 MB)
	e.esperaVideo(func() bool { return e.hay("[medio:documento]") })
	e.esperaVideo(func() bool { return e.hay("Listo") })
	if acc.nMenus() != 2 {
		t.Fatalf("no debía preguntar zip o partes: %d menús", acc.nMenus())
	}
	if !e.hay("te lo mando como archivo") {
		t.Fatalf("debía avisar que sale como archivo: %v", e.enviados())
	}
}

func TestVideoLoChicoSaleComoVideo(t *testing.T) {
	e, acc, _ := videoLista(t, infoCorta, 30<<20)
	pideVideo(t, e, "descargar", "https://youtu.be/abc")
	e.esperaVideo(func() bool { return acc.nMenus() == 1 })
	e.g.VideoResponder(pnAna, pnAna, 1)
	e.esperaVideo(func() bool { return acc.nMenus() == 2 })
	e.g.VideoResponder(pnAna, pnAna, 1)
	e.esperaVideo(func() bool { return e.hay("[medio:video]") })
	if e.hay("[medio:documento]") {
		t.Fatal("lo chico tiene que salir como video")
	}
}
