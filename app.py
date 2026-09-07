#!/usr/bin/env python3
"""ytdl-web — yt-dlp downloader: sidebar Audio/Video/Pengaturan, pilih format,
stream langsung ke browser, zero disk, satu download per waktu."""
import os
import re
import threading

import requests
import yt_dlp
from flask import Flask, Response, jsonify, redirect, request, session, url_for

APP_PASSWORD = os.environ.get("APP_PASSWORD", "jhopan123")

app = Flask(__name__)
app.secret_key = os.environ.get("SECRET_KEY", "ubah-ini-di-render")
if os.environ.get("RENDER"):
    app.config["SESSION_COOKIE_SECURE"] = True

# Satu download global pada satu waktu.
download_lock = threading.Lock()


def logged_in():
    return session.get("ok") is True


def err_page(msg):
    return Response(
        f"<meta charset='utf-8'><body style='font-family:system-ui;background:#0f172a;"
        f"color:#e2e8f0;padding:40px'><h3>Gagal</h3><p>{msg}</p>"
        f"<a style='color:#3b82f6' href='/'>Kembali</a></body>",
        status=200,
    )


LOGIN_HTML = """<!doctype html><html lang="id"><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1"><title>ytdl-web</title>
<style>body{font-family:system-ui;background:#0f172a;color:#e2e8f0;display:flex;
justify-content:center;padding:60px 16px}.card{background:#1e293b;border-radius:12px;
padding:24px;max-width:380px;width:100%}input{width:100%;box-sizing:border-box;padding:10px;
border-radius:8px;border:1px solid #334155;background:#0f172a;color:#e2e8f0;margin:8px 0 16px}
button{width:100%;padding:12px;border:0;border-radius:8px;background:#3b82f6;color:#fff;
font-size:16px;cursor:pointer}</style></head><body><div class="card">
<h2>ytdl-web</h2>
<form method="post" action="/login">
<input type="password" name="password" placeholder="Password" autofocus required>
<button type="submit">Masuk</button>
</form></div></body></html>"""

