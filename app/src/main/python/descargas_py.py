"""Puente de las descargas de video. Es lo único que corre en Python y solo lo llama el módulo (Kotlin) cuando alguien pide una descarga.

Todo entra y sale como texto JSON: así Kotlin y el motor Go no dependen de cómo es yt-dlp por dentro.
"""
import json
import os
import sys

_ESTADO = {"ruta": None}


def _limpiar_modulos():
    for k in [m for m in sys.modules if m == "yt_dlp" or m.startswith("yt_dlp.") or m == "yt_dlp_ejs" or m.startswith("yt_dlp_ejs.")]:
        del sys.modules[k]
    import importlib
    importlib.invalidate_caches()


def usar_ytdlp(ruta, forzar=False):
    """Usa el yt-dlp descargado (un archivo zip ejecutable) en lugar del que viene dentro de la app; sin ruta vuelve al de la app.
    Devuelve la versión que quedó (si no se puede importar, lanza el error y deja las cosas como estaban)."""
    previa = _ESTADO["ruta"]
    if ruta and os.path.isfile(ruta) and (forzar or previa != ruta):
        if previa and previa in sys.path:
            sys.path.remove(previa)
        sys.path_importer_cache.pop(ruta, None)
        try:
            import zipimport
            zipimport._zip_directory_cache.pop(ruta, None)
        except Exception:
            pass
        sys.path.insert(0, ruta)
        _limpiar_modulos()
        _ESTADO["ruta"] = ruta
        try:
            return version()
        except Exception:
            sys.path.remove(ruta)
            _ESTADO["ruta"] = None
            _limpiar_modulos()
            if previa and os.path.isfile(previa):
                usar_ytdlp(previa, True)
            raise
    if not ruta and previa:
        if previa in sys.path:
            sys.path.remove(previa)
        _ESTADO["ruta"] = None
        _limpiar_modulos()
    return version()


def version():
    import yt_dlp
    return yt_dlp.version.__version__


def _opciones(qjs, extra=None):
    o = {
        "quiet": True, "no_warnings": True, "noplaylist": True, "socket_timeout": 25, "retries": 3, "fragment_retries": 3,
        "extractor_retries": 2, "color": "never", "noprogress": True, "geo_bypass": True, "playlist_items": "1",
    }
    if qjs:
        o["js_runtimes"] = {"quickjs": {"path": qjs}}
    if extra:
        o.update(extra)
    return o


def _tam(f, dur):
    t = f.get("filesize") or f.get("filesize_approx")
    if not t and f.get("tbr") and dur:
        t = int(f["tbr"] * 1000 / 8 * dur)
    return int(t or 0)


def _miniatura(d):
    """La portada en jpg o png (el motor no lee webp): la más grande que haya, o la que traiga el sitio."""
    mejores = [t for t in (d.get("thumbnails") or []) if t.get("url") and str(t.get("url")).split("?")[0].lower().endswith((".jpg", ".jpeg", ".png"))]
    mejores.sort(key=lambda t: (t.get("width") or 0) * (t.get("height") or 0))
    # una de tamaño mediano (no la gigante) basta para una miniatura de 160 px
    medias = [t for t in mejores if 200 <= (t.get("width") or 0) <= 800]
    if medias:
        return medias[-1]["url"]
    if mejores:
        return mejores[0]["url"]
    return d.get("thumbnail") or ""


