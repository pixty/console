package image

import (
	"bytes"
	"image"
	"image/png"
	"io/ioutil"
	"os"
	"testing"

	"github.com/pixty/console/service/storage"
)

func getTestImage() image.Image {
	file, err := os.Open("./image-service-test-image.png")
	if err != nil {
		panic(err)
	}

	img, err := png.Decode(file)
	if err != nil {
		panic(err)
	}

	return img
}

func readImage(is *ImageService, id string, w, h int) []byte {
	r, err := is.GetImageByFileName(id, 1200, 1200)
	if err != nil {
		panic(err)
	}
	bts, err := ioutil.ReadAll(r)
	if err != nil {
		panic(err)
	}
	return bts
}

func XTestEmptyFrame(t *testing.T) {
	bs := storage.NewLfsBlobStorage("./test-store", 10000000)
	defer bs.Shutdown()
	defer os.RemoveAll("./test-store")

	is := NewImageService()
	is.BlobStorage = bs

	res, err := is.StoreNewFrame(1, 1, getTestImage(), nil)
	if err != nil {
		t.Fatal("Oops cannot save image err=", err)
	}
	t.Log("Got store new frame result=", res)

	r1 := readImage(is, res[0], 1200, 1200)
	r2 := readImage(is, res[0], 200, 2200)

	if !bytes.Equal(r1, r2) {
		t.Fatal("Two different results were read")
	}
}

func TestMutiFrame(t *testing.T) {
	bs := storage.NewLfsBlobStorage("./test-store", 10000000)
	defer bs.Shutdown()
	//defer os.RemoveAll("./test-store")

	is := NewImageService()
	is.BlobStorage = bs

	res, err := is.StoreNewFrame(1, 123456, getTestImage(),
		[]image.Rectangle{image.Rect(0, 0, 1200, 100), image.Rect(100, 100, 500, 500), image.Rect(10, 10, 350, 300)})
	if err != nil {
		t.Fatal("Oops cannot save image err=", err)
	}
	t.Log("Got store new frame result=", res)
}
