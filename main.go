package main

import (
	"bufio"
	"context"
	"embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	webview "github.com/jchv/go-webview2"
)

// hideWindow: subprocess (yt-dlp/ffmpeg) gak boleh nge-flash console.
func cmdNoWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x08000000} // CREATE_NO_WINDOW
}

//go:embed embed/yt-dlp.exe
var ytdlpBin embed.FS

//go:embed ui.html
var uiHTML []byte

//go:embed icon_64.png
var iconPNG []byte

const appVersion = "1.1.0"

var (
	appDir       string // %LOCALAPPDATA%\WebDownloader
	ytdlpPath    string
	ffmpegPath   string
	ffprobePath  string
	dlLock       sync.Mutex
	dlRunning    bool
	defaultDirMu sync.Mutex
)

type Format struct {
	ID    string `json:"id"`
	Kind  string `json:"kind"`
	Ext   string `json:"ext"`
	Q     string `json:"q"`
	Codec string `json:"codec"`
	Size  string `json:"size"`
}

func extractBins() {
	// yt-dlp — embedded, extract sekali
	ytdlpPath = filepath.Join(appDir, "yt-dlp.exe")
	if _, err := os.Stat(ytdlpPath); os.IsNotExist(err) {
		data, _ := ytdlpBin.ReadFile("embed/yt-dlp.exe")
		os.WriteFile(ytdlpPath, data, 0755)
	}
	// ffmpeg + ffprobe — dibawa installer, ada di folder app
	exe, _ := os.Executable()
	appFolder := filepath.Dir(exe)
	ffmpegPath = filepath.Join(appFolder, "ffmpeg.exe")
	ffprobePath = filepath.Join(appFolder, "ffprobe.exe")
	// fallback ke %LOCALAPPDATA% (mode dev / instalasi portable)
	if _, err := os.Stat(ffmpegPath); os.IsNotExist(err) {
		fdir := filepath.Join(appDir, "ffmpeg")
		os.MkdirAll(fdir, 0755)
		ffmpegPath = filepath.Join(fdir, "ffmpeg.exe")
		ffprobePath = filepath.Join(fdir, "ffprobe.exe")
	}
}

func configDir() string {
	dir := filepath.Join(appDir, "config")
	os.MkdirAll(dir, 0755)
	return dir
}

func loadDefaultDir() string {
	data, err := os.ReadFile(filepath.Join(configDir(), "default_dir.txt"))
	if err != nil {
		d, _ := os.UserHomeDir()
		return filepath.Join(d, "Downloads")
	}
	return strings.TrimSpace(string(data))
}

func saveDefaultDir(dir string) error {
	defaultDirMu.Lock()
	defer defaultDirMu.Unlock()
	return os.WriteFile(filepath.Join(configDir(), "default_dir.txt"), []byte(dir), 0644)
}

func jsonWrite(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func main() {
	// %LOCALAPPDATA%\WebDownloader
	la := os.Getenv("LOCALAPPDATA")
	if la == "" {
		la = os.TempDir()
	}
	appDir = filepath.Join(la, "WebDownloader")
	os.MkdirAll(appDir, 0755)
	logInit()
	extractBins()

	mux := http.NewServeMux()
	mux.HandleFunc("/", handleIndex)
	mux.HandleFunc("/api/formats", handleFormats)
	mux.HandleFunc("/api/download", handleDownload)
	mux.HandleFunc("/api/queue", handleQueue)
	mux.HandleFunc("/api/queue/remove", handleQueueRemove)
	mux.HandleFunc("/api/info", handleInfo)
	mux.HandleFunc("/api/browse", handleBrowse)
	mux.HandleFunc("/api/cookies", handleCookies)
	mux.HandleFunc("/api/config", handleConfig)

	ln, err := net.Listen("tcp", "127.0.0.1:8765")
	if err != nil {
		logf("port 8765 terpakai — app sudah jalan, exit")
		return
	}
	go http.Serve(ln, mux)
	logf("server listen ok")

	// Window WebView2 native
	logf("membuat webview...")
	w := webview.NewWithOptions(webview.WebViewOptions{
		Debug:     false,
		AutoFocus: true,
		WindowOptions: webview.WindowOptions{
			Title:  "DownloaderDesktop",
			Width:  980,
			Height: 720,
			Center: true,
		},
	})
	defer w.Destroy()
	logf("webview ok, navigate...")
	setWindowIcon(uintptr(w.Window()))
	w.Navigate("http://127.0.0.1:8765")
	logf("run loop mulai")
	w.Run()
	logf("run loop selesai")
}

func handleIndex(w http.ResponseWriter, r *http.Request) {
	// logo base64 kecil (64px png) untuk header sidebar
	iconB64 := base64.StdEncoding.EncodeToString(iconPNG)
	html := strings.ReplaceAll(string(uiHTML), "{{ICON}}", iconB64)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(html))
}

