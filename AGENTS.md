# AGENTS.md — DesktopDownloader

## Identitas Proyek
DesktopDownloader — aplikasi desktop Windows untuk mengunduh audio/video
(mesin: yt-dlp, ffmpeg). Go + WebView2. Installer: Inno Setup.

## Aturan Wajib (jangan dilanggar)

1. **Setiap perubahan = commit + push.** Selesai mengubah apa pun (kode, UI,
   installer, dokumen) → `git add` → `git commit` → `git push`. Jangan tumpuk
   perubahan. Pesan commit singkat, imperative, tanpa titik.
   Contoh: `add queue progress parsing` / `fix worker mutex deadlock`.
2. **Jangan pernah commit file berat/binary** kecuali eksplisit diminta:
   `embed/`, `*.exe`, `Output/` sudah di .gitignore.
3. **Jangan pernah commit `cookies.txt`** — itu sesi login user.
4. Build sebelum commit: `go build -ldflags="-s -w -H windowsgui" -o WebDownloader.exe .`
   Kalau build gagal, jangan push.
5. Tag versi per rilis: `v1.0.0` dst. Release = installer dari `Output/`.

## Struktur

- `main.go` — backend HTTP + logika download + antrian + progress parsing
- `pickfolder_windows.go` — dialog folder/file Win32 + cookiesArgs()
- `log.go` — logging ke %LOCALAPPDATA%/WebDownloader/app.log
- `ui.html` — UI lengkap (sidebar, antrian, pengaturan, cookies) — di-embed
- `installer.iss` — Inno Setup
- `embed/yt-dlp.exe` — BINARY (di-embed saat build, jangan di-commit)
- `ffmpeg.exe`, `ffprobe.exe` — BINARY (dibawa installer, jangan di-commit)

## Konvensi

- Bahasa UI: Indonesia. Kode/komentar/commit: singkat, bebas ID/EN.
- Port lokal: 127.0.0.1:8765 (local only — jangan 0.0.0.0).
- Semua subprocess yt-dlp/ffmpeg WAJIB cmdNoWindow() (CREATE_NO_WINDOW).
- Satu download per waktu (antrean di workerLoop). Jangan paralel-kan.
- %LOCALAPPDATA%\WebDownloader = data runtime (yt-dlp extract, cookies, log).
- Opus = codec saran untuk audio (highlight ⭐ di UI audio).

## Rencana Pengembangan

Pola embed-binary + WebView2 + Inno Setup = template untuk app desktop lain.
Bagian yang diganti antar aplikasi: ui.html (UI), handler API, binary di-embed.
Bagian yang dipakai ulang: window WebView2, tray (future), installer, update flow.
