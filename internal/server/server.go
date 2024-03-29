package internalhttp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Dmit1812/imgresizr/internal/lrufilecache"
	"github.com/h2non/bimg"
	"github.com/julienschmidt/httprouter"
)

type Server struct {
	Address             string
	Port                int
	FCacheSize          int
	MCacheSize          int
	CertFile            string
	KeyFile             string
	HTTPReadTimeout     int
	HTTPWriteTimeout    int
	ShutdownTimeout     int
	CurrentVersions     string
	Log                 Logger
	BaseImageCache      Cache
	ConvertedImageCache Cache
	ErrorImage          []byte
}

type Logger interface {
	Debug(msg string)
	Info(msg string)
	Warn(msg string)
	Error(msg string)
}

type Cache interface {
	Set(key string, ci lrufilecache.CacheItem) bool
	Get(key string) (lrufilecache.CacheItem, bool)
	Clear()
}

func (o *Server) Serve(ctx context.Context, restartOnError bool) error {
	var wg sync.WaitGroup
	var server *http.Server
	var err error
	var inshutdown int32

	addr := o.Address + ":" + strconv.Itoa(o.Port)
	handler := o.NewServerMux()

	// allow for cancel of the parent context
	_, cancel := context.WithCancel(ctx)
	defer cancel()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			if atomic.LoadInt32(&inshutdown) > 0 {
				o.Log.Debug("we are in shutdown, will no longer start server")
				// no parent context cancel is needed
				break
			}

			server = &http.Server{
				Addr:           addr,
				Handler:        handler,
				MaxHeaderBytes: 1 << 20,
				ReadTimeout:    time.Duration(o.HTTPReadTimeout) * time.Second,
				WriteTimeout:   time.Duration(o.HTTPWriteTimeout) * time.Second,
			}

			o.Log.Info(fmt.Sprintf("server listening on %s", addr))

			err = o.listenAndServe(server)
			// check for ErrServerClosed and stop the server loop
			// if errors.Is(err, http.ErrServerClosed) {
			// 	o.Log.Info("server shutdown was requested and server closed")
			// 	err = nil
			// 	// no need to cancel the parent context as it was already cancelled
			// 	break
			// }
			// server finished itself without any error - stop the server loop
			if err == nil {
				o.Log.Info("server successfully finished")
				// cancel the parent context
				cancel()
				break
			}
			//
			if !restartOnError {
				o.Log.Debug("server finished with error: " + err.Error() + ", will not restart as restartOnError is false")
				// cancel the parent context
				cancel()
				break
			}
			o.Log.Error(err.Error() + ", will restart")
		}
	}()

	// Wait for the shutdown signal
	<-ctx.Done()

	// Prevent startup of the new server as we are shutting down
	atomic.StoreInt32(&inshutdown, 1)

	// Shutdown the server and wait for it to finish
	shutdownctx, shutdowncancel := context.WithTimeout(context.Background(), time.Duration(o.ShutdownTimeout)*time.Second)
	defer shutdowncancel()

	server.Shutdown(shutdownctx)

	// Wait for the server go function to finish
	wg.Wait()

	if errors.Is(err, http.ErrServerClosed) {
		err = nil
	}
	return err
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

		opts := Options{Width: width, Height: height, Operation: ps.ByName("operation")}

		baseimagekey := ps.ByName("url")[1:]
		convertedimagekey := ps.ByName("width") + "x" + ps.ByName("height") + "-" + ps.ByName("url")[1:]
		o.Log.Info(fmt.Sprintf("will resize to %dx%d with operation %s image at %s",
			width, height, ps.ByName("operation"), baseimagekey))

		var imageResponseHeaders *http.Header
		var cifound, bifound bool

		ci, cifound := o.ConvertedImageCache.Get(convertedimagekey)
		image := ci.Content
		imageResponseHeaders = &ci.Headers

		if !cifound {
			ci, bifound = o.BaseImageCache.Get(baseimagekey)
			image = ci.Content
			imageResponseHeaders = &ci.Headers
		}

		if !cifound && !bifound {
			image, imageResponseHeaders, err = o.LoadImageFromNetwork(baseimagekey, &r.Header)
			if err != nil {
				o.Log.Error(err.Error())
				o.failed(w, opts, err.Error())
				return
			}

			if !imageOK(image) {
				err = fmt.Errorf("invalid image at URL: (url=%s)", baseimagekey)
				o.Log.Error(err.Error())
				o.failed(w, opts, err.Error())
				return
			}

			o.BaseImageCache.Set(baseimagekey, lrufilecache.CacheItem{
				Content: image,
				Headers: cleanHeaders(imageResponseHeaders),
			})
			o.Log.Debug("Loaded base image " + baseimagekey + " from server and saved it to cache")
		}

		if !cifound {
			image, err = Resize(image, opts)
			if err != nil {
				o.Log.Error(err.Error())
				o.failed(w, opts, err.Error())
				return
			}

			o.ConvertedImageCache.Set(convertedimagekey, lrufilecache.CacheItem{
				Content: image, Headers: cleanHeaders(imageResponseHeaders),
			})
			o.Log.Debug("Saved converted image " + convertedimagekey + " to cache")
		}

		writeHeaders(imageResponseHeaders, w)

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

func imageOK(image []byte) bool {
	b := bimg.NewImage(image)
	if b == nil {
		return false
	}
	_, err := b.Size()
	return err == nil
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
