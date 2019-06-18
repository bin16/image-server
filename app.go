package main

import (
	"crypto/sha256"
	"encoding/gob"
	"fmt"
	"image"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"

	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/sqlite"

	"github.com/avct/uasurfer"
	"github.com/gofrs/uuid"
)

const (
	uploadsDir = "./uploads"
)

func main() {
	// Database
	db, err := gorm.Open("sqlite3", "img.db")
	if err != nil {
		panic("failed to connect database")
	}
	defer db.Close()
	db.AutoMigrate(&upload{}, &user{})

	r := gin.Default()
	r.LoadHTMLGlob("views/*")

	// Sessions
	gob.Register(sessInfo{})
	store := cookie.NewStore([]byte("Hello?./22"))
	r.Use(sessions.Sessions("us", store))

	r.GET("/start", genUserStartGetHandler(db))
	r.POST("/start", genUserStartPostHandler(db))
	r.GET("/s/:id", func(c *gin.Context) {
		file := upload{}
		db.Where("uuid = ?", c.Param("id")).First(&file)
		c.File(file.Target)
	})
	r.GET("/s/:id/screenshot", videoScreenshot(db))
	r.GET("/s/:id/auto", func(c *gin.Context) {
		ua := uasurfer.Parse(c.Request.UserAgent())
		switch {
		// Bot
		case ua.IsBot():
			c.Redirect(http.StatusTemporaryRedirect, "/s/"+c.Param("id")+"/mobile,webp")
		// Chrome 32+
		case ua.Browser.Name == uasurfer.BrowserChrome && ua.Browser.Version.Major > 32:
			fallthrough
		// Firefox 65+
		case ua.Browser.Name == uasurfer.BrowserFirefox && ua.Browser.Version.Major > 65:
			fallthrough
		// Opera 19+
		case ua.Browser.Name == uasurfer.BrowserOpera && ua.Browser.Version.Major > 19:
			if ua.DeviceType == uasurfer.DevicePhone {
				// Mobile
				c.Redirect(http.StatusTemporaryRedirect, "/s/"+c.Param("id")+"/mobile,webp")
			} else {
				// Desktop
				c.Redirect(http.StatusTemporaryRedirect, "/s/"+c.Param("id")+"/webp")
			}
		// IE, Safari
		default:
			if ua.DeviceType == uasurfer.DevicePhone {
				// Mobile
				c.Redirect(http.StatusTemporaryRedirect, "/s/"+c.Param("id")+"/mobile,webp")
			} else {
				// Desktop
				c.Redirect(http.StatusTemporaryRedirect, "/s/"+c.Param("id")+"/webp")
			}
		}
	})
	handlerOptions := loadHandlerConfigs()
	for key, value := range handlerOptions {
		r.GET("/s/:id/"+key, convertHander(db, value))
	}
	apiRoot := "/dashboard"
	dashboard := r.Group(apiRoot, func(c *gin.Context) {
		sess := sessions.Default(c)
		info := sess.Get("info")
		log.Println(info)
		if info == nil {
			c.AbortWithStatus(http.StatusNotFound)
			return
		}

		c.Next()
	})
	dashboard.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.html", gin.H{"Root": apiRoot})
	})
	dashboard.GET("/uploads", func(c *gin.Context) {
		items := &[]upload{}
		db.Model(items).Order("created_at desc").Find(items)

		c.JSON(http.StatusOK, items)
	})
	dashboard.DELETE("/uploads/:id", func(c *gin.Context) {
		id, _ := strconv.Atoi(c.Param("id"))
		item := upload{
			ID: id,
		}
		db.Delete(&item)
		c.Status(http.StatusOK)
	})
	dashboard.POST("/uploads", func(c *gin.Context) {
		form, _ := c.MultipartForm()
		files := form.File["images"]
		var uploads []*upload

		for _, file := range files {
			log.Println("File found", file.Filename)

			// UUID
			id, _ := uuid.NewV4()
			basename := id.String()
			// URL
			url := path.Join("/s", basename)

			// SHA256
			f, err := file.Open()
			if err != nil {
				log.Println("ERROR open file", file.Filename)
			}
			defer f.Close()
			h := sha256.New()
			if _, err = io.Copy(h, f); err != nil {
				log.Println("ERROR sha256", file.Filename)
			}
			hs := fmt.Sprintf("%x", h.Sum(nil))

			// ...and Size
			uploadItem := &upload{
				UUID:     basename,
				URL:      url,
				Name:     file.Filename,
				SHA256:   hs,
				Size:     int(file.Size),
				MIMEType: file.Header.Get("Content-Type"),
			}

			item := &upload{}
			db.Where("sha256 = ?", hs).First(item)
			if len(item.Target) > 0 {
				uploadItem.Target = item.Target
				uploadItem.Width = item.Width
				uploadItem.Height = item.Height
				db.Create(uploadItem)
				uploads = append(uploads, uploadItem)
				continue
			}

			// Target
			filename := strings.ToLower(basename + ".original" + path.Ext(file.Filename))
			fullname := path.Join(uploadsDir, filename)
			err = c.SaveUploadedFile(file, fullname)
			if err != nil {
				log.Println("ERROR save file")
				continue
			}
			uploadItem.Target = fullname

			// Width, Height
			imgFile, _ := os.Open(fullname)
			img, _, err := image.DecodeConfig(imgFile)
			if err != nil {
				log.Println("ERROR decode image", file.Filename)
				log.Println(err)
			}
			uploadItem.Width = img.Width
			uploadItem.Height = img.Height

			db.Create(uploadItem)

			uploads = append(uploads, uploadItem)
		}

		c.JSON(http.StatusOK, uploads)
	})
	r.GET("/dashboard/login", genUserLoginGetHandler(db))
	r.POST("/dashboard/login", genUserLoginPostHandler(db))
	r.NoRoute(func(c *gin.Context) {
		abortWith404(c)
	})

	r.Run(":3032")
}

type upload struct {
	ID        int        `json:"id" gorm:"primary_key"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `json:"deleted_at"`
	UUID      string     `json:"uuid"`
	Name      string     `json:"name"`
	URL       string     `json:"url"`
	MIMEType  string     `json:"mime_type"`
	Size      int        `json:"size"`
	Width     int        `json:"width"`
	Height    int        `json:"height"`
	SHA256    string     `json:"sha256"`
	Target    string     `json:"target"`
}

func abortWith404(c *gin.Context) {
	c.HTML(http.StatusNotFound, "404.html", nil)
	c.Abort()
}