def info(url, qjs=None):
    """Datos del enlace SIN descargar: título, duración, miniatura y, por cada altura, cuánto pesaría el video; y el audio."""
    import yt_dlp
    with yt_dlp.YoutubeDL(_opciones(qjs, {"skip_download": True})) as y:
        d = y.extract_info(url, download=False)
    if d.get("_type") == "playlist" and d.get("entries"):
        d = next((e for e in d["entries"] if e), d)
    dur = d.get("duration") or 0
    fs = d.get("formats") or []
    audios = [f for f in fs if f.get("vcodec") in (None, "none") and f.get("acodec") not in (None, "none")]
    m4a = [f for f in audios if f.get("ext") == "m4a"] or audios
    mejor_audio = max(m4a, key=lambda f: f.get("abr") or f.get("tbr") or 0, default=None)
    tam_audio = _tam(mejor_audio, dur) if mejor_audio else 0
    alturas = {}
    for f in fs:
        h = f.get("height")
        if not h or f.get("vcodec") in (None, "none"):
            continue
        avc = str(f.get("vcodec") or "").startswith("avc1")
        solo_video = f.get("acodec") in (None, "none")
        if solo_video and not avc:
            continue  # sin sonido y sin H.264: no se puede unir en un mp4 con las herramientas de Android
        if not solo_video and f.get("ext") != "mp4":
            continue
        tam = _tam(f, dur) + (tam_audio if solo_video else 0)
        previo = alturas.get(h)
        # por altura se queda con la opción más liviana que sirve
        if previo is None or (tam and (not previo["tam"] or tam < previo["tam"])):
            alturas[h] = {"altura": int(h), "tam": int(tam), "directo": not solo_video}
    abrs = sorted({int(f.get("abr") or 0) for f in audios if f.get("abr")})
    return json.dumps({
        "version": version(), "titulo": d.get("title") or "", "subidor": d.get("uploader") or d.get("channel") or "",
        "duracion": int(dur or 0), "miniatura": _miniatura(d), "extractor": d.get("extractor_key") or "",
        "en_vivo": bool(d.get("is_live")), "edad": int(d.get("age_limit") or 0), "id": d.get("id") or "",
        "url": d.get("webpage_url") or url,
        "alturas": [alturas[h] for h in sorted(alturas)],
        "audio": {"abr": abrs[-1] if abrs else 0, "tam": tam_audio, "abrs": abrs},
    })


def bajar(url, carpeta, modo, altura, abr, qjs, progreso=None):
    """Baja el video o el audio a la carpeta. Si el video viene en dos archivos (imagen y sonido por separado) devuelve los dos para
    que Kotlin los una con las herramientas de Android (sin ffmpeg)."""
    import yt_dlp
    os.makedirs(carpeta, exist_ok=True)
    estado = {"parte": ""}

    def gancho(h):
        if progreso is None:
            return
        try:
            tot = h.get("total_bytes") or h.get("total_bytes_estimate") or 0
            if h.get("status") == "downloading" and tot:
                progreso.avance(int(h.get("downloaded_bytes", 0) * 100 / tot), estado["parte"])
        except Exception:
            pass

    def uno(formato, prefijo):
        estado["parte"] = prefijo
        o = _opciones(qjs, {"format": formato, "outtmpl": os.path.join(carpeta, prefijo + "_%(id)s.%(ext)s"), "progress_hooks": [gancho],
                            "overwrites": True})
        with yt_dlp.YoutubeDL(o) as y:
            d = y.extract_info(url, download=True)
            if d.get("_type") == "playlist" and d.get("entries"):
                d = next((e for e in d["entries"] if e), d)
            ruta = y.prepare_filename(d)
            if d.get("requested_downloads"):
                ruta = d["requested_downloads"][0].get("filepath") or ruta
        return ruta

    salida = []
    if modo == "audio":
        tope = "[abr<=%d]" % int(abr) if abr else ""
        ruta = uno("ba[ext=m4a]%s/ba%s/ba/b" % (tope, tope), "a")
        salida.append({"ruta": ruta, "tipo": "audio"})
    else:
        h = int(altura) if altura else 720
        # a esa altura EXACTA con sonido incluido; si no existe (YouTube solo trae 360p así), se bajan imagen y sonido y Kotlin los une
        directo = "b[height=%d][ext=mp4][vcodec^=avc1][acodec!=none]/b[height=%d][ext=mp4][acodec!=none]" % (h, h)
        try:
            ruta = uno(directo, "v")
            salida.append({"ruta": ruta, "tipo": "video"})
        except yt_dlp.utils.DownloadError:
            rv = uno("bv[height=%d][ext=mp4][vcodec^=avc1]/bv[height<=%d][ext=mp4][vcodec^=avc1]" % (h, h), "v")
            ra = uno("ba[ext=m4a]/ba", "a")
            salida.append({"ruta": rv, "tipo": "imagen"})
            salida.append({"ruta": ra, "tipo": "sonido"})
    return json.dumps({"archivos": salida})
