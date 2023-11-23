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
	loglevel   = int(1) // 1 - debug, 2 - info ...
	cachepath  = "/tmp/cache"
	timeout    = 10
)

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
		return nil, res.StatusCode, &res.Header, fmt.Errorf("error downloading image: %w", err)
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

func TestImgresizrIntegration(t *testing.T) {
	var err error
	var wg sync.WaitGroup
	// TODO

	// Start server
	c.InitParams()

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
		defer wg.Done()
		err = opts.Serve(ctx, false)
		require.NoError(t, err, "server should finish without error during integration tests")
	}()

	// image loads faster with cache (cache operational)
	// cache can be cleaned
	// cache does create files
	t.Run("caching verification", func(t *testing.T) {
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
		require.Zerof(t, i, "cache should have 0 files but it had %d", i)

		// - а) загружаем изображение засекая время (загружается с сервера и преобразовывается)
		// http://localhost:9000/fill/10000/10000/http://localhost:8080/large.jpg
		// fill cache with one big item it processes over 6 seconds
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

		require.True(t, compareBytes(image1Bytes, image2Bytes), "images with and without cache should be the same, but they were not")

		// cache should contain exactly 1 item for base image and 1 item for converted image
		i, err = countCacheFiles(cachepath)
		require.NoError(t, err, "should be no error during counting base cache files, but there was")
		require.Equalf(t, 2, i, "base image cache should have 2 files but it had %d", i)

		i, err = countCacheFiles(path.Join(cachepath, c.OCacheConvertedDir))
		require.NoError(t, err, "should be no error during counting base cache files, but there was")
		require.Equalf(t, 2, i, "base image cache should have 2 files but it had %d", i)
	})

	// * удаленный сервер не существует;
	// - запрашиваем левый сервак
	// в хедерах есть Error его проверям если есть всё ок
	// http://localhost:9000/fill/10000/10001/http://localhost:8081/large.jpg

	// * удаленный сервер существует, но изображение не найдено (404 Not Found);
	// http://localhost:9000/fill/100/200/http://localhost:8080/lola

	// * удаленный сервер существует, но изображение не изображение, а скажем, exe-файл;
	// http://localhost:9000/fill/100/200/http://localhost:8080/imgresizr

	// * удаленный сервер вернул ошибку;
	// http://localhost:9000/fill/100/200/http://localhost:8080/error это ошибка 500

	// * удаленный сервер вернул изображение;
	// поскольку уже есть проверки на качество изображения убедится, что изображение соответствует требованиям запроса
	// http://localhost:9000/fill/300/500/http://localhost:8080/_gopher_original_1024x504.jpg

	// * изображение меньше, чем нужный размер;
	// запросить с маленького 50x50 и проверить, что увеличилось до нужного 2000x2000
	// если получилось 2000 то отлично!

	// we are done send to server request to shutdown
	cancel()

	// wait for server to close
	wg.Wait()
}