MAIN_HTML = """<!doctype html><html lang="id"><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1"><title>ytdl-web</title>
<style>
:root{--bg:#0f172a;--bg2:#1e293b;--line:#334155;--txt:#e2e8f0;--mut:#94a3b8;--acc:#3b82f6}
*{box-sizing:border-box}body{font-family:system-ui;background:var(--bg);color:var(--txt);margin:0}
.sidebar{position:fixed;left:0;top:0;width:210px;height:100vh;background:var(--bg2);
display:flex;flex-direction:column;border-right:1px solid var(--line)}
.sidebar h2{margin:0;padding:22px 20px 18px;font-size:20px;border-bottom:1px solid var(--line)}
.sidebar nav{flex:1;padding:10px 0}
.nav-item{display:block;padding:13px 20px;color:var(--mut);text-decoration:none;
font-size:15px;cursor:pointer;border-left:3px solid transparent}
.nav-item.active{color:var(--txt);background:var(--bg);border-left-color:var(--acc)}
.nav-item:hover{color:var(--txt)}
.sidebar .foot{padding:14px 20px;border-top:1px solid var(--line)}
.sidebar .foot a{color:var(--mut);font-size:13px;text-decoration:none}
.sidebar .foot a:hover{color:var(--txt)}
.main{margin-left:210px;padding:36px 40px;max-width:900px}
.page{display:none}.page.active{display:block}
.card{background:var(--bg2);border-radius:12px;padding:24px;margin-bottom:20px}
h1{font-size:22px;margin:0 0 18px}
input[type=text],input[type=password]{width:100%;padding:10px;border-radius:8px;
border:1px solid var(--line);background:var(--bg);color:var(--txt);margin:8px 0 12px;font-size:15px}
label{font-size:13px;color:var(--mut)}
button{padding:12px 24px;border:0;border-radius:8px;background:var(--acc);color:#fff;
font-size:15px;cursor:pointer}
button:disabled{background:#475569;cursor:wait}
button.full{width:100%;margin-top:10px}
table{width:100%;border-collapse:collapse;margin:14px 0;font-size:14px}
th,td{padding:8px 6px;text-align:left;border-bottom:1px solid var(--line);white-space:nowrap}
th{color:var(--mut);font-size:12px;text-transform:uppercase}
.badge{display:inline-block;padding:2px 8px;border-radius:6px;font-size:11px}
.b-video{background:#065f46;color:#a7f3d0}.b-vo{background:#374151;color:#d1d5db}
#title-audio,#title-video{margin:14px 0 4px;font-size:15px}
.meta{display:none;gap:14px;align-items:flex-start;margin-top:16px}
.meta.show{display:flex}
.meta img{width:180px;border-radius:8px;background:var(--bg)}
.meta .m-info{font-size:13px;color:var(--mut);line-height:1.7}
.meta .m-title{font-size:15px;color:var(--txt);font-weight:600}
.status{margin:10px 0;color:var(--mut);font-size:14px}
p.hint{color:var(--mut);font-size:13px;line-height:1.5;margin-top:14px}
.result{display:none}
@media(max-width:768px){.sidebar{position:static;width:100%;height:auto;flex-direction:row;
align-items:center}.sidebar h2{border:0;padding:14px 16px;font-size:17px}
.sidebar nav{display:flex;padding:0}.nav-item{padding:10px 14px;border-left:0;
border-bottom:3px solid transparent}.nav-item.active{border-left:0;border-bottom-color:var(--acc)}
.sidebar .foot{border:0;margin-left:auto;padding:10px 16px}.main{margin:0;padding:20px 16px}}
</style></head><body>
<div class="sidebar">
<h2>ytdl-web</h2>
<nav>
<a class="nav-item active" data-page="audio">Audio</a>
<a class="nav-item" data-page="video">Video</a>
<a class="nav-item" data-page="settings">Pengaturan</a>
</nav>
<div class="foot"><a href="/logout">Keluar</a></div>
</div>
<div class="main">

<div id="page-audio" class="page active">
<h1>Unduh Audio</h1>
<div class="card">
<input type="text" id="url-audio" placeholder="https://youtube.com/watch?v=..." required>
<button id="btn-audio">Cek Format</button>
<div class="status" id="status-audio"></div>
<div class="result" id="result-audio">
<p id="title-audio"></p>
<form method="post" action="/download">
<input type="hidden" name="url" id="f_url-audio">
<input type="hidden" name="format_id" id="f_fmt-audio" value="">
<table><thead><tr><th></th><th>Format</th><th>Kualitas</th><th>Codec</th><th>Ukuran</th></tr></thead>
<tbody id="tbody-audio"></tbody></table>
<button type="submit" id="btndl-audio" class="full" disabled>Download</button>
</form>
</div>
<p class="hint">Format audio lengkap (m4a/aac, webm/opus, dll) langsung stream ke browser.
Tidak disimpan di server. Satu download pada satu waktu. URL playlist = item pertama.</p>
</div>
</div>

<div id="page-video" class="page">
<h1>Unduh Video</h1>
<div class="card">
<input type="text" id="url-video" placeholder="https://youtube.com/watch?v=..." required>
<button id="btn-video">Cek Format</button>
<div class="status" id="status-video"></div>
<div class="result" id="result-video">
<p id="title-video"></p>
<form method="post" action="/download">
<input type="hidden" name="url" id="f_url-video">
<input type="hidden" name="format_id" id="f_fmt-video" value="">
<table><thead><tr><th></th><th>Jenis</th><th>Format</th><th>Kualitas</th><th>Codec</th><th>Ukuran</th></tr></thead>
<tbody id="tbody-video"></tbody></table>
<button type="submit" id="btndl-video" class="full" disabled>Download</button>
</form>
</div>
<p class="hint">Video+Audio = siap putar. Video saja = tanpa suara (stream murni, tanpa merge
ffmpeg). Kualitas 1080p+ umumnya hanya tersedia sebagai video saja. Satu download pada satu waktu.</p>
</div>
</div>

<div id="page-settings" class="page">
<h1>Pengaturan</h1>
<div class="card">
<h3 style="margin-top:0">Ganti Password</h3>
<form onsubmit="return gantiPw(event)">
<label>Password lama</label>
<input type="password" id="pw-old" required>
<label>Password baru</label>
<input type="password" id="pw-new" minlength="4" required>
<button type="submit">Simpan</button>
</form>
<div class="status" id="status-pw"></div>
<p class="hint">Password tersimpan di memori server. Restart server = kembali ke nilai
env var APP_PASSWORD (di Render, set di dashboard).</p>
</div>
<div class="card">
<h3 style="margin-top:0">Info</h3>
<table id="info-table"><tbody></tbody></table>
</div>
</div>

</div>
<script>
const PAGES = {
  audio: {kinds: ['audio'], btn: 'btn-audio', url: 'url-audio', status: 'status-audio',
          result: 'result-audio', title: 'title-audio', tbody: 'tbody-audio',
          f_url: 'f_url-audio', f_fmt: 'f_fmt-audio', btndl: 'btndl-audio'},
  video: {kinds: ['video', 'video_only'], btn: 'btn-video', url: 'url-video', status: 'status-video',
          result: 'result-video', title: 'title-video', tbody: 'tbody-video',
          f_url: 'f_url-video', f_fmt: 'f_fmt-video', btndl: 'btndl-video'}
};
const state = {audio: {fmts: []}, video: {fmts: []}};

document.querySelectorAll('.nav-item').forEach(item => {
  item.addEventListener('click', () => {
    document.querySelectorAll('.nav-item').forEach(n => n.classList.remove('active'));
    item.classList.add('active');
    document.querySelectorAll('.page').forEach(p => p.classList.remove('active'));
    document.getElementById('page-' + item.dataset.page).classList.add('active');
    if (item.dataset.page === 'settings') muatInfo();
  });
});

async function cek(kind) {
  const cfg = PAGES[kind];
  const url = document.getElementById(cfg.url).value.trim();
  if (!url) return;
  const b = document.getElementById(cfg.btn);
  b.disabled = true; b.textContent = 'Memeriksa...';
  document.getElementById(cfg.status).textContent = 'Mengambil daftar format...';
  document.getElementById(cfg.result).style.display = 'none';
  try {
    const res = await fetch('/api/formats', {
      method: 'POST',
      headers: {'Content-Type': 'application/json'},
      body: JSON.stringify({url: url})
    });
    const data = await res.json();
    if (data.error) { document.getElementById(cfg.status).textContent = 'Gagal: ' + data.error; return; }
    state[kind].fmts = data.formats.filter(f => cfg.kinds.includes(f.kind));
    document.getElementById(cfg.status).textContent = '';
    const t = document.getElementById(cfg.title);
    t.innerHTML = '';
    const meta = document.createElement('div');
    meta.className = 'meta show';
    meta.innerHTML = `<img src="${data.thumbnail}" alt="">
      <div class="m-info"><div class="m-title"></div>
      <div>${data.uploader || ''}</div>
      <div>${data.views ? data.views.toLocaleString('id-ID') + ' tayangan' : ''}${data.upload_date ? ' • ' + data.upload_date : ''}</div>
      <div>Durasi ${data.duration}</div></div>`;
    meta.querySelector('.m-title').textContent = data.title;
    t.appendChild(meta);
    document.getElementById(cfg.f_url).value = url;
    renderTable(kind);
    document.getElementById(cfg.result).style.display = 'block';
  } catch (err) {
    document.getElementById(cfg.status).textContent = 'Error: ' + err;
  } finally {
    b.disabled = false; b.textContent = 'Cek Format';
  }
}
document.getElementById('btn-audio').onclick = () => cek('audio');
document.getElementById('btn-video').onclick = () => cek('video');

function renderTable(kind) {
  const cfg = PAGES[kind];
  const tb = document.getElementById(cfg.tbody);
  tb.innerHTML = '';
  const labels = {'video': ['Video+Audio', 'b-video'], 'video_only': ['Video saja', 'b-vo'], 'audio': ['Audio', 'b-audio']};
  state[kind].fmts.forEach((f, i) => {
    const tr = document.createElement('tr');
    const badge = kind === 'video' ? `<td><span class="badge ${labels[f.kind][1]}">${labels[f.kind][0]}</span></td>` : '';
    tr.innerHTML = `<td><input type="radio" name="fmt-${kind}" onchange="pick('${kind}',${i})"></td>
      ${badge}<td>${f.ext}</td><td>${f.q}</td><td>${f.codec}</td><td>${f.size}</td>`;
    tb.appendChild(tr);
  });
  document.getElementById(cfg.btndl).disabled = true;
  document.getElementById(cfg.f_fmt).value = '';
}
function pick(kind, i) {
  const cfg = PAGES[kind];
  document.getElementById(cfg.f_fmt).value = state[kind].fmts[i].id;
  document.getElementById(cfg.btndl).disabled = false;
}

async function gantiPw(e) {
  e.preventDefault();
  const res = await fetch('/api/password', {
    method: 'POST',
    headers: {'Content-Type': 'application/json'},
    body: JSON.stringify({
      old_password: document.getElementById('pw-old').value,
      new_password: document.getElementById('pw-new').value
    })
  });
  const data = await res.json();
  const s = document.getElementById('status-pw');
  s.textContent = data.ok ? 'Password diganti.' : 'Gagal: ' + data.error;
  if (data.ok) { document.getElementById('pw-old').value = ''; document.getElementById('pw-new').value = ''; }
  return false;
}

async function muatInfo() {
  const res = await fetch('/api/info');
  const d = await res.json();
  const tb = document.querySelector('#info-table tbody');
  tb.innerHTML = `<tr><td>Versi aplikasi</td><td>${d.app_version}</td></tr>
    <tr><td>Versi yt-dlp</td><td>${d.ytdlp_version}</td></tr>
    <tr><td>Python</td><td>${d.python_version}</td></tr>`;
}
</script></body></html>"""


