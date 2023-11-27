package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path"
	"runtime"
	rd "runtime/debug"
	"strconv"
	"syscall"
	"time"

	c "github.com/Dmit1812/imgresizr/internal/config"
	"github.com/Dmit1812/imgresizr/internal/logger"
	"github.com/Dmit1812/imgresizr/internal/lrufilecache"
	internalhttp "github.com/Dmit1812/imgresizr/internal/server"
	"github.com/Dmit1812/imgresizr/internal/utilities"
	"github.com/h2non/bimg"
)

func main() {
	var err error

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, c.Usage, utilities.Version(),
			runtime.NumCPU(), c.EnvAddr, c.EnvPort, c.EnvFCacheSize, c.EnvMCacheSize, c.EnvLogLevel)
	}

	c.InitParams()

	if *c.PHelp || *c.PHelpLong {
		flag.Usage()
		os.Exit(1)
	}

	if len(flag.Arg(0)) > 0 {
		fmt.Fprintf(os.Stderr, "no extra args allowed, but got: %v\n", flag.Args())
		flag.Usage()
		os.Exit(1)
	}

	if *c.PVersion || *c.PVersionLong {
		fmt.Printf("imgresizr. bimg version: %s, vips version: %s\n", bimg.Version, bimg.VipsVersion)
		os.Exit(1)
	}

	loglevel := getEnvInt(c.EnvLogLevel, *c.PLogLevel)

	if *c.PLogLevel < int(logger.DEBUG) || *c.PLogLevel > int(logger.ERROR) {
		fmt.Fprintf(os.Stderr, "incorrect log level %d specified with option -loglevel or env variable '%s' "+
			"please use an integer from 1 to 4.\n", *c.PLogLevel, c.EnvLogLevel)
		flag.Usage()
		os.Exit(1)
	}

	log := logger.New(logger.LogLevel(loglevel))
	if log == nil {
		fmt.Fprintf(os.Stderr, "unable to create logger")
		os.Exit(1)
	}

	if c.OMemoryGCInterval > 0 {
		runGCToReleaseMemoryContinuously(c.OMemoryGCInterval, log)
	}

	{
		wd, err := os.Getwd()
		if err == nil {
			log.Info(fmt.Sprintf("Started in working directory: %s", wd))
		}
	}

	addr := getEnvStr(c.EnvAddr, *c.PAddr)
	port := getEnvInt(c.EnvPort, *c.PPort)
	fcachesize := getEnvInt(c.EnvFCacheSize, *c.PFCacheSize)
	mcachesize := getEnvInt(c.EnvMCacheSize, *c.PMCacheSize)

	opts := &internalhttp.Server{
		Address:          addr,
		Port:             port,
		FCacheSize:       fcachesize,
		MCacheSize:       mcachesize,
		CertFile:         c.OCertFile,
		KeyFile:          c.OKeyFile,
		HTTPReadTimeout:  c.OReadTimeout,
		HTTPWriteTimeout: c.OWriteTimeout,
		ShutdownTimeout:  c.OShutdownTimeout,
		CurrentVersions:  utilities.Version(),
		Log:              log,
		BaseImageCache: lrufilecache.NewLRUFileCache(fcachesize, mcachesize,
			*c.PCachePath, log),
		ConvertedImageCache: lrufilecache.NewLRUFileCache(fcachesize, mcachesize,
			path.Join(*c.PCachePath, c.OCacheConvertedDir), log),
	}

	opts.ErrorImage, _, err = utilities.LoadImage(*c.PErrorImage, c.OErrorImage, c.OPaths)
	if err != nil {
		log.Error(err.Error())
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create a channel to receive the signal notifications
	signalChan := make(chan os.Signal, 1)

	// Notify the channel for SIGINT (Ctrl+C) and SIGTERM signals
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		for {
			sig := <-signalChan
			log.Info(fmt.Sprintf("Received signal: %s", sig))
			cancel()
		}
	}()

	err = opts.Serve(ctx, true)
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