func handleInfo(w http.ResponseWriter, r *http.Request) {
	ver := "?"
	vcmd := exec.Command(ytdlpPath, "--version")
	cmdNoWindow(vcmd)
	out, err := vcmd.Output()
	if err == nil {
		ver = strings.TrimSpace(string(out))
	}
	_, ferr := os.Stat(ffmpegPath)
	jsonWrite(w, map[string]interface{}{
		"app_version":   appVersion,
		"ytdlp_version": ver,
		"ffmpeg":        ferr == nil,
	})
}

func handleBrowse(w http.ResponseWriter, r *http.Request) {
	dir := pickFolder("Pilih folder simpan")
	if dir == "" {
		jsonWrite(w, map[string]string{})
		return
	}
	jsonWrite(w, map[string]string{"dir": dir})
}

// cookiePath: file cookies.txt di %LOCALAPPDATA%\WebDownloader
func cookiePath() string {
	return filepath.Join(appDir, "cookies.txt")
}

func handleCookies(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		p := cookiePath()
		if _, err := os.Stat(p); err == nil {
			jsonWrite(w, map[string]interface{}{"has_cookies": true, "file": p})
		} else {
			jsonWrite(w, map[string]interface{}{"has_cookies": false})
		}
	case http.MethodDelete:
		os.Remove(cookiePath())
		jsonWrite(w, map[string]interface{}{"ok": true})
	case http.MethodPost:
		file := pickFile("Pilih file cookies.txt")
		if file == "" {
			jsonWrite(w, map[string]interface{}{"ok": false})
			return
		}
		// validasi: harus mirip netscape cookie file
		data, err := os.ReadFile(file)
		if err != nil || !strings.Contains(string(data), "# Netscape") && !strings.Contains(string(data), "youtube") {
			jsonWrite(w, map[string]interface{}{"ok": false, "error": "file bukan cookies.txt yang valid"})
			return
		}
		if err := os.WriteFile(cookiePath(), data, 0644); err != nil {
			jsonWrite(w, map[string]interface{}{"ok": false, "error": err.Error()})
			return
		}
		jsonWrite(w, map[string]interface{}{"ok": true})
	}
}

func handleConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		var req struct {
			DefaultDir string `json:"default_dir"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		if req.DefaultDir == "" {
			jsonWrite(w, map[string]interface{}{"ok": false, "error": "folder kosong"})
			return
		}
		if err := saveDefaultDir(req.DefaultDir); err != nil {
			jsonWrite(w, map[string]interface{}{"ok": false, "error": err.Error()})
			return
		}
		jsonWrite(w, map[string]interface{}{"ok": true})
		return
	}
	jsonWrite(w, map[string]string{"default_dir": loadDefaultDir()})
}

func runYtdlpJSON(ctx context.Context, url string) (map[string]interface{}, error) {
	args := []string{"--dump-single-json", "--no-warnings", "--no-playlist", "--socket-timeout", "30"}
	args = append(args, cookiesArgs()...)
	args = append(args, url)
	cmd := exec.CommandContext(ctx, ytdlpPath, args...)
	cmdNoWindow(cmd)
	cmd.Env = append(os.Environ(), "FFMPEG_PATH="+ffmpegPath)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("%s", firstLine(stderr.String()))
	}
	var info map[string]interface{}
	if err := json.Unmarshal(out, &info); err != nil {
		return nil, err
	}
	return info, nil
}

func firstLine(s string) string {
	if i := strings.Index(s, "\n"); i >= 0 {
		s = s[:i]
	}
	if len(s) > 300 {
		s = s[:300]
	}
	return s
}

func codecShort(c string) string {
	if c == "" || c == "none" {
		return "—"
	}
	base := strings.Split(c, ".")[0]
	m := map[string]string{"avc1": "h264", "mp4a": "aac", "av01": "av1", "ec3": "eac3", "ac-3": "ac3"}
	if v, ok := m[base]; ok {
		return v
	}
	return base
}

func sizeStr(f map[string]interface{}) string {
	for _, k := range []string{"filesize", "filesize_approx"} {
		if b, ok := f[k].(float64); ok && b > 0 {
			return fmt.Sprintf("%.1f MB", b/1048576)
		}
	}
	return "—"
}

func classify(f map[string]interface{}) string {
	vc, _ := f["vcodec"].(string)
	ac, _ := f["acodec"].(string)
	hasV := vc != "" && vc != "none"
	hasA := ac != "" && ac != "none"
	switch {
	case hasV && hasA:
		return "video"
	case hasV:
		return "video_only"
	case hasA:
		return "audio"
	}
	return ""
}

func handleFormats(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URL string `json:"url"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if req.URL == "" {
		jsonWrite(w, map[string]string{"error": "URL kosong"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 120*time.Second)
	defer cancel()
	info, err := runYtdlpJSON(ctx, req.URL)
	if err != nil {
		jsonWrite(w, map[string]string{"error": err.Error()})
		return
	}
	rawFmts, _ := info["formats"].([]interface{})
	var out []Format
	for _, rf := range rawFmts {
		f, ok := rf.(map[string]interface{})
		if !ok {
			continue
		}
		ext, _ := f["ext"].(string)
		fid, _ := f["format_id"].(string)
		if ext == "mhtml" || fid == "" || strings.HasPrefix(fid, "sb") {
			continue
		}
		if _, ok := f["url"].(string); !ok {
			continue
		}
		kind := classify(f)
		if kind == "" {
			continue
		}
		q := "—"
		if kind == "audio" {
			abr, _ := f["abr"].(float64)
			if abr == 0 {
				abr, _ = f["tbr"].(float64)
			}
			if abr > 0 {
				q = fmt.Sprintf("%.0f kbps", abr)
			}
		} else {
			h, _ := f["height"].(float64)
			if h > 0 {
				q = fmt.Sprintf("%dp", int(h))
			} else if res, _ := f["resolution"].(string); res != "" {
				q = res
			}
		}
		vc := codecShort(f["vcodec"].(string))
		ac := codecShort(f["acodec"].(string))
		codec := vc
		if kind == "video" {
			codec = vc + "+" + ac
		} else if vc == "—" {
			codec = ac
		}
		out = append(out, Format{ID: fid, Kind: kind, Ext: ext, Q: q, Codec: codec, Size: sizeStr(f)})
	}
	sort.SliceStable(out, func(i, j int) bool {
		rank := map[string]int{"video": 0, "video_only": 1, "audio": 2}
		return rank[out[i].Kind] < rank[out[j].Kind]
	})
	if len(out) == 0 {
		jsonWrite(w, map[string]string{"error": "tidak ada format yang bisa diunduh"})
		return
	}
	views := 0.0
	if v, ok := info["view_count"].(float64); ok {
		views = v
	}
	ud, _ := info["upload_date"].(string)
	if len(ud) == 8 {
		ud = ud[6:8] + "/" + ud[4:6] + "/" + ud[0:4]
	}
	dur := 0
	if d, ok := info["duration"].(float64); ok {
		dur = int(d)
	}
	thumb, _ := info["thumbnail"].(string)
	uploader, _ := info["uploader"].(string)
	title, _ := info["title"].(string)
	jsonWrite(w, map[string]interface{}{
		"title":       title,
		"duration":    fmt.Sprintf("%d:%02d", dur/60, dur%60),
		"thumbnail":   thumb,
		"uploader":    uploader,
		"views":       int(views),
		"upload_date": ud,
		"formats":     out,
	})
}

var safeNameRe = regexp.MustCompile(`[\\/:*?"<>|]+`)

type Job struct {
	ID       int64       `json:"id"`
	URL      string      `json:"url"`
	Format   string      `json:"format"`
	Title    string      `json:"title"`
	Status   string      `json:"status"` // queued|running|done|error
	Pos      int         `json:"pos"`
	Err      string      `json:"error,omitempty"`
	File     string      `json:"file,omitempty"`
	Percent  float64     `json:"percent"`
	Progress string      `json:"progress"` // "12.3 MB / 45.6 MB"
	Speed    string      `json:"speed"`
	Req      downloadReq `json:"-"`
}

var (
	queueMu  sync.Mutex
	jobs     []*Job
	qRunning bool
	jobSeq   int64
)

type downloadReq struct {
	URL      string `json:"url"`
	FormatID string `json:"format_id"`
	Dir      string `json:"dir"`
	Mode     string `json:"mode"`
	Thumb    bool   `json:"thumb"`
	Metadata bool   `json:"metadata"`
}

func handleDownload(w http.ResponseWriter, r *http.Request) {
	var req downloadReq
	json.NewDecoder(r.Body).Decode(&req)
	if req.URL == "" || req.FormatID == "" {
		jsonWrite(w, map[string]string{"error": "URL atau format kosong"})
		return
	}
	if req.Dir == "" {
		req.Dir = loadDefaultDir()
	}
	if st, err := os.Stat(req.Dir); err != nil || !st.IsDir() {
		jsonWrite(w, map[string]string{"error": "folder tujuan tidak ada: " + req.Dir})
		return
	}

	// Ambil judul dulu (cepat) untuk ditampilkan di antrian
	title := req.URL
	ctxT, cancelT := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancelT()
	if info, err := runYtdlpJSON(ctxT, req.URL); err == nil {
		if t, ok := info["title"].(string); ok && t != "" {
			title = t
		}
	}

	queueMu.Lock()
	jobSeq++
	j := &Job{ID: jobSeq, URL: req.URL, Format: req.FormatID, Title: title, Status: "queued", Req: req}
	jobs = append(jobs, j)
	queueMu.Unlock()
	go workerLoop()
	jsonWrite(w, map[string]interface{}{"ok": true, "id": j.ID, "queued": qRunningNow()})
}

func qRunningNow() bool {
	queueMu.Lock()
	defer queueMu.Unlock()
	return qRunning
}

func workerLoop() {
	queueMu.Lock()
	if qRunning {
		queueMu.Unlock()
		return
	}
	var next *Job
	for _, jj := range jobs {
		if jj.Status == "queued" {
			next = jj
			break // JANGAN hapus dari daftar — biar tetap terlihat di antrian
		}
	}
	if next == nil {
		qRunning = false
		queueMu.Unlock()
		return
	}
	qRunning = true
	next.Status = "running"
	queueMu.Unlock()

	doDownload(next)

	queueMu.Lock()
	// cek job queued berikutnya; kalau gak ada, lepas flag
	next2 := (*Job)(nil)
	for _, jj := range jobs {
		if jj.Status == "queued" {
			next2 = jj
			break // tetap di daftar
		}
	}
	if next2 == nil {
		qRunning = false
		queueMu.Unlock()
		return
	}
	next2.Status = "running"
	queueMu.Unlock()
	go func() {
		doDownload(next2)
		workerLoop()
	}()
}

func doDownload(j *Job) {
	req := &j.Req
	dlLock.Lock()
	if dlRunning {
		dlLock.Unlock()
		j.Status = "error"
		j.Err = "download lain sedang berjalan"
		return
	}
	dlRunning = true
	dlLock.Unlock()
	defer func() {
		dlLock.Lock()
		dlRunning = false
		dlLock.Unlock()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	info, err := runYtdlpJSON(ctx, req.URL)
	if err != nil {
		j.Status = "error"
		j.Err = err.Error()
		return
	}
	title, _ := info["title"].(string)
	uploader, _ := info["uploader"].(string)
	// Nama file gaya YTDLnis: "Channel - Judul.ext"
	safe := title
	if uploader != "" {
		safe = uploader + " - " + title
	}
	safe = safeNameRe.ReplaceAllString(safe, "_")
	if len(safe) > 120 {
		safe = safe[:120]
	}
	j.Title = title

	rawFmts, _ := info["formats"].([]interface{})
	var sel map[string]interface{}
	for _, rf := range rawFmts {
		f, _ := rf.(map[string]interface{})
		if f != nil && f["format_id"] == req.FormatID {
			sel = f
			break
		}
	}
	if sel == nil {
		j.Status = "error"
		j.Err = "format tidak ditemukan"
		return
	}
	kind := classify(sel)
	ext, _ := sel["ext"].(string)
	outBase := filepath.Join(req.Dir, safe)

	var dlArgs []string
	var finalPath string
	// ponytail: postprocess (thumbnail/metadata/merge) yt-dlp gagal kalau
	// nama output panjang + karakter aneh. Maka: download ke nama aman
	// sementara (job ID), lalu rename ke nama final setelah selesai.
	tmpBase := filepath.Join(req.Dir, fmt.Sprintf("dd_%d", j.ID))
	switch {
	case kind == "audio":
		// webm opus → extract ke container .opus murni (remux cepat, tanpa re-encode)
		if ext == "webm" {
			finalPath = outBase + ".opus"
			dlArgs = []string{"-f", req.FormatID, "-x", "--audio-format", "opus", "-o", tmpBase + ".%(ext)s", "--no-part", "--no-playlist"}
		} else {
			finalPath = outBase + "." + ext
			dlArgs = []string{"-f", req.FormatID, "-o", tmpBase + "." + ext, "--no-part", "--no-playlist"}
		}
	case kind == "video":
		finalPath = outBase + "." + ext
		dlArgs = []string{"-f", req.FormatID, "-o", tmpBase + "." + ext, "--no-part", "--no-playlist"}
	default: // video_only → merge dengan audio terbaik
		finalPath = outBase + ".mp4"
		dlArgs = []string{"-f", req.FormatID + "+bestaudio", "--merge-output-format", "mp4", "-o", tmpBase + ".%(ext)s", "--no-part", "--no-playlist"}
	}
	if req.Thumb {
		dlArgs = append(dlArgs, "--embed-thumbnail")
	}
	if req.Metadata {
		dlArgs = append(dlArgs, "--embed-metadata")
	}
	dlArgs = append(dlArgs, "--ffmpeg-location", ffmpegPath, "--newline", "--quiet", "--no-warnings")
	dlArgs = append(dlArgs, cookiesArgs()...)
	dlArgs = append(dlArgs, req.URL)

	cmd := exec.CommandContext(ctx, ytdlpPath, dlArgs...)
	cmdNoWindow(cmd)
	var stderr strings.Builder
	cmd.Stderr = &stderr

	// Progress: parse stdout yt-dlp (--newline) per baris
	pr, pw, _ := os.Pipe()
	cmd.Stdout = pw
	if err := cmd.Start(); err != nil {
		j.Status = "error"
		j.Err = err.Error()
		return
	}
	sc := bufio.NewScanner(pr)
	reProgress := regexp.MustCompile(`\[download\]\s+([\d.]+)%\s+of\s+~?\s*([\d.]+)(KiB|MiB|GiB)(?:.*?at\s+([\d.]+\s*\S+/s))?`)
	go func() {
		for sc.Scan() {
			line := sc.Text()
			if m := reProgress.FindStringSubmatch(line); m != nil {
				pct, _ := strconv.ParseFloat(m[1], 64)
				val, _ := strconv.ParseFloat(m[2], 64)
				mult := map[string]float64{"KiB": 1024, "MiB": 1048576, "GiB": 1073741824}[m[3]]
				total := val * mult
				done := total * pct / 100
				queueMu.Lock()
				j.Percent = pct
				j.Progress = fmt.Sprintf("%.1f MB / %.1f MB", done/1048576, total/1048576)
				if m[4] != "" {
					j.Speed = m[4]
				}
				queueMu.Unlock()
			}
		}
	}()
	if err := cmd.Wait(); err != nil {
		pw.Close()
		msg := firstLine(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		j.Status = "error"
		j.Err = "yt-dlp: " + msg
		return
	}
	pw.Close()
	// cari hasil download (tmpBase.*) lalu rename ke nama final
	matches, _ := filepath.Glob(tmpBase + ".*")
	var produced string
	for _, m := range matches {
		if strings.HasSuffix(m, ".part") || strings.HasSuffix(m, ".ytdl") {
			continue
		}
		produced = m
		break
	}
	if produced == "" {
		j.Status = "error"
		j.Err = "file hasil tidak ditemukan"
		return
	}
	if produced != finalPath {
		if err := os.Rename(produced, finalPath); err != nil {
			// kalau rename gagal (file tujuan ada), hapus lama dulu
			os.Remove(finalPath)
			if err2 := os.Rename(produced, finalPath); err2 != nil {
				j.Status = "error"
				j.Err = "gagal rename: " + err2.Error()
				return
			}
		}
	}
	queueMu.Lock()
	j.Percent = 100
	j.Speed = ""
	queueMu.Unlock()
	j.Status = "done"
	j.File = finalPath
}

// handleQueue: daftar semua job untuk halaman Antrian.
func handleQueue(w http.ResponseWriter, r *http.Request) {
	queueMu.Lock()
	defer queueMu.Unlock()
	out := make([]*Job, 0, len(jobs))
	running := 0
	for _, j := range jobs {
		if j.Status == "running" {
			running++
		}
	}
	// hitung posisi antre untuk yang queued
	pos := 0
	for _, j := range jobs {
		if j.Status == "queued" {
			pos++
			j.Pos = pos
		} else {
			j.Pos = 0
		}
	}
	_ = running
	for _, j := range jobs {
		cp := *j
		cp.Req = downloadReq{} // jangan bocorin struct internal
		out = append(out, &cp)
	}
	jsonWrite(w, map[string]interface{}{"jobs": out})
}

// handleQueueRemove: hapus job selesai/gagal dari daftar (cancel queued).
func handleQueueRemove(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID int64 `json:"id"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	queueMu.Lock()
	defer queueMu.Unlock()
	for i, j := range jobs {
		if j.ID == req.ID {
			if j.Status == "running" {
				jsonWrite(w, map[string]string{"error": "job sedang berjalan"})
				return
			}
			jobs = append(jobs[:i], jobs[i+1:]...)
			jsonWrite(w, map[string]interface{}{"ok": true})
			return
		}
	}
	jsonWrite(w, map[string]string{"error": "job tidak ditemukan"})
}