@app.route("/", methods=["GET"])
def index():
    if not logged_in():
        return LOGIN_HTML
    return MAIN_HTML


@app.route("/login", methods=["POST"])
def login():
    global APP_PASSWORD
    if request.form.get("password", "") == APP_PASSWORD:
        session["ok"] = True
        return redirect(url_for("index"))
    return err_page("Password salah.")


@app.route("/logout", methods=["GET"])
def logout():
    session.clear()
    return redirect(url_for("index"))


def codec_short(c):
    if not c or c == "none":
        return "—"
    base = c.split(".")[0]
    m = {"avc1": "h264", "mp4a": "aac", "av01": "av1", "ec3": "eac3", "ac-3": "ac3"}
    return m.get(base, base)


def size_str(f):
    b = f.get("filesize") or f.get("filesize_approx")
    if not b:
        return "—"
    return f"{b / 1048576:.1f} MB"


def extract_all(url):
    """Extract info, fallback ke client android kalau default gagal."""
    opts = {
        "quiet": True,
        "no_warnings": True,
        "noplaylist": True,
        "skip_download": True,
        "socket_timeout": 30,
    }
    last_err = None
    for extra in ({}, {"extractor_args": {"youtube": {"player_client": ["android"]}}}):
        try:
            with yt_dlp.YoutubeDL({**opts, **extra}) as ydl:
                info = ydl.extract_info(url, download=False)
            if info:
                if "entries" in info:
                    entries = [e for e in info["entries"] if e]
                    info = entries[0] if entries else None
                if info:
                    return info
        except yt_dlp.utils.DownloadError as e:
            last_err = e
    raise last_err or yt_dlp.utils.DownloadError("gagal extract")


