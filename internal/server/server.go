package internalhttp

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/h2non/bimg"
	"github.com/julienschmidt/httprouter"
)

type Server struct {
	Address          string
	Port             int
	CacheSize        int
	CertFile         string
	KeyFile          string
	HTTPReadTimeout  int
	HTTPWriteTimeout int
	CurrentVersions  string
	Log              Logger
	ErrorImage       []byte
}

type Logger interface {
	Debug(msg string)
	Info(msg string)
	Warn(msg string)
	Error(msg string)
}

func (o *Server) Serve() error {
	addr := o.Address + ":" + strconv.Itoa(o.Port)
	handler := o.NewServerMux()

	server := &http.Server{
		Addr:           addr,
		Handler:        handler,
		MaxHeaderBytes: 1 << 20,
		ReadTimeout:    time.Duration(o.HTTPReadTimeout) * time.Second,
		WriteTimeout:   time.Duration(o.HTTPWriteTimeout) * time.Second,
	}

	o.Log.Info(fmt.Sprintf("server listening on %s", addr))

	return o.listenAndServe(server)
}

func (o *Server) NewServerMux() http.Handler {
	mux := httprouter.New()
	mux.GET("/", o.indexRoute)
	mux.GET("/:operation/:width/:height/*url", o.resizeRoute())
	return mux
}

func (o *Server) listenAndServe(s *http.Server) error {
	if o.CertFile != "" && o.KeyFile != "" {
		return s.ListenAndServeTLS(o.CertFile, o.KeyFile)
	}
	return s.ListenAndServe()
}

func (o *Server) resizeRoute() func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		if r.Method != "GET" {
			o.Log.Error(fmt.Sprintf("unsupported method %s", r.Method))
			o.failedRequest(w, "the method is not allowed")
			return
		}

		width, height, err := parseDimensions(ps.ByName("width") + "x" + ps.ByName("height"))
		if err != nil {
			o.Log.Error("invalid width or height provided")
			o.failedRequest(w, "invalid width or height provided")
			return
		}

		o.Log.Info(fmt.Sprintf("will resize to %dx%d with operation %s", width, height, ps.ByName("operation")))
		opts := Options{Width: width, Height: height, Operation: ps.ByName("operation")}

		image, err := o.LoadImage(ps.ByName("url")[1:], &r.Header)
		if err != nil {
			o.Log.Error(err.Error())
			o.failed(w, opts, err.Error())
			return
		}

		image, err = Resize(image, opts)
		if err != nil {
			o.Log.Error(err.Error())
			o.failed(w, opts, err.Error())
			return
		}

		mime := GetImageMimeType(bimg.DetermineImageType(image))
		w.Header().Set("Content-Type", mime)
		w.Write(image)
	}
}

func (o *Server) indexRoute(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	body, _ := json.Marshal(o.CurrentVersions)
	w.Header().Set("Content-Type", "application/json")
	w.Write(body)
}

func parseDimensions(value string) (int, int, error) {
	var width, height int

	size := strings.Split(value, "x")
	width, err := strconv.Atoi(size[0])
	if err != nil {
		return 0, 0, err
	}

	if len(size) > 1 {
		height, err = strconv.Atoi(size[1])
	}

	return width, height, err
}

func (o *Server) failed(w http.ResponseWriter, opts Options, msg string) {
	opts.Force = true
	image, err := Resize(o.ErrorImage, opts)
	if err != nil {
		o.failedRequest(w, err.Error())
		return
	}

	w.Header().Set("Content-Type", GetImageMimeType(bimg.DetermineImageType(image)))
	w.Header().Set("Error", msg)
	w.WriteHeader(http.StatusBadRequest)
	w.Write(image)
}

func (o *Server) failedRequest(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Error", msg)
	w.WriteHeader(http.StatusBadRequest)
	w.Write(o.ErrorImage)
}