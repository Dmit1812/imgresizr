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
	"github.com/Dmit1812/imgresizr/internal/lrufilecache"
	internalhttp "github.com/Dmit1812/imgresizr/internal/server"
	"github.com/Dmit1812/imgresizr/internal/utilities"
	"github.com/h2non/bimg"
)

var (
	pAddr        = flag.String("a", "", "bind address")
	pPort        = flag.Int("p", 9000, "port to listen")
	pFCacheSize  = flag.Int("cf", 2, "how many images to keep in filesystem as a cache")
	pMCacheSize  = flag.Int("cm", 1, "how many images to keep in memory as a cache")
	pCachePath   = flag.String("cp", "./cache", "where would the cache for images be located")
	pVersion     = flag.Bool("v", false, "Show version")
	pVersionLong = flag.Bool("version", false, "Show version")
	pHelp        = flag.Bool("h", false, "Show help")
	pHelpLong    = flag.Bool("help", false, "Show help")
	pErrorImage  = flag.String("errorimage", "", "Path to image to return as Error")
	pLogLevel    = flag.Int("loglevel", 1, "Set log level (1 - debug, 2 - info, 3 - warn, 4 - error)")

	oPaths = []string{"./", "./assets/", "../../assets/"}
)

const (
	oErrorImage        = "error.png"
	oCertFile          = ""      // TLS certificate file path
	oKeyFile           = ""      // TLS private key file path
	oReadTimeout       = int(30) // HTTP read timeout in seconds
	oWriteTimeout      = int(30) // HTTP write timeout in seconds
	oMemoryGCInterval  = int(30) // Memory release inverval in seconds
	oCacheConvertedDir = "resized"
)

const (
	envPort       = "IMGRESIZR_PORT"
	envAddr       = "IMGRESIZR_ADDR"
	envFCacheSize = "IMGRESIZR_FCASHESIZE"
	envMCacheSize = "IMGRESIZR_MCASHESIZE"
	envLogLevel   = "IMGRESIZR_LOGLEVEL"

	usage = `imgresizr %s

Usage:
   imgresizr -p 80
 
Options:
   -a <addr>                             bind address [default: *]
   -p <port>                             bind port [default: 9000]
   -cf <cache_size>                      how many images to keep in filesystem as a cache [default: 2]
   -cm <cache_size>                      how many images to keep in memory as a cache [default: 1]
   -cp <cache_path>                      where would the cache for images be located [default: ./cache]
   -h, -help                             output help
   -v, -version                          output version
   -errorimage <path_to_image>           image to use on error
   -loglevel <level>                     log level (1 - debug, 2 - info, 3 - warn, 4 - error) [default: warn]

Other:
   On this machine will use %d cores

Note:  
   Environment variables '%s', '%s', '%s', '%s', '%s' can be set prior 
   to execution to override whatever values were provided on command line
   
   To test in browser put:
   http://localhost:9000/
   then
   fill/100/200/
   then
   raw.githubusercontent.com/OtusGolang/final_project/master/examples/image-previewer/_gopher_original_1024x504.jpg

`
)

func main() {
	var err error

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, usage, utilities.Version(),
			runtime.NumCPU(), envAddr, envPort, envFCacheSize, envMCacheSize, envLogLevel)
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
	fcachesize := getEnvInt(envFCacheSize, *pFCacheSize)
	mcachesize := getEnvInt(envMCacheSize, *pMCacheSize)

	opts := &internalhttp.Server{
		Address:          addr,
		Port:             port,
		FCacheSize:       fcachesize,
		MCacheSize:       mcachesize,
		CertFile:         oCertFile,
		KeyFile:          oKeyFile,
		HTTPReadTimeout:  oReadTimeout,
		HTTPWriteTimeout: oWriteTimeout,
		CurrentVersions:  utilities.Version(),
		Log:              log,
		BaseImageCache: lrufilecache.NewLRUFileCache(fcachesize, mcachesize,
			*pCachePath, log),
		ConvertedImageCache: lrufilecache.NewLRUFileCache(fcachesize, mcachesize,
			path.Join(*pCachePath, oCacheConvertedDir), log),
	}

	opts.ErrorImage, _, err = utilities.LoadImage(*pErrorImage, oErrorImage, oPaths)
	if err != nil {
		log.Error(err.Error())
		os.Exit(1)
	}

	err = opts.Serve()
	if err != nil {
		log.Error(fmt.Sprintf("cannot start the server: %s\n", err.Error()))
	}
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