def classify(f):
    has_v = f.get("vcodec") not in (None, "none")
    has_a = f.get("acodec") not in (None, "none")
    if has_v and has_a:
        return "video"
    if has_v:
        return "video_only"
    if has_a:
        return "audio"
    return None


@app.route("/api/formats", methods=["POST"])
def api_formats():
    if not logged_in():
        return jsonify({"error": "belum login"}), 401
    url = (request.get_json(silent=True) or {}).get("url", "").strip()
    if not url:
        return jsonify({"error": "URL kosong"})
    try:
        info = extract_all(url)
    except Exception as e:
        return jsonify({"error": str(e)[:300]})
    out = []
    for f in info.get("formats", []):
        if f.get("ext") == "mhtml" or not f.get("url"):
            continue
        if str(f.get("format_id", "")).startswith("sb"):
            continue  # storyboard
        kind = classify(f)
        if not kind:
            continue
        if kind == "audio":
            abr = f.get("abr") or f.get("tbr") or 0
            q = f"{abr:.0f} kbps" if abr else "—"
        else:
            h = f.get("height")
            q = f"{h}p" if h else (f.get("resolution") or "—")
        vcodec = codec_short(f.get("vcodec"))
        acodec = codec_short(f.get("acodec"))
        codec = f"{vcodec}+{acodec}" if kind == "video" else (vcodec if vcodec != "—" else acodec)
        out.append(
            {
                "id": f.get("format_id"),
                "kind": kind,
                "ext": f.get("ext") or "?",
                "q": q,
                "codec": codec,
                "size": size_str(f),
            }
        )
    out.sort(key=lambda x: {"video": 0, "video_only": 1, "audio": 2}[x["kind"]])
    if not out:
        return jsonify({"error": "tidak ada format yang bisa di-stream"})
    dur = info.get("duration") or 0
    mins, secs = divmod(int(dur), 60)
    ud = info.get("upload_date") or ""
    if len(ud) == 8:
        ud = f"{ud[6:8]}/{ud[4:6]}/{ud[0:4]}"
    return jsonify(
        {
            "title": info.get("title") or "?",
            "duration": f"{mins}:{secs:02d}",
            "thumbnail": info.get("thumbnail") or "",
            "uploader": info.get("uploader") or info.get("channel") or "",
            "views": info.get("view_count") or 0,
            "upload_date": ud,
            "formats": out,
        }
    )


