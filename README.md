# DesktopDownloader

Downloader audio/video desktop untuk Windows — mesin **yt-dlp** (1750+ situs),
ffmpeg untuk merge/convert, thumbnail nempel otomatis sebagai cover.

Satu klik install, tanpa dependensi. File tersimpan ke folder pilihanmu.

## Fitur

- **Menu Audio / Video** — daftar format lengkap (opus/aac/h264/vp9/av1,
  kbps, resolusi, ukuran) + saran codec terbaik
- **Antrian** — download masuk antre otomatis, progress persen + MB + speed
- **Opsi per download** — embed thumbnail (cover), embed metadata
- **Cookies** — untuk video yang butuh login (umur/region/private)
- **Nama file rapi** — `Channel - Judul.opus/.m4a/.mp4`
- **Window native** — WebView2, bukan tab browser; tanpa terminal flash
- **Ringan** — ~25 MB RAM idle; ffmpeg/ffprobe tidur di disk sampai dipanggil
- **Local only** — server bind 127.0.0.1, tidak ada yang bisa akses dari luar

## Install

Jalankan `WebDownloader-Setup-1.0.0.exe` (dari Releases) → Next → Install.
Shortcut desktop otomatis. yt-dlp ter-embed (extract sendiri saat pertama jalan).

## Build dari source

```bash
# siapkan binary (unduh manual, taruh sesuai path):
#   embed/yt-dlp.exe        dari https://github.com/yt-dlp/yt-dlp/releases
#   ffmpeg.exe, ffprobe.exe dari https://www.gyan.dev/ffmpeg/builds/
go build -ldflags="-s -w -H windowsgui" -o WebDownloader.exe .
# opsional: upx --best --lzma WebDownloader.exe
# installer:
iscc installer.iss
```

Lihat `AGENTS.md` untuk konvensi & aturan kontribusi.

## Teknologi

Go 1.26 + go-webview2 (WebView2 runtime bawaan Windows) · yt-dlp · ffmpeg · Inno Setup
