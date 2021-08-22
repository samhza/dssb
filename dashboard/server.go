package dashboard

import (
	"bytes"
	"embed"
	"errors"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"os/exec"
	"sync"

	"github.com/go-chi/chi"
)

type Dashboard struct {
	mux *chi.Mux

	log *LogBuffer

	mu sync.Mutex

	cmd     *exec.Cmd
	stdinw  io.Writer
	running bool
	err     error
}

//go:embed templates/* static/*
var tmplFS embed.FS

var tmpl *template.Template

func init() {
	var err error
	tmpl, err = template.ParseFS(tmplFS, "templates/*")
	if err != nil {
		panic(err)
	}
}

type BaseContext struct {
	Name string
}

func New() *Dashboard {
	s := new(Dashboard)

	s.log = &LogBuffer{buf: &bytes.Buffer{}}

	mux := chi.NewMux()
	s.mux = mux
	mux.Get("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/logs", http.StatusTemporaryRedirect)
	})
	mux.Get("/logs", s.getLogs)
	mux.Get("/status", s.getStatus)
	mux.Post("/exec", s.postExec)
	mux.Post("/start", s.postStart)
	mux.Handle("/static/*", http.FileServer(http.FS(tmplFS)))
	return s
}

func (s *Dashboard) postStart(w http.ResponseWriter, r *http.Request) {
	err := s.startServer()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Dashboard) postExec(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	defer r.Body.Close()
	_, err := fmt.Fprintln(s.stdinw, r.FormValue("command"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	http.Redirect(w, r, "/logs", http.StatusSeeOther)
}

func (s *Dashboard) StartServer() error {
	return s.startServer()
}

func (s *Dashboard) startServer() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running {
		return errors.New("server already running")
	}
	cmd := exec.Command("java", "-Xmx4G", "-Xms4G", "-jar", "minecraft_server.1.17.1.jar", "nogui")
	cmd.Dir = "./mc"
	w, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	s.cmd = cmd
	s.stdinw = w
	cmd.Stderr = new(bytes.Buffer)
	cmd.Stdout = s.log
	err = s.cmd.Start()
	if err == nil {
		s.running = true
	}
	go func() {
		err := cmd.Wait()
		s.mu.Lock()
		defer s.mu.Unlock()
		s.running = false
		var exerr *exec.ExitError
		if errors.As(err, &exerr) {
			exerr.Stderr = cmd.Stderr.(*bytes.Buffer).Bytes()
			s.err = exerr
		} else {
			s.err = err
		}
	}()
	return err
}

type LogsContext struct {
	BaseContext
	Logs string
}

func (s *Dashboard) getLogs(w http.ResponseWriter, r *http.Request) {
	s.log.Lock()
	ctx := LogsContext{BaseContext{"logs"}, s.log.buf.String()}
	defer s.log.Unlock()
	tmpl.ExecuteTemplate(w, "log.html", ctx)
}

type StatusContext struct {
	BaseContext
	Running bool
}

func (s *Dashboard) getStatus(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx := StatusContext{BaseContext{"status"}, s.running}
	tmpl.ExecuteTemplate(w, "status.html", ctx)
}

func (s *Dashboard) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

type LogBuffer struct {
	buf *bytes.Buffer
	sync.Mutex
}

func (l *LogBuffer) Write(p []byte) (int, error) {
	l.Lock()
	defer l.Unlock()
	return l.buf.Write(p)
}