@app.route("/api/password", methods=["POST"])
def api_password():
    global APP_PASSWORD
    if not logged_in():
        return jsonify({"error": "belum login"}), 401
    d = request.get_json(silent=True) or {}
    old, new = d.get("old_password", ""), d.get("new_password", "")
    if old != APP_PASSWORD:
        return jsonify({"error": "password lama salah"})
    if len(new) < 4:
        return jsonify({"error": "password baru minimal 4 karakter"})
    APP_PASSWORD = new
    return jsonify({"ok": True})


@app.route("/api/info", methods=["GET"])
def api_info():
    import sys
    return jsonify(
        {
            "app_version": "2.0",
            "ytdlp_version": yt_dlp.version.__version__,
            "python_version": sys.version.split()[0],
        }
    )


@app.route("/download", methods=["POST"])
def download():
    if not logged_in():
        return redirect(url_for("index"))
    url = (request.form.get("url") or "").strip()
    format_id = request.form.get("format_id") or ""
    if not url or not format_id:
        return err_page("URL atau format kosong.")
    if not download_lock.acquire(blocking=False):
        return err_page("Download lain sedang berjalan. Tunggu selesai dulu.")
    try:
        info = extract_all(url)
        fmt = next(
            (f for f in info.get("formats", []) if str(f.get("format_id")) == format_id),
            None,
        )
        if not fmt or not fmt.get("url"):
            download_lock.release()
            return err_page("Format tidak ditemukan. Kembali dan cek format lagi.")
        media_url = fmt["url"]
        headers = fmt.get("http_headers") or info.get("http_headers") or {}
        upstream = requests.get(media_url, headers=headers, stream=True, timeout=(10, 60))
        if upstream.status_code != 200:
            # URL kadang 403 dari satu client — coba client satunya.
            upstream.close()
            alt = {"extractor_args": {"youtube": {"player_client": ["android"]}}}
            try:
                base = {
                    "quiet": True, "no_warnings": True, "noplaylist": True,
                    "skip_download": True, "socket_timeout": 30,
                }
                with yt_dlp.YoutubeDL({**base, **alt}) as ydl:
                    info2 = ydl.extract_info(url, download=False)
                if "entries" in info2:
                    entries = [e for e in info2["entries"] if e]
                    info2 = entries[0] if entries else info2
                fmt2 = next(
                    (f for f in info2.get("formats", []) if str(f.get("format_id")) == format_id),
                    None,
                )
                if fmt2 and fmt2.get("url"):
                    media_url = fmt2["url"]
                    headers = fmt2.get("http_headers") or info2.get("http_headers") or {}
                    upstream = requests.get(media_url, headers=headers, stream=True, timeout=(10, 60))
            except Exception:
                pass
            if upstream.status_code != 200:
                upstream.close()
                download_lock.release()
                return err_page(f"Gagal ambil media (HTTP {upstream.status_code}). Coba format lain.")
        title = re.sub(r'[\\/:*?"<>|]+', "_", info.get("title") or "download")[:100]
        kind = classify(fmt)
        suffix = " (video saja)" if kind == "video_only" else ""
        ext = fmt.get("ext") or "bin"
        filename = f"{title}{suffix}.{ext}"
        resp_headers = {
            "Content-Disposition": f'attachment; filename="{filename}"',
            "Content-Type": upstream.headers.get(
                "Content-Type", "application/octet-stream"
            ),
        }
        cl = upstream.headers.get("Content-Length")
        if cl:
            resp_headers["Content-Length"] = cl

        def gen():
            try:
                for chunk in upstream.iter_content(256 * 1024):
                    if chunk:
                        yield chunk
            finally:
                upstream.close()
                download_lock.release()

        return Response(gen(), headers=resp_headers)
    except yt_dlp.utils.DownloadError as e:
        download_lock.release()
        return err_page(f"yt-dlp gagal: {str(e)[:300]}")
    except Exception as e:
        download_lock.release()
        return err_page(f"Error: {str(e)[:300]}")


if __name__ == "__main__":
    app.run(host="0.0.0.0", port=int(os.environ.get("PORT", 5000)))
