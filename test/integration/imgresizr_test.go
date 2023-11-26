//go:build integration

package integration_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"sync"
	"testing"
	"time"

	c "github.com/Dmit1812/imgresizr/internal/config"
	"github.com/Dmit1812/imgresizr/internal/logger"
	"github.com/Dmit1812/imgresizr/internal/lrufilecache"
	internalhttp "github.com/Dmit1812/imgresizr/internal/server"
	"github.com/Dmit1812/imgresizr/internal/utilities"
	"github.com/h2non/bimg"
	"github.com/stretchr/testify/require"
)

const (
	addr       = "127.0.0.1"
	port       = int(9000)
	fcachesize = int(2)
	mcachesize = int(1)
	loglevel   = int(5) // 1 - debug, 2 - info ...
	cachepath  = "/tmp/cache"
	timeout    = 10
)

// request url and return image, status, headers, error.
func curl(url string, h *http.Header) ([]byte, int, *http.Header, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, 0, nil, fmt.Errorf("error creating a GET request: %w", err)
	}

	if h != nil {
		internalhttp.CopyHeaders(h, req)
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		if res != nil {
			return nil, res.StatusCode, &res.Header, fmt.Errorf("error downloading image: %w", err)
		}
		return nil, 0, &http.Header{}, fmt.Errorf("error downloading image: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return nil, res.StatusCode, &res.Header,
			fmt.Errorf("error downloading image: (status=%d) (url=%s)", res.StatusCode, req.URL.RequestURI())
	}

	buf, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, res.StatusCode, &res.Header,
			fmt.Errorf("unable to create image from response body: %w (url=%s)", err, req.URL.RequestURI())
	}
	return buf, res.StatusCode, &res.Header, nil
}

func compareBytes(b1 []byte, b2 []byte) bool {
	equal := true
	if len(b1) != len(b2) {
		return false
	}
	for i := 0; i < len(b1); i++ {
		if b1[i] != b2[i] {
			equal = false
			break
		}
	}
	return equal
}

func countCacheFiles(path string) (int, error) {
	files, err := os.ReadDir(path)
	if err != nil {
		return -1, err
	}
	n := 0
	for _, f := range files {
		if !f.IsDir() {
			// fmt.Printf("found file %s", f.Name())
			n++
		}
	}
	return n, nil
}

func forceCleanCache() {
	func() {
		for _, p := range []string{cachepath, path.Join(cachepath, c.OCacheConvertedDir)} {
			files, _ := os.ReadDir(p)
			for _, f := range files {
				if !f.IsDir() {
					os.Remove(path.Join(p, f.Name()))
				}
			}
		}
	}()
}

// func writeImage(name string, image []byte) {
// 	filename := path.Join("/tmp", name)
// 	err := os.WriteFile(filename, image, 0o600)
// 	if err != nil {
// 		_ = os.Remove(filename)
// 	}
// }

