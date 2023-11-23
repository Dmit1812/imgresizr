package config

import "flag"

var (
	PAddr        = flag.String("a", "", "bind address")
	PPort        = flag.Int("p", 9000, "port to listen")
	PFCacheSize  = flag.Int("cf", 2, "how many images to keep in filesystem as a cache")
	PMCacheSize  = flag.Int("cm", 1, "how many images to keep in memory as a cache")
	PCachePath   = flag.String("cp", "./cache", "where would the cache for images be located")
	PVersion     = flag.Bool("v", false, "Show version")
	PVersionLong = flag.Bool("version", false, "Show version")
	PHelp        = flag.Bool("h", false, "Show help")
	PHelpLong    = flag.Bool("help", false, "Show help")
	PErrorImage  = flag.String("errorimage", "", "Path to image to return as Error")
	PLogLevel    = flag.Int("loglevel", 1, "Set log level (1 - debug, 2 - info, 3 - warn, 4 - error)")

	OPaths = []string{"./", "./assets/", "../../assets/"}
)

const (
	OErrorImage        = "error.png"
	OCertFile          = ""      // TLS certificate file path
	OKeyFile           = ""      // TLS private key file path
	OReadTimeout       = int(30) // HTTP read timeout in seconds
	OWriteTimeout      = int(30) // HTTP write timeout in seconds
	OShutdownTimeout   = int(60) // Server shutdown timeout in seconds
	OMemoryGCInterval  = int(30) // Memory release inverval in seconds
	OCacheConvertedDir = "resized"
)

const (
	EnvPort       = "IMGRESIZR_PORT"
	EnvAddr       = "IMGRESIZR_ADDR"
	EnvFCacheSize = "IMGRESIZR_FCASHESIZE"
	EnvMCacheSize = "IMGRESIZR_MCASHESIZE"
	EnvLogLevel   = "IMGRESIZR_LOGLEVEL"

	Usage = `imgresizr %s

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

func InitParams() {
	flag.Parse()
}
