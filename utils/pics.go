package utils

import (
	"crypto/sha1"
	"errors"
	"fmt"
	"github.com/slevchyk/taskeram/models"
	"image"
	"image/draw"
	"image/jpeg"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/nfnt/resize"
)

func CropCenteredSquare(src image.Image) *image.NRGBA {

	var minSideSize int

	//get image bounds
	srcBounds := src.Bounds()
	dX := srcBounds.Dx()
	dY := srcBounds.Dy()

	//let's find min imahe size
	if dX > dY {
		minSideSize = dY
	} else {
		minSideSize = dX
	}

	//calculate start crop point
	x := srcBounds.Min.X + (dX-minSideSize)/2
	y := srcBounds.Min.Y + (dY-minSideSize)/2

	//start point for croping
	sp := image.Pt(x, y)

	//make new rectangle
	r := image.Rect(0, 0, minSideSize, minSideSize)

	//create new image with bounds of r
	dst := image.NewNRGBA(r)

	//cropping
	draw.Draw(dst, r, src, sp, draw.Src)

	return dst
}

func UploadUserpic(mf multipart.File, fh *multipart.FileHeader, userPath string) (string, error) {

	var err error

	//let's find source image extension as a second element of strings slice
	imgExt := strings.Split(fh.Filename, ".")[1]
	imgExt = strings.ToLower(imgExt)
	if imgExt == "" {
		return "", errors.New("uploaded image without extension")
	}

	//making uploaded image name based on sha of source image
	h := sha1.New()
	_, err = io.Copy(h, mf)
	if err != nil {
		return "", err
	}

	imgHash := fmt.Sprintf("%x", h.Sum(nil))
	imgName := imgHash + "." + imgExt

	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	if CheckCreatePath(filepath.Join(wd, "public", "userpics", userPath)) != nil {
		return "", err
	}

	pathOrigin := filepath.Join(wd, "public", "userpics", userPath, imgHash+"-origin."+imgExt)
	path := filepath.Join(wd, "public", "userpics", userPath, imgName)

	newFileOrigin, err := os.Create(pathOrigin)
	if err != nil {
		return "", err
	}
	defer newFileOrigin.Close()

	_, err = mf.Seek(0, 0)
	if err != nil {
		return "", err
	}

	_, err = io.Copy(newFileOrigin, mf)
	if err != nil {
		return "", err
	}

	_, err = mf.Seek(0, 0)
	if err != nil {
		return "", err
	}

	jpegOrigin, err := jpeg.Decode(mf)
	if err != nil {
		return "", errors.New("jpegOrigin" + err.Error())
	}

	jpegCroped := CropCenteredSquare(jpegOrigin)
	jpegResized := resize.Resize(400, 400, jpegCroped, resize.Bicubic)

	newFile, err := os.Create(path)
	if err != nil {
		return "", err
	}
	defer newFile.Close()

	err = jpeg.Encode(newFile, jpegResized, nil)
	if err != nil {
		return "", err
	}

	return imgName, nil
}

func UpdateUserpic(mf multipart.File, fh *multipart.FileHeader, u models.DbUsers) (string, error) {

	if fh.Filename == "" {
		return "", nil
	}

	if u.Userpic != "" && u.Userpic != "default.png" {
		fmt.Println(u.Userpic)
		slImg := strings.Split(u.Userpic, ".")
		imgName := slImg[0]
		imgExt := slImg[1]

		wd, err := os.Getwd()
		if err != nil {
			return "", err
		}

		pathOrigin := filepath.Join(wd, "public", "userpics", imgName+"-origin."+imgExt)
		path := filepath.Join(wd, "public", "userpics", imgName+"."+imgExt)

		_ = os.Remove(path)
		_ = os.Remove(pathOrigin)
	}

	return UploadUserpic(mf, fh, strconv.Itoa(u.ID))
}

func CheckCreatePath(path string) error {

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return os.MkdirAll(path, os.ModePerm)
	}

	return nil
}
