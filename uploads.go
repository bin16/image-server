package main

import (
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jinzhu/gorm"
)

type handlerConfig struct {
	db      *gorm.DB
	tag     string
	resize  string
	extname string
	quality int
}

func loadHandlerConfigs() map[string]*handlerConfig {
	configs := make(map[string]*handlerConfig)
	configs["thumbnail"] = &handlerConfig{
		tag:     ".thumbnail",
		resize:  "256x256",
		extname: ".jpeg",
		quality: 60,
	}
	configs["mobile,webp"] = &handlerConfig{
		tag:     ".mobile",
		resize:  "480x480>",
		extname: ".webp",
		quality: 65,
	}
	configs["mobile"] = &handlerConfig{
		tag:     ".mobile",
		resize:  "480x480>",
		quality: 65,
	}
	configs["webp"] = &handlerConfig{
		resize:  "750x750>",
		extname: ".webp",
		quality: 75,
	}
	configs["jpeg"] = &handlerConfig{
		resize:  "750x750>",
		extname: ".jpeg",
		quality: 75,
	}
	configs["png"] = &handlerConfig{
		resize:  "750x750>",
		extname: ".png",
	}
	configs["original"] = &handlerConfig{
		tag: ".original",
	}

	return configs
}

func convertHander(db *gorm.DB, opts *handlerConfig) gin.HandlerFunc {
	// GET /s/:id/[dstExt]
	return func(c *gin.Context) {
		dstExt := opts.extname
		tag := opts.tag

		item := &upload{}
		db.Model(item).Where("uuid = ?", c.Param("id")).Find(&item)
		imgPath, _ := filepath.Abs(item.Target)
		imgExt := path.Ext(imgPath)
		if dstExt == "" {
			dstExt = imgExt
		}
		dstPath := strings.Replace(imgPath, imgExt, tag+dstExt, -1)
		if _, err := os.Stat(dstPath); err == nil {
			c.File(dstPath)
			return
		}

		convertPath, err := exec.LookPath("convert")
		if err != nil {
			log.Println("ERROR found path convert")
			log.Println(err)
		}

		// convert imgPath -resize xxx dstPath
		args := []string{
			imgPath,
		}
		if len(opts.resize) > 0 {
			args = append(args, "-resize")
			args = append(args, opts.resize)
		}
		args = append(args, dstPath)

		cmd := exec.Command(convertPath, args...)
		err = cmd.Run()
		if err != nil {
			log.Printf("ERROR convert %s to %s | %s\n", imgExt, tag+dstExt, imgPath)
			log.Println(err)
		}
		c.File(dstPath)
	}
}

func videoScreenshot(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		item := &upload{}
		db.Where("uuid = ?", c.Param("id")).Find(&item)

		videoExt := path.Ext(item.Target)
		screenshot := strings.Replace(item.Target, videoExt, ".jpeg", -1)
		if _, err := os.Stat(screenshot); err == nil {
			c.File(screenshot)
			return
		}

		ffmpeg, err := exec.LookPath("ffmpeg")
		if err != nil {
			log.Println("ERROR found path ffmpeg")
			log.Println(err)
		}
		args := []string{
			"-ss",
			"00:00:01",
			"-i",
			item.Target,
			"-vframes",
			"1",
			screenshot,
		}

		cmd := exec.Command(ffmpeg, args...)
		err = cmd.Run()
		if err != nil {
			log.Printf("ERROR take screenshot for video %s", item.Target)
			log.Println(err)
		}
		c.File(screenshot)
	}
}
