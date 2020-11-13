package main

import (
	"encoding/hex"
	"fmt"
	"image"
	"image/jpeg"
	_ "image/jpeg"
	"image/png"
	"io"
	"io/ioutil"
	"os"
	"strconv"
	"time"

	"github.com/kataras/iris/v12"
	"github.com/nfnt/resize"
	"golang.org/x/crypto/blake2b"
)

var images []string

func main() {
	app := iris.New()
	app.Get("/", homePage)
	app.Put("/uploadImage", uploadFile)
	app.Get("/getImage/{img}", getImage)
	app.Get("/deleteImage/{img}", deleteImage)
	app.Run(iris.TLS("0.0.0.0:8081", "cert.pem", "key.pem"))
}

func homePage(ctx iris.Context) {
	ctx.WriteString("Hello World")
	fmt.Println("End Hitpoint: Home Page")
}

func uploadFile(ctx iris.Context) {
	fmt.Println("Hit Upload File")
	file, header, error := ctx.FormFile("myImage")
	if error != nil {
		fmt.Println("Error getting file")
		return
	}
	saveOriginal := ctx.PostValue("saveOriginal")
	width, _ := ctx.PostValues("width")
	height, _ := ctx.PostValues("height")
	scaleS := ctx.PostValue("scale")
	h := blake2b.Sum256([]byte(header.Filename + time.Now().String()))
	encodedHex := hex.EncodeToString(h[:])
	images = append(images, encodedHex)
	ctx.JSON(iris.Map{
		"imageName": encodedHex,
	})
	res := false
	if len(height) > 0 && len(width) > 0 {
		res = true
		err := os.Mkdir("uploads/"+encodedHex, 0755)
		if err != nil {
			fmt.Println(err)
			return
		}
	}
	doScale := false
	if len(scaleS) > 0 {
		doScale = true
		if !res {
			err := os.Mkdir("uploads/"+encodedHex, 0755)
			if err != nil {
				fmt.Println(err)
				return
			}
		}
	}
	go func() {
		defer file.Close()
		if _, err := os.Stat("uploads/"); os.IsNotExist(err) {
			err := os.Mkdir("uploads", 0755)
			if err != nil {
				fmt.Println("Error Making Directory")
			}
		}
		if res || doScale {
			if saveOriginal == "true" {
				dst, err := os.Create("uploads/" + encodedHex + "/" + "original")
				if err != nil {
					fmt.Println(err)
					return
				}
				if _, err := io.Copy(dst, file); err != nil {
					fmt.Println("Error Copying image")
					return
				}
			}
			file.Seek(0, 0)
			contentType := header.Header.Get("Content-Type")
			var im image.Image
			if contentType == "image/jpeg" {
				fmt.Println("jpeg")
				ima, err := jpeg.Decode(file)
				if err != nil {
					fmt.Println(err)
					fmt.Println("Error Converting file to image")
					return
				}
				im = ima
			} else if contentType == "image/png" {
				fmt.Println("png")
				ima, err := png.Decode(file)
				if err != nil {
					fmt.Println(err)
					fmt.Println("Error Converting file to image")
					return
				}
				im = ima
			} else {
				ctx.WriteString("Image Type not supported")
				return
			}
			if res {
				for index, a := range width {
					b := height[index]
					w64, _ := strconv.ParseUint(a, 10, 64)
					h64, _ := strconv.ParseUint(b, 10, 64)
					wi := uint(w64)
					hi := uint(h64)
					newImg := resize.Resize(wi, hi, im, resize.Lanczos3)
					dst, err := os.Create("uploads/" + encodedHex + "/resize" + a + "x" + b)
					if err != nil {
						fmt.Println("Error while saving resize")
					}
					jpeg.Encode(dst, newImg, nil)
					dst.Close()
				}
			}
			if doScale {
				dst, err := os.Create("uploads/" + encodedHex + "/scalex" + scaleS)
				if err != nil {
					fmt.Println(err)
					return
				}
				sca, _ := strconv.ParseUint(scaleS, 10, 64)
				scale := uint(sca)
				wi := uint(im.Bounds().Max.X) * scale
				hi := uint(im.Bounds().Max.Y) * scale
				newImg := resize.Resize(wi, hi, im, resize.Lanczos3)
				jpeg.Encode(dst, newImg, nil)
				dst.Close()
			}
		} else {
			dst, err := os.Create("uploads/" + encodedHex)
			if err != nil {
				fmt.Println("Error creating sub directory")
			}
			if _, err := io.Copy(dst, file); err != nil {
				fmt.Println("Error saving file")
				return
			}
			dst.Close()
		}
		for index, img := range images {
			if img == encodedHex {
				images = append(images[:index], images[index+1:]...)
			}
		}
	}()
}

func getImage(ctx iris.Context) {
	imgName := ctx.Params().Get("img")
	width := ctx.URLParamDefault("width", "nil")
	height := ctx.URLParamDefault("height", "nil")
	scale := ctx.URLParamDefault("scale", "nil")
	pending := false
	for _, img := range images {
		if img == imgName {
			pending = true
			break
		}
	}
	if pending == true {
		fireProblem(ctx)
	} else {
		resiz := true
		sc := true
		if width == "nil" && height == "nil" {
			resiz = false
		}
		if scale == "nil" {
			sc = false
		}
		src := "uploads/" + imgName
		if info, err := os.Stat(src); err == nil && info.IsDir() {
			files, err := ioutil.ReadDir(src)
			if err != nil {
				fmt.Println("Reading directory error")
			}
			if !resiz && !sc {
				f := files[0]
				ctx.SendFile(src+"/"+f.Name(), f.Name())
			} else {
				var rFileName string
				if resiz {
					rFileName = "resize" + width + "x" + height
				}
				if sc {
					rFileName = "scalex" + scale
				}
				flag := true
				for _, f := range files {
					if f.Name() == rFileName {
						flag = false
						ctx.SendFile(src+"/"+f.Name(), f.Name())
						break
					}
				}
				if flag {
					ctx.JSON(iris.Map{
						"Message": "No Image Found",
					})
				}
			}
		} else {
			ctx.SendFile(src, imgName)
		}
	}
	fmt.Println("Hit getImage")
}

func fireProblem(ctx iris.Context) {
	ctx.Problem(iris.NewProblem().Status(iris.StatusAccepted),
		iris.ProblemOptions{
			RetryAfter: 3,
		})
}

func deleteImage(ctx iris.Context) {
	fmt.Println("Hit delete Image")
	imgName := ctx.Params().Get("img")
	src := "uploads/" + imgName
	if info, err := os.Stat(src); err == nil && info.IsDir() {
		files, err := ioutil.ReadDir(src)
		if err != nil {
			fmt.Println("Reading directory error")
		}
		for _, f := range files {
			os.Remove(src + "/" + f.Name())
		}
	}
	e := os.Remove(src)
	if e != nil {
		fmt.Println(e)
		return
	}
	ctx.JSON(iris.Map{
		"Message": "Deleted",
	})
}
