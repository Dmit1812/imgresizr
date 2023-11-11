package main

import (
	"flag"
	"fmt"
	"os"
	"path"
	"runtime"
	rd "runtime/debug"
	"strconv"
	"time"

	"github.com/Dmit1812/imgresizr/internal/logger"
	internalhttp "github.com/Dmit1812/imgresizr/internal/server"
	"github.com/h2non/bimg"
)

const (
	version = "0.0.1"
)

var (
	pAddr        = flag.String("a", "", "bind address")
	pPort        = flag.Int("p", 9000, "port to listen")
	pCacheSize   = flag.Int("c", 2, "how many images to keep in filesystem as a cache")
	pVersion     = flag.Bool("v", false, "Show version")
	pVersionLong = flag.Bool("version", false, "Show version")
	pHelp        = flag.Bool("h", false, "Show help")
	pHelpLong    = flag.Bool("help", false, "Show help")
	pErrorImage  = flag.String("errorimage", "", "Path to image to return as Error")
	pLogLevel    = flag.Int("loglevel", 1, "Set log level (1 - debug, 2 - info, 3 - warn, 4 - error)")

	oPaths = []string{"./", "./assets/", "../../assets/"}
)

const (
	oErrorImage       = "error.png"
	oCertFile         = ""      // TLS certificate file path
	oKeyFile          = ""      // TLS private key file path
	oReadTimeout      = int(30) // HTTP read timeout in seconds
	oWriteTimeout     = int(30) // HTTP write timeout in seconds
	oMemoryGCInterval = int(30) // Memory release inverval in seconds
)

const (
	envPort      = "IMGRESIZR_PORT"
	envAddr      = "IMGRESIZR_ADDR"
	envCacheSize = "IMGRESIZR_CASHESIZE"
	envLogLevel  = "IMGRESIZR_LOGLEVEL"

	usage = `imgresizr %s

Usage:
   imgresizr -p 80
 
Options:
   -a <addr>                             bind address [default: *]
   -p <port>                             bind port [default: 9000]
   -c <cache_size>                       how many images to keep in filesystem as a cache [default: 2]
   -h, -help                             output help
   -v, -version                          output version
   -errorimage <path_to_image>           image to use on error
   -loglevel <level>                     log level (1 - debug, 2 - info, 3 - warn, 4 - error) [default: warn]

Other:
   On this machine will use %d cores

Note:  
   Environment variables '%s', '%s', '%s', '%s' can be set prior 
   to execution to override whatever values were provided on command line

`
)

func main() {
	var err error

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, usage, Version(), runtime.NumCPU(), envAddr, envPort, envCacheSize, envLogLevel)
	}

	flag.Parse()

	if *pHelp || *pHelpLong {
		flag.Usage()
		os.Exit(1)
	}

	if len(flag.Arg(0)) > 0 {
		fmt.Fprintf(os.Stderr, "no extra args allowed, but got: %v\n", flag.Args())
		flag.Usage()
		os.Exit(1)
	}

	if *pVersion || *pVersionLong {
		fmt.Printf("imgresizr. bimg version: %s, vips version: %s\n", bimg.Version, bimg.VipsVersion)
		os.Exit(1)
	}

	loglevel := getEnvInt(envLogLevel, *pLogLevel)

	if *pLogLevel < int(logger.DEBUG) || *pLogLevel > int(logger.ERROR) {
		fmt.Fprintf(os.Stderr, "incorrect log level %d specified with option -loglevel or env variable '%s' "+
			"please use an integer from 1 to 4.\n", *pLogLevel, envLogLevel)
		flag.Usage()
		os.Exit(1)
	}

	log := logger.New(logger.LogLevel(loglevel))
	if log == nil {
		fmt.Fprintf(os.Stderr, "unable to create logger")
		os.Exit(1)
	}

	if oMemoryGCInterval > 0 {
		runGCToReleaseMemoryContinuously(oMemoryGCInterval, log)
	}

	{
		wd, err := os.Getwd()
		if err == nil {
			log.Info(fmt.Sprintf("Started in working directory: %s", wd))
		}
	}

	addr := getEnvStr(envAddr, *pAddr)
	port := getEnvInt(envPort, *pPort)
	cachesize := getEnvInt(envCacheSize, *pCacheSize)

	opts := &internalhttp.Server{
		Address:          addr,
		Port:             port,
		CacheSize:        cachesize,
		CertFile:         oCertFile,
		KeyFile:          oKeyFile,
		HTTPReadTimeout:  oReadTimeout,
		HTTPWriteTimeout: oWriteTimeout,
		CurrentVersions:  Version(),
		Log:              *log,
	}

	opts.ErrorImage, err = LoadImage(*pErrorImage, oPaths)
	if err != nil {
		log.Error(err.Error())
		os.Exit(1)
	}

	err = opts.Serve()
	if err != nil {
		log.Error(fmt.Sprintf("cannot start the server: %s\n", err.Error()))
	}
}

func LoadImage(f string, paths []string) ([]byte, error) {
	var err error
	var fc []byte

	// ensure we have at least one empty in path if user provided f
	if f != "" {
		paths = []string{""}
	} else {
		f = oErrorImage
	}

	for _, p := range paths {
		fn := path.Join(p, f)

		_, err = os.Stat(fn)
		if err != nil {
			continue
		}

		fc, err = os.ReadFile(fn)
		if err != nil {
			continue
		}
		return fc, nil
	}
	return nil, fmt.Errorf("file %s was not found in the paths %v", f, paths)
}

func Version() string {
	return version
}

func getEnvInt(envKey string, val int) int {
	if valEnv := os.Getenv(envKey); valEnv != "" {
		newVal, _ := strconv.Atoi(valEnv)
		if newVal > 0 {
			val = newVal
		}
	}
	return val
}

func getEnvStr(envKey string, val string) string {
	if valEnv := os.Getenv(envKey); valEnv != "" {
		newVal := valEnv
		if len(newVal) > 0 {
			val = newVal
		}
	}
	return val
}

func runGCToReleaseMemoryContinuously(interval int, log *logger.Logger) {
	ticker := time.NewTicker(time.Duration(interval) * time.Second)

	go func() {
		var mb, ma runtime.MemStats
		var st, et time.Time

		for range ticker.C {
			st = time.Now()
			runtime.ReadMemStats(&mb)
			rd.FreeOSMemory()
			et = time.Now()
			runtime.ReadMemStats(&ma)
			log.Info(fmt.Sprintf("FreeOSMemory() - Before: %d KB, After: %d KB. Took: %d ms",
				mb.Alloc/1024, ma.Alloc/1024, et.Sub(st).Milliseconds()))
		}
	}()
}