func TestImgresizrIntegration(t *testing.T) { //nolint:funlen
	var err error
	var wg sync.WaitGroup
	var tg sync.WaitGroup
	// TODO

	// Start server
	c.InitParams()

	// Force clean cache
	forceCleanCache()

	log := logger.New(logger.LogLevel(loglevel))
	if log == nil {
		fmt.Fprintf(os.Stderr, "unable to create logger")
		os.Exit(1)
	}

	opts := &internalhttp.Server{
		Address:          addr,
		Port:             port,
		FCacheSize:       fcachesize,
		MCacheSize:       mcachesize,
		CertFile:         c.OCertFile,
		KeyFile:          c.OKeyFile,
		HTTPReadTimeout:  timeout,
		HTTPWriteTimeout: timeout,
		CurrentVersions:  utilities.Version(),
		Log:              log,
		BaseImageCache: lrufilecache.NewLRUFileCache(fcachesize, mcachesize,
			cachepath, log),
		ConvertedImageCache: lrufilecache.NewLRUFileCache(fcachesize, mcachesize,
			path.Join(cachepath, c.OCacheConvertedDir), log),
	}

	opts.ErrorImage, _, err = utilities.LoadImage(*c.PErrorImage, c.OErrorImage, c.OPaths)
	if err != nil {
		log.Error(err.Error())
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	wg.Add(1)
	go func() {
		t := t
		defer wg.Done()
		err := opts.Serve(ctx, true)
		_ = t
		_ = err
		// require.NoError(t, err, "imgresizr should work without errors during integration tests")
	}()

	// image loads faster with cache (cache operational)
	// cache can be cleaned
	// cache does create files
	tg.Add(1)
	t.Run("cache abilities verification", func(t *testing.T) {
		defer tg.Done()
		// confirming cache has ability to clean itself
		// make any request it would populate cache
		_, _, _, err := curl("http://localhost:9000/fill/50/50/http://localhost:8080/gopher_50x50.jpg", nil)
		require.NoError(t, err)

		// force clean cache
		opts.BaseImageCache.Clear()
		opts.ConvertedImageCache.Clear()
		// give cache cleaning time as it's async
		time.Sleep(time.Second)

		// verify cache is deleted
		i, err := countCacheFiles(cachepath)
		require.NoError(t, err, "should be no error during counting cache files, but there was")
		require.Zerof(t, i, "cleaned cache should have 0 files but it had %d", i)

		// - а) загружаем изображение засекая время (загружается с сервера и преобразовывается)
		// http://localhost:9000/fill/10000/10000/http://localhost:8080/large.jpg
		// fill cache with one big image it processes in 4-5 seconds
		start := time.Now()
		image1Bytes, _, _, err := curl("http://localhost:9000/fill/10000/10000/http://localhost:8080/large.jpg", nil)
		elapsed1 := time.Since(start)
		require.NoError(t, err, "should be no error during downloading image without cache")
		intime := elapsed1 > time.Second
		require.Truef(t, intime, "download time should be over 1 second for item without cache, but it was %s", elapsed1)

		image1 := bimg.NewImage(image1Bytes)
		_, err = image1.Size()
		require.NoError(t, err, "downloaded image without cache should be a valid image with size, but it was not")

		start = time.Now()
		// request the same image as above from cache it should take no more then half the time to process
		// - б) загружаем то-же изображение засекая время (из кеша уже преобразованное)
		// http://localhost:9000/fill/10000/1000/http://localhost:8080/large.jpg
		// тест успешен если время б) < а)
		image2Bytes, _, _, err := curl("http://localhost:9000/fill/10000/10000/http://localhost:8080/large.jpg", nil)
		elapsed2 := time.Since(start)
		require.NoError(t, err, "should be no error during downloading image from cache")
		intime = (elapsed1 / 2) > elapsed2
		require.Truef(t, intime,
			"image download time from cache should be at least two times faster then without cache, but it was %s vs %s",
			elapsed2.String(), elapsed1.String())

		image2 := bimg.NewImage(image2Bytes)
		_, err = image2.Size()
		require.NoError(t, err, "the downloaded item should be a valid image with size, but it was not")

		require.True(t, compareBytes(image1Bytes, image2Bytes),
			"images with and without cache should be the same, but they were not")

		// cache should contain exactly 1 item (2 files) for base image
		// and 1 item (2 files) for converted image
		i, err = countCacheFiles(cachepath)
		require.NoError(t, err, "should be no error during counting base cache files, but there was")
		require.Equalf(t, 2, i, "base image cache should have 2 files but it had %d", i)

		i, err = countCacheFiles(path.Join(cachepath, c.OCacheConvertedDir))
		require.NoError(t, err, "should be no error during counting base cache files, but there was")
		require.Equalf(t, 2, i, "base image cache should have 2 files but it had %d", i)
	})

	// check our image server preserves response headers from image holder
	// nginx configured so it returns a
	// X-Special-Header: "Special Value"
	tg.Add(1)
	t.Run("should preserve response headers from image holder", func(t *testing.T) {
		defer tg.Done()
		// request previously not asked for image
		_, status, headers, err := curl("http://localhost:9000/fill/100/100/http://localhost:8080/gopher_50x50.jpg", nil)
		require.NoError(t, err)
		require.Equalf(t, 200, status, "source server should return 200 status code, but we got %d", status)
		require.Equalf(t, "Special Value", headers.Get("X-Special-Header"),
			"source server returns X-Special-Header: 'Special Value', but we got %v", headers)
	})

	// check that our image server presents client headers to the image holder in request
	// nginx configured so it responds with 50x50 image on a specific url /check-header
	// it returns 403 if no header X-Test-Header: "ATestHeaderFromClient" present in request
	// imgresizr presents this as 400 error and an image
	// it return 200 and 50x50 image if such header present in request
	// so if imgresizer passes this header we are good
	tg.Add(1)
	t.Run("should pass client headers to image holder", func(t *testing.T) {
		defer tg.Done()
		// force clean up file cache
		opts.BaseImageCache.Clear()
		opts.ConvertedImageCache.Clear()
		// skip wait time for deletion - erase ourselves
		forceCleanCache()

		// confirm test is configured correctly (expect 400)
		_, status, _, err := curl("http://localhost:9000/fill/51/51/http://localhost:8080/check-header", nil)
		require.Equalf(t, 400, status, "should get 400 after request without X-Test-Header")
		require.Error(t, err)

		// request with header set
		h := make(http.Header)
		h.Set("X-Test-Header", "ATestHeaderFromClient")
		imageBytes, status, _, err := curl("http://localhost:9000/fill/52/52/http://localhost:8080/check-header", &h)
		require.NoError(t, err)
		require.Equalf(t, 200, status, "source server should return 200 status code, but we got %d", status)

		// check for image validity should be 52x52
		image := bimg.NewImage(imageBytes)
		size, err := image.Size()
		require.NoError(t, err, "the downloaded item should be a valid image with size, but it was not")
		imagecorrect := (size.Width == 52 && size.Height == 52)
		require.Truef(t, imagecorrect, "image should be 52x52, but it was %dx%d", size.Width, size.Height)
	})

	// // * удаленный сервер не существует;
	// // - запрашиваем левый сервак
	// // в хедерах есть Error его проверям если есть всё ок
	// // http://localhost:9000/fill/10000/10001/http://localhost:8081/large.jpg
	tg.Add(1)
	t.Run("remote server doesn't exist", func(t *testing.T) {
		defer tg.Done()
		_, status, headers, _ := curl("http://localhost:9000/fill/50/50/http://zupa:8080/gopher_50x50.jpg", nil)
		require.Equalf(t, 400, status, "should get 400 after request to non-existing server, but got %d", status)
		require.Containsf(t, headers.Get("Error"),
			"dial tcp: lookup zupa: no such host", "should get correct 'Error' header after request")
	})

	// // * удаленный сервер существует, но изображение не найдено (404 Not Found);
	// // http://localhost:9000/fill/100/200/http://localhost:8080/lola
	tg.Add(1)
	t.Run("image on remote server doesn't exist", func(t *testing.T) {
		defer tg.Done()
		_, status, headers, _ := curl("http://localhost:9000/fill/50/50/http://localhost:8080/lola", nil)
		require.Equalf(t, 400, status, "should get 400 after request to non-existing server, but got %d", status)
		require.Containsf(t, headers.Get("Error"),
			"error downloading image: (status=404)", "should get correct 'Error' header after request")
	})

	// // * удаленный сервер существует, но изображение не изображение, а скажем, exe-файл;
	// // http://localhost:9000/fill/100/200/http://localhost:8080/imgresizr
	tg.Add(1)
	t.Run("image on remote server is not an image", func(t *testing.T) {
		defer tg.Done()
		_, status, headers, _ := curl("http://localhost:9000/fill/50/50/http://localhost:8080/imgresizr", nil)
		require.Equalf(t, 400, status, "should get 400 after request to non-existing server, but got %d", status)
		require.Containsf(t, headers.Get("Error"),
			"invalid image at URL", "should get correct 'Error' header after request")
	})

	// // * удаленный сервер вернул ошибку;
	// // http://localhost:9000/fill/100/200/http://localhost:8080/error это ошибка 500
	tg.Add(1)
	t.Run("remote image server returned 500", func(t *testing.T) {
		defer tg.Done()
		_, status, headers, _ := curl("http://localhost:9000/fill/100/200/http://localhost:8080/error", nil)
		require.Equalf(t, 400, status, "should get 400 after request to non-existing server, but got %d", status)
		require.Containsf(t, headers.Get("Error"),
			"error downloading image: (status=500)", "should get correct 'Error' header after request to non-existing server")
	})

	// * удаленный сервер вернул изображение;
	// поскольку уже есть проверки на качество изображения убедится, что изображение соответствует требованиям запроса
	// http://localhost:9000/fill/300/500/http://localhost:8080/_gopher_original_1024x504.jpg
	// этот тест реализован на проверке resize

	// * изображение меньше, чем нужный размер;
	// запросить с маленького 50x50 и проверить, что увеличилось до нужного 2000x2000
	// если получилось 2000 то отлично!
	// этот тест реализован на проверке resize

	// wait for all tests to finish
	tg.Wait()

	// we are done testing, send to server request to shutdown
	cancel()

	// wait for server to close
	wg.Wait()
}
